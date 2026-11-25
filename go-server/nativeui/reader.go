package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func (a *App) readerPane(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinWidth(0).Background(t.Background).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		a.readerToolbar(c)
		switch {
		case a.reader.Loading:
			ui.Column(c).Fill().Center().Gap(10).Children(func() {
				ui.Spinner(c)
				hint(c, "正在载入正文…")
			})
		case a.reader.Err != "":
			ui.Column(c).Fill().Center().Gap(10).Padding(30).Children(func() {
				ui.Text(c, "无法载入这篇内容").FontSize(16).FontWeight(650)
				hint(c, a.reader.Err)
				if a.reader.Item != nil {
					it := *a.reader.Item
					quietButton(c, "重试").OnClick(func() { a.openReader(it) })
				}
			})
		case a.readerWV != nil:
			ui.WebView(c, a.readerWV).Grow(1)
		default:
			// An item is open but its document is not built yet; the toolbar
			// already shows it, so an empty prompt here would contradict.
			if a.reader.Item != nil {
				ui.Column(c).Fill().Center().Gap(10).Children(func() {
					ui.Spinner(c)
					hint(c, "正在载入正文…")
				})
				return
			}
			emptyState(c, "选择一篇内容", "从左侧列表点开一条内容，正文会显示在这里。")
		}
	})
}

func (a *App) readerToolbar(c *ui.Context) {
	it := a.reader.Item
	if it == nil {
		return
	}
	t := c.Theme()
	winW, _ := c.Size()
	paneW := (winW - sidebarWidth) * (100 - a.splitPct) / 100
	compact := paneW < 480
	ui.Column(c).Padding(16, 24, 14, 24).Gap(9).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, shortSourceLabel(*it)).FontSize(10.5).FontWeight(700).TextColor(t.Accent).LetterSpacing(0.2).MaxLines(1)
			if !compact && it.Category != "" {
				softChip(c, categoryLabel(it.Category))
			}
			if !compact && it.AICategory != "" {
				accentChip(c, it.AICategory)
			}
			if it.QualityScore > 0 {
				scoreMark(c, it.QualityScore)
			}
			ui.Spacer(c).Grow(1)

			starred := it.Starred == 1
			sb := ui.ButtonBase(c).Children(func() {
				ui.Icon(c, starIcon(c, starred)).Size(14, 14)
			}).Label("收藏").Tooltip("收藏").Padding(4, 6).Radius(7).TextColor(textFaint(c))
			if sb.Hovered() {
				sb.Background(t.SurfaceHover)
			}
			if starred {
				sb.TextColor(ui.Hex("#e5a83b"))
			}
			if sb.Clicked() {
				a.toggleStar(*it)
			}
			readLabel := "标为未读"
			if it.ReadStatus == 0 {
				readLabel = "标为已读"
			}
			if iconButton(c, iconCheck, readLabel).Clicked() {
				a.markRead(*it, it.ReadStatus == 0)
			}
			if iconButton(c, iconRefresh, "重新分析").Clicked() {
				a.reanalyze(*it)
			}
			a.readerNav(c)
		})

		title := orDefault(it.Title, "未命名内容")
		if tr := a.reader.Translation; tr != nil && strings.TrimSpace(tr.Title) != "" && a.reader.TranslateMode == "replace" {
			title = tr.Title
		}
		ui.Text(c, title).FontSize(21).FontWeight(700).MaxLines(3).LineHeight(1.32).LetterSpacing(-0.35)
		if title := a.readerTranslatedTitle(); title != "" {
			ui.Text(c, title).FontSize(13).TextColor(textSoft(c)).MaxLines(2).LineHeight(1.45)
		}
		meta := joinNonEmpty(formatDate(itemTime(*it)), it.SourceCategory, it.AIModelUsed)
		if meta != "" {
			ui.Text(c, meta).FontSize(11).TextColor(textFaint(c))
		}
		a.readerTools(c)
		if an := a.reader.Analysis; an != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "AI 点评").FontSize(10.5).TextColor(textFaint(c))
				ui.Text(c, truncate(an.AIComment, 120)).FontSize(12).TextColor(t.TextMuted).Grow(1).MaxLines(2).LineHeight(1.5)
				if an.AITags != "" {
					softChip(c, truncate(an.AITags, 30))
				}
			})
		}
	})
}

// readerTranslatedTitle is the secondary title under the original when a
// translation is active. In replace mode the translated title becomes the main
// title instead, so this returns nothing.
func (a *App) readerTranslatedTitle() string {
	tr := a.reader.Translation
	if tr == nil || strings.TrimSpace(tr.Title) == "" {
		return ""
	}
	switch a.reader.TranslateMode {
	case "off", "replace":
		return ""
	}
	return tr.Title
}

