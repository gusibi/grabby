package nativeui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPrepareReaderBodyWrapsTranslatableBlocks(t *testing.T) {
	body, blocks := prepareReaderBody(`<p>Hello</p><div><p>World <b>now</b></p><ul><li>One</li><li>Two</li></ul></div>`, nil, "")
	if len(blocks) != 3 {
		t.Fatalf("blocks = %q, want 3 entries", blocks)
	}
	if blocks[0] != "Hello" || blocks[1] != "World now" {
		t.Fatalf("blocks = %q", blocks)
	}
	if !strings.Contains(blocks[2], "One") || !strings.Contains(blocks[2], "Two") {
		t.Fatalf("list block = %q, want both items", blocks[2])
	}
	for _, want := range []string{`data-gb="0"`, `data-gb="1"`, `data-gb="2"`, "gb-orig", "gb-trans", "<p>Hello</p>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("prepared body misses %q: %s", want, body)
		}
	}
}

func TestPrepareReaderBodySkipsCodeAndMedia(t *testing.T) {
	_, blocks := prepareReaderBody(`<pre>code here</pre><p>Read me</p><img src="x.png" alt="pic">`, nil, "")
	if len(blocks) != 1 || blocks[0] != "Read me" {
		t.Fatalf("blocks = %q, want only the paragraph", blocks)
	}
}

func TestBuildReaderPageEmbedsTranslation(t *testing.T) {
	a := fixtureApp()
	it := Item{ID: 7, Title: "Source title", URL: "https://example.com/a"}
	tr := &TranslationResult{Title: "译文标题", Translations: []string{"第一段译文", "第二段译文"}}
	page := a.buildReaderPage(it, ItemDetail{HTMLContent: "<p>one</p><p>two</p>"}, nil, tr, "bilingual")

	if len(page.Blocks) != 2 {
		t.Fatalf("blocks = %q, want 2", page.Blocks)
	}
	for _, want := range []string{
		`data-gb-mode="bilingual"`,
		`class="gb-trans"`,
		"第一段译文",
		"第二段译文",
	} {
		if !strings.Contains(page.HTML, want) {
			t.Fatalf("reader page misses %q", want)
		}
	}
}

func TestReaderTranslationEscapesMarkup(t *testing.T) {
	a := fixtureApp()
	it := Item{ID: 7, Title: "Source title"}
	tr := &TranslationResult{Translations: []string{`</script><script>alert(1)</script>`}}
	page := a.buildReaderPage(it, ItemDetail{HTMLContent: "<p>one</p>"}, nil, tr, "bilingual")
	if strings.Contains(page.HTML, "<script>alert(1)</script>") {
		t.Fatalf("translation markup was not escaped: %s", page.HTML)
	}
	if !strings.Contains(page.HTML, "&lt;script&gt;alert(1)") {
		t.Fatalf("escaped translation missing: %s", page.HTML)
	}
}

func TestReaderTranslatedTitleModes(t *testing.T) {
	a := fixtureApp()
	a.reader.Translation = &TranslationResult{Title: "译名"}
	a.reader.TranslateMode = "off"
	if got := a.readerTranslatedTitle(); got != "" {
		t.Fatalf("off title = %q, want empty", got)
	}
	a.reader.TranslateMode = "replace"
	if got := a.readerTranslatedTitle(); got != "" {
		t.Fatalf("replace title = %q, want empty (it becomes the main title)", got)
	}
	a.reader.TranslateMode = "bilingual"
	if got := a.readerTranslatedTitle(); got != "译名" {
		t.Fatalf("bilingual title = %q, want 译名", got)
	}
}

func TestPlainTextHTMLKeepsParagraphs(t *testing.T) {
	got := plainTextHTML("First paragraph\nsame block\n\nSecond <b>paragraph</b>")
	for _, want := range []string{"<p>First paragraph<br>same block</p>", "<p>Second &lt;b&gt;paragraph&lt;/b&gt;</p>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("plainTextHTML = %q, misses %q", got, want)
		}
	}
}

func TestSettingsShowsReaderModelSection(t *testing.T) {
	a := fixtureApp()
	a.view, a.reader.Open = "settings", false
	tester := ui.NewTester(a.View, 1280, 900)
	tester.Frame()
	for _, want := range []string{"阅读增强", "翻译模型", "解读模型", "翻译目标语言"} {
		if !tester.HasText(want) {
			t.Fatalf("settings page misses %q", want)
		}
	}
}

