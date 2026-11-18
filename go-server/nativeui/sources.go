package nativeui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) sourcesView(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 30).Gap(16).Children(func() {
			a.errorBanner(c)
			enabled, disabled := 0, 0
			groups := map[string]struct{}{}
			for _, s := range a.sources {
				if s.Enabled == 1 {
					enabled++
				} else {
					disabled++
				}
				if s.Category != "" {
					groups[s.Category] = struct{}{}
				}
			}
			statsBar(c,
				statItem{Value: fmt.Sprint(len(a.sources)), Label: "全部订阅源"},
				statItem{Value: fmt.Sprint(enabled), Label: "已启用"},
				statItem{Value: fmt.Sprint(disabled), Label: "已停用"},
				statItem{Value: fmt.Sprint(len(groups)), Label: "分组"},
			)

			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "订阅源").FontSize(17).FontWeight(650).LetterSpacing(-0.3).Grow(1)
				if quietButton(c, "全部抓取").Clicked() {
					a.runAllSources()
				}
			})

			if len(a.sources) == 0 {
				emptyState(c, "还没有订阅源", "点击「添加数据源」新建一个 RSS / API / 网页抓取源。")
				return
			}

			flatPanel(c, func() {
				for i, s := range a.sources {
					if i > 0 {
						ui.Divider(c)
					}
					a.sourceRow(c, s)
				}
			})
		})
	})
}

func (a *App) sourceRow(c *ui.Context, s Source) {
	t := c.Theme()
	meta := joinNonEmpty("定时 "+orDefault(s.Schedule, "—"), "默认类型 "+orDefault(s.DefaultCategory, "自动"))
	if s.LastFetchAt != nil {
		status := ""
		if s.LastFetchStatus != nil {
			status = *s.LastFetchStatus
		}
		meta = joinNonEmpty(meta, "上次抓取 "+timeAgo(*s.LastFetchAt), status)
	}
	row := ui.Column(c).Padding(12, 16).Gap(4)
	hovered := row.Hovered()
	// The row's actions show on hover, so a list of sources reads as names and
	// schedules rather than a grid of repeated buttons. They keep their place
	// while hidden, so nothing moves as the pointer arrives.
	action := func(label string, danger bool, fn func()) {
		b := quietButton(c, label).FontSize(11)
		if danger {
			b.TextColor(t.Danger)
		}
		if !hovered {
			b.Opacity(0.55)
		}
		if b.Clicked() {
			fn()
		}
	}
	row.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.Name).FontSize(13.5).FontWeight(600).Grow(1).MaxLines(1)
			if s.Category != "" {
				softChip(c, s.Category)
			}
			softChip(c, sourceTypeLabel(s.Type))
			on := s.Enabled == 1
			if ui.Switch(c, &on).Changed() {
				enabled := on
				a.runAsync("", func() error { return a.client.toggleSource(s.ID, enabled) })
			}
		})
		ui.Text(c, s.URL).FontSize(10.5).TextColor(textFaint(c)).MaxLines(1).Font("monospace")
		ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(0, 0, 2, 0).Children(func() {
			ui.Text(c, meta).FontSize(10.5).TextColor(textFaint(c)).Grow(1).MaxLines(1)
			action("立即抓取", false, func() {
				a.runAsync("已开始抓取: "+s.Name, func() error { return a.client.runSource(s.ID) })
			})
			action("日志", false, func() { a.openSourceLogs(s) })
			action("编辑", false, func() { a.openSourceDialog(&s) })
			action("删除", true, func() {
				a.ask("删除订阅源", "「"+s.Name+"」将被删除，已抓取的内容会保留。", "删除", func() error {
					if err := a.client.deleteSource(s.ID); err != nil {
						return err
					}
					a.win.Update(func() { a.reloadView("sources") })
					return nil
				})
			})
		})
	})
}

func sourceTypeLabel(t string) string {
	switch t {
	case "podcast":
		return "播客"
	case "youtube":
		return "视频"
	case "web_scrape":
		return "网页"
	case "api":
		return "API"
	}
	return strings.ToUpper(t)
}

