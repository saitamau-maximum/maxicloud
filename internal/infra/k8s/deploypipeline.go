package k8s

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"time"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	"github.com/saitamau-maximum/maxicloud/internal/config"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelPipelineID  = config.LabelPrefix + "pipeline-id"
	labelAppID       = config.LabelPrefix + "app-id"
	labelOwnerUserID = config.LabelPrefix + "owner-user-id"
	labelPreview     = config.LabelPrefix + "preview"
)

type deploymentPipelineRepository struct {
	client    client.Client
	clientset kubernetes.Interface
}

var _ domain.DeploymentPipelineRepository = (*deploymentPipelineRepository)(nil)

func NewDeploymentPipelineRepository(c client.Client, clientset kubernetes.Interface) domain.DeploymentPipelineRepository {
	return &deploymentPipelineRepository{client: c, clientset: clientset}
}

func (r *deploymentPipelineRepository) WatchBuildLogs(ctx context.Context, deploymentID string) (io.ReadCloser, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelPipelineID: deploymentID}); err != nil {
		return nil, fmt.Errorf("list deployment pipelines: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("pipeline not found: %s", deploymentID)
	}
	namespace := list.Items[0].Namespace

	// Podが起動するまで待機
	var podName string
	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	for {
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("timed out waiting build pod for deployment %s: %w", deploymentID, waitCtx.Err())
		default:
		}
		pods, err := r.clientset.CoreV1().Pods(namespace).List(waitCtx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", deploymentID),
		})
		if err == nil && len(pods.Items) > 0 {
			podName = pods.Items[0].Name
			break
		}
		time.Sleep(1 * time.Second)
	}

	streamDeadline := time.Now().Add(45 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if time.Now().After(streamDeadline) {
			return nil, fmt.Errorf("timed out opening build logs stream for deployment %s", deploymentID)
		}

		req := r.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Follow: true,
		})
		stream, err := req.Stream(ctx)
		if err == nil {
			return maskLogs(stream), nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (r *deploymentPipelineRepository) CreatePipeline(ctx context.Context, pipeline domain.DeploymentPipeline) (string, error) {
	var appList maxicloudv1alpha1.ApplicationList
	if err := r.client.List(ctx, &appList, client.MatchingLabels{labelAppID: pipeline.ApplicationID}); err != nil {
		return "", fmt.Errorf("list applications: %w", err)
	}
	if len(appList.Items) == 0 {
		return "", fmt.Errorf("application not found: %s", pipeline.ApplicationID)
	}
	namespace := appList.Items[0].Namespace

	cr := &maxicloudv1alpha1.DeploymentPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.ID,
			Namespace: namespace,
			Labels: map[string]string{
				labelPipelineID:  pipeline.ID,
				labelAppID:       pipeline.ApplicationID,
				labelOwnerUserID: pipeline.OwnerUserID,
				labelPreview:     strconv.FormatBool(pipeline.PRNumber != nil),
			},
		},
		Spec: maxicloudv1alpha1.DeploymentPipelineSpec{
			ApplicationName: appList.Items[0].Name,
			Owner:           pipeline.Repo.Owner,
			Repo:            pipeline.Repo.Name,
			SHA:             pipeline.Commit.SHA,
			PRNumber:        pipeline.PRNumber,
		},
	}
	if err := r.client.Create(ctx, cr); err != nil {
		return "", fmt.Errorf("create deployment pipeline: %w", err)
	}
	return pipeline.ID, nil
}

func (r *deploymentPipelineRepository) GetPipeline(ctx context.Context, id string) (*domain.DeploymentPipeline, error) {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelPipelineID: id}); err != nil {
		return nil, fmt.Errorf("list deployment pipelines: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return crToPipeline(&list.Items[0]), nil
}

func (r *deploymentPipelineRepository) DeleteOldPipelines(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelAppID: applicationID, labelPreview: strconv.FormatBool(isPreview)}); err != nil {
		return fmt.Errorf("list deployment pipelines: %w", err)
	}
	items := filterByPreview(list, isPreview)
	if len(items) <= maxHistory {
		return nil
	}
	// 古い順にソート
	sort.Slice(items, func(i, j int) bool {
		// StartedAtはnilの可能性があるので、CreationTimestampで比較する
		return items[i].CreationTimestamp.Time.Before(items[j].CreationTimestamp.Time)
	})
	// maxHistory件を残して古いものから削除
	for i := 0; i < len(items)-maxHistory; i++ {
		if err := r.client.Delete(ctx, &items[i]); err != nil {
			return fmt.Errorf("delete old deployment pipeline: %w", err)
		}
	}
	return nil
}

func (r *deploymentPipelineRepository) DeletePipelinesByPR(ctx context.Context, applicationID string, prNumber int) error {
	var list maxicloudv1alpha1.DeploymentPipelineList
	if err := r.client.List(ctx, &list, client.MatchingLabels{labelAppID: applicationID, labelPreview: strconv.FormatBool(true)}); err != nil {
		return fmt.Errorf("list deployment pipelines: %w", err)
	}
	var errs []error
	for i := range list.Items {
		p := &list.Items[i]
		if p.Spec.PRNumber != nil && *p.Spec.PRNumber == prNumber {
			if err := client.IgnoreNotFound(r.client.Delete(ctx, p)); err != nil {
				errs = append(errs, fmt.Errorf("delete pipeline %s: %w", p.Name, err))
			}
		}
	}
	return errors.Join(errs...)
}

func filterByPreview(list maxicloudv1alpha1.DeploymentPipelineList, isPreview bool) []maxicloudv1alpha1.DeploymentPipeline {
	var result []maxicloudv1alpha1.DeploymentPipeline
	for _, p := range list.Items {
		if isPreview && p.Spec.PRNumber != nil {
			result = append(result, p)
		}
		if !isPreview && p.Spec.PRNumber == nil {
			result = append(result, p)
		}
	}
	return result
}

func crToPipeline(cr *maxicloudv1alpha1.DeploymentPipeline) *domain.DeploymentPipeline {
	var startedAt time.Time
	if cr.Status.StartedAt != nil {
		startedAt = cr.Status.StartedAt.Time
	}
	var finishedAt *time.Time
	if cr.Status.FinishedAt != nil {
		t := cr.Status.FinishedAt.Time
		finishedAt = &t
	}
	return &domain.DeploymentPipeline{
		ID:            cr.Labels[labelPipelineID],
		ApplicationID: cr.Labels[labelAppID],
		OwnerUserID:   cr.Labels[labelOwnerUserID],
		Repo: domain.Repository{
			Owner: cr.Spec.Owner,
			Name:  cr.Spec.Repo,
		},
		Commit: domain.Commit{
			SHA: cr.Spec.SHA,
		},
		PRNumber:   cr.Spec.PRNumber,
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

func maskLogs(logStream io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = logStream.Close() }()
		defer func() { _ = pw.Close() }()

		reader := bufio.NewReader(logStream)
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				masked := tokenPattern.ReplaceAllString(line, "x-access-token:*****@")
				if _, writeErr := pw.Write([]byte(masked)); writeErr != nil {
					return
				}
			}
			if err == nil {
				continue
			}
			if err == io.EOF {
				return
			}
			_ = pw.CloseWithError(err)
			return
		}
	}()
	return pr
}
