package domain

import (
	"context"
	"fmt"
	"io"
	"time"
)

type DeploymentStatus string

const (
	DeploymentStatusQueued     DeploymentStatus = "QUEUED"
	DeploymentStatusInProgress DeploymentStatus = "IN_PROGRESS"
	DeploymentStatusSucceeded  DeploymentStatus = "SUCCESS"
	DeploymentStatusFailed     DeploymentStatus = "FAILED"
)

type Deployment struct {
	ID            string
	ApplicationID string
	OwnerUserID   string
	Repo          Repository
	Commit        Commit
	PRNumber      *int // Previewの時に使う
	Status        DeploymentStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
}

func (d Deployment) Duration() time.Duration {
	if d.FinishedAt == nil {
		return time.Since(d.StartedAt)
	}
	return d.FinishedAt.Sub(d.StartedAt)
}

// StatusがFAILEDかSUCCEEDEDのときFinishedAtは必須、QUEUEDかIN_PROGRESSのときFinishedAtはnilでなければならない
type RecordDeploymentStatusParams struct {
	ID         string
	Status     DeploymentStatus
	FinishedAt *time.Time
}

func (d RecordDeploymentStatusParams) Validate() error {
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

type DeploymentRepository interface {
	CreateDeployment(ctx context.Context, deployment Deployment) (string, error)
	GetDeployment(ctx context.Context, id string) (*Deployment, error)
	UpdateDeployment(ctx context.Context, deployment Deployment) error
	RecordDeploymentStatus(ctx context.Context, params RecordDeploymentStatusParams) error
	DeleteDeployment(ctx context.Context, id string) error
	ListDeploymentsByApplication(ctx context.Context, applicationID string) ([]Deployment, error)
}

type DeploymentPipeline struct {
	ID            string
	ApplicationID string
	OwnerUserID   string
	Repo          Repository
	Commit        Commit
	PRNumber      *int // Previewの時に使う
	Status        DeploymentStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
}

type DeploymentPipelineRepository interface {
	CreatePipeline(ctx context.Context, pipeline DeploymentPipeline) (string, error)
	GetPipeline(ctx context.Context, id string) (*DeploymentPipeline, error)
	DeleteOldPipelines(ctx context.Context, applicationID string, maxHistory int, isPreview bool) error
	WatchBuildLogs(ctx context.Context, deploymentID string) (io.ReadCloser, error)
	DeletePipelinesByPR(ctx context.Context, applicationID string, prNumber int) error
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
