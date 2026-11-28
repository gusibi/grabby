package nativeui

import (
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// fixtureApp returns an App populated with representative state, so every view
// can be built without a window or a server.
const dailyFixtureJSON = `{
  "title": "Grabby AI 智能日报",
  "date": "2026-11-18",
  "editor": "Grabby",
  "sections": {
    "headlines": {
      "title": "今日头条",
      "items": [
        {
          "title": "AI 陪伴硬件的账该怎么算？",
          "summary": "文章把硬件成本、订阅收入与留存放在同一张表里，讨论角色型 AI 产品为什么很难只靠卖设备赚钱。",
          "source": "钛媒体",
          "link": "https://example.com/a",
          "score": "8.5",
          "comment": "比单看销量更有说服力，值得对照自家定价再读一遍。"
        },
        {
          "title": "AI 伴侣的孤独陷阱",
          "summary": "斯坦福大学基于 46 万条 Character.AI 聊天记录的研究，讨论陪伴型产品对社交关系的影响。",
          "source": "钛媒体",
          "link": "https://example.com/b",
          "score": "7"
        }
      ]
    },
    "tech": {
      "title": "技术与产品",
      "items": [
        {
          "title": "GPT-6.2 Astra 终于上线，从 PPT 做到发动机",
          "summary": "OpenAI 发布 GPT-6.2 Astra 与 Sol，强调现实任务、专业写作与长链路代理执行。",
          "source": "钛媒体",
          "link": "https://example.com/c",
          "score": "8"
        }
      ]
    }
  }
}`

func fixtureApp() *App {
	a := New(NewClient("http://127.0.0.1:1"))
	now := time.Now()
	a.health = Health{Status: "ok", BrowserConnected: true, BrowserCount: 1}
	a.stats = Stats{
		TotalCount: 1515, UnreadCount: 128, StarredCount: 41, TodayCount: 128,
		Categories:       map[string]int{"科技": 4},
		SourceCategories: []string{"新闻", "播客"},
	}
	first := Item{
		ID: 1, SourceID: "feed", OriginSource: "Grabby Feed",
		Title: "Native UI 迁移记录", URL: "https://example.com/a",
		Summary: "摘要", Content: "正文内容", Category: "科技",
		SourceCategory: "新闻", ReadStatus: 0, Starred: 1,
		FetchedAt: now, AIModelUsed: "claude-sonnet-5-5",
		QualityScore: 8, AICategory: "AI", AISummary: "AI 摘要",
		Metadata: `{"media_url":"https://example.com/a.mp3","duration":"12:30","podcast":"Grabby 电台"}`,
	}
	second := Item{
		ID: 2, SourceID: "feed", OriginSource: "Grabby Feed",
		Title: "第二条内容", URL: "https://example.com/b", Summary: "摘要 2",
		FetchedAt: now, ReadStatus: 1,
	}
	extra := []struct {
		title, source, category string
		score                   int
	}{
		{"模型推理成本一年下降了多少", "量子位", "AI", 9},
		{"Mac 上的原生界面为什么更快", "Hacker News", "科技", 8},
		{"播客：如何设计一个阅读器", "Grabby 电台", "播客", 7},
		{"本周值得关注的十篇长文", "少数派", "阅读", 8},
		{"远程协作工具的一次重做", "36 氪", "商业", 6},
		{"从零搭建本地优先的同步服务", "InfoQ", "科技", 9},
	}
	var items []Item
	items = append(items, first, second)
	for i, e := range extra {
		items = append(items, Item{
			ID: int64(3 + i), SourceID: "feed", OriginSource: e.source,
			Title: e.title, URL: fmt.Sprintf("https://example.com/e%d", i),
			Summary:    "摘要段落，说明这条内容讲了什么，供列表里快速扫读。",
			FetchedAt:  now.Add(-time.Duration(i+1) * 37 * time.Minute),
			ReadStatus: 0, AICategory: e.category, QualityScore: e.score,
		})
	}
	a.items = items
	for _, v := range []string{"today", "library", "ai"} {
		a.viewItems[v] = a.items
		a.viewCursors[v] = ""
	}
	a.browsers = []Browser{{ConnID: "conn-1", Name: "Chrome", Online: true}}
	a.sources = []Source{{
		ID: "feed", Name: "Grabby Feed", Type: "rss", URL: "https://example.com/feed.xml",
		Schedule: "0 * * * *", Enabled: 1, Category: "新闻", DefaultCategory: "科技",
	}}
	a.captures = []ExtractRecord{{
		URL: "https://example.com/x", Title: "抓取记录", Chars: 1024, CreatedAt: now,
	}}
	a.aiCategories = []AICategory{{Name: "AI", Count: 3, AvgScore: 8.2}}
	a.dailyList = []ReportListItem{
		{ID: 1, ReportDate: now.Format("2006-01-02"), ReportType: "daily",
			Title: "Grabby AI 智能日报", TotalItems: 12, QualityItems: 4, GeneratedAt: now},
		{ID: 2, ReportDate: now.AddDate(0, 0, -1).Format("2006-01-02"), ReportType: "morning",
			Title: "Grabby AI 早报", TotalItems: 9, QualityItems: 3, GeneratedAt: now.AddDate(0, 0, -1)},
		{ID: 3, ReportDate: now.AddDate(0, 0, -2).Format("2006-01-02"), ReportType: "daily",
			Title: "Grabby AI 智能日报", TotalItems: 17, QualityItems: 6, GeneratedAt: now.AddDate(0, 0, -2)},
	}
	a.aiSettings = &AISettings{
		Enabled: true, QualityThreshold: 7, Strategy: "single",
		ActiveProfileID: "p1",
		Profiles: []ProviderProfile{{
			ID: "p1", Name: "主力", Provider: "anthropic",
			Model: "claude-sonnet-5-5", Models: []string{"claude-sonnet-5-5"},
			APIKey: "sk-test",
		}},
		ASRMode: "auto", MorningReportTime: "07:20", EveningReportTime: "18:50",
	}
	a.aiStats = AIStats{TotalProcessed: 10, TotalPending: 2, AverageScore: 7.6, QueueLength: 1}
	a.aiUsage = &AIUsage{
		Days: 7, TotalRequests: 20, TotalTokens: 12000, Cost: 0.42, AvgLatencyMS: 830,
		ByDay: []UsageByDay{{Day: now.Format("2006-01-02"), Requests: 20, Tokens: 12000, Cost: 0.42, AvgLatency: 830}},
	}
	a.desktop = DesktopSettings{
		Success: true, DesktopMode: true, DataDir: "/tmp/grabby", Host: "127.0.0.1",
		Port: 5041, TokenEnabled: true, APIToken: "abcdef0123456789",
	}
	a.tokenDraft = a.desktop.APIToken
	a.asrEnv = &ASREnv{
		OS: "darwin", Arch: "arm64", LocalReady: true, CloudReady: true,
		InstallCommand: "bash scripts/install-asr-tools.sh",
		Tools: map[string]ASToolStatus{
			"ffmpeg":      {Installed: true},
			"yt-dlp":      {Installed: true},
			"mlx_whisper": {Installed: false},
			"whisper":     {Installed: false},
		},
	}
	a.daily = &DailyReport{
		Success: true, ID: 1, ReportDate: now.Format("2006-01-02"), ReportType: "daily",
		Title: "Grabby AI 智能日报", Format: "json", Content: dailyFixtureJSON,
		TotalItems: 12, QualityItems: 4, ModelUsed: "claude-sonnet-5-5",
		GeneratedAt: now.Format(time.RFC3339),
	}
	a.reader = Reader{
		Open: true, Item: &a.items[0],
		Analysis: &Analysis{
			ItemID: 1, AICategory: "AI", QualityScore: 8,
			AISummary: "AI 摘要", AIComment: "值得一读", AITags: "AI,Go",
		},
	}
	return a
}

func TestViewsRender(t *testing.T) {
	cases := []struct {
		view string
		text string
	}{
		{"today", "今天"},
		{"library", "内容库"},
		{"sources", "RSS 源"},
		{"news", "新闻源"},
		{"ai", "AI 分析"},
		{"daily", "AI 日报"},
		{"devices", "浏览器设备"},
		{"apihub", "API 接入"},
		{"settings", "设置"},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			a := fixtureApp()
			a.view = tc.view
			tester := ui.NewTester(a.View, 1280, 840)
			tester.Frame()
			if !tester.HasText(tc.text) {
				t.Fatalf("view %q does not show %q", tc.view, tc.text)
			}
		})
	}
}

