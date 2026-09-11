package xlsxsecurity

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestInspectUploadRejectsNonZip(t *testing.T) {
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("not-a-zip"), DefaultLimits())
	if err == nil {
		t.Fatal("expected invalid zip rejection")
	}
}

func TestInspectUploadAcceptsMinimalWorkbookWithoutFormulas(t *testing.T) {
	data := buildMinimalXLSX(t, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`)
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, DefaultLimits())
	if err != nil {
		t.Fatalf("expected valid workbook: %v", err)
	}
}

func TestInspectUploadRejectsFormulaCell(t *testing.T) {
	data := buildMinimalXLSX(t, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><f>SUM(1,2)</f></c></row></sheetData></worksheet>`)
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, DefaultLimits())
	if err == nil {
		t.Fatal("expected formula rejection")
	}
}

func TestInspectUploadRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.xml")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := w.Write([]byte("<x/>")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err = InspectUpload("application/zip", buf.Bytes(), DefaultLimits())
	if err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func buildMinimalXLSX(t *testing.T, worksheetXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	contentTypes := `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`
	for name, payload := range map[string]string{
		"[Content_Types].xml":      contentTypes,
		"xl/workbook.xml":          `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`,
		"xl/worksheets/sheet1.xml": worksheetXML,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(payload)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