var readerTranslateModes = []struct{ id, label string }{
	{"off", "原文"},
	{"replace", "替换"},
	{"bilingual", "对照"},
	{"hover", "悬浮"},
}

func (a *App) readerTools(c *ui.Context) {
	t := c.Theme()
	tr := a.reader.Translation
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		switch {
		case a.reader.Translating:
			ui.Spinner(c)
			ui.Text(c, "正在翻译全文…").FontSize(11.5).TextColor(textSoft(c))
		case tr != nil:
			labels := make([]string, len(readerTranslateModes))
			idx := 0
			for i, m := range readerTranslateModes {
				labels[i] = m.label
				if a.reader.TranslateMode == m.id {
					idx = i
				}
			}
			if ui.Segmented(c, &idx, labels...).Changed() {
				a.setReaderTranslateMode(readerTranslateModes[idx].id)
			}
			if iconButton(c, iconRefresh, "重新翻译").Clicked() {
				a.translateReader(true)
			}
		default:
			if iconTextButton(c, iconTranslate, "翻译").Clicked() {
				a.translateReader(false)
			}
		}

		if label := a.readerChatButtonLabel(); label != "" {
			if iconTextButton(c, iconSparkle, label).Clicked() {
				a.openReaderChat(false)
			}
		}

		errText := a.reader.TranslateErr
		if errText != "" {
			ui.Text(c, truncate(errText, 90)).FontSize(11).TextColor(t.Danger).Grow(1).MaxLines(2)
		}
	})
}

func (a *App) readerChatButtonLabel() string {
	if a.reader.Item == nil {
		return ""
	}
	if s := a.chats[a.reader.Item.ID]; s != nil && (s.Streaming || s.hasVisibleMessages()) {
		return "AI 对话"
	}
	return "AI 解读"
}

func (a *App) readerTargetLanguage() string {
	if a.aiSettings != nil && strings.TrimSpace(a.aiSettings.TranslationTargetLanguage) != "" {
		return a.aiSettings.TranslationTargetLanguage
	}
	if a.aiDraft != nil && strings.TrimSpace(a.aiDraft.TranslationTargetLanguage) != "" {
		return a.aiDraft.TranslationTargetLanguage
	}
	return "简体中文"
}

// translateReader fetches (or reuses) the paragraph-aligned translation and
// rebuilds the document with it embedded. Rebuilding, rather than scripting the
// live page, is what makes the result show in the embedded web view at once.
func (a *App) translateReader(force bool) {
	if a.reader.Translating || a.win == nil {
		return
	}
	it := a.reader.Item
	if it == nil {
		return
	}
	id := it.ID
	if !force {
		if tr := a.translations[id]; tr != nil {
			a.reader.Translation = tr
			a.reader.TranslateErr = ""
			if a.reader.TranslateMode == "off" {
				a.reader.TranslateMode = "bilingual"
			}
			a.reloadReaderDocument()
			return
		}
	}
	blocks := append([]string(nil), a.reader.Blocks...)
	if len(blocks) == 0 {
		a.notifyErr("这篇内容还没有可翻译的正文")
		return
	}
	title, target := it.Title, a.readerTargetLanguage()
	a.reader.Translating = true
	a.reader.TranslateErr = ""
	a.reader.translateSeq++
	tseq := a.reader.translateSeq
	rseq := a.reader.seq
	go func() {
		res, err := a.client.translate(id, title, target, blocks)
		if err != nil {
			a.win.Update(func() {
				if a.reader.translateSeq == tseq && a.reader.Item != nil && a.reader.Item.ID == id {
					a.reader.Translating = false
					a.reader.TranslateErr = err.Error()
				}
			})
			a.notifyErr(err.Error())
			return
		}
		scroll := a.readerScrollY()
		a.win.Update(func() {
			tr := res
			a.translations[id] = &tr
			if a.reader.translateSeq != tseq || a.reader.Item == nil || a.reader.Item.ID != id || !a.reader.Open {
				return
			}
			if a.reader.seq != rseq {
				scroll = 0
			}
			a.reader.Translating = false
			a.reader.Translation = &tr
			a.reader.TranslateErr = ""
			if a.reader.TranslateMode == "off" {
				a.reader.TranslateMode = "bilingual"
			}
			page := a.buildReaderPageAt(*a.reader.Item, a.reader.detail, a.reader.Analysis,
				a.reader.Translation, a.reader.TranslateMode, scroll)
			a.reader.Blocks = page.Blocks
			a.loadReader(page.HTML, a.reader.Item.URL)
		})
	}()
}

func (a *App) setReaderTranslateMode(mode string) {
	a.reader.TranslateMode = mode
	if a.reader.Translation == nil {
		if mode != "off" {
			a.translateReader(false)
		}
		return
	}
	a.reloadReaderDocument()
}

