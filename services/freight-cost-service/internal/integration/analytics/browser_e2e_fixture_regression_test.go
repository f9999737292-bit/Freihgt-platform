//go:build integration

package analytics

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC22G1_BrowserE2EFixture_AnalyticsPublicWithinDefaultWindow(t *testing.T) {
	env := setupFullProjectionEnv(t)
	fix := seedBrowserE2EFixture(t, env)
	ctx := context.Background()

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -90)
	periodStart := domain.PeriodStartFromSummaryUpdatedAt(analyticsPublicDefaultWindowAnchor(now))
	if periodStart.Before(from) || periodStart.After(now) {
		t.Fatalf("fixture period_start %s outside default public window [%s, %s]",
			periodStart.Format(time.RFC3339), from.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	orderFacts := repository.NewAnalyticsOrderFactRepository(env.pool)
	state := repository.NewAnalyticsProjectionStateRepository(env.pool)
	publicSvc := service.NewAnalyticsPublicService(env.analytics, orderFacts, state, true)
	actor := security.TrustedActor{
		TenantID:  fix.TenantID,
		UserID:    fix.UserID,
		CompanyID: fix.BuyerID,
		ActorKind: security.ActorKindBuyer,
	}

	overview, err := publicSvc.Overview(ctx, actor, url.Values{})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.Summary == nil || overview.Summary.OrderCount <= 0 {
		t.Fatalf("expected populated overview summary, got summary=%v mixed=%v quality=%s",
			overview.Summary, overview.MixedCurrency, overview.DataQuality)
	}

	lanes, err := publicSvc.ListLanes(ctx, actor, url.Values{"currency": []string{"RUB"}})
	if err != nil {
		t.Fatalf("lanes: %v", err)
	}
	if len(lanes.Lanes) == 0 {
		t.Fatalf("expected RUB lane items, got 0 (quality=%s mixed=%v)", lanes.DataQuality, lanes.MixedCurrency)
	}

	foreignActor := security.TrustedActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: uuid.New(),
		ActorKind: security.ActorKindBuyer,
	}
	foreignOverview, err := publicSvc.Overview(ctx, foreignActor, url.Values{})
	if err != nil {
		t.Fatalf("foreign overview: %v", err)
	}
	if foreignOverview.Summary != nil && foreignOverview.Summary.OrderCount > 0 {
		t.Fatalf("foreign tenant must not read fixture analytics, order_count=%d", foreignOverview.Summary.OrderCount)
	}
}
