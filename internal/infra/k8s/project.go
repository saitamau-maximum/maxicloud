package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type projectRepository struct {
	client client.Client
}

var _ domain.ProjectRepository = (*projectRepository)(nil)

func NewProjectRepository(c client.Client) domain.ProjectRepository {
	return &projectRepository{client: c}
}

func (r *projectRepository) Create(ctx context.Context, project domain.Project) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: meta.ProjectNamespace(project.ID),
		},
	}
	meta.ProjectMeta{
		Name:        project.Name,
		OwnerID:     project.OwnerID,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}.Apply(&ns.ObjectMeta)
	if err := r.client.Create(ctx, ns); err != nil {
		return "", fmt.Errorf("create namespace: %w", err)
	}
	return meta.ProjectIDFromNamespace(ns.Name), nil
}

func (r *projectRepository) CreatePreview(ctx context.Context, params domain.CreatePreviewProjectParams) (*domain.Project, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: meta.ProjectNamespace(params.ID),
		},
	}
	meta.ProjectMeta{
		Name:      params.Name,
		OwnerID:   params.OwnerID,
		CreatedAt: params.CreatedAt,
		UpdatedAt: params.UpdatedAt,
	}.Apply(&ns.ObjectMeta)
	meta.MarkPreview(&ns.ObjectMeta, params.OriginalApplicationID)
	if err := r.client.Create(ctx, ns); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create preview namespace: %w", err)
		}
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
			return nil, fmt.Errorf("get preview namespace: %w", err)
		}
	}
	return nsToProject(ns)
}

func (r *projectRepository) Get(ctx context.Context, id string) (*domain.Project, error) {
	var ns corev1.Namespace
	if err := r.client.Get(ctx, client.ObjectKey{Name: meta.ProjectNamespace(id)}, &ns); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return nsToProject(&ns)
}

func (r *projectRepository) List(ctx context.Context) ([]*domain.Project, error) {
	var nsList corev1.NamespaceList
	if err := r.client.List(ctx, &nsList, meta.SelectProjects()); err != nil {
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

func (r *projectRepository) Update(ctx context.Context, params domain.UpdateProjectParams) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var ns corev1.Namespace
		if err := r.client.Get(ctx, client.ObjectKey{Name: meta.ProjectNamespace(params.ID)}, &ns); err != nil {
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
			ns.Labels[meta.LabelProjectName] = *params.Name
		}
		if params.OwnerID != nil {
			meta.SetOwner(&ns.ObjectMeta, *params.OwnerID)
		}
		if params.Description != nil {
			ns.Annotations[meta.AnnotationProjectDescription] = *params.Description
		}
		ns.Annotations[meta.AnnotationUpdatedAt] = params.UpdatedAt.Format(time.RFC3339)

		return r.client.Patch(ctx, &ns, client.MergeFrom(base))
	})
}

func (r *projectRepository) Delete(ctx context.Context, id string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: meta.ProjectNamespace(id)}}
	return client.IgnoreNotFound(r.client.Delete(ctx, ns))
}

func nsToProject(ns *corev1.Namespace) (*domain.Project, error) {
	m, err := meta.ProjectMetaFrom(ns)
	if err != nil {
		return nil, err
	}
	return &domain.Project{
		ID:          meta.ProjectIDFromNamespace(ns.Name),
		Name:        m.Name,
		OwnerID:     m.OwnerID,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}
