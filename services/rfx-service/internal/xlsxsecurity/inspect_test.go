package xlsxsecurity

import (
	"archive/zip"
	"bytes"
	"strings"
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

func TestInspectUploadRejectsDuplicateZipEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	payload := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`)
	for _, name := range []string{"[Content_Types].xml", "xl/worksheets/sheet1.xml", "./xl/worksheets/sheet1.xml"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes(), DefaultLimits())
	if err == nil {
		t.Fatal("expected duplicate zip entry rejection")
	}
}

func TestInspectUploadRejectsMacroVBA(t *testing.T) {
	data := buildZipWithEntries(t, map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="bin" ContentType="application/vnd.ms-office.vbaProject"/></Types>`,
		"xl/vbaProject.bin":   "macro",
	})
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, DefaultLimits())
	if err == nil {
		t.Fatal("expected macro/VBA rejection")
	}
}

func TestInspectUploadRejectsOLEEmbeddings(t *testing.T) {
	data := buildZipWithEntries(t, map[string]string{
		"[Content_Types].xml":      `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"xl/embeddings/object.bin": "ole-object",
	})
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, DefaultLimits())
	if err == nil {
		t.Fatal("expected OLE/embeddings rejection")
	}
}

func TestInspectUploadRejectsExternalLinks(t *testing.T) {
	data := buildZipWithEntries(t, map[string]string{
		"[Content_Types].xml":        `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"xl/externalLinks/link1.xml": `<externalLink/>`,
	})
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, DefaultLimits())
	if err == nil {
		t.Fatal("expected external links rejection")
	}
}

func TestInspectUploadRejectsExpandedSizeLimit(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxExpandedBytes = 32
	data := buildMinimalXLSX(t, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>`+strings.Repeat("x", 128)+`</t></is></c></row></sheetData></worksheet>`)
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, limits)
	if err == nil {
		t.Fatal("expected expanded size rejection")
	}
}

func TestInspectUploadRejectsSuspiciousCompressionRatio(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxZipRatio = 2.0
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	payload := []byte(strings.Repeat("A", 4096))
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	for name, body := range map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"xl/workbook.xml":     `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`,
	} {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	_, err = InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes(), limits)
	if err == nil {
		t.Fatal("expected compression ratio rejection")
	}
}

func TestInspectUploadRejectsInvalidMIME(t *testing.T) {
	data := buildMinimalXLSX(t, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`)
	_, err := InspectUpload("text/plain", data, DefaultLimits())
	if err == nil {
		t.Fatal("expected invalid MIME rejection")
	}
}

func TestInspectUploadRejectsInvalidSignature(t *testing.T) {
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte{0x00, 0x01, 0x02, 0x03}, DefaultLimits())
	if err == nil {
		t.Fatal("expected invalid signature rejection")
	}
}

func TestInspectUploadRejectsUploadSizeLimit(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxUploadBytes = 16
	data := buildMinimalXLSX(t, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`)
	_, err := InspectUpload("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data, limits)
	if err == nil {
		t.Fatal("expected upload size rejection")
	}
}

func buildZipWithEntries(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, payload := range entries {
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
