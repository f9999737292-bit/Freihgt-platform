package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

const (
	r2CommercialAmount   = 987654321.12
	r2CommercialNeedle   = "987654321"
	r2CommercialCurrency = "ZZZ"
	r2CommercialMode     = "SECRET_TEST_MODE"
)

func TestNLO04B_R2_MarketplacePostOmitsForeignOwnerAndCommercial(t *testing.T) {
	w, src, load := marketplaceSecrets(t)
	created := evaluateRaw(t, w, src, load, "r2-market-post")
	assertRoutePlanSecretsAbsent(t, string(created.Body), w.shipper)
	if load.Pickup.LocationID == nil || !strings.Contains(string(created.Body), load.Pickup.LocationID.String()) {
		t.Fatal("authorized marketplace geography missing")
	}
}

func TestNLO04B_R2_MarketplaceGetOmitsForeignOwnerAndCommercial(t *testing.T) {
	w, src, load := marketplaceSecrets(t)
	created := evaluateRaw(t, w, src, load, "r2-market-get")
	got, err := w.svc.GetRoutePlan(context.Background(), w.actor(), uuid.MustParse(planID(t, created.Body)))
	if err != nil || got.Status != http.StatusOK {
		t.Fatalf("get %v %+v", err, got)
	}
	assertRoutePlanSecretsAbsent(t, string(got.Body), w.shipper)
	if load.Delivery.LocationID == nil || !strings.Contains(string(got.Body), load.Delivery.LocationID.String()) {
		t.Fatal("authorized marketplace geography missing")
	}
	if !sameProjection(t, created.Body, got.Body) {
		t.Fatal("post and get projections differ")
	}
}

func TestNLO04B_R2_PersistedPublicSnapshotOmitsForeignOwnerAndCommercial(t *testing.T) {
	w, src, load := marketplaceSecrets(t)
	created := evaluateRaw(t, w, src, load, "r2-market-stored")
	graph := storedPlan(t, w, uuid.MustParse(planID(t, created.Body)))
	found := false
	for _, action := range graph.Actions {
		if action.SubjectType != "LOAD_OPPORTUNITY" || action.SubjectID != load.ID {
			continue
		}
		found = true
		assertRoutePlanSecretsAbsent(t, string(action.PublicSubjectSnapshot), w.shipper)
		if !strings.Contains(string(action.PublicSubjectSnapshot), load.ID.String()) {
			t.Fatal("snapshot missing load id")
		}
	}
	if !found {
		t.Fatal("persisted load snapshot missing")
	}
}

func TestNLO04B_R2_AnonymizedPrivacyRegression(t *testing.T) {
	w, src, load, secretLat, secretID := anonymizedWorld(t)
	load.Commercial = secretCommercial()
	replaceLoad(t, w, load)
	created := evaluateRaw(t, w, src, load, "r2-anon")
	got, err := w.svc.GetRoutePlan(context.Background(), w.actor(), uuid.MustParse(planID(t, created.Body)))
	if err != nil {
		t.Fatal(err)
	}
	graph := storedPlan(t, w, uuid.MustParse(planID(t, created.Body)))
	for _, raw := range []string{string(created.Body), string(got.Body)} {
		assertAnonymized(t, raw, secretLat, secretID, w.shipper)
		assertRoutePlanSecretsAbsent(t, raw, w.shipper)
		if !strings.Contains(raw, "Hidden") {
			t.Fatal("authorized coarse geography missing")
		}
	}
	exact := false
	for _, stop := range graph.Stops {
		if stop.StopRole == routeplan.RoleCargo && stop.Latitude == secretLat && stop.LocationID != nil && *stop.LocationID == secretID {
			exact = true
		}
	}
	if !exact {
		t.Fatal("internal cargo point was not stored exactly")
	}
	for _, action := range graph.Actions {
		if action.SubjectID != load.ID {
			continue
		}
		raw := string(action.PublicSubjectSnapshot)
		assertAnonymized(t, raw, secretLat, secretID, w.shipper)
		assertRoutePlanSecretsAbsent(t, raw, w.shipper)
	}
}

