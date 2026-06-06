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

// DeploymentPipelineReconciler reconciles a DeploymentPipeline object
type DeploymentPipelineReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	DeployRepo domain.DeploymentHistoryRepository
	Reporter   domain.DeploymentReporter
}

// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deploymentpipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deploymentpipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=deploymentpipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=buildruns,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=applications,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *DeploymentPipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var pipeline maxicloudv1alpha1.DeploymentPipeline
	if err := r.Get(ctx, req.NamespacedName, &pipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling DeploymentPipeline", "phase", pipeline.Status.Phase)

	switch pipeline.Status.Phase {
	case "", maxicloudv1alpha1.DeploymentPipelinePhaseQueued:
		return r.handlePhaseQueued(ctx, &pipeline)
	case maxicloudv1alpha1.DeploymentPipelinePhaseBuilding:
		return r.handlePhaseBuilding(ctx, &pipeline)
	case maxicloudv1alpha1.DeploymentPipelinePhaseDeploying:
		return r.handlePhaseDeploying(ctx, &pipeline)
	default:
		return ctrl.Result{}, nil
	}
}

func (r *DeploymentPipelineReconciler) handlePhaseQueued(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if err := r.notifyDeploymentStarted(ctx, pipeline); err != nil {
		// GitHub CheckRunの作成失敗はデプロイ本体を止めない
		log.Error(err, "Could not create check run, continuing deployment")
	}
	if err := r.triggerBuild(ctx, pipeline); err != nil {
		return ctrl.Result{}, err
	}

	now := metav1.Now()
	base := pipeline.DeepCopy()
	pipeline.Status.BuildRunRef = pipeline.Name // Pipeline名とBuildRun名は同じにする
	pipeline.Status.Phase = maxicloudv1alpha1.DeploymentPipelinePhaseBuilding
	if pipeline.Status.StartedAt == nil {
		pipeline.Status.StartedAt = &now
	}
	return ctrl.Result{RequeueAfter: RequeueInterval}, r.Status().Patch(ctx, pipeline, client.MergeFrom(base))
}

func (r *DeploymentPipelineReconciler) notifyDeploymentStarted(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) error {
	if pipeline.Status.CheckRunID != 0 {
		return nil
	}
	log := logf.FromContext(ctx)

	checkRunID, err := r.Reporter.CreateCommitStatus(ctx, domain.CreateCommitStatusParams{
		Owner: pipeline.Spec.Owner,
		Repo:  pipeline.Spec.Repo,
		CreateStatusOptions: domain.CreateStatusOptions{
			Name:    "MaxiCloud Deploy",
			HeadSHA: pipeline.Spec.SHA,
			Status:  domain.CheckStatusInProgress,
			Title:   "Building",
			Summary: fmt.Sprintf("Building image for %s@%s", pipeline.Spec.Repo, infragithub.ShortSHA(pipeline.Spec.SHA)),
		},
	})
	if err != nil {
		log.Error(err, "failed to create check run")
		return err
	}

	base := pipeline.DeepCopy()
	pipeline.Status.CheckRunID = checkRunID
	return r.Status().Patch(ctx, pipeline, client.MergeFrom(base))
}

func (r *DeploymentPipelineReconciler) triggerBuild(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) error {
	log := logf.FromContext(ctx)

	var existing maxicloudv1alpha1.BuildRun
	err := r.Get(ctx, types.NamespacedName{Name: pipeline.Name, Namespace: pipeline.Namespace}, &existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "failed to get BuildRun")
		return err
	}

	buildRun := newBuildRunForPipeline(pipeline)
	if err := ctrl.SetControllerReference(pipeline, buildRun, r.Scheme); err != nil {
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

func (r *DeploymentPipelineReconciler) handlePhaseBuilding(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var buildRun maxicloudv1alpha1.BuildRun
	if err := r.Get(ctx, types.NamespacedName{Name: pipeline.Status.BuildRunRef, Namespace: pipeline.Namespace}, &buildRun); err != nil {
		log.Error(err, "failed to get BuildRun", "name", pipeline.Status.BuildRunRef)
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: RequeueInterval}, nil
		}
		return ctrl.Result{}, err
	}

	switch buildRun.Status.Phase {
	case maxicloudv1alpha1.BuildRunPhaseSucceeded:
		return r.handleBuildSucceeded(ctx, pipeline, &buildRun)
	case maxicloudv1alpha1.BuildRunPhaseFailed, maxicloudv1alpha1.BuildRunPhaseCanceled:
		return r.handleBuildFailedOrCanceled(ctx, pipeline)
	default:
		return ctrl.Result{RequeueAfter: RequeueInterval}, nil
	}
}

func (r *DeploymentPipelineReconciler) handleBuildSucceeded(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline, buildRun *maxicloudv1alpha1.BuildRun) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	image := buildRun.Status.Image
	appKey := types.NamespacedName{Name: pipeline.Spec.ApplicationName, Namespace: pipeline.Namespace}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var app maxicloudv1alpha1.Application
		if err := r.Get(ctx, appKey, &app); err != nil {
			return err
		}
		app.Spec.Image = image
		return r.Update(ctx, &app)
	})
	if err != nil {
		log.Error(err, "failed to update Application image", "name", pipeline.Spec.ApplicationName)
		return ctrl.Result{}, err
	}

	base := pipeline.DeepCopy()
	pipeline.Status.Image = image
	pipeline.Status.Phase = maxicloudv1alpha1.DeploymentPipelinePhaseDeploying
	return ctrl.Result{}, r.Status().Patch(ctx, pipeline, client.MergeFrom(base))
}

