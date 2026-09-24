package http_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

func TestBNOFoundationGates(t *testing.T) {
	tenantA := uuid.New()
	userA := uuid.New()
	tenantB := uuid.New()
	userB := uuid.New()
	companyB := uuid.New()
	sourceOK := uuid.New()
	sourceForeign := uuid.New()
	store := repository.NewMemory()
	verifier := sourceverify.MapVerifier{Owned: map[string]bool{
		fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, sourceOK): true,
	}}
	srv := httptest.NewServer(httpserver.NewRouter(slog.New(slog.DiscardHandler), service.New(store, verifier), nil))
	t.Cleanup(srv.Close)
	api := &client{base: srv.URL, t: t}

	// BNO14
	if res := api.do(http.MethodGet, "/v1/network/load-opportunities", "", "", "", nil, ""); res.status != http.StatusUnauthorized {
		t.Fatalf("BNO14 status=%d", res.status)
	}

	// BNO01
	created := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", loadJSON(sourceOK, "PRIVATE", false, 0), "")
	if created.status != http.StatusCreated {
		t.Fatalf("BNO01 status=%d body=%s", created.status, created.raw)
	}
	if created.doc["status"] != "DRAFT" || created.doc["version"] != float64(1) {
		t.Fatalf("BNO01 document=%v", created.doc)
	}
	if _, ok := created.doc["weight_kg"]; ok {
		t.Fatal("BNO01 omitted weight must stay absent")
	}
	loadID := created.doc["id"].(string)

	// BNO02
	published := api.send(http.MethodPost, "/v1/network/load-opportunities/"+loadID+"/publish", tenantA, userA, "", `{"version":1}`, "")
	if published.status != http.StatusOK || published.doc["status"] != "PUBLISHED" {
		t.Fatalf("BNO02 status=%d body=%s", published.status, published.raw)
	}

	// BNO03
	foreign := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", loadJSON(sourceForeign, "MARKETPLACE", true, 0), "")
	if foreign.status != http.StatusNotFound {
		t.Fatalf("BNO03 status=%d body=%s", foreign.status, foreign.raw)
	}

	// BNO20
	rejected := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", withField(loadJSON(sourceOK, "PRIVATE", false, 0), `"weight_kg":0`), "")
	if rejected.status != http.StatusUnprocessableEntity {
		t.Fatalf("BNO20 zero weight status=%d body=%s", rejected.status, rejected.raw)
	}
	pallet := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", withField(loadJSON(sourceOK, "PRIVATE", false, 0), `"pallet_count":0`), "")
	if pallet.status != http.StatusUnprocessableEntity {
		t.Fatalf("BNO20 pallet status=%d body=%s", pallet.status, pallet.raw)
	}
	linear := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", withField(loadJSON(sourceOK, "PRIVATE", false, 0), `"linear_meters":0`), "")
	if linear.status != http.StatusUnprocessableEntity {
		t.Fatalf("BNO20 linear status=%d", linear.status)
	}
	if strings.Contains(created.raw, "pallet_count") || strings.Contains(created.raw, "linear_meters") {
		t.Fatalf("BNO20 stored document leaked unknown fields: %s", created.raw)
	}

	// Withdraw the private published load before further publication of the same source.
	withdrawn := api.send(http.MethodPost, "/v1/network/load-opportunities/"+loadID+"/withdraw", tenantA, userA, "", `{"version":2}`, "")
	if withdrawn.status != http.StatusOK || withdrawn.doc["status"] != "WITHDRAWN" {
		t.Fatalf("BNO08 status=%d body=%s", withdrawn.status, withdrawn.raw)
	}
	again := api.send(http.MethodPost, "/v1/network/load-opportunities/"+loadID+"/withdraw", tenantA, userA, "", `{"version":3}`, "")
	if again.status != http.StatusConflict {
		t.Fatalf("BNO08 second withdraw status=%d", again.status)
	}

	marketSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, marketSource)] = true
	market := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", loadJSON(marketSource, "MARKETPLACE", true, 1200), "pub-market")
	if market.status != http.StatusCreated {
		t.Fatalf("marketplace create status=%d body=%s", market.status, market.raw)
	}
	marketID := market.doc["id"].(string)

	// BNO04 / BNO06
	seen := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+marketID, tenantB, userB, companyB.String(), "", "")
	if seen.status != http.StatusOK {
		t.Fatalf("BNO06 status=%d body=%s", seen.status, seen.raw)
	}
	for _, forbidden := range []string{"source_id", "source_type", "invited_carrier_company_ids"} {
		if _, ok := seen.doc[forbidden]; ok {
			t.Fatalf("BNO04 leaked %s: %s", forbidden, seen.raw)
		}
	}
	if seen.doc["pickup"].(map[string]any)["label"] != "Warehouse A" {
		t.Fatalf("BNO04 pickup=%v", seen.doc["pickup"])
	}
	if _, ok := seen.doc["weight_kg"]; !ok {
		t.Fatal("BNO04 published weight should be present when supplied")
	}

	// BNO05 private invisible. Create a new private load.
	privateSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, privateSource)] = true
	privateLoad := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", loadJSON(privateSource, "PRIVATE", true, 0), "")
	privateID := privateLoad.doc["id"].(string)
	hidden := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+privateID, tenantB, userB, companyB.String(), "", "")
	missing := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+uuid.NewString(), tenantB, userB, companyB.String(), "", "")
	if hidden.status != http.StatusNotFound || missing.status != http.StatusNotFound || hidden.raw != missing.raw {
		t.Fatalf("BNO05/BNO19 hidden=%d %s missing=%d %s", hidden.status, hidden.raw, missing.status, missing.raw)
	}
	list := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities", tenantB, userB, companyB.String(), "", "")
	if strings.Contains(list.raw, privateID) || !strings.Contains(list.raw, marketID) {
		t.Fatalf("BNO05/BNO06 list=%s", list.raw)
	}

	// BNO07 anonymized
	anonSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, anonSource)] = true
	anon := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", loadJSON(anonSource, "ANONYMIZED_MARKETPLACE", true, 0), "")
	anonView := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+anon.doc["id"].(string), tenantB, userB, companyB.String(), "", "")
	if _, ok := anonView.doc["owner_tenant_id"]; ok || anonView.status != http.StatusOK {
		t.Fatalf("BNO07 status=%d body=%s", anonView.status, anonView.raw)
	}

	// BNO21 anonymized load omits exact geography. MARKETPLACE retains it.
	geoSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, geoSource)] = true
	geoSite := uuid.New()
	geoBody := fmt.Sprintf(`{"source_type":"TRANSPORT_ORDER","source_id":"%s","pickup":{"label":"Warehouse A","location_id":"%s","latitude":55.7558,"longitude":37.6173},"delivery":{"label":"Warehouse B","latitude":59.9343,"longitude":30.3351},"visibility_scope":"MARKETPLACE","publish":true,"weight_kg":1200}`, geoSource, geoSite)
	geoLoad := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", geoBody, "")
	if geoLoad.status != http.StatusCreated {
		t.Fatalf("BNO21 marketplace create status=%d body=%s", geoLoad.status, geoLoad.raw)
	}
	geoView := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+geoLoad.doc["id"].(string), tenantB, userB, companyB.String(), "", "")
	geoPickup, _ := geoView.doc["pickup"].(map[string]any)
	if geoView.status != http.StatusOK || geoPickup["label"] != "Warehouse A" || geoPickup["latitude"] != 55.7558 || geoPickup["longitude"] != 37.6173 {
		t.Fatalf("BNO21 marketplace geo=%s", geoView.raw)
	}
	anonGeoSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, anonGeoSource)] = true
	anonSite := uuid.New()
	anonGeoBody := fmt.Sprintf(`{"source_type":"TRANSPORT_ORDER","source_id":"%s","pickup":{"label":"Private Warehouse 9","location_id":"%s","latitude":55.751244,"longitude":37.618423},"delivery":{"label":"Customer Dock 12","latitude":59.939812,"longitude":30.315644},"visibility_scope":"ANONYMIZED_MARKETPLACE","publish":true,"weight_kg":800}`, anonGeoSource, anonSite)
	anonGeo := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", anonGeoBody, "")
	if anonGeo.status != http.StatusCreated {
		t.Fatalf("BNO21 anon create status=%d body=%s", anonGeo.status, anonGeo.raw)
	}
	anonGeoView := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+anonGeo.doc["id"].(string), tenantB, userB, companyB.String(), "", "")
	if anonGeoView.status != http.StatusOK {
		t.Fatalf("BNO21 anon status=%d body=%s", anonGeoView.status, anonGeoView.raw)
	}
	for _, key := range []string{"pickup", "delivery", "owner_tenant_id", "source_id", "source_type", "latitude", "longitude", "location_id", "label"} {
		if _, ok := anonGeoView.doc[key]; ok {
			t.Fatalf("BNO21 leaked %s: %s", key, anonGeoView.raw)
		}
	}
	for _, leak := range []string{"Private Warehouse 9", "Customer Dock 12", anonSite.String(), anonGeoSource.String(), tenantA.String(), "55.751244", "37.618423", "59.939812", "30.315644"} {
		if strings.Contains(anonGeoView.raw, leak) {
			t.Fatalf("BNO21 raw leak %s in %s", leak, anonGeoView.raw)
		}
	}
	if _, ok := anonGeoView.doc["weight_kg"]; !ok {
		t.Fatal("BNO21 published weight should remain on the anonymized projection")
	}
	ownerGeo := api.send(http.MethodGet, "/v1/network/load-opportunities/"+anonGeo.doc["id"].(string), tenantA, userA, "", "", "")
	ownerPickup, _ := ownerGeo.doc["pickup"].(map[string]any)
	if ownerGeo.status != http.StatusOK || ownerPickup["label"] != "Private Warehouse 9" || ownerPickup["latitude"] != 55.751244 {
		t.Fatalf("BNO21 owner view must retain exact geography: %s", ownerGeo.raw)
	}

	invitedSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, invitedSource)] = true
	invitedBody := withField(loadJSON(invitedSource, "INVITED_CARRIERS", true, 0), fmt.Sprintf(`"invited_carrier_company_ids":["%s"]`, companyB))
	invited := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", invitedBody, "")
	if invited.status != http.StatusCreated {
		t.Fatalf("invited create status=%d body=%s", invited.status, invited.raw)
	}
	noCompany := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+invited.doc["id"].(string), tenantB, userB, "", "", "")
	withCompany := api.send(http.MethodGet, "/v1/network/marketplace/load-opportunities/"+invited.doc["id"].(string), tenantB, userB, companyB.String(), "", "")
	if noCompany.status != http.StatusNotFound || withCompany.status != http.StatusOK {
		t.Fatalf("BNO06 invited noCompany=%d withCompany=%d", noCompany.status, withCompany.status)
	}

	// BNO16 optimistic lock
	stale := api.send(http.MethodPatch, "/v1/network/load-opportunities/"+marketID, tenantA, userA, "", `{"version":1,"body_type":"TENT"}`, "")
	if stale.status != http.StatusOK || stale.doc["version"] != float64(2) {
		t.Fatalf("BNO16 update status=%d body=%s", stale.status, stale.raw)
	}
	conflict := api.send(http.MethodPatch, "/v1/network/load-opportunities/"+marketID, tenantA, userA, "", `{"version":1,"body_type":"REEFER"}`, "")
	if conflict.status != http.StatusConflict {
		t.Fatalf("BNO16 conflict status=%d body=%s", conflict.status, conflict.raw)
	}

	// BNO11 / BNO19 mutation
	foreignMut := api.send(http.MethodPatch, "/v1/network/load-opportunities/"+marketID, tenantB, userB, companyB.String(), `{"version":2,"body_type":"BOX"}`, "")
	missingMut := api.send(http.MethodPatch, "/v1/network/load-opportunities/"+uuid.NewString(), tenantB, userB, companyB.String(), `{"version":2,"body_type":"BOX"}`, "")
	if foreignMut.status != http.StatusNotFound || foreignMut.raw != missingMut.raw {
		t.Fatalf("BNO19 mutate foreign=%s missing=%s", foreignMut.raw, missingMut.raw)
	}

	// BNO13 body tenant is not accepted; query tenant mismatch is forbidden.
	spoofBody := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", withField(loadJSON(uuid.New(), "PRIVATE", false, 0), `"owner_tenant_id":"`+tenantB.String()+`"`), "")
	if spoofBody.status != http.StatusUnprocessableEntity {
		t.Fatalf("BNO13 body status=%d", spoofBody.status)
	}
	spoofQuery := api.send(http.MethodGet, "/v1/network/load-opportunities?tenant_id="+tenantB.String(), tenantA, userA, "", "", "")
	if spoofQuery.status != http.StatusForbidden {
		t.Fatalf("BNO13 query status=%d", spoofQuery.status)
	}

	// BNO17
	idemSource := uuid.New()
	verifier.Owned[fmt.Sprintf("%s|TRANSPORT_ORDER|%s", tenantA, idemSource)] = true
	idemBody := loadJSON(idemSource, "MARKETPLACE", true, 0)
	first := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", idemBody, "same-key")
	second := api.send(http.MethodPost, "/v1/network/load-opportunities", tenantA, userA, "", idemBody, "same-key")
	if first.status != http.StatusCreated || second.raw != first.raw {
		t.Fatalf("BNO17 first=%d %s second=%s", first.status, first.raw, second.raw)
	}
	events, err := store.ListOutbox(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	publishedForIdem := 0
	for _, event := range events {
		if event.AggregateID.String() == first.doc["id"].(string) && event.EventName == "network.load_opportunity.published" {
			publishedForIdem++
			assertSafeEvent(t, event)
		}
	}
	if publishedForIdem != 1 {
		t.Fatalf("BNO17/BNO18 published events=%d", publishedForIdem)
	}

	// BNO18 updated event from the market patch
	updated := 0
	for _, event := range events {
		if event.EventName == "network.load_opportunity.updated" && event.AggregateID.String() == marketID {
			updated++
			assertSafeEvent(t, event)
			if event.AggregateVersion != 2 || event.TenantID != tenantA {
				t.Fatalf("BNO18 event=%+v", event)
			}
		}
	}
	if updated != 1 {
		t.Fatalf("BNO18 updated events=%d", updated)
	}

	// BNO09 BNO10 BNO11 BNO12 capacity
	from := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	until := time.Now().UTC().Add(5 * time.Hour).Format(time.RFC3339)
	carrierCompany := uuid.New()
	capBody := fmt.Sprintf(`{"location_label":"Yard","latitude":55.7558,"longitude":37.6173,"available_from":"%s","available_until":"%s","visibility_scope":"MARKETPLACE","carrier_company_id":"%s","vehicle_id":"%s","body_type":"TENT"}`, from, until, carrierCompany, uuid.New())
	cap := api.send(http.MethodPost, "/v1/network/capacities", tenantB, userB, companyB.String(), capBody, "cap-1")
	if cap.status != http.StatusCreated || cap.doc["status"] != "AVAILABLE" || cap.doc["source"] != "MANUAL" {
		t.Fatalf("BNO09 status=%d body=%s", cap.status, cap.raw)
	}
	if _, ok := cap.doc["payload_remaining_kg"]; ok {
		t.Fatal("BNO20 capacity payload must stay unknown")
	}
	capID := cap.doc["id"].(string)
	visibleCap := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+capID, tenantA, userA, "", "", "")
	if visibleCap.status != http.StatusOK || visibleCap.doc["carrier_company_id"] != carrierCompany.String() || visibleCap.doc["location_label"] != "Yard" || visibleCap.doc["latitude"] != 55.7558 || visibleCap.doc["longitude"] != 37.6173 {
		t.Fatalf("BNO10 status=%d body=%s", visibleCap.status, visibleCap.raw)
	}
	privateCapBody := fmt.Sprintf(`{"location_label":"Hidden","available_from":"%s","available_until":"%s","visibility_scope":"PRIVATE"}`, from, until)
	privateCap := api.send(http.MethodPost, "/v1/network/capacities", tenantB, userB, companyB.String(), privateCapBody, "")
	hiddenCap := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+privateCap.doc["id"].(string), tenantA, userA, "", "", "")
	if hiddenCap.status != http.StatusNotFound {
		t.Fatalf("BNO10 private status=%d", hiddenCap.status)
	}
	anonVehicle := uuid.New()
	anonCapBody := fmt.Sprintf(`{"location_label":"Exact Yard 4","latitude":56.3287,"longitude":44.0021,"available_from":"%s","available_until":"%s","visibility_scope":"ANONYMIZED","carrier_company_id":"%s","vehicle_id":"%s","body_type":"TENT","equipment":["CURTAIN"]}`, from, until, carrierCompany, anonVehicle)
	anonCap := api.send(http.MethodPost, "/v1/network/capacities", tenantB, userB, companyB.String(), anonCapBody, "")
	if anonCap.status != http.StatusCreated {
		t.Fatalf("BNO22 create status=%d body=%s", anonCap.status, anonCap.raw)
	}
	anonCapView := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+anonCap.doc["id"].(string), tenantA, userA, "", "", "")
	if _, ok := anonCapView.doc["carrier_company_id"]; ok || anonCapView.doc["vehicle_id"] != nil {
		t.Fatalf("BNO07/BNO10 anonymized capacity leaked identity: %s", anonCapView.raw)
	}
	if _, ok := anonCapView.doc["owner_tenant_id"]; ok {
		t.Fatalf("BNO07 capacity owner leaked: %s", anonCapView.raw)
	}
	for _, key := range []string{"location_label", "latitude", "longitude", "carrier_company_id", "vehicle_id"} {
		if _, ok := anonCapView.doc[key]; ok {
			t.Fatalf("BNO22 leaked %s: %s", key, anonCapView.raw)
		}
	}
	for _, leak := range []string{"Exact Yard 4", "56.3287", "44.0021", anonVehicle.String(), carrierCompany.String(), tenantB.String()} {
		if strings.Contains(anonCapView.raw, leak) {
			t.Fatalf("BNO22 raw leak %s in %s", leak, anonCapView.raw)
		}
	}
	if anonCapView.doc["available_from"] == nil || anonCapView.doc["body_type"] != "TENT" {
		t.Fatalf("BNO22 non-identifying capacity facts missing: %s", anonCapView.raw)
	}
	ownerCap := api.send(http.MethodGet, "/v1/network/capacities/"+anonCap.doc["id"].(string), tenantB, userB, companyB.String(), "", "")
	if ownerCap.status != http.StatusOK || ownerCap.doc["location_label"] != "Exact Yard 4" || ownerCap.doc["latitude"] != 56.3287 || ownerCap.doc["vehicle_id"] != anonVehicle.String() {
		t.Fatalf("BNO22 owner capacity must retain exact location and vehicle: %s", ownerCap.raw)
	}
	anonEvents, err := store.ListOutbox(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var sawAnonLoad, sawAnonCap bool
	for _, event := range anonEvents {
		assertSafeEvent(t, event)
		raw := string(event.Payload)
		for _, leak := range []string{"Private Warehouse 9", "Customer Dock 12", "Exact Yard 4", "55.751244", "56.3287", "latitude", "longitude", "location_label", "vehicle_id", "source_id"} {
			if strings.Contains(raw, leak) {
				t.Fatalf("anonymized event leaked %s: %s", leak, raw)
			}
		}
		if event.AggregateID.String() == anonGeo.doc["id"].(string) && event.EventName == "network.load_opportunity.published" {
			sawAnonLoad = true
		}
		if event.AggregateID.String() == anonCap.doc["id"].(string) && event.EventName == "network.capacity.published" {
			sawAnonCap = true
		}
	}
	if !sawAnonLoad || !sawAnonCap {
		t.Fatalf("anonymized publication events load=%v capacity=%v", sawAnonLoad, sawAnonCap)
	}
	networkBody := fmt.Sprintf(`{"location_label":"Network","available_from":"%s","available_until":"%s","visibility_scope":"SHIPPER_NETWORK","audience_tenant_ids":["%s"]}`, from, until, tenantA)
	networkCap := api.send(http.MethodPost, "/v1/network/capacities", tenantB, userB, companyB.String(), networkBody, "")
	if networkCap.status != http.StatusCreated {
		t.Fatalf("shipper network status=%d body=%s", networkCap.status, networkCap.raw)
	}
	other := uuid.New()
	notAudience := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+networkCap.doc["id"].(string), other, uuid.New(), "", "", "")
	audience := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+networkCap.doc["id"].(string), tenantA, userA, "", "", "")
	if notAudience.status != http.StatusNotFound || audience.status != http.StatusOK || audience.doc["location_label"] != "Network" {
		t.Fatalf("BNO10 audience not=%d yes=%d body=%s", notAudience.status, audience.status, audience.raw)
	}
	predicted := api.send(http.MethodPost, "/v1/network/capacities", tenantB, userB, companyB.String(), withField(capBody, `"source":"CURRENT_SHIPMENT_PREDICTION"`), "")
	if predicted.status != http.StatusUnprocessableEntity {
		t.Fatalf("predictive source status=%d body=%s", predicted.status, predicted.raw)
	}
	foreignCap := api.send(http.MethodPatch, "/v1/network/capacities/"+capID, tenantA, userA, "", `{"version":1,"location_label":"Stolen"}`, "")
	missingCap := api.send(http.MethodPatch, "/v1/network/capacities/"+uuid.NewString(), tenantA, userA, "", `{"version":1,"location_label":"Stolen"}`, "")
	if foreignCap.status != http.StatusNotFound || foreignCap.raw != missingCap.raw {
		t.Fatalf("BNO11 foreign=%s missing=%s", foreignCap.raw, missingCap.raw)
	}
	capWithdraw := api.send(http.MethodPost, "/v1/network/capacities/"+capID+"/withdraw", tenantB, userB, companyB.String(), `{"version":1}`, "")
	if capWithdraw.status != http.StatusOK || capWithdraw.doc["status"] != "WITHDRAWN" {
		t.Fatalf("BNO12 status=%d body=%s", capWithdraw.status, capWithdraw.raw)
	}
	gone := api.send(http.MethodGet, "/v1/network/marketplace/capacities/"+capID, tenantA, userA, "", "", "")
	if gone.status != http.StatusNotFound {
		t.Fatalf("BNO12 still visible status=%d", gone.status)
	}
	events, _ = store.ListOutbox(t.Context())
	var sawPublished, sawWithdrawn bool
	for _, event := range events {
		if event.AggregateID.String() != capID {
			continue
		}
		assertSafeEvent(t, event)
		switch event.EventName {
		case "network.capacity.published":
			sawPublished = true
		case "network.capacity.withdrawn":
			sawWithdrawn = true
		case "network.capacity.predicted", "network.match.generated", "network.chain.generated":
			t.Fatalf("unexpected optimizer event %s", event.EventName)
		}
	}
	if !sawPublished || !sawWithdrawn {
		t.Fatalf("BNO18 capacity events published=%v withdrawn=%v", sawPublished, sawWithdrawn)
	}

	audits, err := store.ListAudit(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var sawAudit bool
	for _, audit := range audits {
		if audit.AggregateID.String() == capID && audit.Action == "create" && audit.ActorUserID == userB && audit.TenantID == tenantB {
			sawAudit = true
		}
	}
	if !sawAudit {
		t.Fatal("audit record missing for capacity publication")
	}
}

