package domain

import (
	"math"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const RegisterItemStatusDraft = "DRAFT"

type BillingRegisterItem struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	RegisterID        uuid.UUID
	ShipmentID        uuid.UUID
	TransportOrderID  *uuid.UUID
	RouteDescription  *string
	PickupDate        *time.Time
	DeliveryDate      *time.Time
	ShipperCompanyID  *uuid.UUID
	ConsigneeCompanyID *uuid.UUID
	CarrierCompanyID  *uuid.UUID
	BaseAmount        float64
	ExtraCharges      float64
	Penalties         float64
	AmountWithoutVAT  float64
	VATRate           *float64
	VATAmount         float64
	AmountWithVAT     float64
	Status            string
	CreatedAt         time.Time
}

type CreateBillingRegisterItemInput struct {
	TenantID           uuid.UUID
	ShipmentID         uuid.UUID
	TransportOrderID   *uuid.UUID
	RouteDescription   *string
	PickupDate         *time.Time
	DeliveryDate       *time.Time
	ShipperCompanyID   *uuid.UUID
	ConsigneeCompanyID *uuid.UUID
	CarrierCompanyID   *uuid.UUID
	BaseAmount         float64
	ExtraCharges       float64
	Penalties          float64
	VATRate            *float64
}

type ItemAmounts struct {
	AmountWithoutVAT float64
	VATAmount        float64
	AmountWithVAT    float64
}

func ValidateCreateBillingRegisterItemInput(in CreateBillingRegisterItemInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.ShipmentID == uuid.Nil {
		return apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	for _, check := range []struct {
		value float64
		field string
	}{
		{in.BaseAmount, "base_amount"},
		{in.ExtraCharges, "extra_charges"},
		{in.Penalties, "penalties"},
	} {
		if err := ValidateNonNegativeAmount(check.value, check.field); err != nil {
			return err
		}
	}
	if in.VATRate != nil {
		if err := ValidateNonNegativeAmount(*in.VATRate, "vat_rate"); err != nil {
			return err
		}
	}
	return nil
}

func CalculateItemAmounts(base, extra, penalties float64, vatRate *float64) ItemAmounts {
	withoutVAT := round2(base + extra - penalties)
	vat := 0.0
	if vatRate != nil {
		vat = round2(withoutVAT * (*vatRate) / 100)
	}
	return ItemAmounts{
		AmountWithoutVAT: withoutVAT,
		VATAmount:        vat,
		AmountWithVAT:    round2(withoutVAT + vat),
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
