package nativeui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Filter is the per-view feed filter, mirroring the web UI's state.
type Filter struct {
	Mode           string // all, unread, starred, quality
	Query          string
	Category       string
	ContentType    string
	SourceCategory string
	SourceID       string
	AICategory     string
	ScoreMin       int
	Since          string // RFC3339 cutoff; today's feed starts at midnight
}

func (f *Filter) limit() int { return 60 }

func newFilter() *Filter { return &Filter{Mode: "all", ScoreMin: 7} }

// startOfToday is midnight local time, in RFC3339, the cutoff of today's feed.
func startOfToday() string {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)
}

// Reader is the reading pane's state.
type Reader struct {
	Open     bool
	Item     *Item
	Analysis *Analysis
	Loading  bool
	Err      string
	seq      int

	// Reader enhancements. Blocks are the translatable paragraphs of the
	// current document, aligned with Translation.Translations.
	Blocks        []string
	detail        ItemDetail
	Translation   *TranslationResult
	Translating   bool
	TranslateErr  string
	TranslateMode string // off, replace, bilingual, hover
	translateSeq  int
}

type Confirm struct {
	Open        bool
	Title       string
	Message     string
	ActionLabel string
	Run         func() error
}

// ChatSession is the conversation about one article. It outlives the dialog so
// reopening the reader keeps the follow-up thread.
type ChatSession struct {
	Messages  []ChatMessage
	ModelUsed string
	Streaming bool
	Err       string
	seq       int
	cancel    context.CancelFunc
}

// ChatState is the dialog's open state and its current draft.
type ChatState struct {
	Open       bool
	ItemID     int64
	Draft      string
	ScrollNext bool
}

type toastMsg struct {
	Text  string
	Error bool
}

type viewData struct {
	health     *Health
	stats      *Stats
	items      *ItemsPage
	browsers   *BrowserList
	categories []AICategory
	dailyList  []ReportListItem
	sources    []Source
	captures   []ExtractRecord
	aiSettings *AISettings
	aiStats    *AIStats
	aiUsage    *AIUsage
	desktop    *DesktopSettings
	asrEnv     *ASREnv
	daily      *DailyReport
	err        error
}

// App is the native UI's root state and view.
type App struct {
	client *Client
	win    *mygo.Window

	view       string
	loadedView string
	loading    string
	seq        int

	health       Health
	stats        Stats
	items        []Item
	cursor       string
	viewItems    map[string][]Item
	viewCursors  map[string]string
	filters      map[string]*Filter
	searchDraft  string
	searchAt     time.Time
	browsers     []Browser
	aiCategories []AICategory
	dailyList    []ReportListItem
	sources      []Source
	captures     []ExtractRecord
	aiSettings   *AISettings
	aiStats      AIStats
	aiUsage      *AIUsage
	desktop      DesktopSettings
	asrEnv       *ASREnv
	errText      string

	layout int // 0 grid, 1 list

	reader     Reader
	readerWV   *mygo.WebView
	readerDocs *readerServer
	appIcon    ui.ImageSource

	// Reader enhancement results are cached per item in memory so switching
	// back to an article does not spend tokens again.
	translations map[int64]*TranslationResult

	// Reader AI conversations are kept per item, so the dialog can be closed
	// and reopened without losing the thread.
	chat  ChatState
	chats map[int64]*ChatSession

	dailyDate   string
	dailyType   string
	daily       *DailyReport
	dailyBusy   bool
	dailyPicked bool // the user chose a day and type; stop following the newest

	sourceToggles    map[string]bool
	sourceDialogOpen bool
	sourceEditing    *SourceForm
	sourceIsEdit     bool
	sourceLogOpen    bool
	sourceLogs       []FetchLog
	sourceLogTitle   string
	sourceLogDetail  *FetchLogDetail
	sourceLogLoading int64

	sourceFullContent     bool
	sourceCrawlLinks      bool
	sourceAdvancedOpen    bool
	sourceListSelector    string
	sourceArticleSelector string
	sourceArticlePattern  string
	sourceMaxItems        string
	sourceMaxBackfill     string
	sourceMinLikes        string
	sourceMinViews        string
	sourceConfigExtra     map[string]any

	aiDraft        *AISettings
	aiProfileIndex int
	aiBusy         bool
	aiTestMsg      string
	aiTestErr      bool
	aiDiscoverBusy bool
	aiDiscoverMsg  string
	aiDiscoverErr  bool

	// The model picker builds its checkboxes from the cached models.dev
	// catalog plus any list the provider itself returned. Both are keyed by
	// endpoint so switching provider or base URL does not show stale entries.
	aiCatalog       []CatalogProvider
	aiCatalogLoaded bool
	aiCatalogTried  bool
	aiCatalogBusy   bool
	aiCatalogMsg    string
	aiCatalogErr    bool
	aiDiscovered    map[string][]string
	aiModelSearch   string
	aiManualModel   string
	aiManualOpen    bool

	settingsTab string

	tokenDraft    string
	externalDraft bool
	tokenReveal   bool
	promptSystem  string
	promptDaily   string
	settingsBusy  bool
	update        updateState

	adapter       string
	adapterValue  string
	adapterBusy   bool
	adapterResult string

	confirm *Confirm

	pendingImport []byte

	toasts []toastMsg

	splitPct float32
}

