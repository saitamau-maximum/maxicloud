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
			Name: meta.ProjectNamespace(project.Name, project.ID),
		},
	}
	meta.ProjectMeta{
		ID:          project.ID,
		Name:        project.Name,
		OwnerID:     project.OwnerID,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}.Apply(&ns.ObjectMeta)
	if err := r.client.Create(ctx, ns); err != nil {
		return "", fmt.Errorf("create namespace: %w", err)
	}
	return project.ID, nil
}

func (r *projectRepository) CreatePreview(ctx context.Context, params domain.CreatePreviewProjectParams) (*domain.Project, error) {
	namespace := meta.ProjectNamespace(params.Name, params.OriginalApplicationID)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(ns), ns); err == nil {
		return nsToProject(ns)
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get preview namespace: %w", err)
	}
	meta.ProjectMeta{
		ID:        params.ID,
		Name:      params.Name,
		OwnerID:   params.OwnerID,
		CreatedAt: params.CreatedAt,
		UpdatedAt: params.UpdatedAt,
	}.Apply(&ns.ObjectMeta)
	meta.MarkPreview(&ns.ObjectMeta, params.OriginalApplicationID)
	if err := r.client.Create(ctx, ns); err != nil {
		return nil, fmt.Errorf("create preview namespace: %w", err)
	}
	return nsToProject(ns)
}

func (r *projectRepository) Get(ctx context.Context, id string) (*domain.Project, error) {
	ns, err := findProjectNamespaceByID(ctx, r.client, id)
	if err != nil {
		return nil, err
	}
	if ns == nil {
		return nil, nil
	}
	return nsToProject(ns)
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
	ns, err := findProjectNamespaceByID(ctx, r.client, params.ID)
	if err != nil {
		return err
	}
	if ns == nil {
		return fmt.Errorf("project not found: %s", params.ID)
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

	return r.client.Patch(ctx, ns, client.MergeFrom(base))
}

func (r *projectRepository) Delete(ctx context.Context, id string) error {
	ns, err := findProjectNamespaceByID(ctx, r.client, id)
	if err != nil {
		return err
	}
	if ns == nil {
		return nil
	}
	return client.IgnoreNotFound(r.client.Delete(ctx, ns))
}

func nsToProject(ns *corev1.Namespace) (*domain.Project, error) {
	m, err := meta.ProjectMetaFrom(ns)
	if err != nil {
		return nil, err
	}
	return &domain.Project{
		ID:          m.ID,
		Name:        m.Name,
		OwnerID:     m.OwnerID,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}

func findProjectNamespaceByID(ctx context.Context, c client.Client, id string) (*corev1.Namespace, error) {
	var nsList corev1.NamespaceList
	if err := c.List(ctx, &nsList, client.MatchingLabels{
		meta.LabelProject:   "true",
		meta.LabelProjectID: id,
	}); err != nil {
		return nil, fmt.Errorf("list namespaces by project id: %w", err)
	}
	if len(nsList.Items) == 0 {
		return nil, nil
	}
	return &nsList.Items[0], nil
}

func projectNamespaceNameByID(ctx context.Context, c client.Client, id string) (string, error) {
	ns, err := findProjectNamespaceByID(ctx, c, id)
	if err != nil {
		return "", err
	}
	if ns == nil {
		return "", fmt.Errorf("project namespace not found: %s", id)
	}
	return ns.Name, nil
}