func TestReaderClosedRender(t *testing.T) {
	a := fixtureApp()
	a.reader.Open = false
	a.view = "library"
	tester := ui.NewTester(a.View, 1180, 780)
	tester.Frame()
	if !tester.HasText("Native UI 迁移记录") {
		t.Fatal("library does not render the item card")
	}
}

func TestConfirmDialogRender(t *testing.T) {
	a := fixtureApp()
	a.view = "sources"
	a.confirm.Open = true
	a.confirm.Title = "删除数据源"
	a.confirm.Message = "删除后不可恢复。"
	a.confirm.ActionLabel = "删除"
	tester := ui.NewTester(a.View, 1180, 780)
	tester.Frame()
	if !tester.HasText("删除数据源") {
		t.Fatal("confirm dialog is not rendered")
	}
}

func TestSidebarNavigate(t *testing.T) {
	a := fixtureApp()
	a.reader.Open = false
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	if err := tester.Click("内容库"); err != nil {
		t.Fatalf("click 内容库: %v", err)
	}
	if a.view != "library" {
		t.Fatalf("view = %q, want library", a.view)
	}
	tester.Frame()
	if !tester.HasText("已归档") {
		t.Fatal("library page did not render after navigation")
	}
}

func TestGroupNamesAreStable(t *testing.T) {
	groups := map[string][]Source{
		"英语学习": {{Name: "a"}, {Name: "b"}},
		"科技新闻": {{Name: "a"}, {Name: "b"}, {Name: "c"}},
		"AI":   {{Name: "a"}},
		"体育新闻": {{Name: "a"}},
	}
	first := strings.Join(groupNames(groups), ",")
	if first != "科技新闻,英语学习,AI,体育新闻" {
		t.Fatalf("groupNames = %q", first)
	}
	for i := 0; i < 50; i++ {
		if got := strings.Join(groupNames(groups), ","); got != first {
			t.Fatalf("call %d changed order: %q != %q", i, got, first)
		}
	}
}

