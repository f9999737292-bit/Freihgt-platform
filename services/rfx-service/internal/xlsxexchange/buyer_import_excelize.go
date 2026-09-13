package xlsxexchange

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

func defaultOpenWorkbook(data []byte) (workbookReader, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return excelizeWorkbook{f: f}, nil
}

type excelizeWorkbook struct {
	f *excelize.File
}

func (w excelizeWorkbook) GetSheetList() []string {
	return w.f.GetSheetList()
}

func (w excelizeWorkbook) GetSheetIndex(sheet string) (int, error) {
	return w.f.GetSheetIndex(sheet)
}

func (w excelizeWorkbook) GetSheetVisible(sheet string) (excelizeSheetVisibility, bool, error) {
	visible, err := w.f.GetSheetVisible(sheet)
	if err != nil {
		return sheetVisible, false, err
	}
	if visible {
		return sheetVisible, true, nil
	}
	return sheetHidden, true, nil
}

func (w excelizeWorkbook) GetMergeCells(sheet string) ([]mergeCellRange, error) {
	merges, err := w.f.GetMergeCells(sheet)
	if err != nil {
		return nil, err
	}
	out := make([]mergeCellRange, 0, len(merges))
	for _, m := range merges {
		out = append(out, mergeCellRange{StartCell: m.GetStartAxis(), EndCell: m.GetEndAxis()})
	}
	return out, nil
}

func (w excelizeWorkbook) GetRows(sheet string) ([][]string, error) {
	return w.f.GetRows(sheet)
}

func (w excelizeWorkbook) GetCellFormula(sheet, cell string) (string, error) {
	return w.f.GetCellFormula(sheet, cell)
}

func (w excelizeWorkbook) GetCellValue(sheet, cell string) (string, error) {
	return w.f.GetCellValue(sheet, cell)
}

func (w excelizeWorkbook) GetRowVisible(sheet string, row int) (bool, error) {
	return w.f.GetRowVisible(sheet, row)
}

func (w excelizeWorkbook) GetColVisible(sheet, col string) (bool, error) {
	return w.f.GetColVisible(sheet, col)
}

func (w excelizeWorkbook) Close() error {
	return w.f.Close()
}

func sheetIndexByName(list []string, name string) int {
	for idx, item := range list {
		if item == name {
			return idx
		}
	}
	return -1
}

func rowHasAnyValue(row []string) bool {
	for _, cell := range row {
		if trimCell(cell) != "" {
			return true
		}
	}
	return false
}

func trimCell(value string) string {
	return stringsTrimSpace(value)
}

func stringsTrimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && isSpace(s[i]) {
		i++
	}
	for j > i && isSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func cellName(col, row int) string {
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return fmt.Sprintf("%c%d", 'A'+col-1, row)
	}
	return name
}

func columnName(col int) string {
	name, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return fmt.Sprintf("COL%d", col)
	}
	return name
}
