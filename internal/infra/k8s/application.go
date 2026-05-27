package k8s

import (
	"context"
	"crypto/sha1"
	"fmt"
	"maps"
	"regexp"
	"strings"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func (r *applicationRepository) CreateApplication(ctx context.Context, app domain.CreateApplicationParams) (*domain.Application, error) {
	branchLabel := normalizeBranchForLabel(app.Spec.Source.Branch)
	cr := &maxicloudv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: projectNamespace(app.Spec.ProjectID),
			Labels: map[string]string{
				labelApplicationID:    app.ID,
				labelApplicationName:  app.Name,
				labelApplicationOwner: app.OwnerID,
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
	if err := r.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return crToApplication(cr), nil
}

func (r *applicationRepository) GetApplication(ctx context.Context, id string) (*domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.List(ctx, &list, client.MatchingLabels{labelApplicationID: id}); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToApplication(&list.Items[0]), nil
}

func (r *applicationRepository) ListApplications(ctx context.Context, projectID string) ([]domain.Application, error) {
	var list maxicloudv1alpha1.ApplicationList
	opts := []client.ListOption{}
	if projectID != "" {
		opts = append(opts, client.InNamespace(projectNamespace(projectID)))
	}
	if err := r.List(ctx, &list, opts...); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	apps := make([]domain.Application, 0, len(list.Items))
	for i := range list.Items {
		apps = append(apps, *crToApplication(&list.Items[i]))
	}
	return apps, nil
}

func (r *applicationRepository) UpdateApplication(ctx context.Context, app domain.Application) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.List(ctx, &list, client.MatchingLabels{labelApplicationID: app.ID}); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("application not found: %s", app.ID)
	}
	cr := list.Items[0]
	cr.Labels[labelApplicationName] = app.Name
	cr.Labels[labelApplicationOwner] = app.OwnerID
	return r.Update(ctx, &cr)
}

func (r *applicationRepository) DeleteApplication(ctx context.Context, id string) error {
	var list maxicloudv1alpha1.ApplicationList
	if err := r.List(ctx, &list, client.MatchingLabels{labelApplicationID: id}); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, &list.Items[0]))
}

func (r *applicationRepository) GetApplicationsByRepo(ctx context.Context, owner, name, branch string) ([]domain.Application, error) {
	matchLabels := client.MatchingLabels{
		labelSourceRepoOwner: owner,
		labelSourceRepoName:  name,
	}
	if branch != "" {
		matchLabels[labelSourceBranch] = normalizeBranchForLabel(branch)
	}
	var list maxicloudv1alpha1.ApplicationList
	if err := r.List(ctx, &list, matchLabels); err != nil {
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
	if err := r.List(ctx, &list); err != nil {
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
	if err := r.List(ctx, &list, client.MatchingLabels{labelApplicationID: originalApplicationID}); err != nil {
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
	if err := r.Get(ctx, key, &existing); err == nil {
		updated := existing.DeepCopy()
		updated.Labels = previewLabelsWithExistingID(existing.Labels[labelApplicationID], desired.Labels)
		updated.Annotations = desired.Annotations
		updated.Spec = desired.Spec
		if err := r.Update(ctx, updated); err != nil {
			return nil, fmt.Errorf("update preview application: %w", err)
		}
		return crToApplication(updated), nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get preview application: %w", err)
	}

	if err := r.Create(ctx, desired); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create preview application: %w", err)
		}
		if err := r.Get(ctx, key, &existing); err != nil {
			return nil, fmt.Errorf("get preview application after already exists: %w", err)
		}
		updated := existing.DeepCopy()
		updated.Labels = previewLabelsWithExistingID(existing.Labels[labelApplicationID], desired.Labels)
		updated.Annotations = desired.Annotations
		updated.Spec = desired.Spec
		if err := r.Update(ctx, updated); err != nil {
			return nil, fmt.Errorf("update preview application after already exists: %w", err)
		}
		return crToApplication(updated), nil
	}
	return crToApplication(desired), nil
}

func buildPreviewApplicationCR(orig maxicloudv1alpha1.Application, namespace, previewName string, prNumber int, id string) *maxicloudv1alpha1.Application {
	newSpec := orig.Spec.DeepCopy()
	if newSpec.Expose != nil {
		root := orig.Annotations[annotationRootDomain]
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
				labelApplicationID:    id,
				labelApplicationName:  previewName,
				labelApplicationOwner: orig.Labels[labelApplicationOwner],
				labelSourceRepoOwner:  orig.Labels[labelSourceRepoOwner],
				labelSourceRepoName:   orig.Labels[labelSourceRepoName],
				labelSourceBranch:     normalizeBranchForLabel(fmt.Sprintf("pr-%d", prNumber)),
			},
			Annotations: map[string]string{
				annotationSourceBranch: fmt.Sprintf("pr-%d", prNumber),
				annotationRootDomain:   orig.Annotations[annotationRootDomain],
			},
		},
		Spec: *newSpec,
	}
}

func previewLabelsWithExistingID(existingID string, desired map[string]string) map[string]string {
	labels := make(map[string]string, len(desired))
	maps.Copy(labels, desired)
	if existingID != "" {
		labels[labelApplicationID] = existingID
	}
	return labels
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

var nonLabelChar = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
var edgeNonAlnum = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)

// Labelには/を使用することができないため正規化する
func normalizeBranchForLabel(branch string) string {
	const (
		maxLabelLen = 63
		hashBytes   = 4
	)

	raw := strings.TrimSpace(branch)
	normalized := nonLabelChar.ReplaceAllString(raw, "-")
	normalized = edgeNonAlnum.ReplaceAllString(normalized, "")
	if normalized == "" {
		normalized = "branch"
	}

	sum := sha1.Sum([]byte(raw))
	suffix := fmt.Sprintf("-%x", sum[:hashBytes])
	maxBaseLen := maxLabelLen - len(suffix)
	maxBaseLen = max(maxBaseLen, 1)

	if len(normalized) > maxBaseLen {
		normalized = normalized[:maxBaseLen]
		normalized = edgeNonAlnum.ReplaceAllString(normalized, "")
	}

	return normalized + suffix
}
