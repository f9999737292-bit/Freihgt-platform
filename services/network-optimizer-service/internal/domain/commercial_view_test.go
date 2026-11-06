package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNLO09C_AnonymizedMarketplaceViewOmitsCommercial(t *testing.T) {
	amount := 771199.0
	load := LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: uuid.New(), VisibilityScope: VisAnonymized, Status: LoadPublished,
		Pickup:     Place{Label: "SECRET_DOCK", City: "Kazan", CountryCode: "RU"},
		Delivery:   Place{Label: "SECRET_DOOR", City: "Ufa", CountryCode: "RU"},
		Commercial: Commercial{Mode: "SECRET_MODE_9C", Amount: &amount, Currency: "ZZX"},
	}
	view := load.MarketplaceView()
	if view.Commercial != nil || view.OwnerTenantID != nil {
		t.Fatalf("anonymized view %+v", view.Commercial)
	}
	raw, err := MarshalMarketplaceLoad(load)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, leaked := range []string{"SECRET_MODE_9C", "771199", "ZZX", "SECRET_DOCK", `"commercial"`} {
		if jsonContains(body, leaked) {
			t.Fatalf("marketplace JSON leaked %s in %s", leaked, body)
		}
	}
	if !jsonContains(body, load.ID.String()) || !jsonContains(body, "Kazan") {
		t.Fatalf("anonymized load disappeared from marketplace JSON %s", body)
	}
}

func TestNLO09C_IdentifiedMarketplaceViewKeepsPublishedCommercial(t *testing.T) {
	amount := 880011.0
	for _, scope := range []string{VisMarketplace, VisInvited} {
		load := LoadOpportunity{
			ID: uuid.New(), OwnerTenantID: uuid.New(), VisibilityScope: scope, Status: LoadPublished,
			Commercial: Commercial{Mode: "PUBLISHED", Amount: &amount, Currency: "RUB"},
		}
		view := load.MarketplaceView()
		if view.Commercial == nil || view.Commercial.Mode != "PUBLISHED" || view.Commercial.Amount == nil || *view.Commercial.Amount != amount || view.Commercial.Currency != "RUB" {
			t.Fatalf("%s dropped published commercial %+v", scope, view.Commercial)
		}
	}
}

func TestNLO09C_CurrencyOnlyCommercialStaysUncopied(t *testing.T) {
	load := LoadOpportunity{
		VisibilityScope: VisAnonymized, Status: LoadPublished,
		Commercial: Commercial{Currency: "NOK"},
	}
	if load.MarketplaceView().Commercial != nil {
		t.Fatal("currency-only commercial was copied")
	}
	raw, err := json.Marshal(load.MarketplaceView())
	if err != nil {
		t.Fatal(err)
	}
	if jsonContains(string(raw), "NOK") || jsonContains(string(raw), `"commercial"`) {
		t.Fatalf("%s", raw)
	}
}

func jsonContains(body, needle string) bool {
	return len(needle) > 0 && len(body) >= len(needle) && (body == needle || indexNeedle(body, needle) >= 0)
}

func indexNeedle(body, needle string) int {
	for i := 0; i+len(needle) <= len(body); i++ {
		if body[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
