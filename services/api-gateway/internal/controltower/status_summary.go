package controltower

// BuildLegacyStatusSummary derives tenant-scoped status counts from the same
// shipment-service list fetch used by legacy Control Tower aggregation.
// ByStatus is computed from fetched rows; totalShipments uses the service total.
func BuildLegacyStatusSummary(shipments []rawShipment, total int) (totalShipments int64, countedShipments int64, byStatus map[string]int64, limitedDataset bool) {
	byStatus = map[string]int64{}
	for _, shipment := range shipments {
		if shipment.Status == "" {
			continue
		}
		byStatus[shipment.Status]++
	}
	if total <= 0 {
		total = len(shipments)
	}
	totalShipments = int64(total)
	countedShipments = int64(len(shipments))
	limitedDataset = total > len(shipments)
	return totalShipments, countedShipments, byStatus, limitedDataset
}
