package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCarrierDocumentFilter(t *testing.T) {
	tenant := uuid.New()
	carrier := uuid.New()
	shipment := uuid.New()
	valid := CarrierDocumentFilter{TenantID: tenant, CarrierCompanyID: carrier, ShipmentID: shipment, Limit: 20}
	if err := ValidateCarrierDocumentFilter(valid); err != nil {
		t.Fatal(err)
	}

	missingTenant := valid
	missingTenant.TenantID = uuid.Nil
	if err := ValidateCarrierDocumentFilter(missingTenant); err == nil {
		t.Fatal("missing tenant was accepted")
	}
	missingCarrier := valid
	missingCarrier.CarrierCompanyID = uuid.Nil
	if err := ValidateCarrierDocumentFilter(missingCarrier); err == nil {
		t.Fatal("missing carrier was accepted")
	}
	missingShipment := valid
	missingShipment.ShipmentID = uuid.Nil
	if err := ValidateCarrierDocumentFilter(missingShipment); err == nil {
		t.Fatal("missing shipment was accepted")
	}

	invoice := "INVOICE"
	claim := "CLAIM"
	draft := DocumentStatusDraft
	cancelled := DocumentStatusCancelled
	for _, bad := range []CarrierDocumentFilter{
		withType(valid, &invoice),
		withType(valid, &claim),
		withStatus(valid, &draft),
		withStatus(valid, &cancelled),
		withLimit(valid, 0),
		withLimit(valid, 51),
		withOffset(valid, -1),
	} {
		if err := ValidateCarrierDocumentFilter(bad); err == nil {
			t.Fatalf("invalid filter accepted: %+v", bad)
		}
	}
}

func withType(f CarrierDocumentFilter, value *string) CarrierDocumentFilter {
	f.DocumentType = value
	return f
}

func withStatus(f CarrierDocumentFilter, value *string) CarrierDocumentFilter {
	f.DocumentStatus = value
	return f
}

func withLimit(f CarrierDocumentFilter, limit int) CarrierDocumentFilter {
	f.Limit = limit
	return f
}

func withOffset(f CarrierDocumentFilter, offset int) CarrierDocumentFilter {
	f.Offset = offset
	return f
}
