package k8s

import (
	"context"
	"fmt"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationRepository struct {
	client           client.Client
	ingressClassName string
}

var _ domain.ApplicationRepository = (*applicationRepository)(nil)

func NewApplicationRepository(c client.Client, ingressClassName string) domain.ApplicationRepository {
	return &applicationRepository{client: c, ingressClassName: ingressClassName}
}

func (r *applicationRepository) Create(ctx context.Context, app domain.CreateApplicationParams) (*domain.Application, error) {
	namespace, err := projectNamespaceNameByID(ctx, r.client, app.Spec.ProjectID)
	if err != nil {
		return nil, err
	}
	cr := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: namespace,
		},
	}
	applyApplicationMetadata(cr, app.ID, app.Name, app.OwnerID, app.Spec)
	applyApplicationSpec(&cr.Spec, app.Spec, r.ingressClassName)
	if err := r.client.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return crToApplication(cr), nil
}

func (r *applicationRepository) CreatePreview(ctx context.Context, params domain.CreatePreviewApplicationParams) (*domain.Application, error) {
	namespace, err := projectNamespaceNameByID(ctx, r.client, params.ProjectID)
	if err != nil {
		return nil, err
	}
	cr := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: namespace,
		},
	}
	applyApplicationMetadata(cr, params.ID, params.Name, params.OwnerID, params.Spec)
	meta.MarkPreview(&cr.ObjectMeta, params.OriginalApplicationID, params.PRNumber)
	applyApplicationSpec(&cr.Spec, params.Spec, r.ingressClassName)

	if err := r.client.Create(ctx, cr); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create preview application: %w", err)
		}
		if err := r.client.Get(ctx, client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
			return nil, fmt.Errorf("get preview application: %w", err)
		}
	}
	return crToApplication(cr), nil
}

func (r *applicationRepository) Get(ctx context.Context, id string) (*domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &list, meta.SelectByAppID(id)); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToApplication(&list.Items[0]), nil
}

func (r *applicationRepository) List(ctx context.Context, projectID string) ([]domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	opts := []client.ListOption{}
	if projectID != "" {
		namespace, err := projectNamespaceNameByID(ctx, r.client, projectID)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.InNamespace(namespace))
	}
	if err := r.client.List(ctx, &list, opts...); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	apps := make([]domain.Application, 0, len(list.Items))
	for i := range list.Items {
		apps = append(apps, *crToApplication(&list.Items[i]))
	}
	return apps, nil
}

func (r *applicationRepository) Update(ctx context.Context, params domain.UpdateApplicationParams) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &list, meta.SelectByAppID(params.ID)); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("application not found: %s", params.ID)
	}
	cr := list.Items[0]
	applyApplicationMetadata(&cr, params.ID, params.Name, params.OwnerID, params.Spec)
	applyApplicationSpec(&cr.Spec, params.Spec, r.ingressClassName)
	return r.client.Update(ctx, &cr)
}

func (r *applicationRepository) Delete(ctx context.Context, id string) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &list, meta.SelectByAppID(id)); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}
	return client.IgnoreNotFound(r.client.Delete(ctx, &list.Items[0]))
}

func (r *applicationRepository) ListByRepo(ctx context.Context, owner, name, branch string) ([]domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &list, meta.SelectAppsBySource(owner, name, branch)); err != nil {
		return nil, fmt.Errorf("list applications by repo: %w", err)
	}
	apps := make([]domain.Application, 0, len(list.Items))
	for i := range list.Items {
		apps = append(apps, *crToApplication(&list.Items[i]))
	}
	return apps, nil
}

func (r *applicationRepository) ExistsByDomain(ctx context.Context, fqdn string) (bool, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &list); err != nil {
		return false, fmt.Errorf("list applications: %w", err)
	}
	for _, cr := range list.Items {
		if cr.Spec.Expose != nil && cr.Spec.Expose.Domain == fqdn {
			return true, nil
		}
	}
	return false, nil
}

