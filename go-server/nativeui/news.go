package nativeui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type adapterInfo struct {
	ID    string
	Label string
	Hint  string
}

var adapters = []adapterInfo{
	{"extract", "网页提取为 Markdown", "https://example.com/article"},
	{"twitter-search", "X 搜索", "关键词，例如：AI agents"},
	{"twitter-timeline", "X 时间线", "for_you 或 following"},
	{"twitter-likes", "X 喜欢", "留空即可"},
	{"reddit-subreddit", "Reddit 板块", "subreddit，例如：golang"},
	{"reddit-search", "Reddit 搜索", "关键词"},
	{"reddit-thread", "Reddit 帖子", "帖子 URL"},
	{"xhs-search", "小红书搜索", "关键词"},
	{"xhs-note", "小红书笔记", "笔记 URL"},
	{"gsc", "Google Search Console", "sites 或 performance"},
}

func adapterByID(id string) adapterInfo {
	for _, a := range adapters {
		if a.ID == id {
			return a
		}
	}
	return adapters[0]
}

func (a *App) news(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 30).Gap(16).Children(func() {
			a.errorBanner(c)

			groups := map[string][]Source{}
			for _, s := range a.sources {
				g := s.Category
				if g == "" {
					g = "未分组"
				}
				groups[g] = append(groups[g], s)
			}

			ui.Text(c, "来源分组").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
			if len(groups) == 0 {
				emptyState(c, "尚无分组", "先在 RSS 源里给订阅源设置「主题分组」。")
			} else {
				cols := int((a.contentWidth(c) - 64) / 250)
				if cols < 2 {
					cols = 2
				}
				if cols > 4 {
					cols = 4
				}
				ui.Grid(c).Columns(cols).Gap(16).Children(func() {
					for _, name := range groupNames(groups) {
						a.groupCard(c, name, groups[name])
					}
				})
			}

			a.platformCapture(c)

			ui.Text(c, "最近的网页提取归档").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
			if len(a.captures) == 0 {
				hint(c, "暂无归档。执行一次「网页提取为 Markdown」后会显示在这里。")
			} else {
				flatPanel(c, func() {
					for i, cap := range a.captures {
						if i > 0 {
							ui.Divider(c)
						}
						ui.Row(c).Padding(11, 16).Gap(12).AlignItems(ui.Center).Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
								ui.Text(c, orDefault(cap.Title, cap.URL)).FontSize(12.5).FontWeight(550).MaxLines(1)
								ui.Text(c, cap.URL).FontSize(10.5).TextColor(textSoft(c)).Font("monospace").MaxLines(1)
							})
							softChip(c, fmt.Sprintf("%d 字符", cap.Chars))
							if cap.URL != "" {
								quietButton(c, "打开").FontSize(11).OnClick(func() { _ = openExternal(cap.URL) })
							}
						})
					}
				})
			}
		})
	})
}

// groupNames orders source groups largest first, ties by name. The view is
// rebuilt every frame, so a map's iteration order would make the cards jump.
func groupNames(groups map[string][]Source) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(groups[names[i]]) != len(groups[names[j]]) {
			return len(groups[names[i]]) > len(groups[names[j]])
		}
		return names[i] < names[j]
	})
	return names
}

func (a *App) groupCard(c *ui.Context, name string, list []Source) {
	t := c.Theme()
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Name)
	}
	card := ui.ButtonBase(c).Padding(0).Height(118).Background(t.Surface).Border(1, t.Border).Radius(14).
		Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond})
	if !t.Dark {
		card = card.Shadow(0, 6, 18, -8, ui.Hex("#000000").Alpha(0.09))
	}
	if card.Hovered() {
		card.BorderColor(t.Text.Alpha(0.16))
	}
	card.Children(func() {
		ui.Column(c).Grow(1).Padding(15, 17).Gap(6).Children(func() {
			ui.Text(c, name).FontSize(14).FontWeight(650).LetterSpacing(-0.2).MaxLines(1)
			ui.Text(c, fmt.Sprintf("%d 个源", len(list))).FontSize(10.5).TextColor(textFaint(c))
			ui.Text(c, truncate(strings.Join(names, "、"), 44)).FontSize(11).TextColor(textSoft(c)).MaxLines(2).LineHeight(1.5)
		})
	})
	if card.Clicked() {
		f := newFilter()
		f.SourceCategory = name
		a.filters["library"] = f
		a.navigate("library")
		a.reloadView("library")
	}
}

