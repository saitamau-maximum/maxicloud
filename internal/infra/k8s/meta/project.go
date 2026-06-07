package meta

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProjectMeta struct {
	ID          string
	Name        string
	OwnerID     string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (m ProjectMeta) Apply(o *metav1.ObjectMeta) {
	ensureMaps(o)
	o.Labels[LabelProject] = "true"
	o.Labels[LabelProjectID] = m.ID
	o.Labels[LabelProjectName] = m.Name
	SetOwner(o, m.OwnerID)

	o.Annotations[AnnotationProjectDescription] = m.Description
	o.Annotations[AnnotationCreatedAt] = m.CreatedAt.Format(time.RFC3339)
	o.Annotations[AnnotationUpdatedAt] = m.UpdatedAt.Format(time.RFC3339)
}

func ProjectMetaFrom(o metav1.Object) (ProjectMeta, error) {
	l, a := o.GetLabels(), o.GetAnnotations()
	createdAt, err := time.Parse(time.RFC3339, a[AnnotationCreatedAt])
	if err != nil {
		return ProjectMeta{}, fmt.Errorf("parse createdAt: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, a[AnnotationUpdatedAt])
	if err != nil {
		return ProjectMeta{}, fmt.Errorf("parse updatedAt: %w", err)
	}
	return ProjectMeta{
		ID:          l[LabelProjectID],
		Name:        l[LabelProjectName],
		OwnerID:     readOwner(l, a),
		Description: a[AnnotationProjectDescription],
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func SelectProjects() client.MatchingLabels {
	return client.MatchingLabels{LabelProject: "true"}
}
