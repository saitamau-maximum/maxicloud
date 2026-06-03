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
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/infra/registry"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type ReconcilerConfig struct {
	IngressClass string
	BaseDomain   string
}

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Registry registry.Registry
	Config   ReconcilerConfig
}

// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maxicloud.maximum.vc,resources=applications/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var application maxicloudv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileSecret(ctx, &application); err != nil {
		log.Error(err, "Failed to reconcile Secret")
		if err = r.updateStatus(ctx, &application, false); err != nil {
			log.Error(err, "Failed to update Application status")
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcileDeployment(ctx, &application); err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		if err = r.updateStatus(ctx, &application, false); err != nil {
			log.Error(err, "Failed to update Application status")
		}
		return ctrl.Result{}, err
	}

	if application.Spec.Expose == nil {
		if err := r.updateStatus(ctx, &application, true); err != nil {
			log.Error(err, "Failed to update Application status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileService(ctx, &application); err != nil {
		log.Error(err, "Failed to reconcile Service")
		if err = r.updateStatus(ctx, &application, false); err != nil {
			log.Error(err, "Failed to update Application status")
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcileIngress(ctx, &application); err != nil {
		log.Error(err, "Failed to reconcile Ingress")
		if err = r.updateStatus(ctx, &application, false); err != nil {
			log.Error(err, "Failed to update Application status")
		}
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(ctx, &application, true); err != nil {
		log.Error(err, "Failed to update Application status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) reconcileSecret(ctx context.Context, application *maxicloudv1alpha1.Application) error {
	log := logf.FromContext(ctx)

	secretName := appRegistrySecretName(application.Name)
	key := types.NamespacedName{Name: secretName, Namespace: application.Namespace}
	dockerConfig := []byte(r.Registry.DockerConfig())

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var secret corev1.Secret
		if err := r.Get(ctx, key, &secret); err != nil {
			if errors.IsNotFound(err) {
				if err := r.Create(ctx, newAppRegistrySecret(application, r.Registry.DockerConfig())); err != nil {
					return ignoreAlreadyExists(err)
				}
				return nil
			}
			return err
		}

		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		unchanged := bytes.Equal(secret.Data[corev1.DockerConfigJsonKey], dockerConfig) &&
			secret.Type == corev1.SecretTypeDockerConfigJson
		if unchanged {
			return nil
		}

		secret.Type = corev1.SecretTypeDockerConfigJson
		secret.Data[corev1.DockerConfigJsonKey] = dockerConfig
		return r.Update(ctx, &secret)
	})
	if err != nil {
		log.Error(err, "failed to reconcile registry secret", "secret", secretName)
		return err
	}
	return nil
}

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, application *maxicloudv1alpha1.Application) error {
	if strings.TrimSpace(application.Spec.Image) == "" {
		return nil
	}

	var deploy appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: application.Name, Namespace: application.Namespace}, &deploy)
	if errors.IsNotFound(err) {
		return r.Create(ctx, newDeployment(application, application.Spec.Image))
	}
	if err != nil {
		return err
	}

	deploy.Spec.Template.Spec.Containers[0].Image = application.Spec.Image
	deploy.Spec.Template.Spec.Containers[0].Env = application.Spec.Env
	return r.Update(ctx, &deploy)
}

func (r *ApplicationReconciler) reconcileService(ctx context.Context, application *maxicloudv1alpha1.Application) error {
	var svc corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: application.Name, Namespace: application.Namespace}, &svc)
	if errors.IsNotFound(err) {
		return r.Create(ctx, newService(application))
	}
	if err != nil {
		return err
	}

	svc.Spec.Ports[0].Port = application.Spec.Expose.Port
	return r.Update(ctx, &svc)
}

func (r *ApplicationReconciler) reconcileIngress(ctx context.Context, application *maxicloudv1alpha1.Application) error {
	var ingress networkingv1.Ingress
	err := r.Get(ctx, types.NamespacedName{Name: application.Name, Namespace: application.Namespace}, &ingress)
	if errors.IsNotFound(err) {
		// 感想: BaseDomainいらんくね？
		return r.Create(ctx, newIngress(application, r.Config.BaseDomain, r.Config.IngressClass))
	}
	if err != nil {
		return err
	}

	ingress.Spec.Rules[0].Host = application.Spec.Expose.Domain
	ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number = application.Spec.Expose.Port
	ingress.Spec.IngressClassName = &application.Spec.Expose.IngressClassName
	return r.Update(ctx, &ingress)
}

func (r *ApplicationReconciler) updateStatus(ctx context.Context, app *maxicloudv1alpha1.Application, ready bool) error {
	if ready {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "ApplicationReady",
			Message: "Application is ready",
		})
	} else {
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "ApplicationNotReady",
			Message: "Application is not ready",
		})
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var currentApp maxicloudv1alpha1.Application
		if err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, &currentApp); err != nil {
			return err
		}
		currentApp.Status.Conditions = app.Status.Conditions
		return r.Status().Update(ctx, &currentApp)
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maxicloudv1alpha1.Application{}).
		Named("application").
		Complete(r)
}