func TestNLO04B_R2_PublicSnapshotReplayStable(t *testing.T) {
	w, src, load := marketplaceSecrets(t)
	counter := &countingRoute{}
	w.svc.UseRouting(counter)
	w.svc.SetClock(func() time.Time { return w.at })
	body := tripBody(src, load)
	first := evaluateBody(t, w, body, "r2-replay")
	calls := counter.calls.Load()
	second := evaluateBody(t, w, body, "r2-replay")
	if string(first.Body) != string(second.Body) || counter.calls.Load() != calls {
		t.Fatalf("replay diverged calls %d->%d", calls, counter.calls.Load())
	}
	assertRoutePlanSecretsAbsent(t, string(second.Body), w.shipper)
	if snapshotText(t, first.Body, load.ID.String()) != snapshotText(t, second.Body, load.ID.String()) {
		t.Fatal("replay snapshot changed")
	}
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evaluated := 0
	for _, event := range events {
		if event.EventName == routePlanEvaluatedEvent && event.AggregateID.String() == planID(t, first.Body) {
			evaluated++
		}
	}
	if evaluated != 1 {
		t.Fatalf("events %d", evaluated)
	}
	_, err = w.svc.EvaluateRoutePlan(context.Background(), w.actor(), "r2-replay", commandFrom(t, body+" "))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Code != apperrors.CodeConflict {
		t.Fatal(err)
	}
}

func TestNLO04B_R2_OwnTenantEvaluateUsesSafeProjection(t *testing.T) {
	w, src, load := newFill(t)
	load.OwnerTenantID = w.carrier
	load.ConsolidationAllowed = true
	load.Commercial = secretCommercial()
	replaceLoad(t, w, load)
	created := evaluateRaw(t, w, src, load, "r2-own")
	if created.Status != http.StatusCreated {
		t.Fatalf("status %d", created.Status)
	}
	raw := snapshotText(t, created.Body, load.ID.String())
	assertRoutePlanSecretsAbsent(t, raw, uuid.Nil)
	if strings.Contains(string(created.Body), r2CommercialMode) || strings.Contains(string(created.Body), r2CommercialNeedle) || strings.Contains(string(created.Body), r2CommercialCurrency) {
		t.Fatalf("own-tenant response leaked commercial terms %s", created.Body)
	}
}

func marketplaceSecrets(t *testing.T) (*world, *fillSources, domain.LoadOpportunity) {
	t.Helper()
	w, src, load := newFill(t)
	load.VisibilityScope = domain.VisMarketplace
	load.CrossShipperConsolidationAllowed = true
	load.Commercial = secretCommercial()
	replaceLoad(t, w, load)
	return w, src, load
}

func secretCommercial() domain.Commercial {
	return domain.Commercial{Mode: r2CommercialMode, Amount: f64(r2CommercialAmount), Currency: r2CommercialCurrency}
}

func evaluateRaw(t *testing.T, w *world, src *fillSources, load domain.LoadOpportunity, key string) Result {
	t.Helper()
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	return evaluateBody(t, w, tripBody(src, load), key)
}

func evaluateBody(t *testing.T, w *world, body, key string) Result {
	t.Helper()
	result, err := w.svc.EvaluateRoutePlan(context.Background(), w.actor(), key, commandFrom(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusCreated {
		t.Fatalf("status %d body %s", result.Status, result.Body)
	}
	return result
}

func commandFrom(t *testing.T, body string) RoutePlanCommand {
	t.Helper()
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Raw = []byte(body)
	return cmd
}

func tripBody(src *fillSources, load domain.LoadOpportunity) string {
	return `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + src.execution.ShipmentID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
}

func planID(t *testing.T, raw []byte) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	id, _ := doc["id"].(string)
	if id == "" {
		t.Fatal("plan id missing")
	}
	return id
}

func snapshotText(t *testing.T, raw []byte, loadID string) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, stop := range doc["stops"].([]any) {
		for _, action := range stop.(map[string]any)["actions"].([]any) {
			row := action.(map[string]any)
			if row["subject_id"] == loadID && row["public_subject_snapshot"] != nil {
				encoded, err := json.Marshal(row["public_subject_snapshot"])
				if err != nil {
					t.Fatal(err)
				}
				return string(encoded)
			}
		}
	}
	t.Fatal("public snapshot missing")
	return ""
}

func storedPlan(t *testing.T, w *world, id uuid.UUID) repository.RoutePlanGraph {
	t.Helper()
	var graph repository.RoutePlanGraph
	err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		var getErr error
		graph, getErr = tx.GetRoutePlan(context.Background(), w.carrier, id)
		return getErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func assertRoutePlanSecretsAbsent(t *testing.T, raw string, owner uuid.UUID) {
	t.Helper()
	needles := []string{r2CommercialNeedle, r2CommercialCurrency, r2CommercialMode, "owner_tenant_id", `"commercial"`}
	if owner != uuid.Nil {
		needles = append(needles, owner.String())
	}
	for _, needle := range needles {
		if strings.Contains(raw, needle) {
			t.Fatalf("privacy leak %q in %s", needle, raw)
		}
	}
}
