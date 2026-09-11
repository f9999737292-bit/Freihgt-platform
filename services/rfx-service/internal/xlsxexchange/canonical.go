package xlsxexchange

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

const metadataExportedAtKey = "exported_at_utc"

// WorkbookCanonical is a semantic snapshot of workbook cell values for comparison.
type WorkbookCanonical struct {
	Sheets map[string][][]string
}

// CanonicalWorkbookSnapshot parses workbook bytes into a deterministic semantic snapshot,
// excluding exported_at_utc from the Metadata sheet.
func CanonicalWorkbookSnapshot(data []byte) (WorkbookCanonical, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return WorkbookCanonical{}, fmt.Errorf("open workbook: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	out := WorkbookCanonical{Sheets: make(map[string][][]string, len(buyerSheetOrder))}
	for _, sheetName := range buyerSheetOrder {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return WorkbookCanonical{}, fmt.Errorf("read sheet %s: %w", sheetName, err)
		}
		if sheetName == sheetMetadata {
			rows = excludeMetadataKey(rows, metadataExportedAtKey)
		}
		out.Sheets[sheetName] = normalizeRows(rows)
	}
	return out, nil
}

func excludeMetadataKey(rows [][]string, key string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	filtered := make([][]string, 0, len(rows))
	for _, row := range rows {
		if len(row) >= 1 && strings.TrimSpace(row[0]) == key {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func normalizeRows(rows [][]string) [][]string {
	if len(rows) == 0 {
		return nil
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	normalized := make([][]string, len(rows))
	for rowIdx, row := range rows {
		copied := make([]string, maxCols)
		copy(copied, row)
		normalized[rowIdx] = copied
	}
	return normalized
}
