package k8s

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"time"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelWorkflowID  = config.LabelPrefix + "workflow-id"
	labelAppID       = config.LabelPrefix + "app-id"
	labelOwnerUserID = config.LabelPrefix + "owner-user-id"
	labelPreview     = config.LabelPrefix + "preview"
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
	if err := r.client.List(ctx, &appList, client.MatchingLabels{labelAppID: spec.ApplicationID}); err != nil {
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
			Labels: map[string]string{
				labelWorkflowID:  deployment.ID,
				labelAppID:       spec.ApplicationID,
				labelOwnerUserID: spec.OwnerUserID,
				labelPreview:     strconv.FormatBool(spec.PRNumber != nil),
			},
		},
		Spec: maxicloudv1alpha1.DeploymentPipelineSpec{
			ApplicationName: appList.Items[0].Name,
			Owner:           spec.Repo.Owner,
			Repo:            spec.Repo.Name,
			SHA:             spec.Commit.SHA,
			PRNumber:        spec.PRNumber,
		},
	}
	if err := r.client.Create(ctx, cr); err != nil {
		return "", fmt.Errorf("create deployment workflow: %w", err)
	}
	return deployment.ID, nil
}

func (r *workflowRepository) Get(ctx context.Context, id string) (*domain.Deployment, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelWorkflowID: id}); err != nil {
		return nil, fmt.Errorf("list deployment workflows: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToDeployment(&list.Items[0]), nil
}

func (r *workflowRepository) Delete(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelAppID: applicationID, labelPreview: strconv.FormatBool(isPreview)}); err != nil {
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

func (r *workflowRepository) Watch(ctx context.Context, deploymentID string) (io.ReadCloser, error) {
	namespace, err := r.resolveWorkflowNamespace(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	raw, err := r.streamer.Stream(ctx, logStreamOptions{
		Namespace:     namespace,
		LabelSelector: fmt.Sprintf("job-name=%s", deploymentID),
	})
	if err != nil {
		return nil, err
	}
	return r.streamer.EachLine(raw, sanitizeLog), nil
}

func (r *workflowRepository) resolveWorkflowNamespace(ctx context.Context, deploymentID string) (string, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelWorkflowID: deploymentID}); err != nil {
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
	return &domain.Deployment{
		ID: cr.Labels[labelWorkflowID],
		Spec: domain.DeploymentSpec{
			ApplicationID: cr.Labels[labelAppID],
			OwnerUserID:   cr.Labels[labelOwnerUserID],
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
