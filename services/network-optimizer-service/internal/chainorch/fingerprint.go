package chainorch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

const timeRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

// ComputeRequestFingerprint hashes frozen I1/ADR-026 request identity.
// It must not include mutable discovered load pools or freshly generated
// wall-clock HorizonStart/HorizonEnd values.
func ComputeRequestFingerprint(in FingerprintInput) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "tenant_id=%s\n", in.TenantID.String())
	fmt.Fprintf(sum, "policy_version=%s\n", in.PolicyVersion)
	fmt.Fprintf(sum, "algorithm_version=%s\n", in.AlgorithmVersion)
	fmt.Fprintf(sum, "catalog_fingerprint=%s\n", in.CatalogFingerprint)
	fmt.Fprintf(sum, "routing_provider_version=%s\n", in.RoutingProviderVersion)
	fmt.Fprintf(sum, "seed_source=%s\n", in.SeedSource)
	writeOptUUID(sum, "seed_capacity_id", in.SeedCapacityID)
	writeOptInt(sum, "seed_capacity_version", in.SeedCapacityVersion)
	writeOptUUID(sum, "seed_shipment_id", in.SeedShipmentID)
	writeOptInt(sum, "seed_shipment_version", in.SeedShipmentVersion)
	writeHorizonAnchors(sum, in)
	fmt.Fprintf(sum, "publication_visibility=%s\n", in.PublicationVisibility)
	fmt.Fprintf(sum, "filter_fingerprint=%s\n", in.FilterFingerprint)
	writeDurationProvenance(sum, in.Duration)
	return hex.EncodeToString(sum.Sum(nil))
}

func writeHorizonAnchors(w io.Writer, in FingerprintInput) {
	fmt.Fprintf(w, "horizon_anchor_policy=%s\n", in.HorizonAnchorPolicy)
	writeOptTime(w, "available_from", in.AvailableFrom)
	writeOptTime(w, "available_until", in.AvailableUntil)
}

func writeDurationProvenance(w io.Writer, snap repository.ServiceDurationSnapshot) {
	fmt.Fprintf(w, "duration_state=%s\n", snap.State)
	writeOptUUID(w, "duration_policy_id", snap.PolicyID)
	writeOptInt(w, "duration_policy_version", snap.PolicyVersion)
	fmt.Fprintf(w, "pickup_known=%t\n", snap.PickupKnown)
	if snap.PickupKnown {
		if snap.PickupSeconds == nil {
			fmt.Fprintf(w, "pickup_seconds=missing\n")
		} else {
			fmt.Fprintf(w, "pickup_seconds=%d\n", *snap.PickupSeconds)
		}
	} else {
		fmt.Fprintf(w, "pickup_seconds=unknown\n")
	}
	fmt.Fprintf(w, "delivery_known=%t\n", snap.DeliveryKnown)
	if snap.DeliveryKnown {
		if snap.DeliverySeconds == nil {
			fmt.Fprintf(w, "delivery_seconds=missing\n")
		} else {
			fmt.Fprintf(w, "delivery_seconds=%d\n", *snap.DeliverySeconds)
		}
	} else {
		fmt.Fprintf(w, "delivery_seconds=unknown\n")
	}
	fmt.Fprintf(w, "duration_fingerprint_provenance=%s\n", snap.FingerprintProvenance)
}

func writeOptUUID(w io.Writer, name string, id *uuid.UUID) {
	if id == nil {
		fmt.Fprintf(w, "%s=\n", name)
		return
	}
	fmt.Fprintf(w, "%s=%s\n", name, id.String())
}

func writeOptInt(w io.Writer, name string, v *int) {
	if v == nil {
		fmt.Fprintf(w, "%s=\n", name)
		return
	}
	fmt.Fprintf(w, "%s=%s\n", name, strconv.Itoa(*v))
}

func writeOptTime(w io.Writer, name string, t *time.Time) {
	if t == nil || t.IsZero() {
		fmt.Fprintf(w, "%s=\n", name)
		return
	}
	fmt.Fprintf(w, "%s=%s\n", name, t.UTC().Format(timeRFC3339Nano))
}
