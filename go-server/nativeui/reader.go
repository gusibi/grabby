package nativeui

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
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

		ui.Text(c, orDefault(it.Title, "未命名内容")).FontSize(21).FontWeight(700).MaxLines(3).LineHeight(1.32).LetterSpacing(-0.35)
		meta := joinNonEmpty(formatDate(itemTime(*it)), it.SourceCategory, it.AIModelUsed)
		if meta != "" {
			ui.Text(c, meta).FontSize(11).TextColor(textFaint(c))
		}
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
	iconButton(c, iconClose, "关闭").OnClick(func() { a.reader.Open = false })
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

func (a *App) buildReaderHTML(it Item, detail ItemDetail, an *Analysis) string {
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
	b.WriteString(`</style></head><body><article>`)

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
		if an.QualityScore > 0 {
			b.WriteString(`<span class="score">AI ` + fmt.Sprint(an.QualityScore) + `/10</span>`)
		}
		if an.AICategory != "" {
			b.WriteString(`<span class="tag">` + html.EscapeString(an.AICategory) + `</span>`)
		}
		b.WriteString(`<p class="ai-summary">` + html.EscapeString(an.AISummary) + `</p>`)
		b.WriteString(`</section>`)
	}

	body := detail.HTMLContent
	if strings.TrimSpace(body) == "" {
		if strings.TrimSpace(detail.SummaryHTML) != "" {
			body = detail.SummaryHTML
		} else if strings.TrimSpace(it.Content) != "" {
			body = "<p>" + html.EscapeString(it.Content) + "</p>"
		} else {
			body = "<p>" + html.EscapeString(it.Summary) + "</p>"
		}
	}
	b.WriteString(`<div class="body">` + body + `</div>`)
	if it.URL != "" {
		b.WriteString(`<div class="origin"><a href="` + html.EscapeString(it.URL) + `" target="_blank" rel="noreferrer">在浏览器中打开原文 →</a></div>`)
	}
	b.WriteString(`</article></body></html>`)
	return b.String()
}

const readerCSS = `
:root{--paper:#ffffff;--ink:#1d1d1f;--muted:#6e6e73;--faint:#8e8e93;--line:#e4e4e9;--signal:#fa2d48;--soft:#f5f5f7;--code:#f5f5f7;color-scheme:light}
@media (prefers-color-scheme:dark){:root{--paper:#1c1c1e;--ink:#f5f5f7;--muted:#a1a1a6;--faint:#8e8e93;--line:#3d3d40;--signal:#ff453a;--soft:#242426;--code:#151516;color-scheme:dark}}
*{box-sizing:border-box}
html,body{margin:0;padding:0;background:var(--paper);color:var(--ink)}
body{font-family:"Iowan Old Style","Palatino Linotype","Songti SC","Noto Serif CJK SC","Source Han Serif SC",Georgia,"Times New Roman",serif;-webkit-font-smoothing:antialiased}
article{max-width:720px;margin:0 auto;padding:34px 40px 80px}
h1{font-size:29px;line-height:1.28;letter-spacing:-.01em;margin:0 0 10px}
.analysis{border:1px solid var(--line);background:var(--soft);border-radius:14px;padding:16px 18px;margin:0 0 26px;font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif}
.analysis .score{display:inline-block;font-size:11px;font-weight:700;color:var(--signal);margin-right:8px}
.analysis .tag{display:inline-block;font-size:10px;border:1px solid var(--line);border-radius:999px;padding:2px 8px;color:var(--muted)}
.ai-summary{font-size:13px;line-height:1.6;margin:10px 0 0;color:var(--ink)}
.body{font-size:17.5px;line-height:1.8;letter-spacing:.002em}
.body p{margin:0 0 1.15em}
.body a{color:var(--signal);text-decoration-thickness:1px;text-underline-offset:2px}
.body img,.body video{max-width:100%;height:auto;border-radius:10px;margin:10px 0}
.body figure{margin:20px 0}
.body figcaption{font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;font-size:11px;color:var(--faint);margin-top:6px;text-align:center}
.body h2,.body h3,.body h4{line-height:1.35;margin:1.6em 0 .6em;letter-spacing:-.005em}
.body h2{font-size:22px}.body h3{font-size:18px}.body h4{font-size:16px}
.body blockquote{margin:1.2em 0;padding:2px 0 2px 16px;border-left:3px solid var(--signal);color:var(--muted)}
.body pre{background:var(--code);border:1px solid var(--line);border-radius:10px;padding:14px 16px;overflow:auto;font-size:13px;line-height:1.55}
.body code{font-family:"SF Mono",ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.88em;background:var(--code);padding:1px 5px;border-radius:5px}
.body pre code{background:none;padding:0}
.body table{width:100%;border-collapse:collapse;font-size:14px;margin:16px 0}
.body th,.body td{border:1px solid var(--line);padding:8px 10px;text-align:left}
.body th{background:var(--soft);font-weight:600}
.body hr{border:none;border-top:1px solid var(--line);margin:28px 0}
.body ul,.body ol{padding-left:1.4em;margin:0 0 1.1em}
.body li{margin:.3em 0}
.player{border:1px solid var(--line);background:var(--soft);border-radius:14px;padding:16px 18px;margin:0 0 24px;font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif}
.player-kicker{font-size:10px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--signal);margin-bottom:10px}
.player audio{width:100%}
.player-duration{font-size:10.5px;color:var(--faint);margin-top:8px}
.video-head{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:10px}
.video-head .player-kicker{margin-bottom:0}
.video-head .player-duration{margin-top:0}
.video-frame{position:relative;width:100%;padding-top:56.25%;background:#000;border-radius:10px;overflow:hidden}
.video-frame iframe{position:absolute;top:0;left:0;width:100%;height:100%;border:0;display:block}
.origin{margin-top:34px;padding-top:18px;border-top:1px solid var(--line);font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","PingFang SC",sans-serif;font-size:12px}
.origin a{color:var(--signal);text-decoration:none}
@media (max-width:560px){article{padding:20px 20px 60px}h1{font-size:24px}.body{font-size:16px}}
`