func buildApplicationEnvVar(spec domain.ApplicationSpec) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(spec.Env)+len(spec.Secrets))
	for _, kv := range spec.Env {
		env = append(env, corev1.EnvVar{
			Name:  kv.Key,
			Value: kv.Value,
		})
	}
	for _, kv := range spec.Secrets {
		env = append(env, corev1.EnvVar{
			Name:  kv.Key,
			Value: kv.Value,
		})
	}
	return env
}

func applyApplicationMetadata(
	cr *maxicloudv1alpha1.Application,
	id string,
	name string,
	ownerID string,
	spec domain.ApplicationSpec,
) {
	m := meta.AppMeta{
		ID:      id,
		Name:    name,
		OwnerID: ownerID,
		Repo:    spec.Source.Repo,
		Branch:  spec.Source.Branch,
	}
	if spec.Domain != nil {
		m.RootDomain = spec.Domain.RootDomain
	}
	m.Apply(&cr.ObjectMeta)
	cr.Annotations[meta.AnnotationProjectID] = spec.ProjectID
}

func applyApplicationSpec(
	spec *maxicloudv1alpha1.ApplicationSpec,
	params domain.ApplicationSpec,
	ingressClassName string,
) {
	spec.Env = buildApplicationEnvVar(params)
	if params.Domain == nil {
		spec.Expose = nil
		return
	}
	spec.Expose = &maxicloudv1alpha1.ExposeConfig{
		Domain:           params.Domain.FQDN(),
		Port:             params.Port,
		IngressClassName: ingressClassName,
	}
}

func crToApplication(app *maxicloudv1alpha1.Application) *domain.Application {
	m := meta.AppMetaFrom(app)
	spec := applicationSpecFromCR(app, m)
	return &domain.Application{
		ID:      m.ID,
		Name:    m.Name,
		OwnerID: m.OwnerID,
		Spec:    spec,
		Condition: domain.ApplicationCondition{
			Status: getAppStatus(app),
			Domain: getAppDomain(app),
		},
		CreatedAt: app.CreationTimestamp.Time,
		UpdatedAt: app.CreationTimestamp.Time,
	}
}

func applicationSpecFromCR(app *maxicloudv1alpha1.Application, m meta.AppMeta) domain.ApplicationSpec {
	spec := domain.ApplicationSpec{
		ProjectID: projectIDFromApp(app),
		Source: domain.ApplicationSource{
			Repo:   m.Repo,
			Branch: m.Branch,
		},
		AccessMode: domain.AccessModePrivate,
	}
	if app.Spec.Expose != nil {
		spec.AccessMode = domain.AccessModePublic
		spec.Port = app.Spec.Expose.Port
		spec.Domain = getAppDomain(app)
	}
	for _, env := range app.Spec.Env {
		if env.ValueFrom != nil {
			continue
		}
		spec.Env = append(spec.Env, domain.KeyValue{
			Key:   env.Name,
			Value: env.Value,
		})
	}
	return spec
}

func projectIDFromApp(app *maxicloudv1alpha1.Application) string {
	return app.Annotations[meta.AnnotationProjectID]
}

func getAppStatus(app *maxicloudv1alpha1.Application) domain.ApplicationStatus {
	status := apimeta.FindStatusCondition(app.Status.Conditions, "Ready")
	if status == nil {
		return domain.ApplicationStatusUnavailable
	}
	if status.Status == metav1.ConditionTrue {
		return domain.ApplicationStatusRunning
	}
	return domain.ApplicationStatusUnavailable
}

func getAppDomain(app *maxicloudv1alpha1.Application) *domain.Domain {
	if app.Spec.Expose == nil {
		return nil
	}
	d, err := domain.NewDomainByFQDN(app.Spec.Expose.Domain, app.Annotations[meta.AnnotationRootDomain])
	if err != nil {
		return nil
	}
	return &d
}
