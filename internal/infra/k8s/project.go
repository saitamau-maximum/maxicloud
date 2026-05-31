package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	projectLabelKey = config.LabelPrefix + "project"

	OwnerUserIDLabelKey = config.LabelPrefix + "owner-user-id"
	ProjectNameLabelKey = config.LabelPrefix + "project-name"

	ProjectDescriptionAnnotationKey = config.AnnotationPrefix + "project-description"
	CreatedAtAnnotationKey          = config.AnnotationPrefix + "created-at"
	UpdatedAtAnnotationKey          = config.AnnotationPrefix + "updated-at"
)

type projectRepository struct {
	client.Client
}

var _ domain.ProjectRepository = (*projectRepository)(nil)

func NewProjectRepository(c client.Client) domain.ProjectRepository {
	return &projectRepository{Client: c}
}

func (r *projectRepository) CreateProject(ctx context.Context, project domain.Project) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: projectNamespace(project.ID),
			Labels: map[string]string{
				projectLabelKey:     "true",
				OwnerUserIDLabelKey: truncateLabelValue(project.OwnerID),
				ProjectNameLabelKey: project.Name,
			},
			Annotations: map[string]string{
				ProjectDescriptionAnnotationKey: project.Description,
				CreatedAtAnnotationKey:          project.CreatedAt.Format(time.RFC3339),
				UpdatedAtAnnotationKey:          project.UpdatedAt.Format(time.RFC3339),
			},
		},
	}
	if err := r.Create(ctx, ns); err != nil {
		return "", fmt.Errorf("create namespace: %w", err)
	}
	return strings.TrimPrefix(ns.Name, NamespacePrefix), nil
}

func (r *projectRepository) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	var ns corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: projectNamespace(id)}, &ns); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return nsToProject(&ns)
}

func (r *projectRepository) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList, client.MatchingLabels{projectLabelKey: "true"}); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	projects := make([]*domain.Project, 0, len(nsList.Items))
	for i := range nsList.Items {
		project, err := nsToProject(&nsList.Items[i])
		if err != nil {
			return nil, fmt.Errorf("convert namespace to project: %w", err)
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (r *projectRepository) UpdateProject(ctx context.Context, params domain.UpdateProjectParams) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var ns corev1.Namespace
		if err := r.Get(ctx, client.ObjectKey{Name: projectNamespace(params.ID)}, &ns); err != nil {
			return fmt.Errorf("get namespace: %w", err)
		}

		base := ns.DeepCopy()

		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}

		if params.Name != nil {
			ns.Labels[ProjectNameLabelKey] = *params.Name
		}
		if params.OwnerID != nil {
			ns.Labels[OwnerUserIDLabelKey] = truncateLabelValue(*params.OwnerID)
		}
		if params.Description != nil {
			ns.Annotations[ProjectDescriptionAnnotationKey] = *params.Description
		}
		ns.Annotations[UpdatedAtAnnotationKey] = params.UpdatedAt.Format(time.RFC3339)

		return r.Patch(ctx, &ns, client.MergeFrom(base))
	})
}

func (r *projectRepository) DeleteProject(ctx context.Context, id string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: projectNamespace(id)}}
	return client.IgnoreNotFound(r.Delete(ctx, ns))
}

func nsToProject(ns *corev1.Namespace) (*domain.Project, error) {
	createdAt, err := time.Parse(time.RFC3339, ns.Annotations[CreatedAtAnnotationKey])
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, ns.Annotations[UpdatedAtAnnotationKey])
	if err != nil {
		return nil, fmt.Errorf("parse updatedAt: %w", err)
	}
	return &domain.Project{
		ID:          projectIDFromNamespace(ns.Name),
		Name:        ns.Labels[ProjectNameLabelKey],
		OwnerID:     ns.Labels[OwnerUserIDLabelKey],
		Description: ns.Annotations[ProjectDescriptionAnnotationKey],
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
