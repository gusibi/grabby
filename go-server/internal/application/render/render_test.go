package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	got := Render("# Title\n\nHello **world**.")
	for _, want := range []string{"<h1", "Title", "<strong>world</strong>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() = %q, missing %q", got, want)
		}
	}
}

func TestRenderRawHTML(t *testing.T) {
	got := Render(`<p>Hello <a href="https://example.com">world</a></p>`)
	for _, want := range []string{"<p>Hello", `<a href="https://example.com"`, "world</a>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() = %q, missing %q", got, want)
		}
	}
}

func TestRenderMixedMarkdownAndHTML(t *testing.T) {
	got := Render("Intro **bold**\n\n<figure><img src=\"https://example.com/a.png\" alt=\"a\"><figcaption>Caption</figcaption></figure>\n\n- one\n- two")
	for _, want := range []string{"<strong>bold</strong>", "<figure>", "<img", "Caption", "<li>one</li>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() = %q, missing %q", got, want)
		}
	}
}

func TestRenderEscapedBlockHTML(t *testing.T) {
	got := Render(`&lt;p&gt;Hello &lt;a href="https://example.com"&gt;world&lt;/a&gt;&lt;/p&gt;`)
	if strings.Contains(got, "&lt;p&gt;") {
		t.Errorf("Render() still contains escaped HTML: %q", got)
	}
	for _, want := range []string{"<p>Hello", `<a href="https://example.com"`, "world</a>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() = %q, missing %q", got, want)
		}
	}
}

func TestRenderDoubleEscapedHTML(t *testing.T) {
	got := Render(`&amp;lt;p&amp;gt;Hello&amp;lt;/p&amp;gt;`)
	if !strings.Contains(got, "<p>Hello</p>") {
		t.Errorf("Render() = %q, want decoded paragraph", got)
	}
}

func TestRenderStripsDangerousHTML(t *testing.T) {
	got := Render(`<p onclick="alert(1)">safe</p><script>alert(1)</script><a href="javascript:alert(1)">bad</a>`)
	if strings.Contains(strings.ToLower(got), "script") || strings.Contains(strings.ToLower(got), "onclick") || strings.Contains(strings.ToLower(got), "javascript:") {
		t.Errorf("Render() leaked dangerous HTML: %q", got)
	}
	if !strings.Contains(got, "safe") {
		t.Errorf("Render() removed safe content: %q", got)
	}
}

func TestRenderDoesNotDecodeInlineCodeEscapes(t *testing.T) {
	got := Render("Use `&lt;span&gt;` in HTML.")
	if strings.Contains(got, "<span>") {
		t.Errorf("Render() = %q, inline code escape became a real element", got)
	}
}

func TestRenderFrontMatterAsCollapsibleTable(t *testing.T) {
	raw := "---\n" +
		"title: \"Interstate Weather\"\n" +
		"source: \"https://www.interstateweather.com/\"\n" +
		"description: \"Driving weather along US highways\"\n" +
		"site: \"interstateweather.com\"\n" +
		"captured_at: \"2026-11-25T06:00:31.288Z\"\n" +
		"---\n\n" +
		"Today\n\nForecast body."

	got := Render(raw)
	if strings.Contains(got, "<hr") || strings.Contains(got, "<h2") {
		t.Fatalf("frontmatter leaked into markdown heading/hr: %q", got)
	}
	for _, want := range []string{
		`<details class="gb-frontmatter">`,
		`<summary>页面元信息 · 5 项</summary>`,
		`<th>标题</th><td>Interstate Weather</td>`,
		`<th>来源链接</th><td><a href="https://www.interstateweather.com/"`,
		`<p>Today</p>`,
		`<p>Forecast body.</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "<details class=\"gb-frontmatter\" open") {
		t.Errorf("frontmatter details should be collapsed by default: %q", got)
	}
}

func TestExtractFrontMatterIgnoresHorizontalRule(t *testing.T) {
	raw := "---\n\nJust a normal paragraph after a rule."
	fields, body := ExtractFrontMatter(raw)
	if len(fields) != 0 {
		t.Fatalf("expected no frontmatter fields, got %+v", fields)
	}
	if body != raw {
		t.Fatalf("expected body unchanged, got %q", body)
	}
}

func TestRenderFrontMatterEscapesDangerousValues(t *testing.T) {
	raw := "---\ntitle: \"<script>alert(1)</script>\"\nsource: \"javascript:alert(1)\"\n---\n\nBody"
	got := Render(raw)
	if strings.Contains(got, "<script>") || strings.Contains(got, `href="javascript:`) {
		t.Fatalf("Render() leaked dangerous frontmatter values: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped title missing: %q", got)
	}
}
