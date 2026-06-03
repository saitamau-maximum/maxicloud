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
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/infra/github"
	"github.com/saitamau-maximum/maxicloud/internal/infra/registry"
	batchv1 "k8s.io/api/batch/v1"
)

// ここでしか使わないのでcontroller側で定義しちゃう
type gitHubClient interface {
	GetInstallationAccessToken(ctx context.Context) (string, error)
}

type BuildRunReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Registry     registry.Registry
	GitHubClient gitHubClient
}

// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=buildruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=buildruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=buildruns/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *BuildRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var buildRun maxicloudv1alpha1.BuildRun
	if err := r.Get(ctx, req.NamespacedName, &buildRun); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileSecret(ctx, &buildRun); err != nil {
		log.Error(err, "failed to reconcile secret")
		return ctrl.Result{}, err
	}

	if err := r.reconcileJob(ctx, &buildRun); err != nil {
		log.Error(err, "failed to reconcile job")
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, &buildRun); err != nil {
		log.Error(err, "failed to reconcile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BuildRunReconciler) reconcileSecret(ctx context.Context, buildRun *maxicloudv1alpha1.BuildRun) error {
	log := logf.FromContext(ctx)
	secretName := buildRun.Name

	token, err := r.GitHubClient.GetInstallationAccessToken(ctx)
	if err != nil {
		log.Error(err, "failed to get installation access token")
		return err
	}

	key := types.NamespacedName{Name: secretName, Namespace: buildRun.Namespace}
	dockerConfig := []byte(r.Registry.DockerConfig())
	installationAccessToken := []byte(token)

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var secret corev1.Secret
		if err := r.Get(ctx, key, &secret); err != nil {
			if errors.IsNotFound(err) {
				if err := r.Create(ctx, newBuildRunSecret(buildRun, r.Registry.DockerConfig(), token)); err != nil {
					return ignoreAlreadyExists(err)
				}
				return nil
			}
			return err
		}

		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		unchanged := bytes.Equal(secret.Data[installationAccessTokenKey], installationAccessToken) &&
			bytes.Equal(secret.Data[corev1.DockerConfigJsonKey], dockerConfig)
		if unchanged {
			return nil
		}

		secret.Data[installationAccessTokenKey] = installationAccessToken
		secret.Data[corev1.DockerConfigJsonKey] = dockerConfig
		return r.Update(ctx, &secret)
	})
	if err != nil {
		log.Error(err, "failed to reconcile repo secret", "secret", secretName)
		return err
	}
	return nil
}

func (r *BuildRunReconciler) reconcileJob(ctx context.Context, buildRun *maxicloudv1alpha1.BuildRun) error {
	destination, owner, repo, err := r.buildDestination(buildRun)
	if err != nil {
		return err
	}

	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: buildRun.Name, Namespace: buildRun.Namespace}, &job); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, newBuildJob(BuildJobParams{
				buildRun:       buildRun,
				jobName:        buildRun.Name,
				sha:            buildRun.Spec.Source.SHA,
				repoSecretName: buildRun.Name,
				owner:          owner,
				repo:           repo,
				destination:    destination,
				buildOutput:    r.Registry.BuildOutput(destination),
			})); err != nil {
				return ignoreAlreadyExists(err)
			}
			return nil
		}
		return err
	}
	return nil
}

func ignoreAlreadyExists(err error) error {
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (r *BuildRunReconciler) reconcileStatus(ctx context.Context, buildRun *maxicloudv1alpha1.BuildRun) error {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: buildRun.Name, Namespace: buildRun.Namespace}, &job); err != nil {
		return client.IgnoreNotFound(err)
	}

	phase := buildRun.Status.Phase
	image := buildRun.Status.Image
	startedAt := buildRun.Status.StartedAt
	finishedAt := buildRun.Status.FinishedAt
	now := metav1.Now()

	if job.Status.Active > 0 {
		phase = maxicloudv1alpha1.BuildRunPhaseBuilding
		startedAt = resolveStartedAt(startedAt, job.Status.StartTime, now)
	}

	if job.Status.Succeeded > 0 {
		phase = maxicloudv1alpha1.BuildRunPhaseSucceeded
		startedAt = resolveStartedAt(startedAt, job.Status.StartTime, now)
		finishedAt = resolveFinishedAt(finishedAt, job.Status.CompletionTime, now)
		destination, _, _, err := r.buildDestination(buildRun)
		if err != nil {
			return err
		}
		image = destination
	}

	if job.Status.Failed > 0 && job.Status.Active == 0 {
		phase = maxicloudv1alpha1.BuildRunPhaseFailed
		startedAt = resolveStartedAt(startedAt, job.Status.StartTime, now)
		finishedAt = resolveFinishedAt(finishedAt, job.Status.CompletionTime, now)
	}

	if isBuildRunStatusUnchanged(buildRun, phase, image, startedAt, finishedAt) {
		return nil
	}

	base := buildRun.DeepCopy()
	buildRun.Status.Phase = phase
	buildRun.Status.Image = image
	buildRun.Status.StartedAt = startedAt
	buildRun.Status.FinishedAt = finishedAt
	return r.Status().Patch(ctx, buildRun, client.MergeFrom(base))
}

// currentがセットされてなければjobStartをセットして、なければfallbackを使用する
func resolveStartedAt(current *metav1.Time, jobStart *metav1.Time, fallback metav1.Time) *metav1.Time {
	if current != nil {
		return current
	}
	if jobStart != nil {
		t := *jobStart
		return &t
	}
	return &fallback
}

// currentがセットされてなければjobCompletionをセットして、なければfallbackを使用する
func resolveFinishedAt(current *metav1.Time, jobCompletion *metav1.Time, fallback metav1.Time) *metav1.Time {
	if current != nil {
		return current
	}
	if jobCompletion != nil {
		t := *jobCompletion
		return &t
	}
	return &fallback
}

func isBuildRunStatusUnchanged(
	buildRun *maxicloudv1alpha1.BuildRun,
	phase maxicloudv1alpha1.BuildRunPhase,
	image string,
	startedAt *metav1.Time,
	finishedAt *metav1.Time,
) bool {
	return phase == buildRun.Status.Phase &&
		image == buildRun.Status.Image &&
		((startedAt == nil && buildRun.Status.StartedAt == nil) || (startedAt != nil && buildRun.Status.StartedAt != nil && startedAt.Equal(buildRun.Status.StartedAt))) &&
		((finishedAt == nil && buildRun.Status.FinishedAt == nil) || (finishedAt != nil && buildRun.Status.FinishedAt != nil && finishedAt.Equal(buildRun.Status.FinishedAt)))
}

func (r *BuildRunReconciler) buildDestination(buildRun *maxicloudv1alpha1.BuildRun) (destination string, owner string, repo string, err error) {
	owner, repo, err = github.ParseRepoURL(buildRun.Spec.Source.RepoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse repository URL: %w", err)
	}
	destination = fmt.Sprintf("%s/%s:%s", r.Registry.Host(), repo, github.ShortSHA(buildRun.Spec.Source.SHA))
	return destination, owner, repo, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maxicloudv1alpha1.BuildRun{}).
		Named("buildrun").
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
