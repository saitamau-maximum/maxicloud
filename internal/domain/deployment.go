package domain

import (
	"context"
	"fmt"
	"time"
)

type DeploymentStatus string

const (
	DeploymentStatusQueued     DeploymentStatus = "QUEUED"
	DeploymentStatusInProgress DeploymentStatus = "IN_PROGRESS"
	DeploymentStatusSucceeded  DeploymentStatus = "SUCCESS"
	DeploymentStatusFailed     DeploymentStatus = "FAILED"
)

type DeploymentSpec struct {
	ApplicationID string
	OwnerUserID   string
	Repo          Repository
	Commit        Commit
	PRNumber      *int // Preview の時のみ
}

func (s DeploymentSpec) IsPreview() bool {
	return s.PRNumber != nil
}

type Deployment struct {
	ID         string
	Spec       DeploymentSpec
	Status     DeploymentStatus
	StartedAt  time.Time
	FinishedAt *time.Time
}

func (d Deployment) Duration() time.Duration {
	if d.FinishedAt == nil {
		return time.Since(d.StartedAt)
	}
	return d.FinishedAt.Sub(d.StartedAt)
}

// StatusがFAILEDかSUCCEEDEDのときFinishedAtは必須、QUEUEDかIN_PROGRESSのときFinishedAtはnilでなければならない
type RecordStatusParams struct {
	ID         string
	Status     DeploymentStatus
	FinishedAt *time.Time
}

func (d RecordStatusParams) Validate() error {
	switch d.Status {
	case DeploymentStatusInProgress, DeploymentStatusQueued:
		if d.FinishedAt != nil {
			return fmt.Errorf("finishedAt must be nil when status is %s", d.Status)
		}
	case DeploymentStatusSucceeded, DeploymentStatusFailed:
		if d.FinishedAt == nil {
			return fmt.Errorf("finishedAt must be set when status is %s", d.Status)
		}
	default:
		return fmt.Errorf("invalid status: %s", d.Status)
	}
	return nil
}

type DeploymentHistoryRepository interface {
	Create(ctx context.Context, deployment Deployment) (string, error)
	Get(ctx context.Context, id string) (*Deployment, error)
	RecordStatus(ctx context.Context, params RecordStatusParams) error
	Delete(ctx context.Context, id string) error
	ListByApplication(ctx context.Context, applicationID string) ([]Deployment, error)
}

type DeployRunRepository interface {
	Create(ctx context.Context, deployment Deployment) (string, error)
	Get(ctx context.Context, id string) (*Deployment, error)
	Delete(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error
	Watch(ctx context.Context, deploymentID string) (<-chan string, <-chan error, error)
}

type DeploymentEventType string

const (
	DeploymentEventTypeProductionRequested DeploymentEventType = "ProductionRequested"
	DeploymentEventTypePreviewRequested    DeploymentEventType = "PreviewRequested"
	DeploymentEventTypePreviewDeleted      DeploymentEventType = "PreviewDeleted"
)

type DeploymentEvent struct {
	Type     DeploymentEventType
	Repo     Repository
	Branch   string
	Commit   Commit
	PRNumber *int
}
