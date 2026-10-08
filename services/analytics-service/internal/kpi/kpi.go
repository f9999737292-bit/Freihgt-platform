package kpi

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"
)

const DefinitionVersion = 1

const (
	ShipmentsTotal      = "OPS_SHIPMENTS_TOTAL"
	OnTimeDelivery      = "OPS_ON_TIME_DELIVERY"
	OnTimeDeliveryRate  = "OPS_ON_TIME_DELIVERY_RATE"
	ReturnCases         = "OPS_RETURN_CASES"
	RedirectCases       = "OPS_REDIRECT_CASES"
	OnTimePickup        = "OPS_ON_TIME_PICKUP"
	OnTimePickupRate    = "OPS_ON_TIME_PICKUP_RATE"
	LatePickup          = "OPS_LATE_PICKUP"
	LateDelivery        = "OPS_LATE_DELIVERY"
	CarrierPickupRate   = "CAR_ON_TIME_PICKUP_RATE"
	CarrierDeliveryRate = "CAR_ON_TIME_DELIVERY_RATE"
)

type Snapshot struct {
	TenantID                  string
	ShipmentTotal             int64
	OnTimePickupDenominator   int64
	OnTimePickupNumerator     int64
	OnTimeDeliveryDenominator int64
	OnTimeDeliveryNumerator   int64
	ReturnCaseCount           int64
	RedirectCaseCount         int64
	Carriers                  []CarrierSnapshot
}

type CarrierSnapshot struct {
	CarrierCompanyID          string
	OnTimePickupDenominator   int64
	OnTimePickupNumerator     int64
	OnTimeDeliveryDenominator int64
	OnTimeDeliveryNumerator   int64
}

type Response struct {
	KPIID             string    `json:"kpiId"`
	DefinitionVersion int       `json:"definitionVersion"`
	Measure           Measure   `json:"measure"`
	GeneratedAt       string    `json:"generatedAt"`
	DataFreshness     Freshness `json:"dataFreshness"`
	Completeness      string    `json:"completeness"`
}

type Measure struct {
	Type        string       `json:"type"`
	Value       *json.Number `json:"value"`
	Numerator   *int64       `json:"numerator,omitempty"`
	Denominator *int64       `json:"denominator,omitempty"`
}

type Freshness struct {
	Status string `json:"status"`
}

type CarrierResponse struct {
	KPIID             string        `json:"kpiId"`
	DefinitionVersion int           `json:"definitionVersion"`
	Dimension         string        `json:"dimension"`
	Items             []CarrierItem `json:"items"`
	GeneratedAt       string        `json:"generatedAt"`
	DataFreshness     Freshness     `json:"dataFreshness"`
	Completeness      string        `json:"completeness"`
}

type CarrierItem struct {
	CarrierCompanyID string  `json:"carrierCompanyId"`
	Measure          Measure `json:"measure"`
}

func Known(id string) bool {
	switch id {
	case ShipmentsTotal, OnTimeDelivery, OnTimeDeliveryRate, ReturnCases, RedirectCases,
		OnTimePickup, OnTimePickupRate, LatePickup, LateDelivery, CarrierPickupRate, CarrierDeliveryRate:
		return true
	default:
		return false
	}
}

func CarrierKPI(id string) bool {
	return id == CarrierPickupRate || id == CarrierDeliveryRate
}

func Build(id string, snap Snapshot, now time.Time) (Response, error) {
	generated := now.UTC().Format(time.RFC3339)
	switch id {
	case ShipmentsTotal:
		return countResponse(id, snap.ShipmentTotal, "COMPLETE", generated), nil
	case OnTimeDelivery:
		return countResponse(id, snap.OnTimeDeliveryNumerator, "PARTIAL", generated), nil
	case OnTimePickup:
		return countResponse(id, snap.OnTimePickupNumerator, "PARTIAL", generated), nil
	case ReturnCases:
		return countResponse(id, snap.ReturnCaseCount, "COMPLETE", generated), nil
	case RedirectCases:
		return countResponse(id, snap.RedirectCaseCount, "COMPLETE", generated), nil
	case OnTimeDeliveryRate:
		return rateResponse(id, snap.OnTimeDeliveryNumerator, snap.OnTimeDeliveryDenominator, generated)
	case OnTimePickupRate:
		return rateResponse(id, snap.OnTimePickupNumerator, snap.OnTimePickupDenominator, generated)
	case LatePickup:
		value, err := lateCount(snap.OnTimePickupDenominator, snap.OnTimePickupNumerator)
		if err != nil {
			return Response{}, err
		}
		return countResponse(id, value, "PARTIAL", generated), nil
	case LateDelivery:
		value, err := lateCount(snap.OnTimeDeliveryDenominator, snap.OnTimeDeliveryNumerator)
		if err != nil {
			return Response{}, err
		}
		return countResponse(id, value, "PARTIAL", generated), nil
	default:
		return Response{}, errUnknown
	}
}

