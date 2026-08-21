//go:build integration

package testkit

import (
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/config"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

const DefaultInternalToken = "integration-internal-token"

type ServerOptions struct {
	Pool                 *pgxpool.Pool
	InternalServiceToken string
}

type Server struct {
	URL     string
	Cleanup func()
}

func StartServer(t *testing.T, opts ServerOptions) *Server {
	t.Helper()
	token := opts.InternalServiceToken
	if token == "" {
		token = DefaultInternalToken
	}
	if opts.Pool == nil {
		t.Fatal("testkit: pool is required")
	}

	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(opts.Pool, audit)
	rateCards := repository.NewRateCardRepository(opts.Pool, contracts, audit)
	locations := repository.NewLocationRepository(opts.Pool)
	rateLines := repository.NewRateLineRepository(opts.Pool, rateCards, locations, audit)
	rateComponents := repository.NewRateComponentRepository(opts.Pool, rateLines, rateCards, audit)
	resolutions := repository.NewResolutionRepository(opts.Pool, audit)
	memberships := repository.NewMembershipRepository(opts.Pool)
	actors := handlers.NewActorResolver(memberships)

	contractSvc := service.NewContractService(contracts, memberships)
	rateCardSvc := service.NewRateCardService(rateCards, contracts)
	rateLineSvc := service.NewRateLineService(rateLines, rateCards, contracts)
	rateComponentSvc := service.NewRateComponentService(rateComponents, rateLines, rateCards, contracts)
	resolutionSvc := service.NewResolutionService(resolutions, memberships, nil, nil)

	router := newIntegrationRouter(
		config.Config{InternalServiceToken: token, Environment: "test"},
		contractSvc, rateCardSvc, rateLineSvc, rateComponentSvc, resolutionSvc, actors,
	)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &Server{URL: srv.URL, Cleanup: srv.Close}
}
