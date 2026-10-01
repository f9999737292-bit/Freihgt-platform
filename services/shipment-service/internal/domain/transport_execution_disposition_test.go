package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateDeliveryQuantities(t *testing.T) {
	if err := ValidateDeliveryQuantities(17, 3); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeliveryQuantities(0, 10); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeliveryQuantities(10, 0); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]int{{-1, 2}, {1, -1}, {0, 0}} {
		if got := reasonText(ValidateDeliveryQuantities(pair[0], pair[1])); got != ReasonQuantityInvalid {
			t.Fatalf("pair %v reason %s", pair, got)
		}
	}
}

func TestRejectionReasonAndActorPolicy(t *testing.T) {
	cmd := RecordDeliveryDispositionCommand{
		ExecutionID: uuid.New(), ExpectedRevisionID: uuid.New(), OperatingTenantID: uuid.New(),
		SourceStopID: uuid.New(), ShipmentID: uuid.New(), CargoID: uuid.New(),
		AcceptedQuantity: 0, RejectedQuantity: 1, UOM: UOMPallet, ReasonCode: "not-a-reason",
		IdempotencyKey: "k", OccurredAt: time.Now().UTC(), ActorKind: ActorKindDriver, ActorID: uuid.New(),
	}
	if reasonText(cmd.Validate()) != ReasonReasonUnknown {
		t.Fatal(cmd.Validate())
	}
	cmd.ReasonCode = ReasonOther
	if reasonText(cmd.Validate()) != ReasonCommentRequired {
		t.Fatal(cmd.Validate())
	}
	cmd.ReasonComment = "customer note"
	if err := cmd.Validate(); err != nil {
		t.Fatal(err)
	}
	cmd.UOM = "KG"
	if reasonText(cmd.Validate()) != ReasonUOMInvalid {
		t.Fatal(cmd.Validate())
	}
	auth := AuthorizeDispositionCommand{
		CaseID: uuid.New(), ExecutionID: uuid.New(), ExpectedRevisionID: uuid.New(), OperatingTenantID: uuid.New(),
		TargetLocationID: uuid.New(), IdempotencyKey: "k", OccurredAt: time.Now().UTC(),
		ActorKind: ActorKindDriver, ActorID: uuid.New(),
	}
	if reasonText(auth.Validate()) != ReasonActorDenied {
		t.Fatal(auth.Validate())
	}
	hold := HoldDispositionCommand{
		CaseID: auth.CaseID, ExecutionID: auth.ExecutionID, ExpectedRevisionID: auth.ExpectedRevisionID,
		OperatingTenantID: auth.OperatingTenantID, IdempotencyKey: "k", OccurredAt: auth.OccurredAt,
		ActorKind: ActorKindDriver, ActorID: uuid.New(),
	}
	if reasonText(hold.Validate()) != ReasonActorDenied {
		t.Fatal(hold.Validate())
	}
}

func reasonText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	for _, reason := range []string{
		ReasonQuantityInvalid, ReasonReasonUnknown, ReasonCommentRequired, ReasonActorDenied, ReasonUOMInvalid,
	} {
		if strings.Contains(text, reason) {
			return reason
		}
	}
	return text
}
