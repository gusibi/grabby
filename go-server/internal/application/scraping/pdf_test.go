package scraping

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildTestPDF writes a one-page PDF whose content stream is supplied verbatim.
// The xref offsets are computed, so the file is valid enough for any reader.
func buildTestPDF(contentStream string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStream), contentStream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objects)+1)
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(b.String())
}

const pdfWithText = `BT
/F1 12 Tf
72 720 Td
(Hello) Tj
30 0 Td
(world) Tj
0 -20 Td
(Second line.) Tj
ET`

func TestExtractPDFTextRebuildsWordsFromPositions(t *testing.T) {
	doc, err := ExtractPDFText(buildTestPDF(pdfWithText))
	if err != nil {
		t.Fatalf("ExtractPDFText: %v", err)
	}
	if doc.Pages != 1 {
		t.Fatalf("pages = %d, want 1", doc.Pages)
	}
	// The gap between the two Tj operators is positioning only; the
	// extractor must recover the word space.
	if !strings.Contains(doc.Markdown, "Hello world") {
		t.Fatalf("word space not rebuilt: %q", doc.Markdown)
	}
	if !strings.Contains(doc.Markdown, "Second line.") {
		t.Fatalf("second line missing: %q", doc.Markdown)
	}
}

func TestExtractPDFTextReportsMissingTextLayer(t *testing.T) {
	doc, err := ExtractPDFText(buildTestPDF(""))
	if err == nil {
		t.Fatalf("empty text layer should be reported, got %q", doc.Markdown)
	}
	if err != ErrPDFNoText {
		t.Fatalf("err = %v, want ErrPDFNoText", err)
	}
}

func TestPDFURLAndDataDetection(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/paper.pdf":          true,
		"https://example.com/paper.PDF?dl=1":     true,
		"https://example.com/paper.pdf/":         false,
		"https://example.com/paper":              false,
		"https://example.com/pdf-guide":          false,
		"https://example.com/wp-content/a.PDF#p": true,
	}
	for raw, want := range cases {
		if got := IsPDFURL(raw); got != want {
			t.Errorf("IsPDFURL(%q) = %v, want %v", raw, got, want)
		}
	}
	if !LooksLikePDFData(buildTestPDF("")) {
		t.Fatal("PDF signature not detected")
	}
	if LooksLikePDFData([]byte("<html><body>hi</body></html>")) {
		t.Fatal("HTML misdetected as PDF")
	}
	if got := PDFTitleFromURL("https://example.com/files/2026-annual_report.pdf"); got != "2026 annual report" {
		t.Fatalf("PDFTitleFromURL = %q", got)
	}
}

func TestExtractWebArticleDetailsDetectsPDF(t *testing.T) {
	pdf := buildTestPDF(pdfWithText)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/paper.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(pdf)
		case "/download":
			// Generic content type: the bytes have to identify the file.
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(pdf)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	for _, path := range []string{"/paper.pdf", "/download"} {
		res, err := ExtractWebArticleDetails(context.Background(), srv.URL+path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !res.IsPDF || res.PDFPages != 1 {
			t.Fatalf("%s: IsPDF=%v pages=%d", path, res.IsPDF, res.PDFPages)
		}
		if !strings.Contains(res.Markdown, "Hello world") {
			t.Fatalf("%s: PDF bytes were not extracted as text: %q", path, res.Markdown)
		}
	}
}

func TestPDFSpaceRules(t *testing.T) {
	if needsPDFSpace("中文", "内容") {
		t.Fatal("CJK text must not gain a space")
	}
	if !needsPDFSpace("Hello", "world") {
		t.Fatal("Latin words separated by positioning need a space")
	}
	if got := joinPDFLine("你好", "世界"); got != "世界" {
		t.Fatalf("CJK lines should join without a space, got %q", got)
	}
	if got := joinPDFLine("well-", "known"); got != "known" {
		t.Fatalf("hyphenated lines should join without a space, got %q", got)
	}
	if got := joinPDFLine("hello", "world"); got != " world" {
		t.Fatalf("Latin lines should join with a space, got %q", got)
	}
}