// reloadReaderDocument rebuilds the reader page from the current state,
// keeping the scroll position the page reports.
func (a *App) reloadReaderDocument() {
	if a.reader.Item == nil {
		return
	}
	if a.win == nil {
		page := a.buildReaderPageAt(*a.reader.Item, a.reader.detail, a.reader.Analysis,
			a.reader.Translation, a.reader.TranslateMode, 0)
		a.reader.Blocks = page.Blocks
		a.loadReader(page.HTML, a.reader.Item.URL)
		return
	}
	id := a.reader.Item.ID
	seq := a.reader.seq
	go func() {
		scroll := a.readerScrollY()
		a.win.Update(func() {
			if a.reader.Item == nil || a.reader.Item.ID != id || a.reader.seq != seq || !a.reader.Open {
				return
			}
			page := a.buildReaderPageAt(*a.reader.Item, a.reader.detail, a.reader.Analysis,
				a.reader.Translation, a.reader.TranslateMode, scroll)
			a.reader.Blocks = page.Blocks
			a.loadReader(page.HTML, a.reader.Item.URL)
		})
	}()
}

func (a *App) readerScrollY() int {
	if a.readerWV == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	v, err := a.readerWV.Page().EvalContext(ctx, "window.scrollY")
	if err != nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func (a *App) readerNav(c *ui.Context) {
	items := a.currentItems()
	if len(items) == 0 || a.reader.Item == nil {
		return
	}
	idx := -1
	for i := range items {
		if items[i].ID == a.reader.Item.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if idx > 0 {
		prev := items[idx-1]
		iconButton(c, iconChevronL, "上一篇").OnClick(func() { a.openReader(prev) })
	}
	if idx < len(items)-1 {
		next := items[idx+1]
		iconButton(c, iconChevronR, "下一篇").OnClick(func() { a.openReader(next) })
	}
	iconButton(c, iconClose, "关闭").OnClick(func() { a.reader.Open = false; a.chat.Open = false })
}

var audioURLRe = regexp.MustCompile(`(?i)https?://[^\s"'<>]+\.(?:mp3|m4a|aac|ogg|wav)(?:\?[^\s"'<>]*)?`)

// podcastMedia pulls the audio URL and duration out of an item's metadata,
// falling back to a direct audio link in the body.
func podcastMedia(it Item) (audio, duration, podcast string) {
	if it.Metadata != "" {
		var meta map[string]any
		if json.Unmarshal([]byte(it.Metadata), &meta) == nil {
			pick := func(keys ...string) string {
				for _, k := range keys {
					if v, ok := meta[k].(string); ok && strings.TrimSpace(v) != "" {
						return strings.TrimSpace(v)
					}
				}
				return ""
			}
			audio = pick("media_url", "audio_url", "audio")
			duration = pick("duration")
			podcast = pick("podcast", "show", "channel")
		}
	}
	if audio == "" {
		raw := it.Content + " " + it.Summary
		if m := audioURLRe.FindString(raw); m != "" {
			audio = m
		}
	}
	if strings.HasPrefix(audio, "//") {
		audio = "https:" + audio
	}
	if podcast == "" {
		podcast = it.OriginSource
	}
	return
}

var youTubeIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)

// videoMedia pulls the YouTube video ID, channel and duration out of an item's
// metadata, falling back to the watch URL when the metadata predates video_id.
func videoMedia(it Item) (video, channel, duration string) {
	if it.Metadata != "" {
		var meta map[string]any
		if json.Unmarshal([]byte(it.Metadata), &meta) == nil {
			pick := func(keys ...string) string {
				for _, k := range keys {
					if v, ok := meta[k].(string); ok && strings.TrimSpace(v) != "" {
						return strings.TrimSpace(v)
					}
				}
				return ""
			}
			video = pick("video_id")
			channel = pick("channel", "author")
			duration = pick("duration")
		}
	}
	if !youTubeIDRe.MatchString(video) {
		video = youTubeVideoID(it.URL)
	}
	if channel == "" {
		channel = it.OriginSource
	}
	return
}

// youTubeVideoID extracts the ID from watch, shorts, live, embed and youtu.be
// URLs. The desktop UI is a separate consumer of the HTTP API, so it keeps its
// own parser instead of importing the scraping package.
func youTubeVideoID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	valid := func(id string) bool { return youTubeIDRe.MatchString(id) }
	switch {
	case host == "youtu.be":
		if id := strings.SplitN(path, "/", 2)[0]; valid(id) {
			return id
		}
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com"):
		if path == "watch" {
			if id := u.Query().Get("v"); valid(id) {
				return id
			}
		}
		for _, prefix := range []string{"shorts/", "live/", "embed/"} {
			if strings.HasPrefix(path, prefix) {
				if id := strings.SplitN(strings.TrimPrefix(path, prefix), "/", 2)[0]; valid(id) {
					return id
				}
			}
		}
	}
	return ""
}