func New(client *Client) *App {
	a := &App{
		client:      client,
		view:        "today",
		viewItems:   map[string][]Item{},
		viewCursors: map[string]string{},
		filters:     map[string]*Filter{},
		layout:      0,
		dailyDate:   time.Now().Format("2006-01-02"),
		dailyType:   "daily",
		adapter:     "extract",
		confirm:     &Confirm{},
		splitPct:    52,

		settingsTab:  "general",
		aiDiscovered: map[string][]string{},

		translations: map[int64]*TranslationResult{},
		chats:        map[int64]*ChatSession{},
	}
	for _, v := range []string{"today", "library", "ai"} {
		a.filters[v] = newFilter()
	}
	// The AI page opens on its quality feed, not the unfiltered library.
	a.filters["ai"].Mode = "quality"
	return a
}

// SetIconPNG hands the app icon (a PNG bitmap) to the sidebar. Decoding it is
// best-effort: the native window can still draw without one.
func (a *App) SetIconPNG(data []byte) {
	if len(data) == 0 {
		return
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return
	}
	a.appIcon = src
}

// Start attaches the app to its window, making the embedded reading web view.
// It runs on the UI thread, after NewWindow returned.
func (a *App) Start(win *mygo.Window) {
	a.win = win
	wv, err := win.NewWebView(mygo.WebViewOptions{
		BackgroundColor:   "light-dark(#ffffff, #1c1c1e)",
		ScrollToContainer: false,
	})
	if err != nil {
		a.notifyErr("无法创建阅读视图: " + err.Error())
		return
	}
	a.readerWV = wv
	a.readerDocs = newReaderServer()
	wv.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if e.URL == "" || strings.HasPrefix(e.URL, "about:") {
			return
		}
		if e.UserInitiated {
			e.PreventDefault()
			_ = mygo.Shell.OpenExternal(e.URL)
		}
	})
	// Start may run after the window drew its first frame; repaint so the
	// view can load data now that win is set.
	win.Invalidate()
}

// View is the window's content function.
func (a *App) View(c *ui.Context) {
	a.applyTheme(c)
	a.drainToasts(c)
	a.ensureData()

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).Background(canvasBg(c)).Children(func() {
			a.header(c)
			a.pageArea(c)
		})
	})
	a.dialogs(c)
}

func readerEligible(view string) bool {
	return view == "today" || view == "library" || view == "ai"
}

// pageMaxWidth caps the page column on large screens, where a card or a line
// of text stretched across the window stops reading as a composed page.
const pageMaxWidth = 1280

// pageWidth is the width of the page column. A reader pane keeps its split;
// without one, a wide window gets a capped, centered column.
func (a *App) pageWidth(c *ui.Context) float32 {
	winW, _ := c.Size()
	usable := winW - sidebarWidth
	if a.reader.Open && readerEligible(a.view) {
		return usable
	}
	limit := float32(pageMaxWidth)
	switch a.view {
	case "settings":
		// A form reads best at a narrower measure than a feed.
		limit = 920
	case "ai":
		limit = 1040
	}
	if usable > limit {
		return limit
	}
	return usable
}

// pageArea places the page, or the page and the reader beside it, centered
// when the window is wider than the page column.
func (a *App) pageArea(c *ui.Context) ui.Element {
	return ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Justify(ui.Center).Children(func() {
		ui.Column(c).Width(a.pageWidth(c)).MinWidth(0).Children(func() {
			if a.reader.Open && readerEligible(a.view) {
				ui.SplitPercent(c, &a.splitPct, func() {
					a.page(c).Grow(1)
				}, func() {
					a.readerPane(c)
				}).Grow(1)
			} else {
				a.page(c).Grow(1)
			}
		})
	})
}

