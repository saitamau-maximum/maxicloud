package usecase

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

const (
	MaxPreviewPipelineHistory    = 1
	MaxProductionPipelineHistory = 3
)

type deploymentService struct {
	deployRepo   domain.DeploymentRepository
	pipelineRepo domain.DeploymentPipelineRepository
	appRepo      domain.ApplicationRepository
}

type DeploymentStatusChangedEvent struct {
	Status         domain.DeploymentStatus
	ElapsedSeconds int64
	FinishedAt     *time.Time
}

type DeploymentLogChunkEvent struct {
	Lines []string
}

type DeploymentWatchEvent interface {
	isDeploymentWatchEvent()
}

func (DeploymentStatusChangedEvent) isDeploymentWatchEvent() {}
func (DeploymentLogChunkEvent) isDeploymentWatchEvent()      {}

type DeploymentService interface {
	CreateDeployment(ctx context.Context, params CreateDeploymentParams) (string, error)
	RetryDeployment(ctx context.Context, deploymentID string) (*domain.Deployment, error)
	GetDeployment(ctx context.Context, deploymentID string) (*domain.Deployment, error)
	ListDeployments(ctx context.Context, applicationID string) ([]domain.Deployment, error)
	HandleGitHubEvent(ctx context.Context, event domain.DeploymentEvent) error
	WatchDeployment(ctx context.Context, deploymentID string) (<-chan DeploymentWatchEvent, error)
}

func NewDeploymentService(deployRepo domain.DeploymentRepository, pipelineRepo domain.DeploymentPipelineRepository, appRepo domain.ApplicationRepository) *deploymentService {
	return &deploymentService{
		deployRepo:   deployRepo,
		pipelineRepo: pipelineRepo,
		appRepo:      appRepo,
	}
}

type CreateDeploymentParams struct {
	ApplicationID string
	OwnerUserID   string
	Repo          domain.Repository
	Commit        domain.Commit
	PRNumber      *int
}

func (s *deploymentService) CreateDeployment(ctx context.Context, params CreateDeploymentParams) (string, error) {
	deployID, err := s.deployRepo.CreateDeployment(ctx, domain.Deployment{
		ID:            uuid.New().String(),
		ApplicationID: params.ApplicationID,
		OwnerUserID:   params.OwnerUserID,
		Repo:          params.Repo,
		Commit:        params.Commit,
		PRNumber:      params.PRNumber,
		Status:        domain.DeploymentStatusQueued,
		StartedAt:     time.Now(),
	})
	if err != nil {
		return "", err
	}

	if _, err := s.pipelineRepo.CreatePipeline(ctx, domain.DeploymentPipeline{
		ID:            deployID,
		ApplicationID: params.ApplicationID,
		OwnerUserID:   params.OwnerUserID,
		Repo:          params.Repo,
		Commit:        params.Commit,
		PRNumber:      params.PRNumber,
		Status:        domain.DeploymentStatusQueued,
		StartedAt:     time.Now(),
	}); err != nil {
		return "", fmt.Errorf("create deployment pipeline: %w", err)
	}

	// PRNumberがある場合はPreview環境
	isPreview := params.PRNumber != nil
	maxHistory := MaxProductionPipelineHistory
	if isPreview {
		maxHistory = MaxPreviewPipelineHistory
	}

	err = s.pipelineRepo.DeleteOldPipelines(ctx, params.ApplicationID, maxHistory, isPreview)
	if err != nil {
		return "", fmt.Errorf("failed to delete old pipelines: %w", err)
	}

	return deployID, nil
}

