package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	MinPort = 1
	MaxPort = 65535
)

// Applicationのリソースの状態を表す構造体
type Application struct {
	ID      string
	Name    string
	OwnerID string
	Spec    ApplicationSpec

	Condition ApplicationCondition
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ApplicationStatus string

const (
	// 正常に稼働している状態
	ApplicationStatusRunning ApplicationStatus = "Running"
	// 明示的に停止された状態
	ApplicationStatusStopped ApplicationStatus = "Stopped"
	// デプロイ失敗などでアプリケーションが利用できない状態
	ApplicationStatusUnavailable ApplicationStatus = "Unavailable"
)

type ApplicationCondition struct {
	Status ApplicationStatus
	Domain *Domain
}

type ApplicationSource struct {
	Repo   Repository
	Branch string
}

type AccessMode string

const (
	AccessModePublic      AccessMode = "Public"
	AccessModePrivate     AccessMode = "Private"
	AccessModeMembersOnly AccessMode = "MembersOnly"
)

type Domain struct {
	Subdomain  string
	RootDomain string
}

// FQDNとルートドメインを渡してDomainを生成する
func NewDomainByFQDN(fqdn string, rootDomain string) (Domain, error) {
	if fqdn == "" {
		return Domain{}, ValidationError{Message: "fqdn is required"}
	}
	if rootDomain == "" {
		return Domain{}, ValidationError{Message: "root domain is required"}
	}
	if !strings.HasSuffix(fqdn, rootDomain) {
		return Domain{}, ValidationError{Message: "fqdn must end with the root domain"}
	}
	subdomain := strings.TrimSuffix(fqdn, "."+rootDomain)
	if subdomain == "" {
		return Domain{}, ValidationError{Message: "subdomain is required in fqdn"}
	}
	return Domain{
		Subdomain:  subdomain,
		RootDomain: rootDomain,
	}, nil
}

func (d Domain) Validate() error {
	if d.Subdomain == "" {
		return ValidationError{Message: "subdomain is required for public access mode"}
	}
	if d.RootDomain == "" {
		return ValidationError{Message: "root domain is required for public access mode"}
	}
	return nil
}

// FQDN returns the full domain name in the format "subdomain.rootdomain"
func (d Domain) FQDN() string {
	return d.Subdomain + "." + d.RootDomain
}

func (d Domain) URL(scheme string) string {
	return scheme + "://" + d.FQDN()
}

func (d Domain) Preview(prNumber int) Domain {
	return Domain{
		Subdomain:  fmt.Sprintf("%s-pr%d", d.Subdomain, prNumber),
		RootDomain: d.RootDomain,
	}
}

func PreviewName(appName string, prNumber int) string {
	return fmt.Sprintf("%s-pr-%d", appName, prNumber)
}

func PreviewBranch(prNumber int) string {
	return fmt.Sprintf("pr-%d", prNumber)
}

type KeyValue struct {
	Key   string
	Value string
}

func (k KeyValue) Validate() error {
	if k.Key == "" {
		return ValidationError{Message: "key is required"}
	}
	if k.Value == "" {
		return ValidationError{Message: "value is required"}
	}
	return nil
}

// Applicationの設定を表す構造体
type ApplicationSpec struct {
	ProjectID   string
	Source      ApplicationSource
	BuildConfig BuildConfig
	AccessMode  AccessMode
	Domain      *Domain
	Port        int32
	Env         []KeyValue
	Secrets     []KeyValue
}

func (s ApplicationSpec) Validate() error {
	if s.ProjectID == "" {
		return ValidationError{Message: "project_id is required"}
	}
	if s.Source.Branch == "" {
		return ValidationError{Message: "source.branch is required"}
	}
	if s.Port < MinPort || s.Port > MaxPort {
		return ValidationError{Message: fmt.Sprintf("port must be between %d and %d", MinPort, MaxPort)}
	}
	for _, kv := range s.Env {
		if err := kv.Validate(); err != nil {
			return fmt.Errorf("invalid environment variable: %w", err)
		}
	}
	for _, kv := range s.Secrets {
		if err := kv.Validate(); err != nil {
			return fmt.Errorf("invalid secret: %w", err)
		}
	}
	switch s.AccessMode {
	case AccessModePublic, AccessModeMembersOnly:
		if s.Domain == nil {
			return ValidationError{Message: "domain is required"}
		}
	case AccessModePrivate:
		if s.Domain != nil {
			return ValidationError{Message: "domain must be nil for private access mode"}
		}
	default:
		return ValidationError{Message: "invalid access mode"}
	}
	return nil
}

type CreateApplicationParams struct {
	ID      string
	Name    string
	OwnerID string
	Spec    ApplicationSpec
}

type UpdateApplicationParams struct {
	ID      string
	Name    string
	OwnerID string
	Spec    ApplicationSpec
}

type CreatePreviewApplicationParams struct {
	ID                    string
	ProjectID             string
	Name                  string
	OwnerID               string
	OriginalApplicationID string
	Spec                  ApplicationSpec
}

type ApplicationRepository interface {
	Create(ctx context.Context, params CreateApplicationParams) (*Application, error)
	Get(ctx context.Context, id string) (*Application, error)
	List(ctx context.Context, projectID string) ([]Application, error)
	Update(ctx context.Context, params UpdateApplicationParams) error
	Delete(ctx context.Context, id string) error
	ListByRepo(ctx context.Context, owner, name, branch string) ([]Application, error)
	ExistsByDomain(ctx context.Context, domain string) (bool, error)
	CreatePreview(ctx context.Context, params CreatePreviewApplicationParams) (*Application, error)
}
