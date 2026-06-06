package k8s

import (
	"context"
	"fmt"
	"maps"
	"strings"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationRepository struct {
	client.Client
	ingressClassName string
}

var _ domain.ApplicationRepository = (*applicationRepository)(nil)

func NewApplicationRepository(c client.Client, ingressClassName string) domain.ApplicationRepository {
	return &applicationRepository{Client: c, ingressClassName: ingressClassName}
}

func (r *applicationRepository) Create(ctx context.Context, app domain.CreateApplicationParams) (*domain.Application, error) {
	cr := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: meta.ProjectNamespace(app.Spec.ProjectID),
		},
	}
	applyApplicationMetadata(cr, app.ID, app.Name, app.OwnerID, app.Spec)
	applyApplicationSpec(&cr.Spec, app.Spec, r.ingressClassName)
	if err := r.Client.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return crToApplication(cr), nil
}

func (r *applicationRepository) Get(ctx context.Context, id string) (*domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, meta.SelectByAppID(id)); err != nil {
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
		opts = append(opts, client.InNamespace(meta.ProjectNamespace(projectID)))
	}
	if err := r.Client.List(ctx, &list, opts...); err != nil {
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
	if err := r.Client.List(ctx, &list, meta.SelectByAppID(params.ID)); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("application not found: %s", params.ID)
	}
	cr := list.Items[0]
	applyApplicationMetadata(&cr, params.ID, params.Name, params.OwnerID, params.Spec)
	applyApplicationSpec(&cr.Spec, params.Spec, r.ingressClassName)
	return r.Client.Update(ctx, &cr)
}

func (r *applicationRepository) Delete(ctx context.Context, id string) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, meta.SelectByAppID(id)); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}
	return client.IgnoreNotFound(r.Client.Delete(ctx, &list.Items[0]))
}

func (r *applicationRepository) ListByRepo(ctx context.Context, owner, name, branch string) ([]domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, meta.SelectAppsBySource(owner, name, branch)); err != nil {
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
	if err := r.Client.List(ctx, &list); err != nil {
		return false, fmt.Errorf("list applications: %w", err)
	}
	for _, cr := range list.Items {
		if cr.Spec.Expose != nil && cr.Spec.Expose.Domain == fqdn {
			return true, nil
		}
	}
	return false, nil
}

func (r *applicationRepository) CreatePreviewApplication(ctx context.Context, originalApplicationID string, prNumber int, id string) (*domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, meta.SelectByAppID(originalApplicationID)); err != nil {
		return nil, fmt.Errorf("list original application: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("original application not found: %s", originalApplicationID)
	}
	orig := list.Items[0]
	namespace := orig.Namespace

	previewName := fmt.Sprintf("%s-pr-%d", orig.Name, prNumber)
	desired := buildPreviewApplicationCR(orig, namespace, previewName, prNumber, id)
	if desired == nil {
		return nil, fmt.Errorf("build preview application: nil desired CR")
	}

	var existing maxicloudv1alpha1.Application
	key := client.ObjectKey{Name: previewName, Namespace: namespace}
	if err := r.Client.Get(ctx, key, &existing); err == nil {
		updated, err := r.updatePreviewApplication(ctx, key, desired)
		if err != nil {
			return nil, fmt.Errorf("update preview application: %w", err)
		}
		return crToApplication(updated), nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get preview application: %w", err)
	}

	if err := r.Client.Create(ctx, desired); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create preview application: %w", err)
		}
		if err := r.Client.Get(ctx, key, &existing); err != nil {
			return nil, fmt.Errorf("get preview application after already exists: %w", err)
		}
		updated, err := r.updatePreviewApplication(ctx, key, desired)
		if err != nil {
			return nil, fmt.Errorf("update preview application after already exists: %w", err)
		}
		return crToApplication(updated), nil
	}
	return crToApplication(desired), nil
}

func buildPreviewApplicationCR(orig maxicloudv1alpha1.Application, namespace, previewName string, prNumber int, id string) *maxicloudv1alpha1.Application {
	newSpec := orig.Spec.DeepCopy()
	if newSpec.Expose != nil {
		root := orig.Annotations[meta.AnnotationRootDomain]
		if root == "" {
			newSpec.Expose.Domain = fmt.Sprintf("%s-pr-%d", orig.Spec.Expose.Domain, prNumber)
		} else {
			fqdn := orig.Spec.Expose.Domain
			sub := strings.TrimSuffix(fqdn, "."+root)
			if sub == fqdn {
				sub = fqdn
			}
			newSub := fmt.Sprintf("%s-pr%d", sub, prNumber)
			newSpec.Expose.Domain = newSub + "." + root
		}
	}

	return &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      previewName,
			Namespace: namespace,
			Labels: map[string]string{
				meta.LabelAppID:           id,
				meta.LabelAppName:         previewName,
				meta.LabelOwnerUserID:     orig.Labels[meta.LabelOwnerUserID],
				meta.LabelSourceRepoOwner: orig.Labels[meta.LabelSourceRepoOwner],
				meta.LabelSourceRepoName:  orig.Labels[meta.LabelSourceRepoName],
				meta.LabelSourceBranch:    meta.NormalizeBranchForLabel(fmt.Sprintf("pr-%d", prNumber)),
			},
			Annotations: map[string]string{
				meta.AnnotationSourceBranch: fmt.Sprintf("pr-%d", prNumber),
				meta.AnnotationRootDomain:   orig.Annotations[meta.AnnotationRootDomain],
			},
		},
		Spec: *newSpec,
	}
}

func (r *applicationRepository) updatePreviewApplication(ctx context.Context, key client.ObjectKey, desired *maxicloudv1alpha1.Application) (*maxicloudv1alpha1.Application, error) {
	var updated maxicloudv1alpha1.Application
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var current maxicloudv1alpha1.Application
		if err := r.Client.Get(ctx, key, &current); err != nil {
			return err
		}

		base := current.DeepCopy()
		mergePreviewApplication(&current, desired)
		if err := r.Client.Patch(ctx, &current, client.MergeFrom(base)); err != nil {
			return err
		}
		updated = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func mergePreviewApplication(current, desired *maxicloudv1alpha1.Application) {
	existingID := ""
	if current.Labels != nil {
		existingID = current.Labels[meta.LabelAppID]
	} else {
		current.Labels = map[string]string{}
	}
	maps.Copy(current.Labels, desired.Labels)
	if existingID != "" {
		current.Labels[meta.LabelAppID] = existingID
	}

	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	maps.Copy(current.Annotations, desired.Annotations)

	current.Spec = desired.Spec
}

func crToApplication(app *maxicloudv1alpha1.Application) *domain.Application {
	m := meta.AppMetaFrom(app)
	return &domain.Application{
		ID:        m.ID,
		ProjectID: meta.ProjectIDFromNamespace(app.Namespace),
		Name:      m.Name,
		OwnerID:   m.OwnerID,
		Source: domain.ApplicationSource{
			Repo:   m.Repo,
			Branch: m.Branch,
		},
		Condition: domain.ApplicationCondition{
			Status: getAppStatus(app),
			Domain: getAppDomain(app),
		},
		CreatedAt: app.CreationTimestamp.Time,
		UpdatedAt: app.CreationTimestamp.Time,
	}
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
