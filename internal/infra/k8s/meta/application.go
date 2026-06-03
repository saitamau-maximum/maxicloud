package meta

import (
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AppMeta struct {
	ID         string
	Name       string
	OwnerID    string
	Repo       domain.Repository
	Branch     string
	RootDomain string
}

func (m AppMeta) Apply(o *metav1.ObjectMeta) {
	ensureMaps(o)
	o.Labels[LabelAppID] = m.ID
	o.Labels[LabelAppName] = m.Name
	o.Labels[LabelSourceRepoOwner] = m.Repo.Owner
	o.Labels[LabelSourceRepoName] = m.Repo.Name
	o.Labels[LabelSourceBranch] = NormalizeBranchForLabel(m.Branch)
	SetOwner(o, m.OwnerID)

	o.Annotations[AnnotationSourceBranch] = m.Branch
	if m.RootDomain != "" {
		o.Annotations[AnnotationRootDomain] = m.RootDomain
	} else {
		delete(o.Annotations, AnnotationRootDomain)
	}
}

func AppMetaFrom(o metav1.Object) AppMeta {
	l, a := o.GetLabels(), o.GetAnnotations()
	return AppMeta{
		ID:      l[LabelAppID],
		Name:    l[LabelAppName],
		OwnerID: readOwner(l, a),
		Repo: domain.Repository{
			Owner: l[LabelSourceRepoOwner],
			Name:  l[LabelSourceRepoName],
		},
		Branch:     a[AnnotationSourceBranch],
		RootDomain: a[AnnotationRootDomain],
	}
}

func SelectByAppID(id string) client.MatchingLabels {
	return client.MatchingLabels{LabelAppID: id}
}

func SelectAppsBySource(owner, name, branch string) client.MatchingLabels {
	sel := client.MatchingLabels{
		LabelSourceRepoOwner: owner,
		LabelSourceRepoName:  name,
	}
	if branch != "" {
		sel[LabelSourceBranch] = NormalizeBranchForLabel(branch)
	}
	return sel
}
