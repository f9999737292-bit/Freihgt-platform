package kpi

import (
	"encoding/json"
	"math/big"
	"strings"
	"time"
)

const DefinitionVersion = 1

const (
	ShipmentsTotal     = "OPS_SHIPMENTS_TOTAL"
	OnTimeDelivery     = "OPS_ON_TIME_DELIVERY"
	OnTimeDeliveryRate = "OPS_ON_TIME_DELIVERY_RATE"
	ReturnCases        = "OPS_RETURN_CASES"
	RedirectCases      = "OPS_REDIRECT_CASES"
)

type Snapshot struct {
	TenantID                  string
	ShipmentTotal             int64
	OnTimeDeliveryDenominator int64
	OnTimeDeliveryNumerator   int64
	ReturnCaseCount           int64
	RedirectCaseCount         int64
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

func Known(id string) bool {
	switch id {
	case ShipmentsTotal, OnTimeDelivery, OnTimeDeliveryRate, ReturnCases, RedirectCases:
		return true
	default:
		return false
	}
}

func Build(id string, snap Snapshot, now time.Time) (Response, error) {
	generated := now.UTC().Format(time.RFC3339)
	switch id {
	case ShipmentsTotal:
		return countResponse(id, snap.ShipmentTotal, "COMPLETE", generated), nil
	case OnTimeDelivery:
		return countResponse(id, snap.OnTimeDeliveryNumerator, "PARTIAL", generated), nil
	case ReturnCases:
		return countResponse(id, snap.ReturnCaseCount, "COMPLETE", generated), nil
	case RedirectCases:
		return countResponse(id, snap.RedirectCaseCount, "COMPLETE", generated), nil
	case OnTimeDeliveryRate:
		return rateResponse(snap.OnTimeDeliveryNumerator, snap.OnTimeDeliveryDenominator, generated)
	default:
		return Response{}, errUnknown
	}
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

func rateResponse(numerator, denominator int64, generated string) (Response, error) {
	response := Response{
		KPIID:             OnTimeDeliveryRate,
		DefinitionVersion: DefinitionVersion,
		Measure: Measure{
			Type:        "RATIO",
			Numerator:   &numerator,
			Denominator: &denominator,
		},
		GeneratedAt:   generated,
		DataFreshness: Freshness{Status: "UNKNOWN"},
		Completeness:  "PARTIAL",
	}
	if denominator == 0 {
		if numerator != 0 {
			return Response{}, errString("empty rate population must have a zero numerator")
		}
		return response, nil
	}
	number, err := ratioNumber(numerator, denominator)
	if err != nil {
		return Response{}, err
	}
	response.Measure.Value = &number
	return response, nil
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