// TestScreenshots writes one PNG per view when GRABBY_UI_SHOTS names a
// directory. It is a development aid, skipped otherwise.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("GRABBY_UI_SHOTS")
	if dir == "" {
		t.Skip("set GRABBY_UI_SHOTS=<dir> to write screenshots")
	}
	for _, view := range []string{"today", "library", "sources", "news", "ai", "daily", "devices", "apihub", "settings"} {
		a := fixtureApp()
		a.view = view
		a.reader.Open = false
		tester := ui.NewTester(a.View, 1280, 840)
		tester.Frame()
		img := tester.Image()
		f, err := os.Create(filepath.Join(dir, view+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}

	a := fixtureApp()
	a.view = "library"
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	img := tester.Image()
	f, _ := os.Create(filepath.Join(dir, "reader.png"))
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	list := fixtureApp()
	list.view, list.layout, list.reader.Open = "library", 1, false
	listTester := ui.NewTester(list.View, 1280, 840)
	listTester.Frame()
	g, _ := os.Create(filepath.Join(dir, "library-list.png"))
	if err := png.Encode(g, listTester.Image()); err != nil {
		t.Fatal(err)
	}
	_ = g.Close()
}

// TestScreenshotsNarrow renders a couple of views at the minimum window size,
// where the reader split is the tightest. Development aid, like TestScreenshots.
func TestScreenshotsNarrow(t *testing.T) {
	dir := os.Getenv("GRABBY_UI_SHOTS")
	if dir == "" {
		t.Skip("set GRABBY_UI_SHOTS=<dir> to write screenshots")
	}
	for _, view := range []string{"today", "library", "ai", "daily"} {
		a := fixtureApp()
		a.view = view
		a.reader.Open = false
		tester := ui.NewTester(a.View, 960, 640)
		tester.Frame()
		f, _ := os.Create(filepath.Join(dir, view+"-narrow.png"))
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
}

// TestScreenshotsTall renders the long settings-style pages without scrolling.
func TestScreenshotsTall(t *testing.T) {
	dir := os.Getenv("GRABBY_UI_SHOTS")
	if dir == "" {
		t.Skip("set GRABBY_UI_SHOTS=<dir> to write screenshots")
	}
	for _, view := range []string{"settings", "ai", "apihub"} {
		a := fixtureApp()
		a.view = view
		a.reader.Open = false
		tester := ui.NewTester(a.View, 1280, 2000)
		tester.Frame()
		f, _ := os.Create(filepath.Join(dir, view+"-tall.png"))
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}

	// The model tab with a catalog loaded, so the checkbox list is visible.
	a := pickerFixture()
	a.view = "settings"
	a.reader.Open = false
	tester := ui.NewTester(a.View, 1280, 1400)
	tester.Frame()
	f, _ := os.Create(filepath.Join(dir, "settings-models-tall.png"))
	if err := png.Encode(f, tester.Image()); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestSourceDialogRender(t *testing.T) {
	a := fixtureApp()
	a.view = "sources"
	a.openSourceDialog(nil)
	tester := ui.NewTester(a.View, 1180, 780)
	tester.Frame()
	if !tester.HasText("添加数据源") {
		t.Fatal("source dialog is not rendered")
	}
	if a.sourceEditing == nil || a.sourceEditing.ID != "" {
		t.Fatalf("ID must stay empty until save, got %q", a.sourceEditing.ID)
	}
}

func TestSourceLogsDialogRender(t *testing.T) {
	a := fixtureApp()
	a.view = "sources"
	a.sourceLogOpen = true
	a.sourceLogTitle = "Grabby Feed"
	a.sourceLogs = []FetchLog{{
		ID: 1, SourceID: "feed", StartedAt: time.Now(), Status: "success",
		ItemsFound: 12, ItemsAdded: 3,
	}}
	tester := ui.NewTester(a.View, 1180, 780)
	tester.Frame()
	if !tester.HasText("抓取日志") {
		t.Fatal("source log dialog is not rendered")
	}
}

func TestItemsFilterQuery(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[],"cursor":""}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.items(Filter{Mode: "unread", Query: "go", ScoreMin: 7}, ""); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"read_status=0", "q=go"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("unread query %q lacks %q", raw, want)
		}
	}
	if _, err := c.items(Filter{Mode: "starred"}, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "starred=1") {
		t.Fatalf("starred query %q lacks starred=1", raw)
	}
}