func (a *App) runAllSources() {
	var enabled []Source
	for _, s := range a.sources {
		if s.Enabled == 1 {
			enabled = append(enabled, s)
		}
	}
	if len(enabled) == 0 {
		a.notifyErr("没有已启用的订阅源")
		return
	}
	go func() {
		var firstErr error
		for _, s := range enabled {
			if err := a.client.runSource(s.ID); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		a.win.Update(func() {
			if firstErr != nil {
				a.toasts = append(a.toasts, toastMsg{Text: firstErr.Error(), Error: true})
				return
			}
			a.toasts = append(a.toasts, toastMsg{Text: fmt.Sprintf("已启动 %d 个订阅源的抓取", len(enabled))})
		})
	}()
}

func (a *App) openSourceDialog(existing *Source) {
	var form SourceForm
	if existing != nil {
		form = SourceForm{
			ID: existing.ID, Name: existing.Name, Type: existing.Type, URL: existing.URL,
			Schedule: existing.Schedule, DefaultCategory: existing.DefaultCategory,
			Config: existing.Config, Category: existing.Category,
		}
	} else {
		form = SourceForm{Type: "rss", Schedule: "0 * * * *", DefaultCategory: "auto", Config: "{}"}
	}
	a.sourceEditing = &form
	a.sourceIsEdit = existing != nil
	a.sourceDialogOpen = true
}

func (a *App) sourceDialog(c *ui.Context) {
	if a.sourceEditing == nil {
		return
	}
	f := a.sourceEditing
	editing := a.sourceIsEdit
	wasOpen := a.sourceDialogOpen
	ui.Modal(c, &a.sourceDialogOpen, func() {
		ui.Column(c).MinWidth(560).MaxWidth(640).Padding(6).Gap(16).Children(func() {
			ui.Text(c, map[bool]string{true: "编辑订阅源", false: "添加数据源"}[editing]).FontSize(20).FontWeight(700).LetterSpacing(-0.3)
			hint(c, "RSS / API / 网页抓取源会由 cron 定时执行，抓取结果进入内容库并触发 AI 分析。")

			ui.Form(c, func() {
				ui.Field(c, "名称", func() {
					ui.TextInput(c, &f.Name).AutoFocus().Placeholder("例如：少数派").MaxWidth(420)
				})
				typeIdx := 0
				switch f.Type {
				case "api":
					typeIdx = 1
				case "web_scrape":
					typeIdx = 2
				}
				typeLabels := []string{"RSS", "API", "网页抓取"}
				typeLabel := typeLabels[typeIdx]
				ui.Field(c, "类型", func() {
					if ui.Select(c, &typeLabel, typeLabels).MaxWidth(200).Changed() {
						f.Type = []string{"rss", "api", "web_scrape"}[typeIdxFor(typeLabel, typeLabels)]
					}
				})
				ui.Field(c, "URL", func() {
					ui.TextInput(c, &f.URL).Placeholder("https://example.com/feed.xml").MaxWidth(420)
				})
				ui.Field(c, "定时", func() {
					ui.TextInput(c, &f.Schedule).Placeholder("0 * * * *").Font("monospace").MaxWidth(200)
				})
				ui.Field(c, "主题分组", func() {
					ui.TextInput(c, &f.Category).Placeholder("AI / 科技新闻 / 设计").MaxWidth(420)
				})
				ui.Field(c, "内容类型", func() {
					ui.TextInput(c, &f.DefaultCategory).Placeholder("auto / article / podcast / youtube").MaxWidth(420)
				})
				ui.Field(c, "ID", func() {
					if editing {
						ui.TextInput(c, &f.ID).ReadOnly(true).MaxWidth(420)
					} else {
						ui.TextInput(c, &f.ID).Placeholder("留空则按名称生成").MaxWidth(420)
					}
				})
			})
			if f.Type == "" {
				f.Type = "rss"
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if quietButton(c, "取消").Clicked() {
					a.sourceDialogOpen = false
				}
				if primaryButton(c, "保存", false).Clicked() {
					if strings.TrimSpace(f.Name) == "" || strings.TrimSpace(f.URL) == "" {
						a.notifyErr("名称和 URL 不能为空")
						return
					}
					if f.Schedule == "" {
						f.Schedule = "0 * * * *"
					}
					if f.ID == "" {
						f.ID = slugify(f.Name)
					}
					if f.Config == "" {
						f.Config = "{}"
					}
					form := *f
					wasEditing := editing
					a.sourceDialogOpen = false
					a.sourceEditing = nil
					a.runAsync("订阅源已保存", func() error {
						if err := a.client.saveSource(form, wasEditing); err != nil {
							return err
						}
						a.win.Update(func() {
							a.reloadView("sources")
							a.reloadView("news")
						})
						return nil
					})
				}
			})
		})
	})
	if wasOpen && !a.sourceDialogOpen {
		a.sourceEditing = nil
	}
}

func typeIdxFor(label string, labels []string) int {
	for i, l := range labels {
		if l == label {
			return i
		}
	}
	return 0
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = fmt.Sprintf("source-%d", time.Now().Unix()%1000000)
	}
	return id
}

func (a *App) openSourceLogs(s Source) {
	a.sourceLogTitle = s.Name
	a.sourceLogs = nil
	a.sourceLogOpen = true
	go func() {
		logs, err := a.client.logs(s.ID)
		a.win.Update(func() {
			if err != nil {
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}
			a.sourceLogs = logs
		})
	}()
}

func (a *App) sourceLogsDialog(c *ui.Context) {
	if !a.sourceLogOpen {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.sourceLogOpen, func() {
		ui.Column(c).Width(620).MaxHeight(520).Gap(10).Children(func() {
			ui.Text(c, "抓取日志 · "+a.sourceLogTitle).FontSize(18).FontWeight(700).LetterSpacing(-0.2)
			if len(a.sourceLogs) == 0 {
				hint(c, "暂无日志")
			} else {
				ui.Scroll(c).Height(360).Children(func() {
					ui.Column(c).Gap(1).Children(func() {
						for _, l := range a.sourceLogs {
							ui.Row(c).Padding(7, 8).Gap(10).AlignItems(ui.Center).Children(func() {
								color := t.TextMuted
								switch l.Status {
								case "success":
									color = t.Success
								case "error":
									color = t.Danger
								case "partial":
									color = t.Warning
								}
								ui.Text(c, l.Status).FontSize(11).FontWeight(650).TextColor(color).Width(58)
								ui.Text(c, l.StartedAt.Local().Format("01-02 15:04")).FontSize(11).TextColor(textSoft(c)).Width(84)
								ui.Text(c, fmt.Sprintf("+%d / %d", l.ItemsAdded, l.ItemsFound)).FontSize(11).TextColor(textSoft(c)).Width(70)
								ui.Text(c, truncate(l.ErrorMessage, 60)).FontSize(11).TextColor(textSoft(c)).Grow(1).MaxLines(1)
							})
							ui.Divider(c)
						}
					})
				})
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if quietButton(c, "关闭").Clicked() {
					a.sourceLogOpen = false
				}
			})
		})
	})
}
