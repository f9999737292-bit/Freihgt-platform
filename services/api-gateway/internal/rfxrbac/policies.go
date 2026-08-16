package rfxrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var buyerManageRoles = map[string]struct{}{
	"PLATFORM_ADMIN":      {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
}

var buyerReadRoles = buyerManageRoles

var carrierRespondRoles = map[string]struct{}{
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var carrierReadRoles = carrierRespondRoles

func CanBuyerManage(roles []string) bool {
	return routeauth.HasAnyRole(roles, buyerManageRoles)
}

func CanBuyerRead(roles []string) bool {
	return routeauth.HasAnyRole(roles, buyerReadRoles)
}

func CanCarrierRespond(roles []string) bool {
	return routeauth.HasAnyRole(roles, carrierRespondRoles)
}

func CanCarrierRead(roles []string) bool {
	return routeauth.HasAnyRole(roles, carrierReadRoles)
}

func CanReadRfxEvents(roles []string) bool {
	return CanBuyerRead(roles) || CanCarrierRead(roles)
}

func CanReadFreightRequests(roles []string) bool {
	return CanBuyerRead(roles) || CanCarrierRead(roles)
}

func CanReadBids(roles []string) bool {
	return CanBuyerRead(roles) || CanCarrierRead(roles)
}
