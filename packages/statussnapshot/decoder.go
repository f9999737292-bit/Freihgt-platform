package statussnapshot

import (
	"bufio"
	"bytes"
	"io"

	"github.com/google/uuid"
)

type DecoderOptions struct {
	MaxLineBytes int
}

func (o DecoderOptions) maxLineBytes() int {
	if o.MaxLineBytes <= 0 {
		return DefaultMaxLineBytes
	}
	if o.MaxLineBytes > DefaultMaxLineBytes {
		return DefaultMaxLineBytes
	}
	return o.MaxLineBytes
}

type Decoder struct {
	scanner     *bufio.Scanner
	manifest    *ManifestRecord
	completed   bool
	stats       StreamStats
	checksum    *Checksummer
	prevKey     *ShipmentKey
	lastTenant  uuid.UUID
	hasPrevious bool
	hasTenant   bool
}

func NewDecoder(r io.Reader, options DecoderOptions) *Decoder {
	scanner := bufio.NewScanner(r)
	max := options.maxLineBytes()
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, max)
	return &Decoder{
		scanner:  scanner,
		checksum: NewChecksummer(),
		stats:    StreamStats{ChecksumHex: EmptyStreamChecksumSHA256},
	}
}

func (d *Decoder) Completed() bool {
	return d.completed
}

func (d *Decoder) Stats() StreamStats {
	return d.stats
}

func (d *Decoder) Manifest() *ManifestRecord {
	return d.manifest
}

func (d *Decoder) Next() (Record, error) {
	if d.completed {
		if !d.scanner.Scan() {
			if err := d.scanner.Err(); err != nil {
				return nil, &ValidationError{Code: CodeBrokenStream, Err: err}
			}
			return nil, io.EOF
		}
		return nil, &ValidationError{Code: CodeDataAfterCompletion}
	}
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return nil, &ValidationError{Code: CodeBrokenStream, Err: err}
		}
		return nil, io.EOF
	}
	line := bytes.TrimSpace(d.scanner.Bytes())
	if len(line) == 0 {
		return nil, &ValidationError{Code: CodeInvalidJSON}
	}
	if len(line) > DefaultMaxLineBytes {
		return nil, &ValidationError{Code: CodeRecordTooLarge}
	}

	rec, err := decodeTypedRecord(line)
	if err != nil {
		return nil, err
	}

	switch typed := rec.(type) {
	case ManifestRecord:
		if d.manifest != nil {
			return nil, &ValidationError{Code: CodeDuplicateManifest}
		}
		if err := ValidateManifest(typed); err != nil {
			return nil, err
		}
		d.manifest = &typed
		return typed, nil
	case ShipmentRecord:
		if d.manifest == nil {
			return nil, &ValidationError{Code: CodeMissingManifest}
		}
		if err := ValidateShipment(typed, *d.manifest); err != nil {
			return nil, err
		}
		key := ShipmentKey{TenantID: typed.TenantID, ShipmentID: typed.ShipmentID}
		if err := validateShipmentOrder(d.prevKey, key, d.hasPrevious); err != nil {
			return nil, err
		}
		d.prevKey = &key
		d.hasPrevious = true
		if err := d.checksum.AddCanonicalShipment(typed); err != nil {
			return nil, err
		}
		d.stats.RowCount++
		if !d.hasTenant {
			d.lastTenant = typed.TenantID
			d.hasTenant = true
			d.stats.TenantCount = 1
		} else if typed.TenantID != d.lastTenant {
			d.lastTenant = typed.TenantID
			d.stats.TenantCount++
		}
		d.stats.ChecksumHex = d.checksum.SumHex()
		return typed, nil
	case CompletionRecord:
		if d.manifest == nil {
			return nil, &ValidationError{Code: CodeMissingManifest}
		}
		if err := ValidateCompletion(typed, *d.manifest, d.stats); err != nil {
			return nil, err
		}
		d.completed = true
		return typed, nil
	default:
		return nil, &ValidationError{Code: CodeUnknownRecordType}
	}
}

func ValidateStream(r io.Reader, options DecoderOptions) (StreamStats, error) {
	dec := NewDecoder(r, options)
	for {
		_, err := dec.Next()
		if err == io.EOF {
			if dec.manifest == nil {
				return StreamStats{}, &ValidationError{Code: CodeMissingManifest}
			}
			if !dec.completed {
				return StreamStats{}, &ValidationError{Code: CodeMissingCompletion}
			}
			return dec.Stats(), nil
		}
		if err != nil {
			return StreamStats{}, err
		}
	}
}
