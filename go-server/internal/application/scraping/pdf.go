package scraping

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const (
	// PDFs are binary and often larger than an article page; a generous
	// ceiling still keeps a runaway download from filling memory.
	maxPDFFetchBytes = 32 << 20
)

// ErrPDFNoText means the file parsed but has no text layer, as with a scan.
// Callers keep the feed summary and link the reader to the original file.
var ErrPDFNoText = errors.New("PDF has no extractable text layer")

var pdfMagic = []byte("%PDF-")

// PDFDocument is the text extracted from a PDF link. Pages is best-effort and
// is used to describe the document in the reader.
type PDFDocument struct {
	Title    string
	Markdown string
	Pages    int
}

// IsPDFURL reports whether a link points at a PDF by its path suffix.
func IsPDFURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".pdf")
}

func isPDFContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/pdf")
}

// PDFTitleFromURL turns the file name into a readable fallback title, for
// documents whose metadata has no title and whose feed entry has none either.
func PDFTitleFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	return strings.TrimSpace(base)
}

func pdfSummary(pages int) string {
	if pages > 0 {
		return fmt.Sprintf("PDF 文档，共 %d 页，点击阅读原文", pages)
	}
	return "PDF 文档，点击阅读原文"
}

// LooksLikePDFData checks the first kilobyte for the PDF signature. Some
// servers send PDFs with a generic content type, so the bytes are the reliable
// signal.
func LooksLikePDFData(data []byte) bool {
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	return bytes.Contains(head, pdfMagic)
}

// ExtractPDFText reads the text layer of a PDF. PDF text is drawn glyph by
// glyph, so this rebuilds lines from glyph positions: a wide horizontal gap
// becomes a space and a baseline change becomes a line break. That recovers
// word boundaries that are only expressed through positioning, as in most
// LaTeX-generated papers.
func ExtractPDFText(data []byte) (doc PDFDocument, err error) {
	if len(data) == 0 {
		return PDFDocument{}, fmt.Errorf("empty PDF")
	}
	defer func() {
		// A malformed file must not take down a scrape run.
		if r := recover(); r != nil {
			doc = PDFDocument{}
			err = fmt.Errorf("parse PDF: %v", r)
		}
	}()

	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return PDFDocument{}, fmt.Errorf("parse PDF: %w", err)
	}
	doc.Title = pdfMetadataTitle(r)
	doc.Pages = r.NumPage()

	var lines []pdfLine
	for i := 1; i <= doc.Pages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		lines = append(lines, pdfLinesFromPage(page.Content(), i)...)
	}
	doc.Markdown = markdownFromPDFLines(lines)
	if strings.TrimSpace(doc.Markdown) == "" {
		return doc, ErrPDFNoText
	}
	return doc, nil
}

func pdfMetadataTitle(r *pdf.Reader) string {
	title := strings.TrimSpace(r.Trailer().Key("Info").Key("Title").Text())
	if title == "" || strings.EqualFold(title, "untitled") {
		return ""
	}
	// Metadata sometimes carries NULs or newlines; keep it on one line.
	return strings.Join(strings.Fields(title), " ")
}

// pdfLine is one visual line of a PDF page.
type pdfLine struct {
	Text string
	X    float64
	Y    float64
	Size float64
	Page int
}

func pdfLinesFromPage(content pdf.Content, page int) []pdfLine {
	var lines []pdfLine
	var current pdfLine
	var endX float64
	started := false

	flush := func() {
		current.Text = strings.TrimSpace(current.Text)
		if current.Text != "" {
			lines = append(lines, current)
		}
		started = false
	}
	start := func(x, y, size float64) {
		current = pdfLine{X: x, Y: y, Size: size, Page: page}
		endX = x
		started = true
	}

	for _, glyph := range content.Text {
		if glyph.S == "" {
			continue
		}
		chunks := strings.Split(glyph.S, "\n")
		for i, chunk := range chunks {
			if i > 0 {
				// Some extractors keep the line break inside the text piece.
				flush()
			}
			chunk = strings.ReplaceAll(chunk, "\r", "")
			if chunk == "" {
				continue
			}
			if !started {
				start(glyph.X, glyph.Y, glyph.FontSize)
			} else {
				tolerance := math.Max(2, 0.35*math.Max(current.Size, glyph.FontSize))
				if math.Abs(glyph.Y-current.Y) > tolerance || glyph.X < endX-math.Max(4, 0.4*math.Max(current.Size, glyph.FontSize)) {
					// New baseline, or the stream jumped back to the left for a
					// new line or column.
					flush()
					start(glyph.X, glyph.Y, glyph.FontSize)
				} else {
					gap := glyph.X - endX
					if gap > math.Max(1.5, 0.16*glyph.FontSize) && needsPDFSpace(current.Text, chunk) {
						current.Text += " "
					}
					if glyph.FontSize > current.Size && math.Abs(glyph.Y-current.Y) <= tolerance {
						current.Size = glyph.FontSize
					}
				}
			}
			current.Text += chunk
			endX = glyph.X + glyph.W
		}
	}
	flush()
	return lines
}