func TestApplyDataReplacesAIDraft(t *testing.T) {
	a := fixtureApp()
	// A view can build a draft from defaults before settings arrive.
	a.aiDraft = nil
	a.aiSettings = nil
	a.ensureAIDraft()
	loaded := &AISettings{
		Enabled: true,
		Profiles: []ProviderProfile{{
			ID: "real", Name: "Real", Provider: "openai", APIKey: "sk-real",
		}},
	}
	a.applyData("ai", viewData{aiSettings: loaded})
	if a.aiDraft == nil || len(a.aiDraft.Profiles) != 1 {
		t.Fatalf("draft not rebuilt: %+v", a.aiDraft)
	}
	if a.aiDraft.Profiles[0].ID != "real" || a.aiDraft.Profiles[0].APIKey != "sk-real" {
		t.Fatalf("draft kept stale defaults: %+v", a.aiDraft.Profiles[0])
	}
}

func TestReaderHTMLDoesNotRepeatTitle(t *testing.T) {
	a := fixtureApp()
	it := a.items[0]
	doc := a.buildReaderHTML(it, ItemDetail{Item: it, HTMLContent: "<p>body</p>"}, a.reader.Analysis)
	if strings.Contains(doc, "<h1>") {
		t.Fatal("reader HTML repeats the title the native toolbar already shows")
	}
	if !strings.Contains(doc, "<p>body</p>") || !strings.Contains(doc, "AI 摘要") {
		t.Fatal("reader HTML misses the body or the AI summary")
	}
	if !strings.Contains(doc, "https://example.com/a.mp3") {
		t.Fatal("reader HTML misses the podcast player")
	}
}

