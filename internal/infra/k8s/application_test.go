package k8s

import (
	"context"
	"regexp"
	"strings"
	"testing"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNormalizeBranchForLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		in           string
		expectPrefix string
	}{
		{
			name:         "許可文字はそのまま維持する",
			in:           "feature.foo_bar-1",
			expectPrefix: "feature.foo_bar-1-",
		},
		{
			name:         "スラッシュをハイフンに置換する",
			in:           "feature/login-flow",
			expectPrefix: "feature-login-flow-",
		},
		{
			name:         "連続する無効文字を1つのハイフンにまとめる",
			in:           "feature///login@@@flow",
			expectPrefix: "feature-login-flow-",
		},
		{
			name:         "先頭と末尾の英数字以外を除去する",
			in:           "--feature-login--",
			expectPrefix: "feature-login-",
		},
		{
			name:         "空文字の場合はフォールバックのベースを使う",
			in:           "",
			expectPrefix: "branch-",
		},
		{
			name:         "無効文字のみの場合はフォールバックのベースを使う",
			in:           "/////@@@@@-----",
			expectPrefix: "branch-",
		},
		{
			name:         "非ASCIIのみの場合はフォールバックのベースを使う",
			in:           "日本語ブランチ",
			expectPrefix: "branch-",
		},
		{
			name:         "前後の空白を除去する",
			in:           "  feature/new-ui  ",
			expectPrefix: "feature-new-ui-",
		},
	}

	hexSuffixPattern := regexp.MustCompile(`-[0-9a-f]{8}$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeBranchForLabel(tt.in)
			if !strings.HasPrefix(got, tt.expectPrefix) {
				t.Fatalf("normalizeBranchForLabel(%q) = %q, want prefix %q", tt.in, got, tt.expectPrefix)
			}
			if !hexSuffixPattern.MatchString(got) {
				t.Fatalf("normalizeBranchForLabel(%q) = %q, want suffix matching %q", tt.in, got, hexSuffixPattern.String())
			}
			if len(got) > 63 {
				t.Fatalf("normalizeBranchForLabel(%q) len = %d, want <= 63", tt.in, len(got))
			}
		})
	}
}

func TestNormalizeBranchForLabel_DifferentInputsDifferentOutputs(t *testing.T) {
	t.Parallel()

	a := normalizeBranchForLabel("feat/hogehoge-fugafuga")
	b := normalizeBranchForLabel("feat-hogehoge-fugafuga")
	if a == b {
		t.Fatalf("expected different outputs for potentially colliding branches, got %q", a)
	}
}

func TestCreatePreviewApplication_IsIdempotent(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := maxicloudv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add application scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	original := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-app",
			Namespace: projectNamespace("project-1"),
			Labels: map[string]string{
				labelApplicationID:    "app-001",
				labelApplicationName:  "demo-app",
				labelApplicationOwner: "owner-1",
				labelSourceRepoOwner:  "octo",
				labelSourceRepoName:   "demo",
				labelSourceBranch:     normalizeBranchForLabel("feature/test"),
			},
			Annotations: map[string]string{
				annotationSourceBranch: "feature/test",
				annotationRootDomain:   "example.com",
			},
		},
		Spec: maxicloudv1alpha1.ApplicationSpec{
			Image: "ghcr.io/octo/demo:latest",
			Expose: &maxicloudv1alpha1.ExposeConfig{
				Domain:           "demo.example.com",
				Port:             8080,
				IngressClassName: "nginx",
			},
			Env: []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(original).Build()
	repo := &applicationRepository{Client: client, ingressClassName: "nginx"}

	ctx := context.Background()
	first, err := repo.CreatePreviewApplication(ctx, "app-001", 7, "preview-1")
	if err != nil {
		t.Fatalf("first CreatePreviewApplication: %v", err)
	}
	second, err := repo.CreatePreviewApplication(ctx, "app-001", 7, "preview-2")
	if err != nil {
		t.Fatalf("second CreatePreviewApplication: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected preview app id to stay stable, got first=%q second=%q", first.ID, second.ID)
	}
	if first.Name != "demo-app-pr-7" {
		t.Fatalf("unexpected preview name: %q", first.Name)
	}
	if first.Condition.Domain == nil || first.Condition.Domain.FQDN() != "demo-pr7.example.com" {
		t.Fatalf("unexpected preview domain: %#v", first.Condition.Domain)
	}

	var list maxicloudv1alpha1.ApplicationList
	if err := client.List(ctx, &list); err != nil {
		t.Fatalf("list applications: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 applications (original + preview), got %d", len(list.Items))
	}
	previewCount := 0
	for i := range list.Items {
		if list.Items[i].Name == "demo-app-pr-7" {
			previewCount++
			if list.Items[i].Labels[labelApplicationID] != first.ID {
				t.Fatalf("expected preview app-id to stay stable, got %q want %q", list.Items[i].Labels[labelApplicationID], first.ID)
			}
		}
	}
	if previewCount != 1 {
		t.Fatalf("expected exactly one preview application, got %d", previewCount)
	}
}