// page renders the current view in the content column.
func (a *App) page(c *ui.Context) ui.Element {
	switch a.view {
	case "library":
		return a.library(c)
	case "sources":
		return a.sourcesView(c)
	case "news":
		return a.news(c)
	case "ai":
		return a.aiView(c)
	case "daily":
		return a.dailyView(c)
	case "devices":
		return a.devicesView(c)
	case "apihub":
		return a.apiHub(c)
	case "settings":
		return a.settingsView(c)
	default:
		return a.today(c)
	}
}

func (a *App) drainToasts(c *ui.Context) {
	for _, t := range a.toasts {
		typ := "info"
		if t.Error {
			typ = "error"
		}
		c.AddToast(ui.Toast{Title: t.Text, Type: typ})
	}
	a.toasts = nil
}

func (a *App) notify(text string) {
	a.pushToast(toastMsg{Text: text})
}

func (a *App) notifyErr(text string) {
	a.pushToast(toastMsg{Text: text, Error: true})
}

func (a *App) pushToast(t toastMsg) {
	if a.win == nil {
		return
	}
	a.win.Update(func() { a.toasts = append(a.toasts, t) })
}

func (a *App) port() int {
	if a.desktop.Port > 0 {
		return a.desktop.Port
	}
	if a.client != nil {
		return a.client.port()
	}
	return 0
}

// --- navigation ---

const sidebarWidth = 232

func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(sidebarWidth).Shrink(0).Background(sidebarBg(c)).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		top := c.TitleBar().Height
		if top < 34 {
			top = 34
		}
		// Room for the macOS traffic lights; the whole strip drags the window.
		ui.Row(c).Height(top).Padding(0, 14).DragWindow()
		ui.Row(c).Padding(6, 16, 16, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			a.brandMark(c)
			ui.Text(c, "Grabby").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
		})

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 12, 12, 12).Gap(3).Children(func() {
				a.navSection(c, "内容", func() {
					a.navItem(c, "today", iconToday, "今天", "")
					a.navItem(c, "library", iconLibrary, "内容库", countLabel(a.stats.UnreadCount))
					a.navItem(c, "sources", iconRSS, "RSS 源", "")
					a.navItem(c, "news", iconNews, "新闻源", "")
				})
				a.navSection(c, "智能", func() {
					a.navItem(c, "ai", iconAI, "AI 分析", "")
					a.navItem(c, "daily", iconDaily, "AI 日报", "")
				})
				a.navSection(c, "服务", func() {
					a.navItem(c, "devices", iconDevices, "浏览器设备", countLabel(len(a.browsers)))
					a.navItem(c, "apihub", iconAPI, "API 接入", "")
				})
				a.navSection(c, "偏好", func() {
					a.navItem(c, "settings", iconSettings, "设置", "")
				})
			})
		})

		ui.Divider(c)
		ui.Row(c).Padding(12, 16).Gap(7).AlignItems(ui.Center).Children(func() {
			dot := t.Danger
			if a.health.BrowserConnected {
				dot = t.Success
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			label := "扩展未连接"
			if a.health.BrowserConnected {
				label = fmt.Sprintf("%d 个浏览器在线", len(a.browsers))
			}
			ui.Text(c, label).FontSize(11).TextColor(textSoft(c)).Grow(1)
			ui.Text(c, ":"+fmt.Sprint(a.port())).FontSize(10.5).TextColor(textFaint(c)).Font("monospace")
		})
	})
}

// brandMark shows the app icon, falling back to a red monogram tile when no
// bitmap was handed in (as in the headless tests).
func (a *App) brandMark(c *ui.Context) {
	if a.appIcon != nil {
		ui.Image(c, a.appIcon).Width(28).Height(28)
		return
	}
	ui.Box(c).Size(28, 28).Radius(8).Gradient(c.Theme().Accent, ui.Hex("#ff7b89"), 135).Center().Children(func() {
		ui.Text(c, "G").FontSize(14).FontWeight(800).TextColor(ui.Hex("#ffffff"))
	})
}

func (a *App) navSection(c *ui.Context, title string, fn func()) {
	ui.Column(c).Gap(1).Children(func() {
		ui.Text(c, title).FontSize(10.5).FontWeight(600).TextColor(textFaint(c)).LetterSpacing(0.6).Padding(16, 13, 5, 13)
		fn()
	})
}

