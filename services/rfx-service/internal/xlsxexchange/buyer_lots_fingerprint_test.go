package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestBaselineLotsFingerprintOrderIndependent(t *testing.T) {
	t.Parallel()
	lotsA := []domain.RfxLot{
		{LotNumber: "L-002", Name: "B", Status: "ACTIVE"},
		{LotNumber: "L-001", Name: "A", Status: "ACTIVE"},
	}
	lotsB := []domain.RfxLot{
		{LotNumber: "L-001", Name: "A", Status: "ACTIVE"},
		{LotNumber: "L-002", Name: "B", Status: "ACTIVE"},
	}
	hashA, err := ComputeBaselineLotsFingerprint(lotsA)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := ComputeBaselineLotsFingerprint(lotsB)
	if err != nil {
		t.Fatal(err)
	}
	if hashA != hashB {
		t.Fatalf("order changed fingerprint: %s vs %s", hashA, hashB)
	}
}

func TestBaselineLotsFingerprintFieldSensitivity(t *testing.T) {
	t.Parallel()
	base := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"}}
	baseHash, err := ComputeBaselineLotsFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	changedName := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot changed", Status: "ACTIVE"}}
	changedHash, err := ComputeBaselineLotsFingerprint(changedName)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash == changedHash {
		t.Fatal("name change should alter fingerprint")
	}
}

func TestBaselineLotsFingerprintIgnoresUUIDAndTimestamps(t *testing.T) {
	t.Parallel()
	a := []domain.RfxLot{
		{ID: uuid.New(), TenantID: uuid.New(), RfxEventID: uuid.New(), LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"},
	}
	b := []domain.RfxLot{
		{ID: uuid.New(), TenantID: uuid.New(), RfxEventID: uuid.New(), LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"},
	}
	hashA, err := ComputeBaselineLotsFingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := ComputeBaselineLotsFingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if hashA != hashB {
		t.Fatal("UUID fields must not affect fingerprint")
	}
}

func TestBaselineLotsFingerprintNilEmptyNormalization(t *testing.T) {
	t.Parallel()
	desc := ""
	cat := ""
	cur := ""
	var val *float64
	withEmpty := []domain.RfxLot{{
		LotNumber: "L-1", Name: "Lot", Description: &desc, Category: &cat,
		CurrencyCode: &cur, EstimatedValue: val, Status: "ACTIVE",
	}}
	withNil := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"}}
	hashA, err := ComputeBaselineLotsFingerprint(withEmpty)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := ComputeBaselineLotsFingerprint(withNil)
	if err != nil {
		t.Fatal(err)
	}
	if hashA != hashB {
		t.Fatal("nil/empty optional fields should normalize consistently")
	}
}

func TestBaselineLotsFingerprintJSONBRoundTrip(t *testing.T) {
	t.Parallel()
	lots := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"}}
	hashBefore, err := ComputeBaselineLotsFingerprint(lots)
	if err != nil {
		t.Fatal(err)
	}
	entries := baselineLotFingerprintEntries(lots)
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	pgLike, err := normalizeJSONDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []baselineLotFingerprintEntry
	if err := json.Unmarshal(pgLike, &decoded); err != nil {
		t.Fatal(err)
	}
	reencoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256Sum(reencoded)
	if hashBefore != sum {
		t.Fatalf("round-trip changed fingerprint: before=%s after=%s", hashBefore, sum)
	}
}

func TestProposedLotsHashDistinctFromBaseline(t *testing.T) {
	t.Parallel()
	serverLots := []domain.RfxLot{{LotNumber: "L-1", Name: "Server", Status: "ACTIVE"}}
	baseline, err := ComputeBaselineLotsFingerprint(serverLots)
	if err != nil {
		t.Fatal(err)
	}
	proposed, err := ComputeProposedLotsHash([]canonicalLot{{LotNumber: "L-1", Name: "Imported", Status: "ACTIVE"}})
	if err != nil {
		t.Fatal(err)
	}
	if baseline == proposed {
		t.Fatal("baseline and proposed hashes must differ when content differs")
	}
}

func TestContentEquivalentBaselineFingerprint(t *testing.T) {
	t.Parallel()
	original := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"}}
	originalHash, err := ComputeBaselineLotsFingerprint(original)
	if err != nil {
		t.Fatal(err)
	}
	changed := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot changed", Status: "ACTIVE"}}
	changedHash, err := ComputeBaselineLotsFingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if originalHash == changedHash {
		t.Fatal("non-equivalent change must alter fingerprint")
	}
	restored := []domain.RfxLot{{LotNumber: "L-1", Name: "Lot", Status: "ACTIVE"}}
	restoredHash, err := ComputeBaselineLotsFingerprint(restored)
	if err != nil {
		t.Fatal(err)
	}
	if originalHash != restoredHash {
		t.Fatal("content-equivalent restore must match original fingerprint")
	}
}

func sha256Sum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
