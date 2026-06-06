package deployment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DeploymentStatusChangedEvent struct {
	Status         domain.DeploymentStatus
	ElapsedSeconds int64
	FinishedAt     *time.Time
}

type DeploymentLogChunkEvent struct {
	Line string
}

type DeploymentWatchEvent interface {
	isDeploymentWatchEvent()
}

func (DeploymentStatusChangedEvent) isDeploymentWatchEvent() {}
func (DeploymentLogChunkEvent) isDeploymentWatchEvent()      {}

type Watcher interface {
	WatchDeployment(ctx context.Context, deploymentID string) (<-chan DeploymentWatchEvent, error)
}

type watcher struct {
	history   History
	deployRun domain.DeployRunRepository
}

func NewWatcher(history History, deployRun domain.DeployRunRepository) Watcher {
	return &watcher{
		history:   history,
		deployRun: deployRun,
	}
}

func (w *watcher) WatchDeployment(ctx context.Context, deploymentID string) (<-chan DeploymentWatchEvent, error) {
	deployment, err := w.history.Get(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, domain.ValidationError{Message: "deployment not found"}
	}

	ch := make(chan DeploymentWatchEvent, 10)
	go func() {
		var wg sync.WaitGroup
		defer func() {
			wg.Wait()
			close(ch)
		}()

		if current, err := w.deployRun.Get(ctx, deploymentID); err == nil && current != nil {
			deployment.Status = current.Status
			deployment.FinishedAt = current.FinishedAt
			if !current.StartedAt.IsZero() {
				deployment.StartedAt = current.StartedAt
			}
		}
		lastStatus := deployment.Status
		ch <- DeploymentStatusChangedEvent{
			Status:         lastStatus,
			ElapsedSeconds: int64(deployment.Duration().Seconds()),
			FinishedAt:     deployment.FinishedAt,
		}

		wg.Go(func() {
			w.watchBuildLogStream(ctx, deploymentID, ch)
		})

		w.watchDeploymentStatusLoop(ctx, deploymentID, lastStatus, ch)
	}()
	return ch, nil
}

func (w *watcher) watchBuildLogStream(ctx context.Context, deploymentID string, ch chan<- DeploymentWatchEvent) {
	lines, errs, err := w.deployRun.Watch(ctx, deploymentID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		sendDeploymentLogChunk(ctx, ch, "failed to retrieve logs")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				if streamErr, ok := <-errs; ok && streamErr != nil {
					sendDeploymentLogChunk(ctx, ch, "failed to retrieve logs")
				}
				return
			}
			sendDeploymentLogChunk(ctx, ch, line)
		}
	}
}

func sendDeploymentLogChunk(ctx context.Context, ch chan<- DeploymentWatchEvent, line string) {
	select {
	case <-ctx.Done():
	case ch <- DeploymentLogChunkEvent{Line: line}:
	}
}

func (w *watcher) watchDeploymentStatusLoop(
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
			current, err := w.deployRun.Get(ctx, deploymentID)
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

func isTerminalStatus(status domain.DeploymentStatus) bool {
	return status == domain.DeploymentStatusSucceeded || status == domain.DeploymentStatusFailed
}