// navItem draws one navigation row. The chosen row gets the app's gray pill
// (with a red icon), not the theme's accent fill. ButtonBase keeps the rows
// reachable with Tab and Enter, as the sidebar of a Mac app.
func (a *App) navItem(c *ui.Context, id string, icon *ui.SVG, label, badge string) {
	t := c.Theme()
	selected := a.view == id
	row := ui.ButtonBase(c).Height(32).Padding(0, 9).Radius(7).Shrink(0).
		Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond})
	if selected || row.Hovered() {
		row.Background(navActive(c))
	}
	row.OnClick(func() { a.navigate(id) })
	row.Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(9).Children(func() {
			iconColor := t.Text.Alpha(0.62)
			if selected {
				iconColor = t.Accent
			}
			ui.Icon(c, icon).Size(15, 15).TextColor(iconColor)
			weight := 500
			if selected {
				weight = 600
			}
			ui.Text(c, label).FontSize(13).FontWeight(weight).TextColor(t.Text).Grow(1).MaxLines(1)
			if badge != "" {
				ui.Text(c, badge).FontSize(10.5).TextColor(textFaint(c)).MaxLines(1)
			}
		})
	})
}

func countLabel(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func (a *App) navigate(view string) {
	if !readerEligible(view) {
		a.reader.Open = false
		a.chat.Open = false
	}
	a.view = view
	if f := a.filters[view]; f != nil {
		a.searchDraft = f.Query
	}
	if view == "daily" && a.dailyDate == "" {
		a.dailyDate = time.Now().Format("2006-01-02")
	}
}

// --- header ---

func (a *App) header(c *ui.Context) {
	t := c.Theme()
	titles := map[string]string{
		"today": "今天", "library": "内容库", "sources": "RSS 源", "news": "新闻源",
		"ai": "AI 分析", "daily": "AI 日报", "devices": "浏览器设备",
		"apihub": "API 接入", "settings": "设置",
	}
	ui.Row(c).Height(60).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Justify(ui.Center).Children(func() {
			ui.Row(c).Width(a.pageWidth(c)).Padding(0, 30).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, titles[a.view]).FontSize(21).FontWeight(700).LetterSpacing(-0.4)
					sub := a.headerSubtitle()
					if sub != "" {
						ui.Text(c, sub).FontSize(11.5).TextColor(textSoft(c)).MaxLines(1)
					}
				})
				a.headerActions(c)
			})
		})
	})
}

func (a *App) headerSubtitle() string {
	if f := a.filters[a.view]; f != nil && f.SourceID != "" {
		sub := "只看 · " + a.sourceDisplayName(f.SourceID)
		if total, unread := a.sourceContentCount(f.SourceID); total > 0 {
			sub += fmt.Sprintf(" · %d 条内容", total)
			if unread > 0 {
				sub += fmt.Sprintf("，%d 未读", unread)
			}
		}
		return sub
	}
	switch a.view {
	case "today":
		return time.Now().Format("2006 年 1 月 2 日") + " · 本地优先的阅读台"
	case "library":
		return "筛选、搜索并回看全部已归档内容"
	case "sources":
		return fmt.Sprintf("%d 个订阅源", len(a.sources))
	case "news":
		return "按话题分组查看来源，并通过浏览器扩展实时采集平台内容"
	case "ai":
		return "自动评分、分类、摘要，并筛选高质量内容"
	case "daily":
		return "按日期与类型回看 AI 早报 / 晚报"
	case "devices":
		return "一个服务可同时连接多台 Chrome，按名称路由抓取任务"
	case "apihub":
		return "同一进程提供 HTTP / WebSocket / MCP，可作为浏览器自动化 API 中转站"
	case "settings":
		return "服务、模型、提示词、安全与数据"
	}
	return ""
}

func (a *App) headerActions(c *ui.Context) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		switch a.view {
		case "today", "library":
			seg := a.layout
			if ui.Segmented(c, &seg, "卡片", "列表").Changed() {
				a.layout = seg
			}
			iconButton(c, iconRefresh, "刷新").OnClick(func() { a.reloadItems(a.view) })
		case "sources":
			if iconTextButton(c, iconPlus, "添加数据源").Clicked() {
				a.openSourceDialog(nil)
			}
			iconButton(c, iconRefresh, "刷新").OnClick(func() { a.reloadView("sources") })
		case "ai":
			if iconTextButton(c, iconSettings, "模型设置").Clicked() {
				a.settingsTab = "models"
				a.navigate("settings")
			}
			if iconTextButton(c, iconBolt, "立即评测").Clicked() {
				a.runAsync("已开始全量评测", a.client.startEvaluation)
			}
			iconButton(c, iconRefresh, "刷新").OnClick(func() { a.reloadView("ai") })
		case "devices":
			iconButton(c, iconRefresh, "刷新").OnClick(func() { a.reloadView("devices") })
		case "settings":
			if iconTextButton(c, iconCheck, "刷新").Clicked() {
				a.reloadView("settings")
			}
			// The page is longer than a window, so the commit action sits in
			// the header where it stays in reach from any section. Saving
			// before the settings load would push defaults over the server's.
			ready := a.aiDraft != nil || a.aiSettings != nil
			if primaryButton(c, "保存设置", a.aiBusy || !ready).Clicked() && ready && !a.aiBusy {
				a.saveAIDraft()
			}
		}
	})
}

