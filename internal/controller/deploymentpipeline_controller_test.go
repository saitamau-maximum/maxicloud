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
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
)

var _ = Describe("DeploymentPipeline Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		deploymentpipeline := &maxicloudv1alpha1.DeploymentPipeline{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DeploymentPipeline")
			err := k8sClient.Get(ctx, typeNamespacedName, deploymentpipeline)
			if err != nil && errors.IsNotFound(err) {
				resource := &maxicloudv1alpha1.DeploymentPipeline{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &maxicloudv1alpha1.DeploymentPipeline{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DeploymentPipeline")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DeploymentPipelineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Reporter: &fakeGitHubClient{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})

		It("PR向けのpreview成功時にGitHub PRへURLコメントを作成する", func() {
			appName := "preview-app"
			prNumber := 7
			pipelineName := "preview-pipeline"
			app := &maxicloudv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: "default",
				},
				Spec: maxicloudv1alpha1.ApplicationSpec{
					Image: "ghcr.io/example/app:abc123",
					Expose: &maxicloudv1alpha1.ExposeConfig{
						Domain:           "preview.example.com",
						Port:             8080,
						IngressClassName: "nginx",
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			pipeline := &maxicloudv1alpha1.DeploymentPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineName,
					Namespace: "default",
				},
				Spec: maxicloudv1alpha1.DeploymentPipelineSpec{
					ApplicationName: appName,
					Owner:           "octocat",
					Repo:            "hello-world",
					SHA:             "abc123",
					PRNumber:        &prNumber,
				},
				Status: maxicloudv1alpha1.DeploymentPipelineStatus{
					Phase: maxicloudv1alpha1.DeploymentPipelinePhaseDeploying,
				},
			}
			Expect(k8sClient.Create(ctx, pipeline)).To(Succeed())

			createdPipeline := &maxicloudv1alpha1.DeploymentPipeline{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pipelineName, Namespace: "default"}, createdPipeline)).To(Succeed())
			createdPipeline.Status.Phase = maxicloudv1alpha1.DeploymentPipelinePhaseDeploying
			Expect(k8sClient.Status().Update(ctx, createdPipeline)).To(Succeed())

			reporter := &fakeGitHubClient{}
			controllerReconciler := &DeploymentPipelineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Reporter: reporter,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: pipelineName, Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(reporter.createDeploymentSummaryCalls).To(HaveLen(1))
			Expect(reporter.createDeploymentSummaryCalls[0]).To(Equal(domain.CreateDeploymentSummaryParams{
				Owner:    "octocat",
				Repo:     "hello-world",
				PrNumber: prNumber,
				Comment:  "Preview URL: http://preview.example.com",
			}))
		})
	})
})
