package shipmentevents

var allowedMetadataKeys = map[string]struct{}{
	"documentId":            {},
	"documentType":          {},
	"documentStatus":        {},
	"billingRegisterId":     {},
	"billingRegisterNumber": {},
	"billingStatus":         {},
	"plannedAt":             {},
	"actualAt":              {},
	"delayMinutes":          {},
	"slaReason":             {},
	"slaStatus":             {},
}

func sanitizeMetadata(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		if _, ok := allowedMetadataKeys[key]; !ok {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func documentMetadata(doc rawDocument) map[string]interface{} {
	return sanitizeMetadata(map[string]interface{}{
		"documentId":     doc.ID,
		"documentType":   doc.DocumentType,
		"documentStatus": doc.DocumentStatus,
	})
}