// --- data loading ---

func (a *App) ensureData() {
	if a.win == nil || a.loadedView == a.view || a.loading == a.view {
		return
	}
	a.loading = a.view
	a.seq++
	seq := a.seq
	view := a.view
	go func() {
		d := a.fetch(view)
		a.win.Update(func() {
			if seq != a.seq {
				return
			}
			a.loading = ""
			if d.err != nil {
				a.errText = d.err.Error()
				return
			}
			a.errText = ""
			a.applyData(view, d)
			a.loadedView = view
		})
	}()
}

func (a *App) fetch(view string) viewData {
	var d viewData
	fail := func(err error) viewData { d.err = err; return d }

	h, err := a.client.health()
	if err != nil {
		return fail(err)
	}
	d.health = &h

	switch view {
	case "today", "library":
		if s, err := a.client.stats(); err == nil {
			d.stats = &s
		}
		if b, err := a.client.browsers(); err == nil {
			d.browsers = &b
		}
		if c, err := a.client.aiCategories(); err == nil {
			d.categories = c
		}
		if l, err := a.client.dailyList(2); err == nil {
			d.dailyList = l
		}
		f := a.filters[view]
		if view == "today" {
			f.Since = startOfToday()
		}
		var p ItemsPage
		if f.Mode == "quality" {
			p, err = a.client.qualityItems(*f, "")
		} else {
			p, err = a.client.items(*f, "")
		}
		if err != nil {
			return fail(err)
		}
		d.items = &p
	case "sources":
		if s, err := a.client.sources(); err == nil {
			d.sources = s
		}
		if s, err := a.client.stats(); err == nil {
			d.stats = &s
		}
	case "news":
		if s, err := a.client.sources(); err == nil {
			d.sources = s
		}
		if c, err := a.client.captures(); err == nil {
			d.captures = c
		}
	case "ai":
		if s, err := a.client.aiSettings(); err == nil {
			d.aiSettings = s
		}
		if s, err := a.client.aiStats(); err == nil {
			d.aiStats = &s
		}
		if u, err := a.client.aiUsage(); err == nil {
			d.aiUsage = u
		}
		if c, err := a.client.aiCategories(); err == nil {
			d.categories = c
		}
		f := a.filters["ai"]
		var p ItemsPage
		if f.Mode == "quality" {
			p, err = a.client.qualityItems(*f, "")
		} else {
			p, err = a.client.items(*f, "")
		}
		if err == nil {
			d.items = &p
		}
	case "daily":
		if r, err := a.client.daily(a.dailyDate, a.dailyType); err == nil {
			d.daily = &r
		}
		if l, err := a.client.dailyList(30); err == nil {
			d.dailyList = l
		}
	case "devices":
		if b, err := a.client.browsers(); err == nil {
			d.browsers = &b
		}
	case "apihub":
		if s, err := a.client.desktopSettings(); err == nil {
			d.desktop = &s
		}
		if b, err := a.client.browsers(); err == nil {
			d.browsers = &b
		}
	case "settings":
		if s, err := a.client.desktopSettings(); err == nil {
			d.desktop = &s
		}
		if s, err := a.client.aiSettings(); err == nil {
			d.aiSettings = s
		}
		if s, err := a.client.aiStats(); err == nil {
			d.aiStats = &s
		}
		if u, err := a.client.aiUsage(); err == nil {
			d.aiUsage = u
		}
		if e, err := a.client.asrEnv(); err == nil {
			d.asrEnv = e
		}
	}
	return d
}

