package meta

import (
	"strings"
	"testing"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppMeta_RoundTrip(t *testing.T) {
	t.Parallel()

	want := AppMeta{
		ID:         "app-123",
		Name:       "my-app",
		OwnerID:    strings.Repeat("u", 100),
		Repo:       domain.Repository{Owner: "saitamau-maximum", Name: "maxicloud"},
		Branch:     "feature/new-ui",
		RootDomain: "maxicloud.maximum.vc",
	}

	var om metav1.ObjectMeta
	want.Apply(&om)
	got := AppMetaFrom(&om)

	if got != want {
		t.Fatalf("AppMetaFrom(Apply(m)) = %+v, want %+v", got, want)
	}
	if om.Labels[LabelOwnerUserID] != clampLabelValue(want.OwnerID) {
		t.Fatalf("owner label = %q, want truncated", om.Labels[LabelOwnerUserID])
	}
}

func TestAppMeta_RoundTrip_NoRootDomain(t *testing.T) {
	t.Parallel()

	want := AppMeta{
		ID:      "app-1",
		Name:    "app",
		OwnerID: "owner-1",
		Repo:    domain.Repository{Owner: "o", Name: "r"},
		Branch:  "main",
	}

	var om metav1.ObjectMeta
	want.Apply(&om)
	got := AppMetaFrom(&om)

	if got != want {
		t.Fatalf("AppMetaFrom(Apply(m)) = %+v, want %+v", got, want)
	}
	if _, ok := om.Annotations[AnnotationRootDomain]; ok {
		t.Fatalf("root-domain annotation should be absent when RootDomain is empty")
	}
}

func TestDeployRunMeta_RoundTrip(t *testing.T) {
	t.Parallel()

	want := DeployRunMeta{
		DeployRunID: "dr-1",
		AppID:       "app-1",
		OwnerUserID: strings.Repeat("u", 100),
		IsPreview:   true,
	}

	var om metav1.ObjectMeta
	want.Apply(&om)
	got := DeployRunMetaFrom(&om)

	if got != want {
		t.Fatalf("DeployRunMetaFrom(Apply(m)) = %+v, want %+v", got, want)
	}
}