// readerPage is a rendered reader document plus the source text of each
// translatable block, aligned by index with a Translation's entries.
type readerPage struct {
	HTML   string
	Blocks []string
}

func (a *App) buildReaderHTML(it Item, detail ItemDetail, an *Analysis) string {
	return a.buildReaderPage(it, detail, an, nil, "off").HTML
}

func (a *App) buildReaderPage(it Item, detail ItemDetail, an *Analysis, tr *TranslationResult, mode string) readerPage {
	return a.buildReaderPageAt(it, detail, an, tr, mode, 0)
}

func (a *App) buildReaderPageAt(it Item, detail ItemDetail, an *Analysis, tr *TranslationResult, mode string, scroll int) readerPage {
	var b strings.Builder
	// The native toolbar above the web view already shows the source, title,
	// meta and AI comment, so the document starts with the media player and the
	// body to avoid repeating a second headline.
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString(`<meta name="color-scheme" content="light dark">`)
	// The document may be served from a loopback URL so its page has an origin
	// YouTube embeds accept; the base keeps relative links and images resolving
	// against the item's own site, as they did when the page was LoadHTML'd.
	if it.URL != "" {
		b.WriteString(`<base href="` + html.EscapeString(it.URL) + `">`)
	}
	b.WriteString(`<title>` + html.EscapeString(orDefault(it.Title, "未命名内容")) + `</title><style>`)
	b.WriteString(readerCSS)
	b.WriteString(`</style></head><body data-gb-mode="` + html.EscapeString(modeOrDefault(mode)) + `"><article>`)

	if video, channel, duration := videoMedia(it); video != "" {
		b.WriteString(`<section class="player video">`)
		b.WriteString(`<div class="video-head"><span class="player-kicker">视频 · ` + html.EscapeString(channel) + `</span>`)
		if duration != "" {
			b.WriteString(`<span class="player-duration">时长 ` + html.EscapeString(duration) + `</span>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`<div class="video-frame"><iframe src="https://www.youtube.com/embed/` + url.PathEscape(video) +
			`?rel=0&amp;playsinline=1" title="` + html.EscapeString(it.Title) +
			`" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; fullscreen" allowfullscreen referrerpolicy="strict-origin-when-cross-origin"></iframe></div>`)
		b.WriteString(`</section>`)
	}

	if audio, duration, podcast := podcastMedia(it); audio != "" {
		b.WriteString(`<section class="player"><div class="player-kicker">播客 · ` + html.EscapeString(podcast) + `</div>`)
		b.WriteString(`<audio controls preload="none" src="` + html.EscapeString(audio) + `"></audio>`)
		if duration != "" {
			b.WriteString(`<div class="player-duration">时长 ` + html.EscapeString(duration) + `</div>`)
		}
		b.WriteString(`</section>`)
	}

	if an != nil && strings.TrimSpace(an.AISummary) != "" {
		b.WriteString(`<section class="analysis">`)
		b.WriteString(`<div class="analysis-head"><span class="analysis-kicker">AI 摘要</span>`)
		if an.QualityScore > 0 {
			b.WriteString(`<span class="score">` + fmt.Sprint(an.QualityScore) + `/10</span>`)
		}
		if an.AICategory != "" {
			b.WriteString(`<span class="tag">` + html.EscapeString(an.AICategory) + `</span>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`<p class="ai-summary">` + html.EscapeString(an.AISummary) + `</p>`)
		b.WriteString(`</section>`)
	}

	pdfURL, pdfPages := itemPDF(it)
	if pdfURL != "" {
		hasText := strings.TrimSpace(detail.HTMLContent) != "" || strings.TrimSpace(it.Content) != ""
		b.WriteString(`<section class="pdf-card">`)
		kicker := "PDF 文档"
		if pdfPages > 0 {
			kicker = fmt.Sprintf("PDF 文档 · %d 页", pdfPages)
		}
		b.WriteString(`<div class="pdf-kicker">` + html.EscapeString(kicker) + `</div>`)
		note := "这份 PDF 没有可提取的文字层（可能是扫描件），请打开原文阅读。"
		if hasText {
			note = "Grabby 已提取 PDF 的文字版，下面的排版可能与原文不同。"
		}
		b.WriteString(`<p class="pdf-note">` + note + `</p>`)
		b.WriteString(`<a class="pdf-open" href="` + html.EscapeString(pdfURL) + `" target="_blank" rel="noreferrer">打开 PDF 原文 →</a>`)
		b.WriteString(`</section>`)
	}

	body := detail.HTMLContent
	if strings.TrimSpace(body) == "" {
		if strings.TrimSpace(detail.SummaryHTML) != "" {
			body = detail.SummaryHTML
		} else if strings.TrimSpace(it.Content) != "" {
			body = plainTextHTML(it.Content)
		} else {
			body = plainTextHTML(it.Summary)
		}
	}
	var translations []string
	if tr != nil {
		translations = tr.Translations
	}
	prepared, blocks := prepareReaderBody(body, translations, it.Title)
	b.WriteString(`<div class="body">` + prepared + `</div>`)
	originLabel := "在浏览器中打开原文 →"
	if pdfURL != "" {
		originLabel = "打开 PDF 原文 →"
	}
	if it.URL != "" {
		b.WriteString(`<div class="origin"><a href="` + html.EscapeString(it.URL) + `" target="_blank" rel="noreferrer">` + originLabel + `</a></div>`)
	}
	b.WriteString(`</article>`)
	if scroll > 0 {
		y := strconv.Itoa(scroll)
		b.WriteString(`<script>(function(y){window.scrollTo(0,y);requestAnimationFrame(function(){window.scrollTo(0,y)});window.addEventListener("load",function(){window.scrollTo(0,y)},{once:true})})(` + y + `);</script>`)
	}
	b.WriteString(`</body></html>`)
	return readerPage{HTML: b.String(), Blocks: blocks}
}

func modeOrDefault(mode string) string {
	switch mode {
	case "off", "replace", "bilingual", "hover":
		return mode
	}
	return "off"
}

// plainTextHTML keeps the paragraph structure of plain-text items instead of
// flattening them into one run-on paragraph.
func plainTextHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(text, "\n\n")
	var b strings.Builder
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lines := strings.Split(part, "\n")
		for i, line := range lines {
			lines[i] = html.EscapeString(strings.TrimSpace(line))
		}
		b.WriteString("<p>" + strings.Join(lines, "<br>") + "</p>")
	}
	return b.String()
}

// readerTranslatable reports the block elements wrapped one-by-one for
// translation. Containers are recursed into instead, so nested paragraphs in
// feed HTML still map to their own translations.
func readerTranslatable(tag string) bool {
	switch tag {
	case "p", "h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "ul", "ol", "dl", "table", "figcaption":
		return true
	}
	return false
}

// readerInlineNode reports whether n is an inline node that belongs inside a
// synthetic paragraph when a container mixes text and inline tags directly.
func readerInlineNode(n *nethtml.Node) bool {
	if n.Type == nethtml.TextNode {
		return true
	}
	if n.Type != nethtml.ElementNode {
		return false
	}
	switch n.Data {
	case "a", "abbr", "b", "bdi", "bdo", "br", "cite", "data", "del", "dfn",
		"em", "font", "i", "ins", "kbd", "label", "mark", "q", "rp", "rt",
		"ruby", "s", "samp", "small", "span", "strong", "sub", "sup", "time",
		"u", "var", "wbr":
		return true
	}
	return false
}

// groupInlineRuns wraps contiguous runs of inline children inside a container
// into a single <p class="gb-lead"> when the run contains translatable text,
// preventing inline elements like <a> or <b> from splitting a sentence across
// multiple blocks or remaining unwrapped outside .gb-block.
func groupInlineRuns(container *nethtml.Node) {
	for child := container.FirstChild; child != nil; {
		if !readerInlineNode(child) {
			child = child.NextSibling
			continue
		}
		var run []*nethtml.Node
		cur := child
		for cur != nil && readerInlineNode(cur) {
			run = append(run, cur)
			cur = cur.NextSibling
		}
		child = cur

		var textBuilder strings.Builder
		for _, rn := range run {
			if t := readerNodeText(rn); t != "" {
				if textBuilder.Len() > 0 {
					textBuilder.WriteByte(' ')
				}
				textBuilder.WriteString(t)
			}
		}
		if !readerHasText(textBuilder.String()) {
			continue
		}

		p := &nethtml.Node{
			Type:     nethtml.ElementNode,
			Data:     "p",
			DataAtom: atom.P,
			Attr:     []nethtml.Attribute{{Key: "class", Val: "gb-lead"}},
		}
		container.InsertBefore(p, run[0])
		for _, rn := range run {
			container.RemoveChild(rn)
			p.AppendChild(rn)
		}
	}
}

// prepareReaderBody wraps each translatable block in a stable container and
// returns the source text of those blocks. Wrapping preserves the original
// markup, so the reading styles and stored links keep working.
func prepareReaderBody(bodyHTML string, translations []string, title string) (string, []string) {
	ctx := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := nethtml.ParseFragment(strings.NewReader(bodyHTML), ctx)
	if err != nil || len(nodes) == 0 {
		return bodyHTML, nil
	}
	root := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	stripDuplicateLeadHeading(root, title)
	groupInlineRuns(root)
	var blocks []string
	wrapReaderBlocks(root, &blocks, translations)
	var b strings.Builder
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		_ = nethtml.Render(&b, n)
	}
	return b.String(), blocks
}

// stripDuplicateLeadHeading drops the article's own <h1> when it repeats the
// title the native toolbar already shows above the web view.
func stripDuplicateLeadHeading(root *nethtml.Node, title string) {
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.TextNode && strings.TrimSpace(child.Data) == "" {
			continue
		}
		if child.Type == nethtml.ElementNode && child.Data == "details" && hasHTMLClass(child, "gb-frontmatter") {
			continue
		}
		if child.Type == nethtml.ElementNode && (child.Data == "h1" || child.Data == "h2") {
			heading := strings.Join(strings.Fields(readerNodeText(child)), " ")
			lowerHeading, lowerTitle := strings.ToLower(heading), strings.ToLower(title)
			if heading != "" && isDuplicateHeading(lowerHeading, lowerTitle) {
				root.RemoveChild(child)
			}
		}
		return
	}
}

func hasHTMLClass(n *nethtml.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, field := range strings.Fields(a.Val) {
				if field == class {
					return true
				}
			}
		}
	}
	return false
}

func isDuplicateHeading(lowerHeading, lowerTitle string) bool {
	if lowerHeading == lowerTitle {
		return true
	}
	if !strings.Contains(lowerTitle, lowerHeading) && !strings.Contains(lowerHeading, lowerTitle) {
		return false
	}
	hr, tr := len([]rune(lowerHeading)), len([]rune(lowerTitle))
	shorter, longer := hr, tr
	if shorter > longer {
		shorter, longer = longer, shorter
	}
	return shorter >= 12 || (longer > 0 && shorter*10 >= longer*6)
}

func wrapReaderBlocks(n *nethtml.Node, blocks *[]string, translations []string) {
	switch n.Type {
	case nethtml.TextNode:
		if strings.TrimSpace(n.Data) == "" || n.Parent == nil {
			return
		}
		p := &nethtml.Node{Type: nethtml.ElementNode, Data: "p", DataAtom: atom.P, Attr: []nethtml.Attribute{{Key: "class", Val: "gb-lead"}}}
		parent := n.Parent
		parent.InsertBefore(p, n)
		parent.RemoveChild(n)
		p.AppendChild(n)
		wrapReaderBlocks(p, blocks, translations)
		return
	case nethtml.ElementNode:
	default:
		return
	}

	switch n.Data {
	case "script", "style", "pre", "code", "iframe", "audio", "video", "svg", "object", "embed":
		return
	case "details":
		if hasHTMLClass(n, "gb-frontmatter") {
			return
		}
		groupInlineRuns(n)
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			wrapReaderBlocks(child, blocks, translations)
			child = next
		}
		return
	case "div", "section", "article", "main", "aside", "header", "footer", "body", "figure":
		groupInlineRuns(n)
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			wrapReaderBlocks(child, blocks, translations)
			child = next
		}
		return
	}
	if !readerTranslatable(n.Data) || n.Parent == nil {
		return
	}
	text := readerNodeText(n)
	if !readerHasText(text) {
		return
	}
	idx := len(*blocks)
	*blocks = append(*blocks, text)

	attrs := []nethtml.Attribute{
		{Key: "class", Val: "gb-block"},
		{Key: "data-gb", Val: strconv.Itoa(idx)},
	}
	translation := ""
	if idx < len(translations) {
		translation = strings.TrimSpace(translations[idx])
	}
	if translation != "" {
		attrs = append(attrs, nethtml.Attribute{Key: "data-gb-filled", Val: "1"})
	}
	wrapper := &nethtml.Node{
		Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div,
		Attr: attrs,
	}
	orig := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div, Attr: []nethtml.Attribute{{Key: "class", Val: "gb-orig"}}}
	trans := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div, Attr: []nethtml.Attribute{{Key: "class", Val: "gb-trans"}}}
	if translation != "" {
		trans.AppendChild(&nethtml.Node{Type: nethtml.TextNode, Data: translation})
	}
	parent := n.Parent
	parent.InsertBefore(wrapper, n)
	parent.RemoveChild(n)
	orig.AppendChild(n)
	wrapper.AppendChild(orig)
	wrapper.AppendChild(trans)
}

func readerNodeText(n *nethtml.Node) string {
	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(x *nethtml.Node) {
		if x.Type == nethtml.TextNode {
			b.WriteString(x.Data)
			return
		}
		if x.Type == nethtml.ElementNode {
			switch x.Data {
			case "script", "style":
				return
			case "br":
				b.WriteString(" ")
				return
			case "li":
				if strings.TrimSpace(b.String()) != "" {
					b.WriteString("；")
				}
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func readerHasText(s string) bool {
	if len([]rune(s)) < 2 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// itemPDF reports the PDF behind an item. Newer items record the file in
// metadata (content_source "pdf" means no text layer); items stored before
// that can still be recognised by a .pdf link.
func itemPDF(it Item) (string, int) {
	var meta struct {
		PDFURL   string `json:"pdf_url"`
		PDFPages int    `json:"pdf_pages"`
	}
	if strings.TrimSpace(it.Metadata) != "" {
		_ = json.Unmarshal([]byte(it.Metadata), &meta)
	}
	if meta.PDFURL == "" && looksLikePDFURL(it.URL) {
		meta.PDFURL = it.URL
	}
	return meta.PDFURL, meta.PDFPages
}

func looksLikePDFURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".pdf")
}

const readerCSS = `
:root{
  --paper:#ffffff;--ink:#1b1b1f;--muted:#5d6168;--faint:#8d9199;
  --line:#e7e6e2;--line-soft:#f0efec;--signal:#d93a4e;--soft:#f7f7f4;
  --code:#f4f4f1;--accent-soft:rgba(217,58,78,.09);--shadow:0 18px 50px -30px rgba(24,24,28,.35);
  color-scheme:light;
}
@media (prefers-color-scheme:dark){
  :root{--paper:#1b1b1d;--ink:#f2f2f4;--muted:#a6a9b0;--faint:#83868d;
  --line:#37373b;--line-soft:#2b2b2f;--signal:#ff5a6b;--soft:#242427;
  --code:#141416;--accent-soft:rgba(255,90,107,.13);--shadow:0 22px 60px -34px rgba(0,0,0,.8);color-scheme:dark}
}
*{box-sizing:border-box}
html,body{margin:0;padding:0;background:var(--paper);color:var(--ink)}
body{
  font-family:"Source Serif 4","Iowan Old Style","Palatino Linotype","Songti SC","Noto Serif CJK SC","Source Han Serif SC",Georgia,"Times New Roman",serif;
  font-size:18px;line-height:1.86;letter-spacing:.003em;
  -webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;
}
article{max-width:700px;margin:0 auto;padding:42px 44px 96px}
.body{font-size:18px;line-height:1.86;color:var(--ink)}
.body p{margin:0 0 1.24em}
.body .gb-lead{font-size:1.04em;color:var(--ink)}
.body a{color:var(--ink);text-decoration:none;border-bottom:1px solid var(--signal);padding-bottom:1px;transition:color .15s ease,border-color .15s ease}
.body a:hover{color:var(--signal)}
.body img,.body video{max-width:100%;height:auto;border-radius:12px;margin:14px 0;box-shadow:0 18px 46px -30px rgba(20,20,24,.45)}
.body figure{margin:24px 0}
.body figcaption{font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;font-size:11.5px;color:var(--faint);margin-top:8px;text-align:center;letter-spacing:.01em}
.body h1,.body h2,.body h3,.body h4,.body h5{line-height:1.38;letter-spacing:-.012em;margin:1.9em 0 .65em;color:var(--ink)}
.body h2{font-size:23px;padding-top:.2em}
.body h3{font-size:19px}
.body h4{font-size:17px}
.body h5{font-size:15.5px;color:var(--muted)}
.body strong{font-weight:680}
.body blockquote{margin:1.6em 0;padding:14px 18px;border-left:3px solid var(--signal);background:var(--soft);border-radius:0 12px 12px 0;color:var(--muted);font-size:.98em}
.body blockquote p:last-child{margin-bottom:0}
.body pre{background:var(--code);border:1px solid var(--line);border-radius:12px;padding:15px 17px;overflow:auto;font-size:13px;line-height:1.62;margin:1.5em 0}
.body code{font-family:"SF Mono",ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.86em;background:var(--code);padding:1.5px 5px;border-radius:5px}
.body pre code{background:none;padding:0;font-size:1em}
.body table{width:100%;border-collapse:collapse;font-size:14.5px;line-height:1.6;margin:1.7em 0}
.body th,.body td{border:1px solid var(--line);padding:9px 12px;text-align:left;vertical-align:top}
.body th{background:var(--soft);font-weight:640}
.body tr:nth-child(even) td{background:var(--line-soft)}
.body hr{border:none;border-top:2px solid var(--line);width:48px;margin:40px auto}
.body ul,.body ol{padding-left:1.45em;margin:0 0 1.2em}
.body li{margin:.38em 0}
.body li::marker{color:var(--faint)}
.body small{color:var(--muted)}
.body .gb-frontmatter{
  border:1px solid var(--line);border-radius:12px;background:var(--soft);
  margin:0 0 24px;overflow:hidden;
  font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;
}
.body .gb-frontmatter > summary{
  cursor:pointer;list-style:none;padding:10px 14px;
  font-size:12px;font-weight:600;color:var(--muted);
  display:flex;align-items:center;justify-content:space-between;user-select:none;
}
.body .gb-frontmatter > summary::-webkit-details-marker{display:none}
.body .gb-frontmatter > summary::after{content:"▸";font-size:11px;color:var(--faint);transition:transform .15s ease}
.body .gb-frontmatter[open] > summary::after{transform:rotate(90deg)}
.body .gb-frontmatter > summary:hover{color:var(--ink)}
.body .gb-frontmatter table{
  width:100%;margin:0;border-collapse:collapse;border-top:1px solid var(--line);
  font-size:12.5px;line-height:1.55;table-layout:fixed;
}
.body .gb-frontmatter th,.body .gb-frontmatter td{
  border:none;border-bottom:1px solid var(--line-soft);padding:7px 12px;
  vertical-align:top;background:transparent!important;
}
.body .gb-frontmatter th{width:84px;color:var(--faint);font-weight:550;white-space:nowrap}
.body .gb-frontmatter td{color:var(--ink);word-break:break-word;overflow-wrap:anywhere}
.body .gb-frontmatter tr:last-child th,.body .gb-frontmatter tr:last-child td{border-bottom:none}
.body .gb-frontmatter a{color:var(--muted);border-bottom:1px solid var(--line)}
.body .gb-frontmatter a:hover{color:var(--signal);border-bottom-color:var(--signal)}
.analysis{
  border:1px solid var(--line);border-radius:16px;padding:17px 20px;margin:0 0 30px;
  background:linear-gradient(180deg,var(--soft),var(--paper));
  font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;
  box-shadow:var(--shadow);
}
.analysis-head{display:flex;align-items:center;gap:8px;margin-bottom:9px;flex-wrap:wrap}
.analysis-kicker{font-size:10px;font-weight:700;letter-spacing:.13em;text-transform:uppercase;color:var(--signal)}
.analysis .score{display:inline-block;font-size:10.5px;font-weight:700;color:var(--signal);background:var(--accent-soft);border-radius:999px;padding:2px 8px}
.analysis .tag{display:inline-block;font-size:10px;border:1px solid var(--line);border-radius:999px;padding:2px 8px;color:var(--muted)}
.ai-summary{font-size:13.5px;line-height:1.7;margin:0;color:var(--ink)}
.player,.pdf-card{
  border:1px solid var(--line);background:var(--soft);border-radius:16px;padding:16px 18px;margin:0 0 26px;
  font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;box-shadow:var(--shadow);
}
.player-kicker,.pdf-kicker{font-size:10px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--signal);margin-bottom:10px}
.player audio{width:100%}
.player-duration,.pdf-note{font-size:11.5px;color:var(--muted);margin-top:0;margin-bottom:12px;line-height:1.6}
.pdf-note{margin-top:0}
.pdf-open{display:inline-block;background:var(--signal);color:#fff;font-size:12.5px;font-weight:600;text-decoration:none;padding:8px 15px;border-radius:9px}
.video-head{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:10px}
.video-head .player-kicker{margin-bottom:0}
.video-head .player-duration{margin-top:0;margin-bottom:0}
.video-frame{position:relative;width:100%;padding-top:56.25%;background:#000;border-radius:12px;overflow:hidden}
.video-frame iframe{position:absolute;top:0;left:0;width:100%;height:100%;border:0;display:block}
.origin{margin-top:40px;padding-top:20px;border-top:1px solid var(--line);font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;font-size:12px}
.origin a{color:var(--signal);text-decoration:none;font-weight:550}

/* Translation layers */
.gb-block{position:relative}
.gb-trans{display:none}
.gb-trans:empty{display:none!important}
body[data-gb-mode="replace"] .gb-block[data-gb-filled="1"] .gb-orig{display:none}
body[data-gb-mode="replace"] .gb-trans{display:block;margin:0 0 1.24em;line-height:1.86}
body[data-gb-mode="bilingual"] .gb-trans{
  display:block;margin:-.1em 0 1.25em;padding:.05em 0 .05em 14px;
  border-left:2px solid var(--signal);color:var(--muted);font-size:.92em;line-height:1.76;
}
body[data-gb-mode="bilingual"] .gb-orig > :last-child,
body[data-gb-mode="replace"] .gb-orig > :last-child{margin-bottom:0}
body[data-gb-mode="hover"] .gb-block:hover .gb-orig{background:var(--accent-soft);border-radius:6px;box-shadow:0 0 0 5px var(--accent-soft)}
body[data-gb-mode="hover"] .gb-block:hover .gb-trans{
  display:block;position:absolute;left:0;right:0;top:calc(100% + 8px);z-index:30;
  background:var(--paper);border:1px solid var(--line);border-radius:12px;
  padding:11px 15px;font-size:.88em;line-height:1.7;color:var(--ink);box-shadow:0 24px 60px -28px rgba(20,20,24,.55);
}
@media (max-width:560px){article{padding:26px 20px 70px}.body{font-size:17px;line-height:1.8}.body h2{font-size:21px}.body h3{font-size:18px}}
`
