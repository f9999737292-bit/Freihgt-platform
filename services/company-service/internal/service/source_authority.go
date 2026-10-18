package service

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/freight-platform/company-service/internal/domain"
)

const RelationshipServiceTokenHeader = "X-Internal-Service-Token"

type FailClosedSourceAuthority struct{}

func (FailClosedSourceAuthority) ConfirmCreate(context.Context, domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	return domain.SourceCreateUnavailable, nil
}

func identifyRelationshipCaller(shipmentToken, rfxToken, presented string) (string, bool) {
	shipmentToken = strings.TrimSpace(shipmentToken)
	rfxToken = strings.TrimSpace(rfxToken)
	presented = strings.TrimSpace(presented)
	if presented == "" || (shipmentToken == "" && rfxToken == "") {
		return "", false
	}
	if shipmentToken != "" && shipmentToken == rfxToken {
		return "", false
	}
	shipmentMatch := shipmentToken != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(shipmentToken)) == 1
	rfxMatch := rfxToken != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(rfxToken)) == 1
	if shipmentMatch == rfxMatch {
		return "", false
	}
	if shipmentMatch {
		return domain.CallerShipmentService, true
	}
	return domain.CallerRfxService, true
}
