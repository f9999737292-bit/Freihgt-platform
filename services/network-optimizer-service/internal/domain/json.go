package domain

import "encoding/json"

func MarshalLoad(l LoadOpportunity) ([]byte, error) {
	doc := map[string]any{
		"id": l.ID, "owner_tenant_id": l.OwnerTenantID,
		"source_type": l.SourceType, "source_id": l.SourceID,
		"pickup": l.Pickup, "pickup_window": l.PickupWindow,
		"delivery": l.Delivery, "delivery_window": l.DeliveryWindow,
		"visibility_scope": l.VisibilityScope, "status": l.Status,
		"version": l.Version, "created_at": l.CreatedAt, "updated_at": l.UpdatedAt,
	}
	putFloat(doc, "weight_kg", l.WeightKg)
	putFloat(doc, "volume_m3", l.VolumeM3)
	putString(doc, "body_type", l.BodyType)
	putSlice(doc, "equipment", l.Equipment)
	if l.Cargo != (CargoConstraints{}) {
		doc["cargo"] = l.Cargo
	}
	if l.Commercial != (Commercial{}) {
		doc["commercial"] = l.Commercial
	}
	if len(l.InvitedCarrierCompanyIDs) > 0 {
		doc["invited_carrier_company_ids"] = l.InvitedCarrierCompanyIDs
	}
	return json.Marshal(doc)
}

func MarshalMarketplaceLoad(l LoadOpportunity) ([]byte, error) {
	view := l.MarketplaceView()
	doc := map[string]any{
		"id": view.ID, "pickup_window": view.PickupWindow, "delivery_window": view.DeliveryWindow,
		"visibility_scope": view.VisibilityScope, "status": view.Status,
		"version": view.Version, "created_at": view.CreatedAt, "updated_at": view.UpdatedAt,
	}
	if l.VisibilityScope != VisAnonymized {
		doc["pickup"] = view.Pickup
		doc["delivery"] = view.Delivery
	}
	if view.OwnerTenantID != nil {
		doc["owner_tenant_id"] = *view.OwnerTenantID
	}
	putFloat(doc, "weight_kg", view.WeightKg)
	putFloat(doc, "volume_m3", view.VolumeM3)
	putString(doc, "body_type", view.BodyType)
	putSlice(doc, "equipment", view.Equipment)
	if view.Cargo != (CargoConstraints{}) {
		doc["cargo"] = view.Cargo
	}
	if view.Commercial != nil {
		doc["commercial"] = view.Commercial
	}
	return json.Marshal(doc)
}

func MarshalCapacity(c Capacity) ([]byte, error) {
	doc := map[string]any{
		"id": c.ID, "owner_tenant_id": c.OwnerTenantID,
		"available_from": c.AvailableFrom, "available_until": c.AvailableUntil,
		"source": c.Source, "visibility_scope": c.VisibilityScope,
		"status": c.Status, "version": c.Version,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
	if c.CarrierCompanyID != nil {
		doc["carrier_company_id"] = *c.CarrierCompanyID
	}
	if c.VehicleID != nil {
		doc["vehicle_id"] = *c.VehicleID
	}
	putString(doc, "location_label", c.LocationLabel)
	putFloat(doc, "latitude", c.Latitude)
	putFloat(doc, "longitude", c.Longitude)
	putString(doc, "body_type", c.BodyType)
	putSlice(doc, "equipment", c.Equipment)
	putFloat(doc, "payload_remaining_kg", c.PayloadRemainingKg)
	putFloat(doc, "volume_remaining_m3", c.VolumeRemainingM3)
	if len(c.AudienceTenantIDs) > 0 {
		doc["audience_tenant_ids"] = c.AudienceTenantIDs
	}
	return json.Marshal(doc)
}

func MarshalMarketplaceCapacity(c Capacity) ([]byte, error) {
	view := c.MarketplaceView()
	doc := map[string]any{
		"id": view.ID, "available_from": view.AvailableFrom, "available_until": view.AvailableUntil,
		"source": view.Source, "visibility_scope": view.VisibilityScope,
		"status": view.Status, "version": view.Version,
		"created_at": view.CreatedAt, "updated_at": view.UpdatedAt,
	}
	if view.OwnerTenantID != nil {
		doc["owner_tenant_id"] = *view.OwnerTenantID
	}
	if view.CarrierCompanyID != nil {
		doc["carrier_company_id"] = *view.CarrierCompanyID
	}
	if view.VehicleID != nil {
		doc["vehicle_id"] = *view.VehicleID
	}
	if c.VisibilityScope != CapVisAnonymized {
		putString(doc, "location_label", view.LocationLabel)
		putFloat(doc, "latitude", view.Latitude)
		putFloat(doc, "longitude", view.Longitude)
	}
	putString(doc, "body_type", view.BodyType)
	putSlice(doc, "equipment", view.Equipment)
	putFloat(doc, "payload_remaining_kg", view.PayloadRemainingKg)
	putFloat(doc, "volume_remaining_m3", view.VolumeRemainingM3)
	return json.Marshal(doc)
}

func putFloat(doc map[string]any, key string, v *float64) {
	if v != nil {
		doc[key] = *v
	}
}

func putString(doc map[string]any, key, v string) {
	if v != "" {
		doc[key] = v
	}
}

func putSlice(doc map[string]any, key string, v []string) {
	if len(v) > 0 {
		doc[key] = v
	}
}
