package k8s

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deployRunRepository struct {
	client   client.Client
	streamer *logStreamer
}

var _ domain.DeployRunRepository = (*deployRunRepository)(nil)

func NewDeployRunRepository(c client.Client, streamer *logStreamer) domain.DeployRunRepository {
	return &deployRunRepository{
		client:   c,
		streamer: streamer,
	}
}

func (r *deployRunRepository) Create(ctx context.Context, deployment domain.Deployment) (string, error) {
	spec := deployment.Spec
	var appList maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &appList, meta.SelectByAppID(spec.ApplicationID)); err != nil {
		return "", fmt.Errorf("list applications: %w", err)
	}
	if len(appList.Items) == 0 {
		return "", fmt.Errorf("application not found: %s", spec.ApplicationID)
	}
	namespace := appList.Items[0].Namespace

	cr := &maxicloudv1alpha1.DeployRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.ID,
			Namespace: namespace,
		},
		Spec: maxicloudv1alpha1.DeployRunSpec{
			ApplicationName: appList.Items[0].Name,
			Owner:           spec.Repo.Owner,
			Repo:            spec.Repo.Name,
			SHA:             spec.Commit.SHA,
			PRNumber:        spec.PRNumber,
		},
	}
	meta.DeployRunMeta{
		DeployRunID: deployment.ID,
		AppID:       spec.ApplicationID,
		OwnerUserID: spec.OwnerUserID,
		IsPreview:   spec.IsPreview(),
	}.Apply(&cr.ObjectMeta)
	if err := r.client.Create(ctx, cr); err != nil {
		return "", fmt.Errorf("create deploy run: %w", err)
	}
	return deployment.ID, nil
}

func (r *deployRunRepository) Get(ctx context.Context, id string) (*domain.Deployment, error) {
	var list maxicloudv1alpha1.DeployRunList
	if err := r.client.List(ctx, &list, meta.SelectByDeployRunID(id)); err != nil {
		return nil, fmt.Errorf("list deploy runs: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToDeployment(&list.Items[0]), nil
}

func (r *deployRunRepository) Delete(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error {
	var list maxicloudv1alpha1.DeployRunList
	if err := r.client.List(ctx, &list, meta.SelectDeployRunsByApp(applicationID, isPreview)); err != nil {
		return fmt.Errorf("list deploy runs: %w", err)
	}
	if len(list.Items) <= maxHistory {
		return nil
	}
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].CreationTimestamp.Time.Before(list.Items[j].CreationTimestamp.Time)
	})
	for i := 0; i < len(list.Items)-maxHistory; i++ {
		if err := r.client.Delete(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("delete old deploy run: %w", err)
		}
	}
	return nil
}

func (r *deployRunRepository) Watch(ctx context.Context, deploymentID string) (<-chan string, <-chan error, error) {
	namespace, err := r.resolveDeployRunNamespace(ctx, deploymentID)
	if err != nil {
		return nil, nil, err
	}

	raw, errs, err := r.streamer.StreamLines(ctx, logStreamOptions{
		Namespace:     namespace,
		LabelSelector: fmt.Sprintf("job-name=%s", deploymentID),
	})
	if err != nil {
		return nil, nil, err
	}

	lines := make(chan string)
	go func() {
		defer close(lines)
		for line := range raw {
			select {
			case <-ctx.Done():
				return
			case lines <- sanitizeLog(line):
			}
		}
	}()
	return lines, errs, nil
}

func (r *deployRunRepository) resolveDeployRunNamespace(ctx context.Context, deploymentID string) (string, error) {
	var list maxicloudv1alpha1.DeployRunList
	if err := r.client.List(ctx, &list, meta.SelectByDeployRunID(deploymentID)); err != nil {
		return "", fmt.Errorf("list deploy runs: %w", err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("deploy run not found: %s", deploymentID)
	}
	return list.Items[0].Namespace, nil
}

func crToDeployment(cr *maxicloudv1alpha1.DeployRun) *domain.Deployment {
	var startedAt time.Time
	if cr.Status.StartedAt != nil {
		startedAt = cr.Status.StartedAt.Time
	}
	var finishedAt *time.Time
	if cr.Status.FinishedAt != nil {
		t := cr.Status.FinishedAt.Time
		finishedAt = &t
	}
	m := meta.DeployRunMetaFrom(cr)
	return &domain.Deployment{
		ID: m.DeployRunID,
		Spec: domain.DeploymentSpec{
			ApplicationID: m.AppID,
			OwnerUserID:   m.OwnerUserID,
			Repo: domain.Repository{
				Owner: cr.Spec.Owner,
				Name:  cr.Spec.Repo,
			},
			Commit: domain.Commit{
				SHA: cr.Spec.SHA,
			},
			PRNumber: cr.Spec.PRNumber,
		},
		Status:     phaseToStatus(cr.Status.Phase),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
}

func phaseToStatus(phase maxicloudv1alpha1.DeployRunPhase) domain.DeploymentStatus {
	switch phase {
	case maxicloudv1alpha1.DeployRunPhaseBuilding, maxicloudv1alpha1.DeployRunPhaseDeploying:
		return domain.DeploymentStatusInProgress
	case maxicloudv1alpha1.DeployRunPhaseSucceeded:
		return domain.DeploymentStatusSucceeded
	case maxicloudv1alpha1.DeployRunPhaseFailed:
		return domain.DeploymentStatusFailed
	default:
		return domain.DeploymentStatusQueued
	}
}

var tokenPattern = regexp.MustCompile(`x-access-token:[^@]+@`)

func sanitizeLog(line string) string {
	return tokenPattern.ReplaceAllString(line, "x-access-token:*****@")
}
