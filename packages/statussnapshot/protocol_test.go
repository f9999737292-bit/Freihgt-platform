package statussnapshot

import (
	"bytes"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func sampleManifest(id uuid.UUID) ManifestRecord {
	return ManifestRecord{
		RecordType: RecordTypeManifest, SchemaVersion: SchemaVersionV1, SnapshotID: id,
		Scope: ScopeAll, Ordering: OrderingTenantIDShipmentID,
		StartedAt: time.Now().UTC(), TransactionIsolation: IsolationRepeatableRead,
		Source: SourceShipmentService,
	}
}

func tenantManifest(id, tenantID uuid.UUID) ManifestRecord {
	m := sampleManifest(id)
	m.Scope = ScopeTenant
	m.TenantID = &tenantID
	return m
}

func sampleShipment(snapshotID, tenantID, shipmentID uuid.UUID) ShipmentRecord {
	prev := "CARRIER_ASSIGNED"
	eventID := uuid.New()
	sourceID := uuid.New()
	eventType := "shipment.status.changed"
	return ShipmentRecord{
		RecordType: RecordTypeShipment, SchemaVersion: SchemaVersionV1, SnapshotID: snapshotID,
		TenantID: tenantID, ShipmentID: shipmentID, CurrentStatus: "IN_TRANSIT", PreviousStatus: &prev,
		AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, LastEventType: &eventType,
		SourceUpdatedAt: time.Now().UTC(),
	}
}

func buildStream(t *testing.T, shipments int) []byte {
	t.Helper()
	id := uuid.New()
	tenantID := uuid.New()
	checksum := NewChecksummer()
	var buf bytes.Buffer
	m := sampleManifest(id)
	line, _ := MarshalNDJSON(m)
	buf.Write(line)
	shipIDs := make([]uuid.UUID, shipments)
	for i := 0; i < shipments; i++ {
		shipIDs[i] = uuid.New()
	}
	sort.Slice(shipIDs, func(i, j int) bool { return shipIDs[i].String() < shipIDs[j].String() })
	for _, shipID := range shipIDs {
		rec := sampleShipment(id, tenantID, shipID)
		_ = checksum.AddCanonicalShipment(rec)
		sline, _ := MarshalNDJSON(rec)
		buf.Write(sline)
	}
	c := CompletionRecord{
		RecordType: RecordTypeCompletion, SchemaVersion: SchemaVersionV1, SnapshotID: id,
		RowCount: int64(shipments), TenantCount: boolTenantCount(shipments), SHA256: checksum.SumHex(), CompletedAt: time.Now().UTC(),
	}
	cline, _ := MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func boolTenantCount(shipments int) int64 {
	if shipments == 0 {
		return 0
	}
	return 1
}

func TestValidManifestEncodeDecode(t *testing.T) {
	id := uuid.New()
	rec := sampleManifest(id)
	raw, err := MarshalNDJSON(rec)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTypedRecord(bytes.TrimSpace(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifest(decoded.(ManifestRecord)); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownRecordType(t *testing.T) {
	_, err := decodeTypedRecord([]byte(`{"recordType":"foo"}`))
	if err == nil || !strings.Contains(err.Error(), CodeUnknownRecordType) {
		t.Fatalf("expected unknown record type, got %v", err)
	}
}

func TestUnsupportedSchemaVersion(t *testing.T) {
	rec := sampleManifest(uuid.New())
	rec.SchemaVersion = 2
	if err := ValidateManifest(rec); err == nil {
		t.Fatal("expected unsupported schema")
	}
}

func TestZeroSnapshotUUID(t *testing.T) {
	rec := sampleManifest(uuid.Nil)
	if err := ValidateManifest(rec); err == nil {
		t.Fatal("expected invalid uuid")
	}
}

func TestZeroTenantUUID(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.Nil, uuid.New())
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err == nil {
		t.Fatal("expected invalid tenant")
	}
}

func TestUnknownStatus(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	rec.CurrentStatus = "CREATED"
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err == nil {
		t.Fatal("expected unknown status")
	}
}

func TestVersionZero(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	rec.AggregateVersion = 0
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err == nil {
		t.Fatal("expected invalid version")
	}
}

func TestStableChecksum(t *testing.T) {
	id, tenant, ship := uuid.New(), uuid.New(), uuid.New()
	rec := sampleShipment(id, tenant, ship)
	a := NewChecksummer()
	b := NewChecksummer()
	_ = a.AddCanonicalShipment(rec)
	_ = b.AddCanonicalShipment(rec)
	if a.SumHex() != b.SumHex() {
		t.Fatal("checksum not stable")
	}
}

func TestChecksumChangesOnRecordChange(t *testing.T) {
	rec1 := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	rec2 := sampleShipment(rec1.SnapshotID, rec1.TenantID, rec1.ShipmentID)
	rec2.CurrentStatus = "DELIVERED"
	a, b := NewChecksummer(), NewChecksummer()
	_ = a.AddCanonicalShipment(rec1)
	_ = b.AddCanonicalShipment(rec2)
	if a.SumHex() == b.SumHex() {
		t.Fatal("checksum should change")
	}
}

func TestEmptyStreamChecksum(t *testing.T) {
	if NewChecksummer().SumHex() != EmptyStreamChecksumSHA256 {
		t.Fatalf("expected empty checksum %s", EmptyStreamChecksumSHA256)
	}
}

func TestEmptyAllScopeSnapshot(t *testing.T) {
	id := uuid.New()
	var buf bytes.Buffer
	mline, _ := MarshalNDJSON(sampleManifest(id))
	buf.Write(mline)
	c := CompletionRecord{
		RecordType: RecordTypeCompletion, SchemaVersion: SchemaVersionV1, SnapshotID: id,
		RowCount: 0, TenantCount: 0, SHA256: EmptyStreamChecksumSHA256, CompletedAt: time.Now().UTC(),
	}
	cline, _ := MarshalNDJSON(c)
	buf.Write(cline)
	stats, err := ValidateStream(bytes.NewReader(buf.Bytes()), DecoderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != 0 || stats.TenantCount != 0 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	if stats.ChecksumHex != EmptyStreamChecksumSHA256 {
		t.Fatalf("checksum mismatch: %s", stats.ChecksumHex)
	}
}

func TestEmptyTenantScopeSnapshot(t *testing.T) {
	id, tenantID := uuid.New(), uuid.New()
	var buf bytes.Buffer
	mline, _ := MarshalNDJSON(tenantManifest(id, tenantID))
	buf.Write(mline)
	c := CompletionRecord{
		RecordType: RecordTypeCompletion, SchemaVersion: SchemaVersionV1, SnapshotID: id,
		RowCount: 0, TenantCount: 0, SHA256: EmptyStreamChecksumSHA256, CompletedAt: time.Now().UTC(),
	}
	cline, _ := MarshalNDJSON(c)
	buf.Write(cline)
	stats, err := ValidateStream(bytes.NewReader(buf.Bytes()), DecoderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TenantCount != 0 {
		t.Fatalf("tenant scope empty snapshot must have tenantCount=0, got %d", stats.TenantCount)
	}
}

func TestValidateStreamValid(t *testing.T) {
	raw := buildStream(t, 2)
	stats, err := ValidateStream(bytes.NewReader(raw), DecoderOptions{})
	if err != nil || stats.RowCount != 2 {
		t.Fatalf("valid stream failed: %v stats=%+v", err, stats)
	}
}

func TestMissingManifest(t *testing.T) {
	_, err := ValidateStream(bytes.NewReader([]byte("")), DecoderOptions{})
	if err == nil {
		t.Fatal("expected missing manifest")
	}
}

func TestDuplicateManifest(t *testing.T) {
	raw := buildStream(t, 1)
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	dup := append(lines[:1], lines...)
	_, err := ValidateStream(bytes.NewReader(bytes.Join(dup[:2], []byte("\n"))), DecoderOptions{})
	if err == nil {
		t.Fatal("expected duplicate manifest")
	}
}

func TestDataAfterCompletion(t *testing.T) {
	raw := buildStream(t, 1)
	corrupt := append(raw, []byte("{\"recordType\":\"shipment\"}\n")...)
	_, err := ValidateStream(bytes.NewReader(corrupt), DecoderOptions{})
	if err == nil {
		t.Fatal("expected data after completion")
	}
}

func TestSnapshotIDMismatch(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	if err := ValidateShipment(rec, sampleManifest(uuid.New())); err == nil {
		t.Fatal("expected snapshot mismatch")
	}
}

func TestMissingCompletion(t *testing.T) {
	id := uuid.New()
	line, _ := MarshalNDJSON(sampleManifest(id))
	_, err := ValidateStream(bytes.NewReader(line), DecoderOptions{})
	if err == nil {
		t.Fatal("expected missing completion")
	}
}

func TestShipmentBeforeManifest(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	line, _ := MarshalNDJSON(rec)
	_, err := ValidateStream(bytes.NewReader(line), DecoderOptions{})
	if err == nil {
		t.Fatal("expected missing manifest")
	}
}

func TestInvalidTimestamp(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	rec.SourceUpdatedAt = time.Time{}
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err == nil {
		t.Fatal("expected invalid timestamp")
	}
}

func TestNewlineSemantics(t *testing.T) {
	var buf bytes.Buffer
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	if err := EncodeCanonicalShipment(&buf, rec); err != nil {
		t.Fatal(err)
	}
	if buf.Bytes()[len(buf.Bytes())-1] != '\n' {
		t.Fatal("expected trailing newline")
	}
}

func TestOversizedRecordRejected(t *testing.T) {
	dec := NewDecoder(bytes.NewReader([]byte(strings.Repeat("a", DefaultMaxLineBytes+1))), DecoderOptions{})
	_, err := dec.Next()
	if err == nil {
		t.Fatal("expected oversized record error")
	}
}

func TestEmptyLineRejected(t *testing.T) {
	dec := NewDecoder(bytes.NewReader([]byte("\n")), DecoderOptions{})
	_, err := dec.Next()
	if err == nil {
		t.Fatal("expected invalid json on empty line")
	}
}

func TestDuplicateShipmentInStream(t *testing.T) {
	id, tenant, ship := uuid.New(), uuid.New(), uuid.New()
	checksum := NewChecksummer()
	var buf bytes.Buffer
	mline, _ := MarshalNDJSON(sampleManifest(id))
	buf.Write(mline)
	rec := sampleShipment(id, tenant, ship)
	_ = checksum.AddCanonicalShipment(rec)
	sline, _ := MarshalNDJSON(rec)
	buf.Write(sline)
	buf.Write(sline)
	_, err := ValidateStream(bytes.NewReader(buf.Bytes()), DecoderOptions{})
	if err == nil || ValidationCode(err) != CodeDuplicateShipment {
		t.Fatalf("expected duplicate shipment, got %v", err)
	}
}

func TestRecordOrderViolation(t *testing.T) {
	id := uuid.New()
	tenantA, tenantB := uuid.New(), uuid.New()
	if tenantA.String() > tenantB.String() {
		tenantA, tenantB = tenantB, tenantA
	}
	checksum := NewChecksummer()
	var buf bytes.Buffer
	mline, _ := MarshalNDJSON(sampleManifest(id))
	buf.Write(mline)
	recA := sampleShipment(id, tenantB, uuid.New())
	recB := sampleShipment(id, tenantA, uuid.New())
	_ = checksum.AddCanonicalShipment(recA)
	_ = checksum.AddCanonicalShipment(recB)
	la, _ := MarshalNDJSON(recA)
	lb, _ := MarshalNDJSON(recB)
	buf.Write(la)
	buf.Write(lb)
	_, err := ValidateStream(bytes.NewReader(buf.Bytes()), DecoderOptions{})
	if err == nil || ValidationCode(err) != CodeRecordOrderViolation {
		t.Fatalf("expected order violation, got %v", err)
	}
}

func TestInvalidOrdering(t *testing.T) {
	rec := sampleManifest(uuid.New())
	rec.Ordering = "SHIPMENT_ID"
	if err := ValidateManifest(rec); err == nil || ValidationCode(err) != CodeInvalidOrdering {
		t.Fatalf("expected invalid ordering, got %v", err)
	}
}

func TestTenantScopeMismatch(t *testing.T) {
	id, tenantA, tenantB := uuid.New(), uuid.New(), uuid.New()
	var buf bytes.Buffer
	mline, _ := MarshalNDJSON(tenantManifest(id, tenantA))
	buf.Write(mline)
	rec := sampleShipment(id, tenantB, uuid.New())
	sline, _ := MarshalNDJSON(rec)
	buf.Write(sline)
	_, err := ValidateStream(bytes.NewReader(buf.Bytes()), DecoderOptions{})
	if err == nil || ValidationCode(err) != CodeTenantScopeMismatch {
		t.Fatalf("expected tenant scope mismatch, got %v", err)
	}
}

func TestTenantScopeRequiresManifestTenantID(t *testing.T) {
	rec := sampleManifest(uuid.New())
	rec.Scope = ScopeTenant
	if err := ValidateManifest(rec); err == nil {
		t.Fatal("expected tenant scope without tenantId to fail")
	}
}

func TestAllScopeRejectsManifestTenantID(t *testing.T) {
	rec := sampleManifest(uuid.New())
	tid := uuid.New()
	rec.TenantID = &tid
	if err := ValidateManifest(rec); err == nil {
		t.Fatal("expected ALL scope with tenantId to fail")
	}
}

func TestInconsistentMetadata(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	eventID := uuid.New()
	rec.LastEventID = &eventID
	rec.LastSourceEventID = nil
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err == nil || ValidationCode(err) != CodeInconsistentMetadata {
		t.Fatalf("expected inconsistent metadata, got %v", err)
	}
}

func TestLastSourceEventIDWithoutLastEventIDAllowed(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	sourceID := uuid.New()
	rec.LastEventID = nil
	rec.LastSourceEventID = &sourceID
	if err := ValidateShipment(rec, sampleManifest(rec.SnapshotID)); err != nil {
		t.Fatalf("lastSourceEventId without lastEventId should be allowed: %v", err)
	}
}

func TestWrongChecksum(t *testing.T) {
	raw := buildStream(t, 1)
	text := string(raw)
	idx := strings.LastIndex(text, `"sha256":"`)
	if idx < 0 {
		t.Fatal("no checksum field")
	}
	corrupt := text[:idx+10] + strings.Repeat("a", 64) + text[idx+74:]
	_, err := ValidateStream(strings.NewReader(corrupt), DecoderOptions{})
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestValidCompletionEncodeDecode(t *testing.T) {
	id := uuid.New()
	started := time.Now().UTC()
	sha := EmptyStreamChecksumSHA256
	manifest := sampleManifest(id)
	manifest.StartedAt = started
	c := CompletionRecord{RecordType: RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: 0, TenantCount: 0, SHA256: sha, CompletedAt: started.Add(time.Second)}
	raw, _ := MarshalNDJSON(c)
	decoded, err := decodeTypedRecord(bytes.TrimSpace(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCompletion(decoded.(CompletionRecord), manifest, StreamStats{ChecksumHex: sha}); err != nil {
		t.Fatal(err)
	}
}

func TestValidShipmentEncodeDecode(t *testing.T) {
	rec := sampleShipment(uuid.New(), uuid.New(), uuid.New())
	raw, _ := MarshalNDJSON(rec)
	decoded, err := decodeTypedRecord(bytes.TrimSpace(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateShipment(decoded.(ShipmentRecord), sampleManifest(rec.SnapshotID)); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolV1StatusesSorted(t *testing.T) {
	statuses := ProtocolV1ShipmentStatuses()
	if len(statuses) != len(KnownShipmentStatuses) {
		t.Fatalf("status list length mismatch")
	}
	for i := 1; i < len(statuses); i++ {
		if statuses[i] <= statuses[i-1] {
			t.Fatal("statuses must be sorted")
		}
	}
}