func (a *App) applyData(view string, d viewData) {
	if d.health != nil {
		a.health = *d.health
	}
	if d.stats != nil {
		a.stats = *d.stats
	}
	if d.items != nil {
		a.items = d.items.Items
		a.cursor = d.items.Next()
		a.viewItems[view] = a.items
		a.viewCursors[view] = a.cursor
	}
	if d.browsers != nil {
		a.browsers = d.browsers.Browsers
	}
	if d.categories != nil {
		a.aiCategories = d.categories
	}
	if d.dailyList != nil {
		a.dailyList = d.dailyList
	}
	if d.sources != nil {
		a.sources = d.sources
	}
	if d.captures != nil {
		a.captures = d.captures
	}
	if d.aiSettings != nil {
		a.aiSettings = d.aiSettings
		// Rebuild from the server copy: the first frame of a view may have
		// created a draft from defaults before settings arrived, and keeping it
		// would silently overwrite the real profiles on save.
		a.aiDraft = nil
		a.initAIDraft()
	}
	if d.aiStats != nil {
		a.aiStats = *d.aiStats
	}
	if d.aiUsage != nil {
		a.aiUsage = d.aiUsage
	}
	if d.desktop != nil {
		a.desktop = *d.desktop
		a.tokenDraft = d.desktop.APIToken
		a.externalDraft = d.desktop.ExternalEnabled
	}
	if d.asrEnv != nil {
		a.asrEnv = d.asrEnv
	}
	adopt := view == "daily" && a.adoptNewestDaily(d)
	if d.daily != nil && !adopt {
		a.daily = d.daily
	}
	if adopt {
		// The pane keeps loading rather than flashing "none" for the day and
		// type the page is leaving.
		a.reloadView("daily")
	}
}

// adoptNewestDaily opens the newest report the first time the page lands on a
// day and type that were never generated. Without it an empty pane hides the
// thirty reports the rail right beside it lists.
func (a *App) adoptNewestDaily(d viewData) bool {
	if a.dailyPicked || len(a.dailyList) == 0 {
		return false
	}
	// The request for a day and type never generated answers 200 with
	// success:true and no content, so an empty report is the "none" the pane
	// shows, not a report worth staying on.
	if d.daily != nil && d.daily.Success && (d.daily.Content != "" || d.daily.HTMLContent != "") {
		return false
	}
	newest := a.dailyList[0]
	if newest.ReportDate == a.dailyDate && newest.ReportType == a.dailyType {
		return false
	}
	a.dailyDate = newest.ReportDate
	a.dailyType = newest.ReportType
	a.dailyPicked = true
	return true
}

func (a *App) reloadView(view string) {
	if a.loadedView == view {
		a.loadedView = ""
	}
	a.ensureData()
}

func (a *App) reloadItems(view string) {
	f := a.filters[view]
	if f == nil {
		return
	}
	if view == "today" {
		f.Since = startOfToday()
	}
	go func() {
		var p ItemsPage
		var err error
		if f.Mode == "quality" {
			p, err = a.client.qualityItems(*f, "")
		} else {
			p, err = a.client.items(*f, "")
		}
		a.win.Update(func() {
			if err != nil {
				a.errText = err.Error()
				return
			}
			a.errText = ""
			a.items = p.Items
			a.cursor = p.Next()
			a.viewItems[view] = a.items
			a.viewCursors[view] = a.cursor
		})
	}()
}

func (a *App) loadMore(view string) {
	f := a.filters[view]
	cursor := a.viewCursors[view]
	if f == nil || cursor == "" {
		return
	}
	go func() {
		var p ItemsPage
		var err error
		if f.Mode == "quality" {
			p, err = a.client.qualityItems(*f, cursor)
		} else {
			p, err = a.client.items(*f, cursor)
		}
		a.win.Update(func() {
			if err != nil {
				a.notifyErr(err.Error())
				return
			}
			items := append(append([]Item(nil), a.viewItems[view]...), p.Items...)
			a.viewItems[view] = items
			a.viewCursors[view] = p.Next()
			if a.view == view {
				a.items = items
				a.cursor = p.Next()
			}
		})
	}()
}

func (a *App) currentItems() []Item {
	if items, ok := a.viewItems[a.view]; ok {
		return items
	}
	return a.items
}

// --- item actions ---

