/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	infragithub "github.com/saitamau-maximum/maxicloud/internal/infra/github"
)

const (
	RequeueInterval = 10 * time.Second
)

// DeployRunReconciler reconciles a DeployRun object
type DeployRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	DeployRepo domain.DeploymentHistoryRepository
	Reporter   domain.DeploymentReporter
}

// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deployruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deployruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deployruns/finalizers,verbs=update
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=buildruns,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=applications,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *DeployRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var run maxicloudv1alpha1.DeployRun
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling DeployRun", "phase", run.Status.Phase)

	switch run.Status.Phase {
	case "", maxicloudv1alpha1.DeployRunPhaseQueued:
		return r.handlePhaseQueued(ctx, &run)
	case maxicloudv1alpha1.DeployRunPhaseBuilding:
		return r.handlePhaseBuilding(ctx, &run)
	case maxicloudv1alpha1.DeployRunPhaseDeploying:
		return r.handlePhaseDeploying(ctx, &run)
	default:
		return ctrl.Result{}, nil
	}
}

func (r *DeployRunReconciler) handlePhaseQueued(ctx context.Context, run *maxicloudv1alpha1.DeployRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if err := r.notifyDeploymentStarted(ctx, run); err != nil {
		// GitHub CheckRunの作成失敗はデプロイ本体を止めない
		log.Error(err, "Could not create check run, continuing deployment")
	}
	if err := r.triggerBuild(ctx, run); err != nil {
		return ctrl.Result{}, err
	}

	now := metav1.Now()
	base := run.DeepCopy()
	run.Status.BuildRunRef = run.Name // DeployRun名とBuildRun名は同じにする
	run.Status.Phase = maxicloudv1alpha1.DeployRunPhaseBuilding
	if run.Status.StartedAt == nil {
		run.Status.StartedAt = &now
	}
	return ctrl.Result{RequeueAfter: RequeueInterval}, r.Status().Patch(ctx, run, client.MergeFrom(base))
}

func (r *DeployRunReconciler) notifyDeploymentStarted(ctx context.Context, run *maxicloudv1alpha1.DeployRun) error {
	if run.Status.CheckRunID != 0 {
		return nil
	}
	log := logf.FromContext(ctx)

	checkRunID, err := r.Reporter.CreateCommitStatus(ctx, domain.CreateCommitStatusParams{
		Owner: run.Spec.Owner,
		Repo:  run.Spec.Repo,
		CreateStatusOptions: domain.CreateStatusOptions{
			Name:    "MaxiCloud Deploy",
			HeadSHA: run.Spec.SHA,
			Status:  domain.CheckStatusInProgress,
			Title:   "Building",
			Summary: fmt.Sprintf("Building image for %s@%s", run.Spec.Repo, infragithub.ShortSHA(run.Spec.SHA)),
		},
	})
	if err != nil {
		log.Error(err, "failed to create check run")
		return err
	}

	base := run.DeepCopy()
	run.Status.CheckRunID = checkRunID
	return r.Status().Patch(ctx, run, client.MergeFrom(base))
}

func (r *DeployRunReconciler) triggerBuild(ctx context.Context, run *maxicloudv1alpha1.DeployRun) error {
	log := logf.FromContext(ctx)

	var existing maxicloudv1alpha1.BuildRun
	err := r.Get(ctx, types.NamespacedName{Name: run.Name, Namespace: run.Namespace}, &existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "failed to get BuildRun")
		return err
	}

	buildRun := newBuildRunForDeployRun(run)
	if err := ctrl.SetControllerReference(run, buildRun, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}
	if err := r.Create(ctx, buildRun); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		log.Error(err, "failed to create BuildRun")
		return err
	}
	return nil
}

func (r *DeployRunReconciler) handlePhaseBuilding(ctx context.Context, run *maxicloudv1alpha1.DeployRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var buildRun maxicloudv1alpha1.BuildRun
	if err := r.Get(ctx, types.NamespacedName{Name: run.Status.BuildRunRef, Namespace: run.Namespace}, &buildRun); err != nil {
		log.Error(err, "failed to get BuildRun", "name", run.Status.BuildRunRef)
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: RequeueInterval}, nil
		}
		return ctrl.Result{}, err
	}

	switch buildRun.Status.Phase {
	case maxicloudv1alpha1.BuildRunPhaseSucceeded:
		return r.handleBuildSucceeded(ctx, run, &buildRun)
	case maxicloudv1alpha1.BuildRunPhaseFailed, maxicloudv1alpha1.BuildRunPhaseCanceled:
		return r.handleBuildFailedOrCanceled(ctx, run)
	default:
		return ctrl.Result{RequeueAfter: RequeueInterval}, nil
	}
}

func (r *DeployRunReconciler) handleBuildSucceeded(ctx context.Context, run *maxicloudv1alpha1.DeployRun, buildRun *maxicloudv1alpha1.BuildRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	image := buildRun.Status.Image
	appKey := types.NamespacedName{Name: run.Spec.ApplicationName, Namespace: run.Namespace}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var app maxicloudv1alpha1.Application
		if err := r.Get(ctx, appKey, &app); err != nil {
			return err
		}
		app.Spec.Image = image
		return r.Update(ctx, &app)
	})
	if err != nil {
		log.Error(err, "failed to update Application image", "name", run.Spec.ApplicationName)
		return ctrl.Result{}, err
	}

	base := run.DeepCopy()
	run.Status.Image = image
	run.Status.Phase = maxicloudv1alpha1.DeployRunPhaseDeploying
	return ctrl.Result{}, r.Status().Patch(ctx, run, client.MergeFrom(base))
}