func assertSafeEvent(t *testing.T, event repository.OutboxEvent) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"eventId", "eventName", "tenantId", "aggregateId", "aggregateVersion", "occurredAt"} {
		if payload[key] == nil || payload[key] == "" {
			t.Fatalf("event %s missing %s", event.EventName, key)
		}
	}
	for _, forbidden := range []string{"weight_kg", "source_id", "source_type", "amount", "payload_remaining_kg", "pallet_count", "linear_meters", "latitude", "longitude", "location_label", "pickup", "delivery", "vehicle_id", "carrier_company_id", "owner_tenant_id"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("event leaked %s", forbidden)
		}
	}
}

func withField(body, field string) string {
	return body[:len(body)-1] + "," + field + "}"
}

func loadJSON(source uuid.UUID, visibility string, publish bool, weight float64) string {
	weightField := ""
	if weight > 0 {
		weightField = fmt.Sprintf(`,"weight_kg":%g`, weight)
	}
	return fmt.Sprintf(`{"source_type":"TRANSPORT_ORDER","source_id":"%s","pickup":{"label":"Warehouse A"},"delivery":{"label":"Warehouse B"},"visibility_scope":"%s","publish":%t%s}`, source, visibility, publish, weightField)
}

type client struct {
	base string
	t    *testing.T
}

type response struct {
	status int
	raw    string
	doc    map[string]any
}

func (c *client) send(method, path string, tenant, user uuid.UUID, company, body, idem string) response {
	c.t.Helper()
	return c.do(method, path, tenant.String(), user.String(), company, strings.NewReader(body), idem)
}

func (c *client) do(method, path, tenant, user, company string, body io.Reader, idem string) response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	if user != "" {
		req.Header.Set("X-User-ID", user)
	}
	if company != "" {
		req.Header.Set("X-Company-ID", company)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode, raw: string(raw)}
	_ = json.Unmarshal(raw, &out.doc)
	if res.StatusCode >= 400 {
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &envelope)
		out.doc = map[string]any{"code": envelope.Error.Code, "message": envelope.Error.Message}
	}
	return out
}