func (a *App) openReader(it Item) {
	a.reader.Open = true
	a.reader.Item = &it
	a.reader.Analysis = nil
	a.reader.Loading = true
	a.reader.Err = ""
	a.reader.Blocks = nil
	a.reader.Translation = a.translations[it.ID]
	a.reader.Translating = false
	a.reader.TranslateErr = ""
	if a.reader.Translation != nil {
		a.reader.TranslateMode = "bilingual"
	} else {
		a.reader.TranslateMode = "off"
	}
	a.chat.Open = false
	a.chat.Draft = ""
	a.reader.seq++
	seq := a.reader.seq
	id := it.ID
	if readerEligible(a.view) {
		// Keep the selected row in view on narrow windows.
	}
	go func() {
		detail, err := a.client.item(id)
		var an *Analysis
		if err == nil {
			an, _ = a.client.analysis(id)
		}
		a.win.Update(func() {
			if seq != a.reader.seq {
				return
			}
			a.reader.Loading = false
			if err != nil {
				a.reader.Err = err.Error()
				return
			}
			item := detail.Item
			if item.ID == 0 {
				item = it
			}
			a.reader.Item = &item
			a.reader.Analysis = an
			a.reader.detail = detail
			a.reader.Translation = a.translations[item.ID]
			if a.reader.Translation != nil {
				a.reader.TranslateMode = "bilingual"
			}
			page := a.buildReaderPage(item, detail, an, a.reader.Translation, a.reader.TranslateMode)
			a.reader.Blocks = page.Blocks
			a.loadReader(page.HTML, item.URL)
			if item.ReadStatus == 0 {
				_ = a.client.markRead(item.ID, true)
				a.applyLocalRead(item.ID)
			}
		})
	}()
}

func (a *App) applyLocalRead(id int64) {
	if it := a.findItem(id); it != nil {
		it.ReadStatus = 1
	}
	if a.reader.Item != nil && a.reader.Item.ID == id {
		a.reader.Item.ReadStatus = 1
	}
	if a.stats.UnreadCount > 0 {
		a.stats.UnreadCount--
	}
}

func (a *App) findItem(id int64) *Item {
	for i := range a.items {
		if a.items[i].ID == id {
			return &a.items[i]
		}
	}
	for k := range a.viewItems {
		for i := range a.viewItems[k] {
			if a.viewItems[k][i].ID == id {
				return &a.viewItems[k][i]
			}
		}
	}
	return nil
}

func (a *App) toggleStar(it Item) {
	next := it.Starred != 1
	go func() {
		if err := a.client.star(it.ID, next); err != nil {
			a.notifyErr(err.Error())
			return
		}
		a.win.Update(func() {
			if p := a.findItem(it.ID); p != nil {
				if next {
					p.Starred = 1
				} else {
					p.Starred = 0
				}
			}
			if a.reader.Item != nil && a.reader.Item.ID == it.ID {
				if next {
					a.reader.Item.Starred = 1
				} else {
					a.reader.Item.Starred = 0
				}
			}
			if a.stats.StarredCount > 0 || next {
				if next {
					a.stats.StarredCount++
				} else {
					a.stats.StarredCount--
				}
			}
		})
	}()
}

func (a *App) markRead(it Item, read bool) {
	go func() {
		if err := a.client.markRead(it.ID, read); err != nil {
			a.notifyErr(err.Error())
			return
		}
		a.win.Update(func() {
			if p := a.findItem(it.ID); p != nil {
				if read {
					p.ReadStatus = 1
				} else {
					p.ReadStatus = 0
				}
			}
			if read && a.stats.UnreadCount > 0 {
				a.stats.UnreadCount--
			}
			if !read {
				a.stats.UnreadCount++
			}
			// Keep the source filter's unread badges in step with the item.
			for i := range a.stats.SourceCounts {
				sc := &a.stats.SourceCounts[i]
				if sc.ID != it.SourceID {
					continue
				}
				if read && sc.Unread > 0 {
					sc.Unread--
				}
				if !read {
					sc.Unread++
				}
				break
			}
		})
	}()
}

func (a *App) reanalyze(it Item) {
	a.runAsync("已重新提交分析", func() error { return a.client.reanalyze(it.ID) })
}

func (a *App) runAsync(success string, fn func() error) {
	go func() {
		err := fn()
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}
			if success != "" {
				a.toasts = append(a.toasts, toastMsg{Text: success})
			}
		})
	}()
}

// --- dialogs ---

func (a *App) dialogs(c *ui.Context) {
	if a.confirm != nil && a.confirm.Open {
		title := a.confirm.Title
		msg := a.confirm.Message
		action := a.confirm.ActionLabel
		if action == "" {
			action = "确定"
		}
		switch ui.AlertDialog(c, &a.confirm.Open, title, msg, "取消", action) {
		case 1:
			a.confirm.Open = false
			run := a.confirm.Run
			if run != nil {
				a.runAsync("", run)
			}
		}
	}
	a.sourceDialog(c)
	a.sourceLogsDialog(c)
	a.readerChatDialog(c)
}

