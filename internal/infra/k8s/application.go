package k8s

import (
	"context"
	"fmt"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelApplicationID    = config.LabelPrefix + "app-id"
	labelApplicationName  = config.LabelPrefix + "app-name"
	labelApplicationOwner = config.LabelPrefix + "owner-user-id"
	labelSourceRepoOwner  = config.LabelPrefix + "source-repo-owner"
	labelSourceRepoName   = config.LabelPrefix + "source-repo-name"
	labelSourceBranch     = config.LabelPrefix + "source-branch"

	annotationSourceBranch = config.AnnotationPrefix + "source-branch"
	annotationRootDomain   = config.AnnotationPrefix + "root-domain"
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
	branchLabel := normalizeBranchForLabel(app.Spec.Source.Branch)
	cr := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: projectNamespace(app.Spec.ProjectID),
			Labels: map[string]string{
				labelApplicationID:    app.ID,
				labelApplicationName:  app.Name,
				labelApplicationOwner: truncateLabelValue(app.OwnerID),
				labelSourceRepoOwner:  app.Spec.Source.Repo.Owner,
				labelSourceRepoName:   app.Spec.Source.Repo.Name,
				labelSourceBranch:     branchLabel,
			},
			Annotations: map[string]string{
				annotationSourceBranch: app.Spec.Source.Branch,
				annotationRootDomain:   app.Spec.Domain.RootDomain,
			},
		},
		Spec: maxicloudv1alpha1.ApplicationSpec{
			Env: buildApplicationEnvVar(app.Spec),
		},
	}
	if app.Spec.Domain != nil {
		cr.Spec.Expose = &maxicloudv1alpha1.ExposeConfig{
			Domain:           app.Spec.Domain.FQDN(),
			Port:             app.Spec.Port,
			IngressClassName: r.ingressClassName,
		}
	}
	if err := r.Client.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return crToApplication(cr), nil
}

func (r *applicationRepository) Get(ctx context.Context, id string) (*domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, client.MatchingLabels{labelApplicationID: id}); err != nil {
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
		opts = append(opts, client.InNamespace(projectNamespace(projectID)))
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

func (r *applicationRepository) Update(ctx context.Context, app domain.Application) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, client.MatchingLabels{labelApplicationID: app.ID}); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("application not found: %s", app.ID)
	}
	cr := list.Items[0]
	cr.Labels[labelApplicationName] = app.Name
	cr.Labels[labelApplicationOwner] = truncateLabelValue(app.OwnerID)
	return r.Client.Update(ctx, &cr)
}

func (r *applicationRepository) Delete(ctx context.Context, id string) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, client.MatchingLabels{labelApplicationID: id}); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}
	return client.IgnoreNotFound(r.Client.Delete(ctx, &list.Items[0]))
}

func (r *applicationRepository) ListByRepo(ctx context.Context, owner, name, branch string) ([]domain.Application, error) {
	matchLabels := client.MatchingLabels{
		labelSourceRepoOwner: owner,
		labelSourceRepoName:  name,
	}
	if branch != "" {
		matchLabels[labelSourceBranch] = normalizeBranchForLabel(branch)
	}
	var list maxicloudv1alpha1.ApplicationList
	if err := r.Client.List(ctx, &list, matchLabels); err != nil {
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

func crToApplication(app *maxicloudv1alpha1.Application) *domain.Application {
	return &domain.Application{
		ID:        app.Labels[labelApplicationID],
		ProjectID: projectIDFromNamespace(app.Namespace),
		Name:      app.Labels[labelApplicationName],
		OwnerID:   app.Labels[labelApplicationOwner],
		Source: domain.ApplicationSource{
			Repo: domain.Repository{
				Owner: app.Labels[labelSourceRepoOwner],
				Name:  app.Labels[labelSourceRepoName],
			},
			Branch: app.Annotations[annotationSourceBranch],
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
	status := meta.FindStatusCondition(app.Status.Conditions, "Ready")
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
	d, err := domain.NewDomainByFQDN(app.Spec.Expose.Domain, app.Annotations[annotationRootDomain])
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
