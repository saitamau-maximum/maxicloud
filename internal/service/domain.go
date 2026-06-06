package service

import (
	"context"
	"slices"
	"strings"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DomainService interface {
	ListAvailableDomains(ctx context.Context) ([]string, error)
	CheckDomainAvailability(ctx context.Context, d domain.Domain) (bool, error)
}

type domainService struct {
	availableDomains []string
	appRepo          domain.ApplicationRepository
}

func NewDomainService(appRepo domain.ApplicationRepository, availableDomains []string) *domainService {
	return &domainService{
		appRepo:          appRepo,
		availableDomains: normalizeDomains(availableDomains),
	}
}

func (s *domainService) ListAvailableDomains(ctx context.Context) ([]string, error) {
	return append([]string(nil), s.availableDomains...), nil
}

// 現在管理しているアプリケーションがそのドメインを使用していないかどうかを確認する。
func (s *domainService) CheckDomainAvailability(ctx context.Context, d domain.Domain) (bool, error) {
	if !containsDomain(s.availableDomains, d.RootDomain) {
		return false, nil
	}

	isAvailable, err := s.appRepo.ExistsByDomain(ctx, d.FQDN())
	if err != nil {
		return false, err
	}
	return !isAvailable, nil
}

// TODO: Domain層で正規化してもいいかも
func normalizeDomains(domains []string) []string {
	seen := make(map[string]struct{}, len(domains))
	result := make([]string, 0, len(domains))
	for _, d := range domains {
		normalized := strings.TrimSpace(strings.ToLower(d))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func containsDomain(domains []string, rootDomain string) bool {
	normalized := strings.ToLower(strings.TrimSpace(rootDomain))
	return slices.Contains(domains, normalized)
}
