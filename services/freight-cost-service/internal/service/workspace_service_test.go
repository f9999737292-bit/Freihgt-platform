package service

import (
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/security"
)

func TestWorkspaceParseFilterPaginationBounds(t *testing.T) {
	t.Parallel()
	svc := &WorkspaceService{}
	actor := security.TrustedActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: uuid.New(),
		ActorKind: security.ActorKindBuyer,
	}

	for _, tc := range []struct {
		name      string
		limit     string
		wantLimit int
	}{
		{"default", "", 50},
		{"zero normalized later", "0", 0},
		{"negative", "-1", -1},
		{"max", "100", 100},
		{"over max parsed", "101", 101},
		{"absurd", "999999", 999999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := url.Values{}
			if tc.limit != "" {
				values.Set("limit", tc.limit)
			}
			filter := svc.parseFilter(actor, values)
			if filter.Limit != tc.wantLimit {
				t.Fatalf("limit=%q got %d want %d", tc.limit, filter.Limit, tc.wantLimit)
			}
		})
	}
}

func TestWorkspaceListClampsPagination(t *testing.T) {
	t.Parallel()
	svc := &WorkspaceService{}
	actor := security.TrustedActor{TenantID: uuid.New(), CompanyID: uuid.New(), ActorKind: security.ActorKindBuyer}

	values := url.Values{"limit": {"101"}}
	filter := svc.parseFilter(actor, values)
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if limit != 100 {
		t.Fatalf("expected clamp to 100, got %d", limit)
	}

	values = url.Values{"limit": {"0"}}
	filter = svc.parseFilter(actor, values)
	limit = filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit != 50 {
		t.Fatalf("expected zero limit normalized to 50, got %d", limit)
	}
}
