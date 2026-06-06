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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
)

func newTestAppReconciler() *ApplicationReconciler {
	return &ApplicationReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Registry: &fakeRegistry{},
		Config: ReconcilerConfig{
			IngressClass: "nginx",
			BaseDomain:   "example.com",
		},
	}
}

func newTestApplication(name, namespace, image string, expose *maxicloudv1alpha1.ExposeConfig) *maxicloudv1alpha1.Application {
	return &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maxicloudv1alpha1.ApplicationSpec{
			Image:  image,
			Expose: expose,
		},
	}
}

var _ = Describe("Application Controller", func() {
	ctx := context.Background()
	const ns = "default"

	Context("Expose未設定", func() {
		const name = "app-no-expose"

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, newTestApplication(name, ns, "ghcr.io/test/app:latest", nil))).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &maxicloudv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			})
			_ = k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: appRegistrySecretName(name), Namespace: ns},
			})
		})

		It("ReconcileでDeploymentが作成される", func() {
			r := newTestAppReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var deploy appsv1.Deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/test/app:latest"))
		})

		It("ReconcileでRegistry Secretが作成される", func() {
			r := newTestAppReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var secret corev1.Secret
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appRegistrySecretName(name), Namespace: ns}, &secret)).To(Succeed())
			Expect(secret.Data).To(HaveKey(corev1.DockerConfigJsonKey))
		})

		It("ServiceとIngressが作成されない", func() {
			r := newTestAppReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var svc corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &svc)).NotTo(Succeed())

			var ingress networkingv1.Ingress
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &ingress)).NotTo(Succeed())
		})
	})

	Context("Expose設定あり", func() {
		const name = "app-with-expose"
		expose := &maxicloudv1alpha1.ExposeConfig{
			Port:   8080,
			Domain: "app.example.com",
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, newTestApplication(name, ns, "ghcr.io/test/app:latest", expose))).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &maxicloudv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			})
			_ = k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: appRegistrySecretName(name), Namespace: ns},
			})
		})

		It("ServiceとIngressが作成される", func() {
			r := newTestAppReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var svc corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &svc)).To(Succeed())
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))

			var ingress networkingv1.Ingress
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &ingress)).To(Succeed())
			Expect(ingress.Spec.Rules[0].Host).To(Equal("app.example.com"))
		})
	})

	Context("Applicationが存在しない場合", func() {
		It("NotFoundは無視してエラーにならない", func() {
			r := newTestAppReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