func needsPDFSpace(before, after string) bool {
	if before == "" || after == "" {
		return false
	}
	prev, _ := utf8.DecodeLastRuneInString(before)
	next, _ := utf8.DecodeRuneInString(after)
	if prev == ' ' || next == ' ' {
		return false
	}
	return !(isCJK(prev) && isCJK(next))
}

// markdownFromPDFLines groups lines into paragraphs. Repeated short lines are
// dropped as running headers and page numbers, and notably larger lines are
// promoted to headings so a long PDF stays scannable.
func markdownFromPDFLines(lines []pdfLine) string {
	if len(lines) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, line := range lines {
		if n := utf8.RuneCountInString(line.Text); n > 0 && n <= 90 {
			counts[line.Text]++
		}
	}

	var sizes []float64
	for _, line := range lines {
		if line.Size > 0 {
			sizes = append(sizes, line.Size)
		}
	}
	sort.Float64s(sizes)
	bodySize := 0.0
	if len(sizes) > 0 {
		bodySize = sizes[len(sizes)/2]
	}

	var paragraphs []string
	var current strings.Builder
	var previous *pdfLine
	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			paragraphs = append(paragraphs, s)
		}
		current.Reset()
	}
	appendLine := func(line pdfLine) {
		if current.Len() == 0 {
			current.WriteString(line.Text)
			return
		}
		current.WriteString(joinPDFLine(current.String(), line.Text))
	}

	for _, line := range lines {
		line.Text = strings.TrimSpace(line.Text)
		if line.Text == "" || isPDFNoiseLine(line.Text, counts) {
			continue
		}
		heading := bodySize > 0 && line.Size >= bodySize*1.25 &&
			utf8.RuneCountInString(line.Text) <= 120
		if heading {
			flush()
			paragraphs = append(paragraphs, "## "+line.Text)
			previous = nil
			continue
		}
		if previous != nil && newPDFParagraph(*previous, line) {
			flush()
		}
		appendLine(line)
		prev := line
		previous = &prev
	}
	flush()
	return strings.Join(paragraphs, "\n\n")
}

func newPDFParagraph(previous, line pdfLine) bool {
	step := previous.Y - line.Y
	if step < 0 {
		// Moving up means a new column or page even if the gap is large.
		return true
	}
	size := math.Max(previous.Size, line.Size)
	if size <= 0 {
		size = 10
	}
	if step > 1.55*size {
		return true
	}
	// A short line that closes a sentence usually ends the paragraph, even
	// when the layout does not add extra vertical space.
	prevLen := utf8.RuneCountInString(previous.Text)
	return prevLen < 60 && endsPDFParagraph(previous.Text) && line.X <= previous.X+2
}

func isPDFNoiseLine(line string, counts map[string]int) bool {
	if counts[line] >= 3 {
		return true
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if len(trimmed) <= 6 && isAllDigits(strings.Trim(trimmed, "-–— ")) {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "page ") && len(trimmed) <= 20 && strings.ContainsAny(trimmed, "0123456789") {
		return true
	}
	if strings.HasPrefix(trimmed, "第") && strings.HasSuffix(trimmed, "页") && len([]rune(trimmed)) <= 10 {
		return true
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// joinPDFLine joins two visual lines of the same paragraph. CJK text is joined
// without a space, and hyphenated words keep their hyphen without a gap.
func joinPDFLine(previous, line string) string {
	if previous == "" {
		return line
	}
	prevRune, _ := utf8.DecodeLastRuneInString(previous)
	nextRune, _ := utf8.DecodeRuneInString(line)
	if prevRune == '-' && !isCJK(nextRune) {
		return line
	}
	if isCJK(prevRune) && isCJK(nextRune) {
		return line
	}
	return " " + line
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x2E80 && r <= 0x9FFF, // CJK radicals through unified ideographs
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFF00 && r <= 0xFFEF, // full-width forms
		r >= 0x20000 && r <= 0x3FFFF:
		return true
	}
	return false
}

// endsPDFParagraph starts a new paragraph after a line that closes a sentence
// and is short enough that it likely ended the block rather than wrapping.
func endsPDFParagraph(line string) bool {
	n := utf8.RuneCountInString(line)
	if n >= 90 {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(line)
	switch last {
	case '.', '。', '!', '！', '?', '？', ':', '：', ';', '；':
		return true
	}
	return false
}