func BuildCarrier(id string, snap Snapshot, now time.Time) (CarrierResponse, error) {
	if !CarrierKPI(id) {
		return CarrierResponse{}, errUnknown
	}
	generated := now.UTC().Format(time.RFC3339)
	ordered := append([]CarrierSnapshot(nil), snap.Carriers...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].CarrierCompanyID < ordered[j].CarrierCompanyID
	})
	items := make([]CarrierItem, 0, len(ordered))
	for _, carrier := range ordered {
		var numerator, denominator int64
		if id == CarrierPickupRate {
			numerator = carrier.OnTimePickupNumerator
			denominator = carrier.OnTimePickupDenominator
		} else {
			numerator = carrier.OnTimeDeliveryNumerator
			denominator = carrier.OnTimeDeliveryDenominator
		}
		measure, err := ratioMeasure(numerator, denominator)
		if err != nil {
			return CarrierResponse{}, err
		}
		items = append(items, CarrierItem{CarrierCompanyID: carrier.CarrierCompanyID, Measure: measure})
	}
	return CarrierResponse{
		KPIID:             id,
		DefinitionVersion: DefinitionVersion,
		Dimension:         "CARRIER",
		Items:             items,
		GeneratedAt:       generated,
		DataFreshness:     Freshness{Status: "UNKNOWN"},
		Completeness:      "PARTIAL",
	}, nil
}

var errUnknown = errString("unknown kpi")

type errString string

func (e errString) Error() string { return string(e) }

func countResponse(id string, value int64, completeness, generated string) Response {
	number := json.Number(intString(value))
	return Response{
		KPIID:             id,
		DefinitionVersion: DefinitionVersion,
		Measure: Measure{
			Type:  "COUNT",
			Value: &number,
		},
		GeneratedAt:   generated,
		DataFreshness: Freshness{Status: "UNKNOWN"},
		Completeness:  completeness,
	}
}

func lateCount(denominator, numerator int64) (int64, error) {
	if numerator < 0 || denominator < 0 || numerator > denominator {
		return 0, errString("late count would be negative")
	}
	return denominator - numerator, nil
}

func rateResponse(id string, numerator, denominator int64, generated string) (Response, error) {
	measure, err := ratioMeasure(numerator, denominator)
	if err != nil {
		return Response{}, err
	}
	return Response{
		KPIID:             id,
		DefinitionVersion: DefinitionVersion,
		Measure:           measure,
		GeneratedAt:       generated,
		DataFreshness:     Freshness{Status: "UNKNOWN"},
		Completeness:      "PARTIAL",
	}, nil
}

func ratioMeasure(numerator, denominator int64) (Measure, error) {
	measure := Measure{
		Type:        "RATIO",
		Numerator:   &numerator,
		Denominator: &denominator,
	}
	if denominator == 0 {
		if numerator != 0 {
			return Measure{}, errString("empty rate population must have a zero numerator")
		}
		return measure, nil
	}
	number, err := ratioNumber(numerator, denominator)
	if err != nil {
		return Measure{}, err
	}
	measure.Value = &number
	return measure, nil
}

func ratioNumber(numerator, denominator int64) (json.Number, error) {
	if denominator == 0 {
		return "", errString("division by zero")
	}
	ratio := new(big.Rat).SetFrac(big.NewInt(numerator), big.NewInt(denominator))
	text := strings.TrimRight(strings.TrimRight(ratio.FloatString(16), "0"), ".")
	if text == "" || text == "-" || strings.ContainsAny(text, "eEinN") {
		return "", errString("ratio is not a finite decimal")
	}
	return json.Number(text), nil
}

func intString(value int64) string {
	return strings.TrimSpace(json.Number(big.NewInt(value).String()).String())
}
