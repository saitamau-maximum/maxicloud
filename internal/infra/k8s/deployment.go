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

type workflowRepository struct {
	client   client.Client
	streamer *logStreamer
}

var _ domain.DeploymentWorkflowRepository = (*workflowRepository)(nil)

func NewDeployRepository(c client.Client, streamer *logStreamer) domain.DeploymentWorkflowRepository {
	return &workflowRepository{
		client:   c,
		streamer: streamer,
	}
}

func (r *workflowRepository) Create(ctx context.Context, deployment domain.Deployment) (string, error) {
	spec := deployment.Spec
	var appList maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &appList, meta.SelectByAppID(spec.ApplicationID)); err != nil {
		return "", fmt.Errorf("list applications: %w", err)
	}
	if len(appList.Items) == 0 {
		return "", fmt.Errorf("application not found: %s", spec.ApplicationID)
	}
	namespace := appList.Items[0].Namespace

	cr := &maxicloudv1alpha1.DeploymentPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.ID,
			Namespace: namespace,
		},
		Spec: maxicloudv1alpha1.DeploymentPipelineSpec{
			ApplicationName: appList.Items[0].Name,
			Owner:           spec.Repo.Owner,
			Repo:            spec.Repo.Name,
			SHA:             spec.Commit.SHA,
			PRNumber:        spec.PRNumber,
		},
	}
	meta.WorkflowMeta{
		WorkflowID:  deployment.ID,
		AppID:       spec.ApplicationID,
		OwnerUserID: spec.OwnerUserID,
		IsPreview:   spec.PRNumber != nil,
	}.Apply(&cr.ObjectMeta)
	if err := r.client.Create(ctx, cr); err != nil {
		return "", fmt.Errorf("create deployment workflow: %w", err)
	}
	return deployment.ID, nil
}

func (r *workflowRepository) Get(ctx context.Context, id string) (*domain.Deployment, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, meta.SelectByWorkflowID(id)); err != nil {
		return nil, fmt.Errorf("list deployment workflows: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToDeployment(&list.Items[0]), nil
}

func (r *workflowRepository) Delete(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, meta.SelectWorkflowsByApp(applicationID, isPreview)); err != nil {
		return fmt.Errorf("list deployment workflows: %w", err)
	}
	if len(list.Items) <= maxHistory {
		return nil
	}
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].CreationTimestamp.Time.Before(list.Items[j].CreationTimestamp.Time)
	})
	for i := 0; i < len(list.Items)-maxHistory; i++ {
		if err := r.client.Delete(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("delete old deployment workflow: %w", err)
		}
	}
	return nil
}

func (r *workflowRepository) Watch(ctx context.Context, deploymentID string) (<-chan string, <-chan error, error) {
	namespace, err := r.resolveWorkflowNamespace(ctx, deploymentID)
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

func (r *workflowRepository) resolveWorkflowNamespace(ctx context.Context, deploymentID string) (string, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, meta.SelectByWorkflowID(deploymentID)); err != nil {
		return "", fmt.Errorf("list deployment workflows: %w", err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("deployment workflow not found: %s", deploymentID)
	}
	return list.Items[0].Namespace, nil
}

func crToDeployment(cr *maxicloudv1alpha1.DeploymentPipeline) *domain.Deployment {
	var startedAt time.Time
	if cr.Status.StartedAt != nil {
		startedAt = cr.Status.StartedAt.Time
	}
	var finishedAt *time.Time
	if cr.Status.FinishedAt != nil {
		t := cr.Status.FinishedAt.Time
		finishedAt = &t
	}
	m := meta.WorkflowMetaFrom(cr)
	return &domain.Deployment{
		ID: m.WorkflowID,
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

func phaseToStatus(phase maxicloudv1alpha1.DeploymentPipelinePhase) domain.DeploymentStatus {
	switch phase {
	case maxicloudv1alpha1.DeploymentPipelinePhaseBuilding, maxicloudv1alpha1.DeploymentPipelinePhaseDeploying:
		return domain.DeploymentStatusInProgress
	case maxicloudv1alpha1.DeploymentPipelinePhaseSucceeded:
		return domain.DeploymentStatusSucceeded
	case maxicloudv1alpha1.DeploymentPipelinePhaseFailed:
		return domain.DeploymentStatusFailed
	default:
		return domain.DeploymentStatusQueued
	}
}

var tokenPattern = regexp.MustCompile(`x-access-token:[^@]+@`)

func sanitizeLog(line string) string {
	return tokenPattern.ReplaceAllString(line, "x-access-token:*****@")
}