func (a *App) ask(title, message, action string, run func() error) {
	a.confirm.Open = true
	a.confirm.Title = title
	a.confirm.Message = message
	a.confirm.ActionLabel = action
	a.confirm.Run = run
}

// --- helpers shared by views ---

func (a *App) filter(c *ui.Context, view string) {
	f := a.filters[view]
	if f == nil {
		return
	}
	modes := []struct{ id, label string }{
		{"all", "全部"}, {"unread", "未读"}, {"starred", "收藏"}, {"quality", "AI 优质"},
	}
	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
		// The modes are one segmented control: a row of separately filled
		// buttons reads as four competing actions.
		labels := make([]string, len(modes))
		idx := 0
		for i, m := range modes {
			labels[i] = m.label
			if f.Mode == m.id {
				idx = i
			}
		}
		if ui.Segmented(c, &idx, labels...).Changed() {
			f.Mode = modes[idx].id
			if f.Mode == "quality" {
				f.ScoreMin = 7
			}
			a.reloadItems(view)
		}
		if f.Mode == "quality" {
			for _, cat := range a.aiCategories {
				if cat.Count == 0 {
					continue
				}
				cat := cat
				b := pill(c, cat.Name, f.AICategory == cat.Name)
				b.OnClick(func() {
					if f.AICategory == cat.Name {
						f.AICategory = ""
					} else {
						f.AICategory = cat.Name
					}
					a.reloadItems(view)
				})
			}
		} else if f.SourceCategory != "" {
			pill(c, f.SourceCategory, true).OnClick(func() {
				f.SourceCategory = ""
				a.reloadItems(view)
			})
		}
		if f.SourceID != "" {
			pill(c, "订阅源: "+truncate(a.sourceDisplayName(f.SourceID), 18), true).OnClick(func() {
				f.SourceID = ""
				a.reloadItems(view)
			})
		}
		if f.Query != "" {
			pill(c, "搜索: "+truncate(f.Query, 18), true).OnClick(func() {
				f.Query = ""
				a.searchDraft = ""
				a.reloadItems(view)
			})
		}
		if f.Mode != "all" || f.Query != "" || f.AICategory != "" || f.SourceCategory != "" || f.SourceID != "" {
			quietButton(c, "清除筛选").FontSize(11.5).OnClick(func() {
				*f = *newFilter()
				a.searchDraft = ""
				a.reloadItems(view)
			})
		}
	})
}

func (a *App) searchBar(c *ui.Context, view string) {
	f := a.filters[view]
	if f == nil {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		sf := ui.SearchField(c, &a.searchDraft).Grow(1).Height(36).Placeholder("搜索标题、摘要与正文…").Label("搜索")
		if sf.Changed() {
			a.searchAt = time.Now()
			c.After(350 * time.Millisecond)
		}
		if sf.Submitted() || (!a.searchAt.IsZero() && time.Since(a.searchAt) > 320*time.Millisecond) {
			a.searchAt = time.Time{}
			if f.Query != strings.TrimSpace(a.searchDraft) {
				f.Query = strings.TrimSpace(a.searchDraft)
				a.reloadItems(view)
			}
		}
	})
}

func (a *App) errorBanner(c *ui.Context) {
	if a.errText == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Padding(10, 14).Background(t.Danger.Alpha(0.08)).Radius(10).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.errText).FontSize(12).TextColor(t.Danger).Grow(1)
		quietButton(c, "重试").FontSize(12).OnClick(func() {
			a.errText = ""
			a.reloadView(a.view)
		})
	})
}

func (a *App) contentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	usable := w - sidebarWidth
	if a.reader.Open && readerEligible(a.view) {
		usable = usable * (100 - a.splitPct) / 100
	}
	return usable
}

// centered holds a page column at maxWidth and centers it once the window is
// wider, so a large window doesn't leave the page hugging the left edge.
// centeredIn centers a column of at most maxWidth inside avail, so a page
// narrower than its pane sits in the middle instead of hugging the left.
func centeredIn(c *ui.Context, avail, maxWidth float32, fn func()) {
	w := avail
	if w > maxWidth {
		w = maxWidth
	}
	if w < 0 {
		w = 0
	}
	ui.Row(c).Justify(ui.Center).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Width(w).Children(fn)
	})
}

func (a *App) cardColumns(c *ui.Context) int {
	n := int((a.contentWidth(c) - 64) / 280)
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}