func (r *DeployRunReconciler) handleBuildFailedOrCanceled(ctx context.Context, run *maxicloudv1alpha1.DeployRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if run.Status.CheckRunID != 0 {
		if err := r.Reporter.UpdateCommitStatus(ctx, domain.UpdateCommitStatusParams{
			Owner:      run.Spec.Owner,
			Repo:       run.Spec.Repo,
			CheckRunID: run.Status.CheckRunID,
			UpdateStatusOptions: domain.UpdateCommitStatusOptions{
				Name:       "MaxiCloud Deploy",
				Status:     domain.CheckStatusCompleted,
				Conclusion: domain.CheckConclusionFailure,
				Title:      "Build failed",
				Summary:    fmt.Sprintf("Build failed for %s@%s", run.Spec.Repo, infragithub.ShortSHA(run.Spec.SHA)),
			},
		}); err != nil {
			// GitHub CheckRun更新失敗はパイプライン状態反映を止めない
			log.Error(err, "Could not update check run, marking run as failed")
		}
	}

	now := metav1.Now()
	base := run.DeepCopy()
	run.Status.FinishedAt = &now
	run.Status.Phase = maxicloudv1alpha1.DeployRunPhaseFailed
	return ctrl.Result{RequeueAfter: RequeueInterval}, r.Status().Patch(ctx, run, client.MergeFrom(base))
}

func (r *DeployRunReconciler) handlePhaseDeploying(ctx context.Context, run *maxicloudv1alpha1.DeployRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var app maxicloudv1alpha1.Application
	if err := r.Get(ctx, types.NamespacedName{Name: run.Spec.ApplicationName, Namespace: run.Namespace}, &app); err != nil {
		log.Error(err, "failed to get Application", "name", run.Spec.ApplicationName)
		return ctrl.Result{}, err
	}

	var appDomain string
	if app.Spec.Expose != nil {
		appDomain = app.Spec.Expose.Domain
	}

	if run.Spec.PRNumber != nil && appDomain != "" {
		if err := r.notifyDeploymentSummary(ctx, run, fmt.Sprintf("Preview URL: http://%s:8080", appDomain)); err != nil {
			// コメント作成/更新失敗はデプロイ成功判定を止めない
			log.Error(err, "Could not create or update preview comment")
		}
	}

	if run.Status.CheckRunID != 0 {
		if err := r.Reporter.UpdateCommitStatus(ctx, domain.UpdateCommitStatusParams{
			Owner:      run.Spec.Owner,
			Repo:       run.Spec.Repo,
			CheckRunID: run.Status.CheckRunID,
			UpdateStatusOptions: domain.UpdateCommitStatusOptions{
				Name:       "MaxiCloud Deploy",
				Status:     domain.CheckStatusCompleted,
				Conclusion: domain.CheckConclusionSuccess,
				Title:      "Deploy succeeded",
				Summary:    fmt.Sprintf("Successfully deployed %s@%s. Preview available at http://%s:8080", run.Spec.Repo, infragithub.ShortSHA(run.Spec.SHA), appDomain),
			},
		}); err != nil {
			// GitHub CheckRun更新失敗は成功ステータス反映を止めない
			log.Error(err, "Could not update check run, marking run as succeeded")
		}
	}

	now := metav1.Now()
	base := run.DeepCopy()
	run.Status.FinishedAt = &now
	run.Status.Phase = maxicloudv1alpha1.DeployRunPhaseSucceeded
	return ctrl.Result{}, r.Status().Patch(ctx, run, client.MergeFrom(base))
}

func (r *DeployRunReconciler) notifyDeploymentSummary(ctx context.Context, run *maxicloudv1alpha1.DeployRun, comment string) error {
	if run.Status.DeploymentSummaryCommentID != 0 {
		return r.Reporter.UpdateDeploymentSummary(ctx, domain.UpdateDeploymentSummaryParams{
			Owner:     run.Spec.Owner,
			Repo:      run.Spec.Repo,
			CommentID: run.Status.DeploymentSummaryCommentID,
			Comment:   comment,
		})
	}

	commentID, err := r.Reporter.CreateDeploymentSummary(ctx, domain.CreateDeploymentSummaryParams{
		Owner:    run.Spec.Owner,
		Repo:     run.Spec.Repo,
		PrNumber: *run.Spec.PRNumber,
		Comment:  comment,
	})
	if err != nil {
		return err
	}
	run.Status.DeploymentSummaryCommentID = commentID

	key := types.NamespacedName{Name: run.Name, Namespace: run.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest maxicloudv1alpha1.DeployRun
		if err := r.Get(ctx, key, &latest); err != nil {
			return err
		}
		base := latest.DeepCopy()
		latest.Status.DeploymentSummaryCommentID = commentID
		return r.Status().Patch(ctx, &latest, client.MergeFrom(base))
	}); err != nil {
		return err
	}
	return nil
}

func newBuildRunForDeployRun(run *maxicloudv1alpha1.DeployRun) *maxicloudv1alpha1.BuildRun {
	return &maxicloudv1alpha1.BuildRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.Name,
			Namespace: run.Namespace,
		},
		Spec: maxicloudv1alpha1.BuildRunSpec{
			Build: run.Spec.Build.DeepCopy(),
			Source: maxicloudv1alpha1.BuildSource{
				RepoURL: fmt.Sprintf("https://github.com/%s/%s", run.Spec.Owner, run.Spec.Repo),
				SHA:     run.Spec.SHA,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeployRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maxicloudv1alpha1.DeployRun{}).
		Owns(&maxicloudv1alpha1.BuildRun{}).
		Named("deployrun").
		Complete(r)
}