func TestReaderHTMLEmbedsYouTubePlayer(t *testing.T) {
	a := fixtureApp()
	it := Item{
		ID: 42, SourceID: "yt", OriginSource: "Matt Pocock", Category: "youtube",
		Title:    "Mario Zechner: STOP Running Away From Understanding",
		URL:      "https://www.youtube.com/watch?v=Vo33extKAeA",
		Metadata: `{"video_id":"Vo33extKAeA","channel":"Matt Pocock","duration":"12:30"}`,
	}
	doc := a.buildReaderHTML(it, ItemDetail{Item: it, HTMLContent: "<p>简介</p>"}, nil)
	for _, want := range []string{
		"https://www.youtube.com/embed/Vo33extKAeA",
		"视频 · Matt Pocock",
		"时长 12:30",
		"<p>简介</p>",
		`<base href="https://www.youtube.com/watch?v=Vo33extKAeA">`,
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("reader HTML misses %q", want)
		}
	}
	if strings.Contains(doc, "<audio") {
		t.Fatal("a video item got the podcast player")
	}
}

func TestReaderHTMLFindsVideoWithoutMetadata(t *testing.T) {
	a := fixtureApp()
	it := Item{
		ID: 43, OriginSource: "Grabby", Category: "youtube",
		Title: "旧条目", URL: "https://youtu.be/Vo33extKAeA?t=30",
		Metadata: `{"channel":"旧频道"}`,
	}
	doc := a.buildReaderHTML(it, ItemDetail{Item: it, HTMLContent: "<p>body</p>"}, nil)
	if !strings.Contains(doc, "https://www.youtube.com/embed/Vo33extKAeA") {
		t.Fatal("reader HTML misses the player for a youtu.be URL")
	}
	if !strings.Contains(doc, "视频 · 旧频道") {
		t.Fatal("reader HTML misses the channel from metadata")
	}
}

func TestReaderHTMLShowsPDFCard(t *testing.T) {
	a := fixtureApp()
	it := Item{
		ID: 45, Title: "Annual Report", URL: "https://example.com/paper.pdf",
		Metadata: `{"content_source":"pdf_text","pdf_url":"https://example.com/paper.pdf","pdf_pages":12}`,
	}
	doc := a.buildReaderHTML(it, ItemDetail{Item: it, HTMLContent: "<p>extracted text</p>"}, nil)
	for _, want := range []string{
		`class="pdf-card"`,
		"PDF 文档 · 12 页",
		"已提取 PDF 的文字版",
		`href="https://example.com/paper.pdf"`,
		"打开 PDF 原文",
		"<p>extracted text</p>",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("reader HTML misses %q", want)
		}
	}
}

func TestReaderHTMLShowsPDFCardWithoutTextLayer(t *testing.T) {
	a := fixtureApp()
	// Legacy items carry no pdf metadata; the .pdf link is enough to show the
	// document card, and the feed summary remains as the body.
	it := Item{ID: 46, Title: "Scanned Memo", URL: "https://example.com/scan.pdf", Summary: "扫描件摘要"}
	doc := a.buildReaderHTML(it, ItemDetail{Item: it, SummaryHTML: "<p>扫描件摘要</p>"}, nil)
	for _, want := range []string{
		"PDF 文档",
		"没有可提取的文字层",
		"打开 PDF 原文",
		"<p>扫描件摘要</p>",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("reader HTML misses %q", want)
		}
	}
}

func TestYouTubeVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=Vo33extKAeA":    "Vo33extKAeA",
		"https://m.youtube.com/watch?v=Vo33extKAeA&t=12": "Vo33extKAeA",
		"https://youtu.be/Vo33extKAeA?si=abc":            "Vo33extKAeA",
		"https://www.youtube.com/shorts/Vo33extKAeA":     "Vo33extKAeA",
		"https://www.youtube.com/live/Vo33extKAeA":       "Vo33extKAeA",
		"https://example.com/watch?v=Vo33extKAeA":        "",
		"https://www.youtube.com/@mattpocockuk/videos":   "",
		"": "",
	}
	for raw, want := range cases {
		if got := youTubeVideoID(raw); got != want {
			t.Errorf("youTubeVideoID(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestReaderServerPublishesDocument(t *testing.T) {
	s := newReaderServer()
	if s == nil {
		t.Skip("loopback listener unavailable")
	}
	url := s.publish("<!doctype html><p>阅读器文档</p>")
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("document URL %q is not a loopback page", url)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "阅读器文档") {
		t.Fatalf("GET %s = %d %q", url, resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type %q", ct)
	}
	if resp2, err := http.Get(s.base + "/r/unknown"); err == nil {
		resp2.Body.Close()
		if resp2.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown document = %d, want 404", resp2.StatusCode)
		}
	}
}

// TestScreenshotsWide renders the pages at a large window, where a page
// narrower than the window should sit centered rather than hug the left.
func TestScreenshotsWide(t *testing.T) {
	dir := os.Getenv("GRABBY_UI_SHOTS")
	if dir == "" {
		t.Skip("set GRABBY_UI_SHOTS=<dir> to write screenshots")
	}
	for _, view := range []string{"ai", "daily", "settings", "today", "library", "news"} {
		a := fixtureApp()
		a.view = view
		a.reader.Open = false
		tester := ui.NewTester(a.View, 2560, 1400)
		tester.Frame()
		f, _ := os.Create(filepath.Join(dir, view+"-wide.png"))
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
}

func TestTodayFilters(t *testing.T) {
	a := fixtureApp()
	a.view, a.reader.Open = "today", false
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	// Today carries the same type and source filters as the library.
	if !tester.HasText("类型") || !tester.HasText("全部类型") {
		t.Fatal("today page is missing the content-type filters")
	}
	if err := tester.Click("播客"); err != nil {
		t.Fatalf("click 播客: %v", err)
	}
	if a.filters["today"].ContentType != "podcast" {
		t.Fatalf("today content type = %q, want podcast", a.filters["today"].ContentType)
	}
	if a.filters["library"].ContentType != "" {
		t.Fatal("filtering today changed the library filter")
	}
}

func TestTodayLayoutSwitch(t *testing.T) {
	a := fixtureApp()
	a.view, a.reader.Open = "today", false
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	if err := tester.Click("列表"); err != nil {
		t.Fatalf("click 列表: %v", err)
	}
	if a.layout != 1 {
		t.Fatalf("layout = %d, want the list layout", a.layout)
	}
}

func TestFormatCount(t *testing.T) {
	cases := map[int]string{0: "0", 209: "209", 1515: "1,515", 15568: "15,568"}
	for n, want := range cases {
		if got := formatCount(n); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDailyAdoptsNewestReport(t *testing.T) {
	a := fixtureApp()
	a.view = "daily"
	a.dailyDate = time.Now().Format("2006-01-02")
	a.dailyType = "daily"
	a.daily = nil
	a.dailyPicked = false
	a.dailyList = []ReportListItem{
		{ReportDate: time.Now().Format("2006-01-02"), ReportType: "evening"},
		{ReportDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"), ReportType: "morning"},
	}
	// What the server answers for a day and type never generated: 200,
	// success, no content.
	a.applyData("daily", viewData{daily: &DailyReport{Success: true}})
	if a.dailyType != "evening" {
		t.Fatalf("daily type = %q, want the newest report's evening", a.dailyType)
	}
	if !a.dailyPicked {
		t.Fatal("adopting the newest report should end the auto-pick")
	}
	if a.daily != nil {
		t.Fatal("the pane should keep loading instead of showing the report it left")
	}
}

func TestDailyKeepsUserChoice(t *testing.T) {
	a := fixtureApp()
	a.view = "daily"
	a.dailyDate = time.Now().Format("2006-01-02")
	a.dailyType = "daily"
	a.dailyPicked = true
	a.dailyList = []ReportListItem{
		{ReportDate: time.Now().Format("2006-01-02"), ReportType: "evening"},
	}
	a.applyData("daily", viewData{daily: &DailyReport{Success: false}})
	if a.dailyType != "daily" {
		t.Fatalf("daily type = %q, want the day and type the user asked for", a.dailyType)
	}
	if a.daily == nil || a.daily.Success {
		t.Fatal("the empty report should be shown for the chosen day")
	}
}

func TestSourceSwitchUpdatesCurrentPage(t *testing.T) {
	requests := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/sources/feed/toggle" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Enabled int `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- body.Enabled
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	a := fixtureApp()
	a.client = NewClient(server.URL)
	a.view, a.loadedView, a.reader.Open = "sources", "sources", false
	a.win = &mygo.Window{}
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	if err := tester.Click("启用订阅源 Grabby Feed"); err != nil {
		t.Fatal(err)
	}

	select {
	case enabled := <-requests:
		if enabled != 0 {
			t.Fatalf("toggle sent enabled=%d, want 0", enabled)
		}
	case <-time.After(time.Second):
		t.Fatal("click did not send a toggle request")
	}
	tester.Frame()
	texts := tester.Texts()
	for i, label := range texts {
		if label == "已启用" && i > 0 && texts[i-1] == "0" {
			return
		}
	}
	t.Fatalf("API received enabled=0 but the page did not update the enabled count: %q", texts)
}

func TestOfflineBrowserDeleteOpensConfirmation(t *testing.T) {
	a := fixtureApp()
	a.view = "devices"
	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	if tester.HasText("删除") {
		t.Fatal("online device has a delete action")
	}
	a.browsers[0].Online = false
	a.browsers[0].Banned = true
	tester.Frame()
	if err := tester.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if !a.confirm.Open || a.confirm.Title != "删除设备" || !strings.Contains(a.confirm.Message, "Chrome") {
		t.Fatalf("unexpected confirmation: %+v", a.confirm)
	}
	if len(a.browsers) != 1 {
		t.Fatal("opening a confirmation removed the device")
	}
}

func TestLibraryFiltersBySubscriptionSource(t *testing.T) {
	a := fixtureApp()
	a.view, a.reader.Open = "library", false
	a.stats.SourceCounts = []SourceCount{
		{ID: "hi-news", Name: "嗨 news", Total: 12, Unread: 5},
		{ID: "podcast", Name: "Grabby 电台", Total: 3, Unread: 0},
		{ID: "empty", Name: "空源", Total: 0, Unread: 0},
	}
	a.filters["library"].SourceID = "hi-news"

	if got := a.headerSubtitle(); got != "只看 · 嗨 news · 12 条内容，5 未读" {
		t.Fatalf("header subtitle = %q, want 只看 · 嗨 news · 12 条内容，5 未读", got)
	}

	tester := ui.NewTester(a.View, 1280, 840)
	tester.Frame()
	// The source select shows the active subscription, not "全部订阅源", and
	// sources with no content are left out of its options.
	if !tester.HasText("订阅源") || !tester.HasText("嗨 news · 5 未读") {
		t.Fatal("library page is missing the subscription-source filter")
	}

	labels, ids := a.sourceFilterOptions("hi-news")
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "Grabby 电台 · 3 条") {
		t.Fatalf("source options = %q, want sources that have content", labels)
	}
	if strings.Contains(joined, "空源") {
		t.Fatalf("source options = %q, should hide sources with no content", labels)
	}
	if ids[0] != "" || ids[1] != "hi-news" {
		t.Fatalf("source ids = %v, want all first then the selected source", ids)
	}
}