func (r *DeploymentPipelineReconciler) handleBuildFailedOrCanceled(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if pipeline.Status.CheckRunID != 0 {
		if err := r.Reporter.UpdateCommitStatus(ctx, domain.UpdateCommitStatusParams{
			Owner:      pipeline.Spec.Owner,
			Repo:       pipeline.Spec.Repo,
			CheckRunID: pipeline.Status.CheckRunID,
			UpdateStatusOptions: domain.UpdateCommitStatusOptions{
				Name:       "MaxiCloud Deploy",
				Status:     domain.CheckStatusCompleted,
				Conclusion: domain.CheckConclusionFailure,
				Title:      "Build failed",
				Summary:    fmt.Sprintf("Build failed for %s@%s", pipeline.Spec.Repo, infragithub.ShortSHA(pipeline.Spec.SHA)),
			},
		}); err != nil {
			// GitHub CheckRun更新失敗はパイプライン状態反映を止めない
			log.Error(err, "Could not update check run, marking pipeline as failed")
		}
	}

	now := metav1.Now()
	base := pipeline.DeepCopy()
	pipeline.Status.FinishedAt = &now
	pipeline.Status.Phase = maxicloudv1alpha1.DeploymentPipelinePhaseFailed
	return ctrl.Result{RequeueAfter: RequeueInterval}, r.Status().Patch(ctx, pipeline, client.MergeFrom(base))
}

func (r *DeploymentPipelineReconciler) handlePhaseDeploying(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// HACK: 一旦普通にCRからDomainを読み取る
	var app maxicloudv1alpha1.Application
	if err := r.Get(ctx, types.NamespacedName{Name: pipeline.Spec.ApplicationName, Namespace: pipeline.Namespace}, &app); err != nil {
		log.Error(err, "failed to get Application", "name", pipeline.Spec.ApplicationName)
		return ctrl.Result{}, err
	}

	var appDomain string
	if app.Spec.Expose != nil {
		appDomain = app.Spec.Expose.Domain
	}

	if pipeline.Spec.PRNumber != nil && appDomain != "" {
		if err := r.notifyDeploymentSummary(ctx, pipeline, fmt.Sprintf("Preview URL: http://%s:8080", appDomain)); err != nil {
			// コメント作成/更新失敗はデプロイ成功判定を止めない
			log.Error(err, "Could not create or update preview comment")
		}
	}

	if pipeline.Status.CheckRunID != 0 {
		if err := r.Reporter.UpdateCommitStatus(ctx, domain.UpdateCommitStatusParams{
			Owner:      pipeline.Spec.Owner,
			Repo:       pipeline.Spec.Repo,
			CheckRunID: pipeline.Status.CheckRunID,
			UpdateStatusOptions: domain.UpdateCommitStatusOptions{
				Name:       "MaxiCloud Deploy",
				Status:     domain.CheckStatusCompleted,
				Conclusion: domain.CheckConclusionSuccess,
				Title:      "Deploy succeeded",
				Summary:    fmt.Sprintf("Successfully deployed %s@%s. Preview available at http://%s:8080", pipeline.Spec.Repo, infragithub.ShortSHA(pipeline.Spec.SHA), appDomain),
			},
		}); err != nil {
			// GitHub CheckRun更新失敗は成功ステータス反映を止めない
			log.Error(err, "Could not update check run, marking pipeline as succeeded")
		}
	}

	now := metav1.Now()
	base := pipeline.DeepCopy()
	pipeline.Status.FinishedAt = &now
	pipeline.Status.Phase = maxicloudv1alpha1.DeploymentPipelinePhaseSucceeded
	return ctrl.Result{}, r.Status().Patch(ctx, pipeline, client.MergeFrom(base))
}

func (r *DeploymentPipelineReconciler) notifyDeploymentSummary(ctx context.Context, pipeline *maxicloudv1alpha1.DeploymentPipeline, comment string) error {
	if pipeline.Status.DeploymentSummaryCommentID != 0 {
		return r.Reporter.UpdateDeploymentSummary(ctx, domain.UpdateDeploymentSummaryParams{
			Owner:     pipeline.Spec.Owner,
			Repo:      pipeline.Spec.Repo,
			CommentID: pipeline.Status.DeploymentSummaryCommentID,
			Comment:   comment,
		})
	}

	commentID, err := r.Reporter.CreateDeploymentSummary(ctx, domain.CreateDeploymentSummaryParams{
		Owner:    pipeline.Spec.Owner,
		Repo:     pipeline.Spec.Repo,
		PrNumber: *pipeline.Spec.PRNumber,
		Comment:  comment,
	})
	if err != nil {
		return err
	}
	pipeline.Status.DeploymentSummaryCommentID = commentID

	key := types.NamespacedName{Name: pipeline.Name, Namespace: pipeline.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest maxicloudv1alpha1.DeploymentPipeline
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

func newBuildRunForPipeline(pipeline *maxicloudv1alpha1.DeploymentPipeline) *maxicloudv1alpha1.BuildRun {
	return &maxicloudv1alpha1.BuildRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.Name,
			Namespace: pipeline.Namespace,
		},
		Spec: maxicloudv1alpha1.BuildRunSpec{
			Source: maxicloudv1alpha1.BuildSource{
				RepoURL: fmt.Sprintf("https://github.com/%s/%s", pipeline.Spec.Owner, pipeline.Spec.Repo),
				SHA:     pipeline.Spec.SHA,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maxicloudv1alpha1.DeploymentPipeline{}).
		Owns(&maxicloudv1alpha1.BuildRun{}).
		Named("deploymentpipeline").
		Complete(r)
}