func (s *deploymentService) RetryDeployment(ctx context.Context, deploymentID string) (*domain.Deployment, error) {
	deploy, err := s.deployRepo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	if deploy == nil {
		return nil, domain.ValidationError{Message: "deployment not found"}
	}

	newDeployID, err := s.CreateDeployment(ctx, CreateDeploymentParams{
		ApplicationID: deploy.ApplicationID,
		OwnerUserID:   deploy.OwnerUserID,
		Repo:          deploy.Repo,
		Commit:        deploy.Commit,
		PRNumber:      deploy.PRNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}
	return s.deployRepo.GetDeployment(ctx, newDeployID)
}

func (s *deploymentService) GetDeployment(ctx context.Context, deploymentID string) (*domain.Deployment, error) {
	deploy, err := s.deployRepo.GetDeployment(ctx, deploymentID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return deploy, nil
}

func (s *deploymentService) ListDeployments(ctx context.Context, applicationID string) ([]domain.Deployment, error) {
	if applicationID == "" {
		return nil, domain.ValidationError{Message: "application_id is required"}
	}
	deploys, err := s.deployRepo.ListDeploymentsByApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return deploys, nil
}

func (s *deploymentService) HandleGitHubEvent(ctx context.Context, event domain.DeploymentEvent) error {
	switch event.Type {
	case domain.DeploymentEventTypeProductionRequested:
		return s.handleRepoDeploymentEvent(ctx, event, nil)
	case domain.DeploymentEventTypePreviewRequested:
		if event.PRNumber == nil {
			return domain.ValidationError{Message: "PR number is required for preview deployment"}
		}
		return s.handleRepoDeploymentEvent(ctx, event, event.PRNumber)
	case domain.DeploymentEventTypePreviewDeleted:
		if event.PRNumber == nil {
			return domain.ValidationError{Message: "PR number is required for preview deletion"}
		}
		apps, err := s.appRepo.GetApplicationsByRepo(ctx, event.Repo.Owner, event.Repo.Name, fmt.Sprintf("pr-%d", *event.PRNumber))
		if err != nil {
			return fmt.Errorf("get preview applications: %w", err)
		}
		for _, app := range apps {
			if err := s.pipelineRepo.DeletePipelinesByPR(ctx, app.ID, *event.PRNumber); err != nil {
				return fmt.Errorf("delete pipelines for preview app %s: %w", app.ID, err)
			}

			deps, err := s.deployRepo.ListDeploymentsByApplication(ctx, app.ID)
			if err != nil {
				return fmt.Errorf("list deployments for preview app %s: %w", app.ID, err)
			}
			for _, d := range deps {
				if err := s.deployRepo.DeleteDeployment(ctx, d.ID); err != nil {
					return fmt.Errorf("delete deployment %s: %w", d.ID, err)
				}
			}

			if err := s.appRepo.DeleteApplication(ctx, app.ID); err != nil {
				return fmt.Errorf("delete preview application %s: %w", app.ID, err)
			}
		}
		return nil
	default:
		return domain.ValidationError{Message: fmt.Sprintf("unsupported deployment event type: %s", event.Type)}
	}
}

func (s *deploymentService) handleRepoDeploymentEvent(ctx context.Context, event domain.DeploymentEvent, prNumber *int) error {
	apps, err := s.appRepo.GetApplicationsByRepo(ctx, event.Repo.Owner, event.Repo.Name, event.Branch)
	if err != nil {
		return fmt.Errorf("get applications by repo: %w", err)
	}
	for _, app := range apps {
		if prNumber == nil {
			_, err := s.CreateDeployment(ctx, CreateDeploymentParams{
				ApplicationID: app.ID,
				OwnerUserID:   app.OwnerID,
				Repo:          event.Repo,
				Commit:        event.Commit,
				PRNumber:      prNumber,
			})
			if err != nil {
				return fmt.Errorf("create deployment for application %s: %w", app.ID, err)
			}
		} else {

			previewAppID := uuid.New().String()
			previewApp, err := s.appRepo.CreatePreviewApplication(ctx, app.ID, *prNumber, previewAppID)
			if err != nil {
				return fmt.Errorf("create preview application for %s: %w", app.ID, err)
			}
			_, err = s.CreateDeployment(ctx, CreateDeploymentParams{
				ApplicationID: previewApp.ID,
				OwnerUserID:   previewApp.OwnerID,
				Repo:          event.Repo,
				Commit:        event.Commit,
				PRNumber:      prNumber,
			})
			if err != nil {
				return fmt.Errorf("create deployment for preview application %s: %w", previewApp.ID, err)
			}
		}
	}
	return nil
}

// TODO: このコードやばすぎるので修正する
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func isTerminalStatus(status domain.DeploymentStatus) bool {
	return status == domain.DeploymentStatusSucceeded || status == domain.DeploymentStatusFailed
}

func (s *deploymentService) WatchDeployment(ctx context.Context, deploymentID string) (<-chan DeploymentWatchEvent, error) {
	deploy, err := s.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if deploy == nil {
		return nil, domain.ValidationError{Message: "deployment not found"}
	}

	ch := make(chan DeploymentWatchEvent, 10)
	go func() {
		var wg sync.WaitGroup
		defer func() {
			wg.Wait()
			close(ch)
		}()

		if current, err := s.getDeploymentTemporarily(ctx, deploymentID); err == nil && current != nil {
			deploy = current
		}
		lastStatus := deploy.Status
		ch <- DeploymentStatusChangedEvent{
			Status:         lastStatus,
			ElapsedSeconds: int64(deploy.Duration().Seconds()),
			FinishedAt:     deploy.FinishedAt,
		}

		wg.Go(func() {
			s.watchBuildLogStream(ctx, deploymentID, ch)
		})

		// すでに完了済みでも、上のログ取得 goroutine は実行させる
		if isTerminalStatus(lastStatus) {
			return
		}

		s.watchDeploymentStatusLoop(ctx, deploymentID, lastStatus, ch)
	}()
	return ch, nil
}

func (s *deploymentService) watchBuildLogStream(ctx context.Context, deploymentID string, ch chan<- DeploymentWatchEvent) {
	stream, err := s.pipelineRepo.WatchBuildLogs(ctx, deploymentID)
	if err != nil {
		sendDeploymentLogChunk(ctx, ch, "failed to retrieve logs")
		return
	}
	defer func() { _ = stream.Close() }()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		sendDeploymentLogChunk(ctx, ch, scanner.Text())
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		sendDeploymentLogChunk(ctx, ch, "failed to retrieve logs")
	}
}

func sendDeploymentLogChunk(ctx context.Context, ch chan<- DeploymentWatchEvent, line string) {
	select {
	case <-ctx.Done():
	case ch <- DeploymentLogChunkEvent{Lines: []string{line}}:
	}
}

func (s *deploymentService) watchDeploymentStatusLoop(
	ctx context.Context,
	deploymentID string,
	lastStatus domain.DeploymentStatus,
	ch chan<- DeploymentWatchEvent,
) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// current, err := s.GetDeployment(ctx, deploymentID)
			current, err := s.getDeploymentTemporarily(ctx, deploymentID)
			if err != nil {
				sendDeploymentLogChunk(ctx, ch, fmt.Sprintf("[status error] %v", err))
				continue
			}
			if current == nil {
				continue
			}
			if current.Status != lastStatus {
				lastStatus = current.Status
				ch <- DeploymentStatusChangedEvent{
					Status:         lastStatus,
					ElapsedSeconds: int64(current.Duration().Seconds()),
					FinishedAt:     current.FinishedAt,
				}
			}
			if isTerminalStatus(lastStatus) {
				return
			}
		}
	}
}

// 現状DBをメモリでMockしていて、Gatewayの更新が別スレッドで反映されないため、PipelineからDeploymentを取得する
// TODO: データベースに移行したらこの関数を削除する
func (s *deploymentService) getDeploymentTemporarily(ctx context.Context, deploymentID string) (*domain.Deployment, error) {
	pipeline, err := s.pipelineRepo.GetPipeline(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, nil
	}

	deploy, err := s.deployRepo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if deploy == nil {
		return nil, nil
	}

	deploy.Status = pipeline.Status
	deploy.FinishedAt = pipeline.FinishedAt
	if !pipeline.StartedAt.IsZero() {
		deploy.StartedAt = pipeline.StartedAt
	}
	return deploy, nil
}
