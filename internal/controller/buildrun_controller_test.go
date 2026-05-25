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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type fakeRegistry struct{}

func (f *fakeRegistry) DockerConfig() string { return `{"auths":{}}` }
func (f *fakeRegistry) Host() string         { return "ghcr.io/test" }
func (f *fakeRegistry) Token() string        { return "token" }
func (f *fakeRegistry) BuildOutput(destination string) string {
	return "type=image,name=" + destination + ",push=true"
}

type fakeGitHubClient struct{}

var _ domain.DeploymentReporter = (*fakeGitHubClient)(nil)

func (f *fakeGitHubClient) GetInstallationAccessToken(_ context.Context) (string, error) {
	return "fake-token", nil
}
func (f *fakeGitHubClient) CreateCommitStatus(_ context.Context, _ domain.CreateCommitStatusParams) (int64, error) {
	return 0, nil
}
func (f *fakeGitHubClient) UpdateCommitStatus(_ context.Context, _ domain.UpdateCommitStatusParams) error {
	return nil
}
func (f *fakeGitHubClient) GetDeploymentSummary(_ context.Context, _, _ string, _ int64) (*domain.DeploymentSummary, error) {
	return nil, nil
}
func (f *fakeGitHubClient) CreateDeploymentSummary(_ context.Context, _ domain.CreateDeploymentSummaryParams) (int64, error) {
	return 0, nil
}
func (f *fakeGitHubClient) UpdateDeploymentSummary(_ context.Context, _ domain.UpdateDeploymentSummaryParams) error {
	return nil
}

func newTestReconciler() *BuildRunReconciler {
	return &BuildRunReconciler{
		Client:       k8sClient,
		Scheme:       k8sClient.Scheme(),
		Registry:     &fakeRegistry{},
		GitHubClient: &fakeGitHubClient{},
	}
}

func newTestBuildRun(name, namespace, appName string) *maxicloudv1alpha1.BuildRun {
	labels := map[string]string{}
	if appName != "" {
		labels[config.ApplicationLabelKey] = appName
	}
	return &maxicloudv1alpha1.BuildRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: maxicloudv1alpha1.BuildRunSpec{
			Source: maxicloudv1alpha1.BuildSource{
				RepoURL: "https://github.com/saitamau-maximum/maxicloud",
				SHA:     "abc1234567890",
			},
		},
	}
}

var _ = Describe("BuildRun Controller", func() {
	ctx := context.Background()
	const ns = "default"

	Context("Jobの作成", func() {
		const name = "buildrun-job-create"

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, newTestBuildRun(name, ns, "my-app"))).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &maxicloudv1alpha1.BuildRun{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			})
		})

		It("ReconcileでJobが作成される", func() {
			r := newTestReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var job batchv1.Job
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &job)).To(Succeed())
		})

		It("2回Reconcileしても重複してJobが作られない", func() {
			r := newTestReconciler()
			for range 2 {
				_, err := r.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
				})
				Expect(err).NotTo(HaveOccurred())
			}

			var jobList batchv1.JobList
			Expect(k8sClient.List(ctx, &jobList,
				client.InNamespace(ns),
				client.MatchingLabels{config.ApplicationLabelKey: "my-app"},
			)).To(Succeed())
			Expect(jobList.Items).To(HaveLen(1))
		})

		It("Secretが作成される", func() {
			r := newTestReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())

			var secret corev1.Secret
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &secret)).To(Succeed())
			Expect(secret.Data).To(HaveKey(config.InstallationAccessTokenKey))
			Expect(secret.Data).To(HaveKey(corev1.DockerConfigJsonKey))
		})
	})

	Context("古いJobの削除", func() {
		AfterEach(func() {
			var jobList batchv1.JobList
			_ = k8sClient.List(ctx, &jobList, client.InNamespace(ns))
			for i := range jobList.Items {
				_ = k8sClient.Delete(ctx, &jobList.Items[i])
			}
		})
	})

	Context("BuildRunが存在しない場合", func() {
		It("NotFoundは無視してエラーにならない", func() {
			r := newTestReconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: ns},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
