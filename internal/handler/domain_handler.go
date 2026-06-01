package handler

import (
	"context"

	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service"
)

type DomainHandler struct {
	maxicloudv1connect.UnimplementedDomainServiceHandler
	service service.DomainService
}

func NewDomainHandler(domainService service.DomainService) *DomainHandler {
	return &DomainHandler{
		service: domainService,
	}
}

func (h *DomainHandler) ListAvailableDomains(ctx context.Context, req *v1.ListAvailableDomainsRequest) (*v1.ListAvailableDomainsResponse, error) {
	domains, err := h.service.ListAvailableDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ListAvailableDomainsResponse{Domains: domains}, nil
}

func (h *DomainHandler) CheckDomainAvailability(ctx context.Context, req *v1.CheckDomainAvailabilityRequest) (*v1.CheckDomainAvailabilityResponse, error) {
	if req.Domain == nil {
		return &v1.CheckDomainAvailabilityResponse{Available: false}, nil
	}
	isAvailable, err := h.service.CheckDomainAvailability(ctx, domain.Domain{
		Subdomain:  req.Domain.Subdomain,
		RootDomain: req.Domain.RootDomain,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CheckDomainAvailabilityResponse{Available: isAvailable}, nil
}
