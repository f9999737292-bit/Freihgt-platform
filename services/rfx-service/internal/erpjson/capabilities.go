package erpjson

// DeferredFieldsV1 is the frozen ERP v1 deferred / out-of-scope field list for capabilities.
func DeferredFieldsV1() []string {
	return []string{
		"lanes",
		"routes",
		"origin",
		"destination",
		"cargo",
		"weight",
		"volume",
		"packaging",
		"vehicle_requirements",
		"incoterms",
		"temperature",
		"hazardous",
		"invited_carriers",
		"mapping_context",
	}
}

// SupportedMappingTypesV1 lists mapping types accepted by ERP v1 (fail-closed and warning-only).
func SupportedMappingTypesV1() []string {
	return []string{
		"CURRENCY",
		"COUNTRY",
		"UNIT",
		"TIMEZONE",
		"CARRIER_CODE",
		"CARGO_TYPE",
		"VEHICLE_BODY",
	}
}
