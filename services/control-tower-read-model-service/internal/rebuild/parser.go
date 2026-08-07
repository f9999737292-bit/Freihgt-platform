package rebuild

import (
	"io"

	"github.com/freight-platform/statussnapshot"
)

func DryRunValidate(r io.Reader) (DryRunReport, error) {
	dec := statussnapshot.NewDecoder(r, statussnapshot.DecoderOptions{})
	report := DryRunReport{
		SchemaVersion:    statussnapshot.SchemaVersionV1,
		ValidationResult: "INVALID",
	}
	for {
		_, err := dec.Next()
		if err == io.EOF {
			if dec.Manifest() == nil {
				return report, &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingManifest}
			}
			if !dec.Completed() {
				return report, &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingCompletion}
			}
			stats := dec.Stats()
			report.Scope = dec.Manifest().Scope
			report.RowCount = stats.RowCount
			report.TenantCount = stats.TenantCount
			report.ChecksumMatched = true
			report.ValidationResult = "VALID"
			return report, nil
		}
		if err != nil {
			if dec.Manifest() != nil {
				report.Scope = dec.Manifest().Scope
			}
			stats := dec.Stats()
			report.RowCount = stats.RowCount
			report.TenantCount = stats.TenantCount
			return report, err
		}
	}
}