func TestPrepareReaderBodyEmbedsTranslation(t *testing.T) {
	body, _ := prepareReaderBody("<p>Hello</p>", []string{"你好"}, "")
	if !strings.Contains(body, "你好") || !strings.Contains(body, `class="gb-trans"`) || !strings.Contains(body, `data-gb-filled="1"`) {
		t.Fatalf("translation not embedded: %s", body)
	}
}

func TestReaderChatDialogShowsConversation(t *testing.T) {
	a := fixtureApp()
	a.view = "today"
	a.chat.Open = true
	a.chat.ItemID = a.reader.Item.ID
	a.chats[a.reader.Item.ID] = &ChatSession{
		ModelUsed: "gpt-test",
		Messages: []ChatMessage{
			{Role: "user", Content: "这篇文章的核心结论是什么？"},
			{Role: "assistant", Content: "### 核心结论\n\n测试回答。"},
		},
	}
	tester := ui.NewTester(a.View, 1280, 900)
	tester.Frame()
	for _, want := range []string{"AI 解读", "这篇文章的核心结论是什么？", "核心结论", "测试回答"} {
		if !tester.HasText(want) {
			t.Fatalf("chat dialog misses %q", want)
		}
	}
}

func TestPrepareReaderBodyStripsDuplicateTitle(t *testing.T) {
	body, blocks := prepareReaderBody(`<h1>Who in Washington owns and trades Nvidia stock</h1><p>Body</p>`, nil, "Who in Washington owns and trades Nvidia stock")
	if strings.Contains(body, "<h1>") {
		t.Fatalf("duplicate title kept: %s", body)
	}
	if len(blocks) != 1 || blocks[0] != "Body" {
		t.Fatalf("blocks = %q, want only Body", blocks)
	}
}

func TestPrepareReaderBodyKeepsShortSectionHeading(t *testing.T) {
	body, blocks := prepareReaderBody(`<h2>AI</h2><p>Body</p>`, nil, "How AI changes software engineering")
	if !strings.Contains(body, "<h2>AI</h2>") {
		t.Fatalf("short section heading was stripped: %s", body)
	}
	if len(blocks) != 2 || blocks[0] != "AI" || blocks[1] != "Body" {
		t.Fatalf("blocks = %q, want [AI Body]", blocks)
	}
}

func TestPrepareReaderBodyGroupsInlineRunsInContainer(t *testing.T) {
	body, blocks := prepareReaderBody(`<div>Visit <a href="https://example.com">our website</a> for <b>details</b>.</div>`, []string{"访问我们的网站了解详情。"}, "")
	if len(blocks) != 1 {
		t.Fatalf("blocks = %q, want 1 combined inline block", blocks)
	}
	if blocks[0] != "Visit our website for details ." && blocks[0] != "Visit our website for details." {
		t.Fatalf("block[0] = %q", blocks[0])
	}
	if !strings.Contains(body, "访问我们的网站了解详情。") {
		t.Fatalf("translation missing from grouped inline run: %s", body)
	}
}

func TestReaderChatButtonLabelIgnoresHiddenFailedPrompt(t *testing.T) {
	a := fixtureApp()
	id := a.reader.Item.ID
	a.chats[id] = &ChatSession{
		Messages: []ChatMessage{{Role: "user", Content: "请解读这篇文章", Hidden: true}},
		Err:      "连接失败",
	}
	if got := a.readerChatButtonLabel(); got != "AI 解读" {
		t.Fatalf("label = %q, want AI 解读 when only a hidden failed prompt exists", got)
	}
	a.chats[id].Messages = append(a.chats[id].Messages, ChatMessage{Role: "assistant", Content: "解读内容"})
	if got := a.readerChatButtonLabel(); got != "AI 对话" {
		t.Fatalf("label = %q, want AI 对话 once a visible reply exists", got)
	}
}

func TestPrepareReaderBodySkipsFrontMatterDetailsForTranslationAndDuplicateHeading(t *testing.T) {
	html := `<details class="gb-frontmatter"><summary>页面元信息 · 2 项</summary><table><tbody><tr><th>标题</th><td>Interstate Weather</td></tr></tbody></table></details><h1>Interstate Weather</h1><p>Forecast body.</p>`
	body, blocks := prepareReaderBody(html, nil, "Interstate Weather")
	if strings.Contains(body, "<h1>") {
		t.Fatalf("duplicate heading after frontmatter was not stripped: %s", body)
	}
	if len(blocks) != 1 || blocks[0] != "Forecast body." {
		t.Fatalf("blocks = %q, want only [Forecast body.]", blocks)
	}
	if !strings.Contains(body, `<details class="gb-frontmatter">`) {
		t.Fatalf("frontmatter details missing from prepared body: %s", body)
	}
}
