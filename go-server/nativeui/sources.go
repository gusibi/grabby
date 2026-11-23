package nativeui

import (
	"encoding/json"
	"fmt"
	"strconv"
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
				if a.sourceEnabled(s) {
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
	if total, unread := a.sourceContentCount(s.ID); total > 0 {
		if unread > 0 {
			meta = joinNonEmpty(meta, fmt.Sprintf("%d 条内容 · %d 未读", total, unread))
		} else {
			meta = joinNonEmpty(meta, fmt.Sprintf("%d 条内容", total))
		}
	}
	if s.LastFetchAt != nil {
		status := ""
		if s.LastFetchStatus != nil {
			status = logStatusLabel(*s.LastFetchStatus)
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
			on := a.sourceEnabled(s)
			_, saving := a.sourceToggles[s.ID]
			if ui.Switch(c, &on).Label("启用订阅源 "+s.Name).Disabled(saving).DimDisabled(false).Changed() && !saving {
				a.toggleSource(s.ID, on)
			}
		})
		ui.Text(c, s.URL).FontSize(10.5).TextColor(textFaint(c)).MaxLines(1).Font("monospace")
		ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(0, 0, 2, 0).Children(func() {
			ui.Text(c, meta).FontSize(10.5).TextColor(textFaint(c)).Grow(1).MaxLines(1)
			action("查看内容", false, func() {
				f := newFilter()
				f.SourceID = s.ID
				a.filters["library"] = f
				a.searchDraft = ""
				a.navigate("library")
				a.reloadView("library")
			})
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

func (a *App) sourceEnabled(s Source) bool {
	if enabled, saving := a.sourceToggles[s.ID]; saving {
		return enabled
	}
	return s.Enabled == 1
}

func (a *App) toggleSource(id string, enabled bool) {
	if a.sourceToggles == nil {
		a.sourceToggles = make(map[string]bool)
	}
	a.sourceToggles[id] = enabled
	go func() {
		err := a.client.toggleSource(id, enabled)
		a.win.Update(func() {
			delete(a.sourceToggles, id)
			if err != nil {
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}

			for i := range a.sources {
				if a.sources[i].ID == id {
					a.sources[i].Enabled = 0
					if enabled {
						a.sources[i].Enabled = 1
					}
					break
				}
			}
			// Invalidate a list fetch that could still return the pre-toggle value.
			if a.loading == "sources" || a.loading == "news" {
				a.seq++
				a.loading = ""
				a.ensureData()
			}
		})
	}()
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
	a.loadSourceConfigDraft(form.Config)
	a.sourceEditing = &form
	a.sourceIsEdit = existing != nil
	a.sourceDialogOpen = true
}

func (a *App) loadSourceConfigDraft(raw string) {
	cfg := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	full := false
	if b, ok := cfg["full_content"].(bool); ok && b {
		full = true
	}
	if b, ok := cfg["fetch_full_via_scrape"].(bool); ok && b {
		full = true
	}
	a.sourceFullContent = full
	a.sourceListSelector = configString(cfg, "list_selector")
	a.sourceArticleSelector = configString(cfg, "article_link_selector")
	a.sourceArticlePattern = configString(cfg, "article_url_pattern")
	// Older sources expressed list mode only through selectors. Treat those as
	// "crawl links" so the simple switch reflects what the source actually does.
	crawl, _ := cfg["crawl_links"].(bool)
	if a.sourceListSelector != "" || a.sourceArticleSelector != "" || a.sourceArticlePattern != "" {
		crawl = true
	}
	a.sourceCrawlLinks = crawl
	a.sourceAdvancedOpen = false
	a.sourceMaxItems = ""
	if n, ok := cfg["max_items"].(float64); ok && n > 0 {
		a.sourceMaxItems = fmt.Sprint(int(n))
	}
	a.sourceMaxBackfill = ""
	if n, ok := cfg["max_backfill"].(float64); ok && n > 0 {
		a.sourceMaxBackfill = fmt.Sprint(int(n))
	}
	// Preserve keys the form does not expose (headers, transcribe_audio, ...).
	a.sourceConfigExtra = map[string]any{}
	for k, v := range cfg {
		switch k {
		case "full_content", "fetch_full_via_scrape", "crawl_links", "list_selector",
			"article_link_selector", "article_url_pattern", "max_items", "max_backfill":
			continue
		}
		a.sourceConfigExtra[k] = v
	}
}

func configString(cfg map[string]any, key string) string {
	s, _ := cfg[key].(string)
	return strings.TrimSpace(s)
}

func (a *App) buildSourceConfig() string {
	cfg := map[string]any{}
	for k, v := range a.sourceConfigExtra {
		cfg[k] = v
	}
	cfg["full_content"] = a.sourceFullContent
	cfg["fetch_full_via_scrape"] = a.sourceFullContent
	if a.sourceCrawlLinks {
		cfg["crawl_links"] = true
	}
	if v := strings.TrimSpace(a.sourceListSelector); v != "" {
		cfg["list_selector"] = v
	}
	if v := strings.TrimSpace(a.sourceArticleSelector); v != "" {
		cfg["article_link_selector"] = v
	}
	if v := strings.TrimSpace(a.sourceArticlePattern); v != "" {
		cfg["article_url_pattern"] = v
	}
	if v := strings.TrimSpace(a.sourceMaxItems); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg["max_items"] = n
		}
	}
	if v := strings.TrimSpace(a.sourceMaxBackfill); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg["max_backfill"] = n
		}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
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
			hint(c, "填好名称和 URL 即可保存；抓取结果会进入内容库并触发 AI 分析。")

			ui.Form(c, func() {
				ui.Field(c, "名称", func() {
					ui.TextInput(c, &f.Name).AutoFocus().Placeholder("例如：少数派").MaxWidth(420)
				})
				typeIdx := 0
				typeValues := []string{"rss", "podcast", "youtube", "api", "web_scrape"}
				for i, v := range typeValues {
					if f.Type == v {
						typeIdx = i
						break
					}
				}
				typeLabels := []string{"RSS", "播客", "视频", "API", "网页 / 文章"}
				typeLabel := typeLabels[typeIdx]
				ui.Field(c, "类型", func() {
					if ui.Select(c, &typeLabel, typeLabels).MaxWidth(200).Changed() {
						f.Type = typeValues[typeIdxFor(typeLabel, typeLabels)]
					}
				})
				ui.Field(c, "URL", func() {
					placeholder := "https://example.com/feed.xml"
					switch f.Type {
					case "youtube":
						placeholder = "https://www.youtube.com/@handle/videos 或频道 ID"
					case "podcast":
						placeholder = "https://podcasts.example.com/feed.rss"
					case "web_scrape":
						placeholder = "https://example.com/news 或单篇文章 URL"
					}
					ui.TextInput(c, &f.URL).Placeholder(placeholder).MaxWidth(420)
				})
				ui.Field(c, "主题分组", func() {
					ui.TextInput(c, &f.Category).Placeholder("AI / 科技新闻 / 设计").MaxWidth(420)
				})
				if f.Type == "rss" {
					ui.Field(c, "抓取全文", func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Switch(c, &a.sourceFullContent)
							hint(c, "RSS 只有简介时打开原文并保存正文；已有正文不会重复抓取。")
						})
					})
				}
				if f.Type == "web_scrape" {
					ui.Field(c, "抓取页面中的链接", func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Switch(c, &a.sourceCrawlLinks)
							hint(c, "打开后会把本页正文里的链接分别保存成文章；导航、分类和图片链接会自动跳过。")
						})
					})
				}
				ui.Collapsible(c, "高级选项（一般不需要）", &a.sourceAdvancedOpen, func() {
					ui.Field(c, "定时", func() {
						ui.TextInput(c, &f.Schedule).Placeholder("0 * * * *").Font("monospace").MaxWidth(200)
					})
					ui.Field(c, "内容类型", func() {
						ui.TextInput(c, &f.DefaultCategory).Placeholder("auto / article / blog / podcast / youtube").MaxWidth(420)
					})
					if f.Type == "rss" {
						ui.Field(c, "补抓上限", func() {
							ui.TextInput(c, &a.sourceMaxBackfill).Placeholder("默认 15").MaxWidth(120)
						})
					}
					if f.Type == "web_scrape" {
						ui.Field(c, "列表区域", func() {
							ui.TextInput(c, &a.sourceListSelector).Placeholder("可选 CSS 选择器，例如 main 或 .post-list").MaxWidth(420)
						})
						ui.Field(c, "文章链接", func() {
							ui.TextInput(c, &a.sourceArticleSelector).Placeholder("可选 CSS 选择器，例如 a.post-link").MaxWidth(420)
						})
						ui.Field(c, "链接规则", func() {
							ui.TextInput(c, &a.sourceArticlePattern).Placeholder(`可选正则，例如 /news/[^/]+$`).MaxWidth(420)
						})
						ui.Field(c, "每轮上限", func() {
							ui.TextInput(c, &a.sourceMaxItems).Placeholder("默认 20").MaxWidth(120)
						})
					}
					if f.Type == "youtube" || f.Type == "podcast" {
						ui.Field(c, "每轮上限", func() {
							ui.TextInput(c, &a.sourceMaxItems).Placeholder("默认 10").MaxWidth(120)
						})
					}
					ui.Field(c, "ID", func() {
						if editing {
							ui.TextInput(c, &f.ID).ReadOnly(true).MaxWidth(420)
						} else {
							ui.TextInput(c, &f.ID).Placeholder("留空则按名称生成").MaxWidth(420)
						}
					})
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
					f.Config = a.buildSourceConfig()
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
	a.sourceLogDetail = nil
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

func (a *App) openLogDetail(id int64) {
	a.sourceLogLoading = id
	a.sourceLogDetail = nil
	go func() {
		detail, err := a.client.logDetail(id)
		a.win.Update(func() {
			a.sourceLogLoading = 0
			if err != nil {
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}
			a.sourceLogDetail = &detail
		})
	}()
}

func logStatusLabel(status string) string {
	switch status {
	case "success":
		return "成功"
	case "partial":
		return "部分成功"
	case "error":
		return "失败"
	case "running":
		return "处理中"
	case "not_modified":
		return "Feed 未变更"
	case "no_new_items":
		return "无新条目"
	case "skipped":
		return "已跳过"
	case "stored":
		return "已入库"
	case "updated":
		return "已补全"
	case "kept_existing":
		return "保留已有"
	case "pending":
		return "处理中"
	case "no_public_text":
		return "无公开文本"
	case "not_attempted":
		return "未尝试"
	case "unverified":
		return "未确认"
	case "failed":
		return "失败"
	}
	return status
}

func logStatusColor(status string, t *ui.Theme) ui.Color {
	switch status {
	case "success", "stored", "updated":
		return t.Success
	case "partial", "not_modified", "no_new_items", "kept_existing", "skipped",
		"no_public_text", "unverified", "not_attempted":
		return t.Warning
	case "error", "failed":
		return t.Danger
	}
	return t.TextMuted
}

func triggerLabel(trigger string) string {
	switch trigger {
	case "scheduled":
		return "定时"
	case "manual":
		return "手动"
	}
	return orDefault(trigger, "手动")
}

func (a *App) sourceLogsDialog(c *ui.Context) {
	if !a.sourceLogOpen {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.sourceLogOpen, func() {
		if a.sourceLogDetail != nil {
			a.sourceLogDetailView(c, t)
			return
		}
		ui.Column(c).Width(720).MaxHeight(560).Gap(10).Children(func() {
			ui.Text(c, "抓取日志 · "+a.sourceLogTitle).FontSize(18).FontWeight(700).LetterSpacing(-0.2)
			hint(c, "每一轮手动或定时抓取都会记录结果；点「明细」查看逐条文本来源、跳过与失败原因。")
			if len(a.sourceLogs) == 0 {
				hint(c, "暂无日志")
			} else {
				ui.Scroll(c).Height(400).Children(func() {
					ui.Column(c).Gap(1).Children(func() {
						for _, l := range a.sourceLogs {
							ui.Row(c).Padding(7, 8).Gap(10).AlignItems(ui.Center).Children(func() {
								ui.Text(c, logStatusLabel(l.Status)).FontSize(11).FontWeight(650).
									TextColor(logStatusColor(l.Status, t)).Width(72)
								ui.Text(c, l.StartedAt.Local().Format("01-02 15:04")).FontSize(11).TextColor(textSoft(c)).Width(84)
								ui.Text(c, triggerLabel(l.Trigger)).FontSize(11).TextColor(textSoft(c)).Width(32)
								ui.Text(c, fmt.Sprintf("+%d / %d", l.ItemsAdded, l.ItemsFound)).FontSize(11).TextColor(textSoft(c)).Width(70)
								ui.Text(c, truncate(firstNonEmptyStr(l.Summary, l.ErrorMessage), 70)).FontSize(11).
									TextColor(textSoft(c)).Grow(1).MaxLines(1)
								if a.sourceLogLoading == l.ID {
									ui.Text(c, "载入…").FontSize(11).TextColor(textFaint(c))
								} else if quietButton(c, "明细").FontSize(11).Clicked() {
									a.openLogDetail(l.ID)
								}
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

func (a *App) sourceLogDetailView(c *ui.Context, t *ui.Theme) {
	d := a.sourceLogDetail
	l := d.Log
	ui.Column(c).Width(760).MaxHeight(600).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if quietButton(c, "← 返回").Clicked() {
				a.sourceLogDetail = nil
				return
			}
			ui.Text(c, fmt.Sprintf("抓取明细 #%d", l.ID)).FontSize(18).FontWeight(700).LetterSpacing(-0.2)
			ui.Text(c, logStatusLabel(l.Status)).FontSize(12).FontWeight(650).TextColor(logStatusColor(l.Status, t))
		})
		meta := joinNonEmpty(
			triggerLabel(l.Trigger)+"触发",
			"开始 "+l.StartedAt.Local().Format("01-02 15:04:05"),
		)
		if l.FinishedAt != nil {
			meta = joinNonEmpty(meta, "结束 "+l.FinishedAt.Local().Format("15:04:05"))
		}
		meta = joinNonEmpty(meta, fmt.Sprintf("新增 %d · 补全 %d · 跳过 %d · 失败 %d",
			l.ItemsAdded, l.ItemsUpdated, l.ItemsSkipped, l.ItemsFailed))
		ui.Text(c, meta).FontSize(11).TextColor(textFaint(c))
		if l.Summary != "" {
			ui.Text(c, l.Summary).FontSize(11.5).TextColor(textSoft(c))
		}
		if l.ErrorMessage != "" {
			ui.Text(c, l.ErrorMessage).FontSize(11).TextColor(t.Danger).MaxLines(3)
		}
		if len(d.Items) == 0 {
			hint(c, "本轮没有逐条明细（历史日志或未发现条目）。")
		} else {
			ui.Scroll(c).Height(430).Children(func() {
				ui.Column(c).Gap(8).Children(func() {
					for _, it := range d.Items {
						a.sourceLogItemRow(c, t, it)
					}
				})
			})
		}
		ui.Row(c).Justify(ui.End).Children(func() {
			if quietButton(c, "关闭").Clicked() {
				a.sourceLogOpen = false
				a.sourceLogDetail = nil
			}
		})
	})
}

func (a *App) sourceLogItemRow(c *ui.Context, t *ui.Theme, it FetchLogItem) {
	ui.Column(c).Padding(10, 12).Gap(3).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Radius(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, logStatusLabel(it.Status)).FontSize(10.5).FontWeight(650).
				TextColor(logStatusColor(it.Status, t)).Width(70)
			ui.Text(c, firstNonEmptyStr(it.Title, it.URL)).FontSize(12.5).FontWeight(600).Grow(1).MaxLines(2)
			if it.ItemID > 0 {
				softChip(c, fmt.Sprintf("内容 #%d", it.ItemID))
			}
			if it.Category != "" {
				softChip(c, it.Category)
			}
		})
		if it.URL != "" {
			ui.Text(c, it.URL).FontSize(10).TextColor(textFaint(c)).Font("monospace").MaxLines(1)
		}
		if it.Reason != "" {
			reason := it.Reason
			if it.ErrorStage != "" {
				reason = "[" + it.ErrorStage + "] " + reason
			}
			ui.Text(c, reason).FontSize(10.5).TextColor(textSoft(c)).MaxLines(3)
		}
		for _, txt := range it.Texts {
			parts := []string{txt.Kind + ": " + logStatusLabel(txt.Status)}
			if txt.Source != "" {
				parts = append(parts, txt.Source)
			}
			if txt.Chars > 0 {
				parts = append(parts, fmt.Sprintf("%d 字", txt.Chars))
			}
			if txt.Language != "" {
				parts = append(parts, txt.Language)
			}
			line := strings.Join(parts, " · ")
			if txt.Stage != "" {
				line += " · 阶段 " + txt.Stage
			}
			if txt.Reason != "" {
				line += " · " + truncate(txt.Reason, 140)
			}
			ui.Text(c, line).FontSize(10.5).TextColor(logStatusColor(txt.Status, t)).MaxLines(2)
		}
	})
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