func (a *App) platformCapture(c *ui.Context) {
	info := adapterByID(a.adapter)
	cardSurface(c, 16).Children(func() {
		ui.Column(c).Padding(20, 22).Gap(12).Children(func() {
			ui.Text(c, "平台采集").FontSize(17).FontWeight(650).LetterSpacing(-0.2)
			hint(c, "经 APP 中转下发给 Chrome 扩展，在真实浏览器里带登录态执行。请先确认扩展已连接。")
			ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
				ui.Column(c).Gap(5).Width(230).Shrink(0).Children(func() {
					ui.Text(c, "采集动作").FontSize(11).TextColor(textSoft(c))
					labels := make([]string, 0, len(adapters))
					idx := 0
					for i, ad := range adapters {
						labels = append(labels, ad.Label)
						if ad.ID == a.adapter {
							idx = i
						}
					}
					sel := labels[idx]
					if ui.Select(c, &sel, labels).Changed() {
						for _, ad := range adapters {
							if ad.Label == sel {
								a.adapter = ad.ID
								break
							}
						}
					}
				})
				ui.Column(c).Gap(5).Grow(1).Children(func() {
					ui.Text(c, "参数").FontSize(11).TextColor(textSoft(c))
					ui.TextInput(c, &a.adapterValue).Placeholder(info.Hint)
				})
				btnLabel := "执行采集"
				if a.adapterBusy {
					btnLabel = "采集中…"
				}
				b := primaryButton(c, btnLabel, a.adapterBusy)
				if b.Clicked() && !a.adapterBusy {
					a.runCapture()
				}
			})
			if a.adapterResult != "" {
				ui.Divider(c)
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "采集结果").FontSize(12).FontWeight(600).Grow(1)
					quietButton(c, "复制").FontSize(11).OnClick(func() {
						a.copyText(a.adapterResult)
					})
					quietButton(c, "清空").FontSize(11).OnClick(func() { a.adapterResult = "" })
				})
				ui.Scroll(c).Height(260).Children(func() {
					ui.Text(c, a.adapterResult).Font("monospace").FontSize(11).Selectable().MaxWidth(900)
				})
			}
		})
	})
}

func (a *App) runCapture() {
	value := strings.TrimSpace(a.adapterValue)
	a.adapterBusy = true
	a.adapterResult = ""
	adapter := a.adapter
	go func() {
		var raw json.RawMessage
		var err error
		if adapter == "extract" {
			raw, err = a.client.extract(value, adapter)
		} else {
			raw, err = a.client.platformCapture(adapter, value)
		}
		a.win.Update(func() {
			a.adapterBusy = false
			if err != nil {
				a.adapterResult = "采集失败: " + err.Error()
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}
			a.adapterResult = prettyJSON(raw)
			a.toasts = append(a.toasts, toastMsg{Text: "采集完成"})
			a.reloadView("news")
		})
	}()
}

func prettyJSON(raw []byte) string {
	if len(raw) == 0 {
		return "（空响应）"
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	s := out.String()
	if len(s) > 60000 {
		s = s[:60000] + "\n…（结果已截断）"
	}
	return s
}

func (a *App) copyText(s string) {
	a.win.Update(func() {
		// Clipboard access needs the view's context; queue a one-shot field read
		// by the view is overkill, so use the app-level clipboard module.
		clipboardWrite(s)
		a.toasts = append(a.toasts, toastMsg{Text: "已复制"})
	})
}
