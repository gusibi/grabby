package nativeui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// dailyView is a master-detail page: the rail lists every report generated,
// and the one chosen is read on the right, as an item's reader pane. A narrow
// window drops the rail and stacks the report above the history.
func (a *App) dailyView(c *ui.Context) ui.Element {
	avail, rail := a.dailyAvail(c)
	return ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		if rail {
			a.dailyRail(c)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			// The toolbar spans the pane as a toolbar does; the report below it
			// keeps a reading width and centers in what is left.
			ui.Column(c).Padding(20, 26, 12, 26).Shrink(0).Gap(12).Children(func() {
				a.errorBanner(c)
				a.dailyToolbar(c)
			})
			ui.Scroll(c).Grow(1).Children(func() {
				centeredIn(c, avail, 940, func() {
					ui.Column(c).Grow(1).Padding(0, 26, 32, 26).Gap(16).Children(func() {
						a.dailyBody(c)
						if !rail {
							a.dailyHistory(c)
						}
					})
				})
			})
		})
	})
}

const dailyRailWidth = 273

// dailyAvail is the width the report pane gets, and whether the history rail
// is shown beside it.
func (a *App) dailyAvail(c *ui.Context) (float32, bool) {
	avail := a.contentWidth(c)
	if avail >= 900 {
		return avail - dailyRailWidth, true
	}
	return avail, false
}

func (a *App) dailyToolbar(c *ui.Context) {
	avail, _ := a.dailyAvail(c)
	compact := avail < 780
	fields := func() {
		ui.Column(c).Gap(5).Width(150).Shrink(0).Children(func() {
			ui.Text(c, "日期").FontSize(10.5).TextColor(textFaint(c))
			if ui.TextInput(c, &a.dailyDate).Font("monospace").Submitted() {
				a.chooseDaily(a.dailyDate, a.dailyType)
			}
		})
		ui.Column(c).Gap(5).Width(120).Shrink(0).Children(func() {
			ui.Text(c, "类型").FontSize(10.5).TextColor(textFaint(c))
			labels := []string{"日报", "早报", "晚报"}
			ids := []string{"daily", "morning", "evening"}
			cur := labels[0]
			for i, id := range ids {
				if a.dailyType == id {
					cur = labels[i]
				}
			}
			if ui.Select(c, &cur, labels).Changed() {
				for i, label := range labels {
					if label == cur {
						a.chooseDaily(a.dailyDate, ids[i])
					}
				}
			}
		})
	}
	nav := func() {
		quietButton(c, "前一天").FontSize(12).OnClick(func() { a.shiftDaily(-1) })
		quietButton(c, "后一天").FontSize(12).OnClick(func() { a.shiftDaily(1) })
		quietButton(c, "今天").FontSize(12).OnClick(func() {
			a.chooseDaily(time.Now().Format("2006-01-02"), a.dailyType)
		})
	}
	generate := func() {
		if a.dailyBusy {
			ui.Spinner(c)
		}
		primaryButton(c, "生成日报", a.dailyBusy).OnClick(func() { a.generateDaily() })
	}
	week := func() {
		quietButton(c, "补生成近 7 天").FontSize(12).OnClick(func() { a.generateWeek() })
	}
	if compact {
		// The pane cannot hold fields, navigation and actions in one line, so
		// the day navigation moves under the date fields.
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
				fields()
				ui.Spacer(c).Grow(1)
				generate()
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				nav()
				ui.Spacer(c).Grow(1)
				week()
			})
		})
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		fields()
		nav()
		ui.Spacer(c).Grow(1)
		generate()
		week()
	})
}

func (a *App) shiftDaily(delta int) {
	t, err := time.Parse("2006-01-02", a.dailyDate)
	if err != nil {
		t = time.Now()
	}
	a.chooseDaily(t.AddDate(0, 0, delta).Format("2006-01-02"), a.dailyType)
}

// chooseDaily is the user's own pick of a day and type: from then on the page
// shows exactly what was asked for, generated or not.
func (a *App) chooseDaily(date, typ string) {
	a.dailyDate, a.dailyType = date, typ
	a.dailyPicked = true
	a.reloadView("daily")
}

// dailyRail lists the generated reports, newest first, as the reader's item
// list does: pick a day and it opens on the right.
func (a *App) dailyRail(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(dailyRailWidth).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).
		Background(sidebarBg(c)).Children(func() {
		ui.Row(c).Padding(14, 16, 10, 16).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "历史日报").FontSize(12.5).FontWeight(650).Grow(1)
			ui.Text(c, fmt.Sprint(len(a.dailyList))).FontSize(10.5).TextColor(textFaint(c))
		})
		if len(a.dailyList) == 0 {
			ui.Column(c).Padding(2, 16).Children(func() {
				hint(c, "还没有生成过日报。")
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(4, 8, 12, 8).Gap(1).Children(func() {
				for _, r := range a.dailyList {
					report := r
					selected := report.ReportDate == a.dailyDate && report.ReportType == a.dailyType
					row := ui.ButtonBase(c).Padding(7, 9).Radius(8).Shrink(0).Cursor(ui.CursorPointer).
						Label(report.ReportDate + " " + reportTypeLabel(report.ReportType)).
						Transition(ui.ElementTransition{Duration: 150 * time.Millisecond})
					if selected || row.Hovered() {
						row.Background(navActive(c))
					}
					row.OnClick(func() {
						a.chooseDaily(report.ReportDate, report.ReportType)
					})
					row.Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, report.ReportDate).FontSize(10.5).Font("monospace").FontWeight(600).TextColor(textSoft(c))
								ui.Text(c, reportTypeLabel(report.ReportType)).FontSize(10.5).TextColor(textFaint(c))
								ui.Spacer(c).Grow(1)
								ui.Text(c, fmt.Sprintf("优质 %d/%d", report.QualityItems, report.TotalItems)).FontSize(10).TextColor(textFaint(c))
							})
							titleColor := textSoft(c)
							if selected {
								titleColor = t.Text
							}
							ui.Text(c, orDefault(report.Title, "Grabby "+reportTypeLabel(report.ReportType))).
								FontSize(12).TextColor(titleColor).LineHeight(1.4).MaxLines(2)
						})
					})
				}
			})
		})
	})
}

func (a *App) dailyBody(c *ui.Context) {
	d := a.daily
	if d == nil {
		// Nothing has come back yet: saying "none" here would contradict the
		// reports the rail beside it lists.
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Spinner(c)
			hint(c, "正在载入日报…")
		})
		return
	}
	if !d.Success || (d.Content == "" && d.HTMLContent == "") {
		a.dailyEmpty(c)
		return
	}
	title := d.Title
	if title == "" {
		title = "Grabby " + reportTypeLabel(d.ReportType)
	}
	cardSurface(c, 16).Children(func() {
		ui.Column(c).Padding(24, 28).Gap(16).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				accentChip(c, reportTypeLabel(d.ReportType))
				ui.Text(c, d.ReportDate).FontSize(11).TextColor(textSoft(c)).Font("monospace")
				ui.Text(c, fmt.Sprintf("· 收录 %d · 优质 %d", d.TotalItems, d.QualityItems)).FontSize(11).TextColor(textSoft(c))
				ui.Spacer(c).Grow(1)
				if d.ModelUsed != "" {
					softChip(c, d.ModelUsed)
				}
			})
			ui.Text(c, title).FontSize(24).FontWeight(700).LineHeight(1.34).LetterSpacing(-0.4)

			switch d.Format {
			case "json":
				// Stored as JSON, shown as a document: sections of the day's
				// items with their summaries, AI comments and scores.
				if !dailyReportDocument(c, d.Content) {
					hint(c, "这份日报的内容无法解析，重新生成一次即可。")
				}
			default:
				markdown.View(c, d.Content, nil)
			}
		})
	})
}

// dailyEmpty names the gap and offers the newest report, since a day without
// one still leaves months of generated reports in the rail.
func (a *App) dailyEmpty(c *ui.Context) {
	var newest *ReportListItem
	if len(a.dailyList) > 0 {
		newest = &a.dailyList[0]
	}
	ui.Column(c).Fill().Center().Gap(10).Padding(48).Children(func() {
		ui.Box(c).Size(52, 52).Radius(26).Background(bgSubtle(c)).Center().Children(func() {
			ui.Icon(c, iconInbox).Size(24, 24).TextColor(textSoft(c))
		})
		ui.Text(c, a.dailyDate+" 暂无"+reportTypeLabel(a.dailyType)).FontSize(15).FontWeight(600)
		desc := "点击「生成日报」在后台生成，完成后这一页会自动刷新。"
		if newest != nil {
			desc = "最近一期是 " + newest.ReportDate + " " + reportTypeLabel(newest.ReportType) + "，可以先读它，或为这一天生成日报。"
		}
		ui.Text(c, desc).FontSize(12.5).TextColor(textSoft(c)).TextAlign(ui.Center).MaxWidth(420).LineHeight(1.5)
		if newest != nil {
			report := *newest
			if quietButton(c, "查看最近一期").Clicked() {
				a.chooseDaily(report.ReportDate, report.ReportType)
			}
		}
	})
}

func (a *App) dailyHistory(c *ui.Context) {
	if len(a.dailyList) == 0 {
		return
	}
	ui.Text(c, "历史日报").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
	flatPanel(c, func() {
		for i, r := range a.dailyList {
			if i > 0 {
				ui.Divider(c)
			}
			report := r
			row := ui.Row(c).Padding(11, 16).Gap(12).AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
				ui.Text(c, report.ReportDate).FontSize(11).Font("monospace").TextColor(textSoft(c)).Width(84)
				softChip(c, reportTypeLabel(report.ReportType))
				ui.Text(c, orDefault(report.Title, "Grabby "+reportTypeLabel(report.ReportType))).FontSize(12.5).Grow(1).MaxLines(1)
				ui.Text(c, fmt.Sprintf("%d / %d", report.QualityItems, report.TotalItems)).FontSize(10.5).TextColor(textFaint(c)).Width(64)
				ui.Text(c, timeAgo(report.GeneratedAt)).FontSize(10.5).TextColor(textFaint(c)).Width(70)
			})
			if row.Hovered() {
				row.Background(bgSubtle(c))
			}
			row.OnClick(func() {
				a.chooseDaily(report.ReportDate, report.ReportType)
			})
		}
	})
}

func (a *App) generateDaily() {
	if a.dailyBusy {
		return
	}
	date, typ := a.dailyDate, a.dailyType
	a.dailyBusy = true
	go func() {
		err := a.client.generateDaily(date, typ)
		if err != nil {
			a.win.Update(func() {
				a.dailyBusy = false
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
			})
			return
		}
		time.Sleep(1800 * time.Millisecond)
		a.win.Update(func() {
			a.dailyBusy = false
			a.toasts = append(a.toasts, toastMsg{Text: "日报已生成"})
			a.reloadView("daily")
			a.reloadView("today")
		})
	}()
}

func (a *App) generateWeek() {
	start := time.Now()
	typ := a.dailyType
	a.dailyBusy = true
	go func() {
		var firstErr error
		for i := 0; i < 7; i++ {
			date := start.AddDate(0, 0, -i).Format("2006-01-02")
			if err := a.client.generateDaily(date, typ); err != nil && firstErr == nil {
				firstErr = err
			}
			time.Sleep(300 * time.Millisecond)
		}
		a.win.Update(func() {
			a.dailyBusy = false
			if firstErr != nil {
				a.toasts = append(a.toasts, toastMsg{Text: firstErr.Error(), Error: true})
				return
			}
			a.toasts = append(a.toasts, toastMsg{Text: "已提交近 7 天的日报生成"})
			a.reloadView("daily")
		})
	}()
}

// dailyDoc mirrors the stored JSON of a report: a payload of sections, each
// holding the day's items, so the view can lay them out as a page.
type dailyDoc struct {
	Title    string                     `json:"title"`
	Date     string                     `json:"date"`
	Editor   string                     `json:"editor"`
	Sections map[string]dailyDocSection `json:"sections"`
}

type dailyDocSection struct {
	Title string         `json:"title"`
	Items []dailyDocItem `json:"items"`
}

type dailyDocItem struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Source  string `json:"source"`
	Link    string `json:"link"`
	Score   string `json:"score"`
	Comment string `json:"comment"`
}

// dailyReportDocument renders a JSON report as readable sections and returns
// whether it could: the content is stored as JSON but a reader never sees it.
func dailyReportDocument(c *ui.Context, raw string) bool {
	var doc dailyDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || len(doc.Sections) == 0 {
		return false
	}
	keys := make([]string, 0, len(doc.Sections))
	for k := range doc.Sections {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ui.Column(c).Gap(16).Children(func() {
		if editor := strings.TrimSpace(doc.Editor); editor != "" {
			ui.Text(c, "编辑 · "+editor).FontSize(11.5).TextColor(textSoft(c))
		}
		first := true
		for _, k := range keys {
			sec := doc.Sections[k]
			if len(sec.Items) == 0 {
				continue
			}
			if !first {
				ui.Divider(c)
			}
			first = false
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, orDefault(sec.Title, "重点")).FontSize(16).FontWeight(650).LetterSpacing(-0.2)
				for j, it := range sec.Items {
					if j > 0 {
						ui.Divider(c)
					}
					dailyReportItem(c, it)
				}
			})
		}
	})
	return true
}

// dailyLink matches a markdown link. Report summaries from the generator
// sometimes carry "[阅读原文](url)" mid-sentence, and a reader should not see
// the syntax: the item's own link is a button below the text.
var dailyLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

func cleanDailyText(s string) string {
	s = dailyLink.ReplaceAllStringFunc(s, func(m string) string {
		label := dailyLink.FindStringSubmatch(m)[1]
		if strings.TrimSpace(label) == "阅读原文" {
			return ""
		}
		return label
	})
	return strings.Join(strings.Fields(s), " ")
}

func dailyReportItem(c *ui.Context, it dailyDocItem) {
	t := c.Theme()
	ui.Column(c).Gap(5).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Text(c, it.Title).FontSize(14).FontWeight(600).LineHeight(1.45).Grow(1).MaxLines(3)
			if s, err := strconv.ParseFloat(strings.TrimSpace(it.Score), 64); err == nil && s > 0 {
				scoreChip(c, s)
			}
		})
		if sum := cleanDailyText(strings.TrimSpace(it.Summary)); sum != "" {
			ui.Text(c, sum).FontSize(12.5).TextColor(textSoft(c)).LineHeight(1.65).MaxWidth(720)
		}
		if cm := strings.TrimSpace(it.Comment); cm != "" {
			ui.Row(c).Background(bgSubtle(c)).Radius(8).Padding(7, 10).MaxWidth(720).Children(func() {
				ui.Text(c, cm).FontSize(11.5).TextColor(t.TextMuted).LineHeight(1.55)
			})
		}
		if it.Source != "" || it.Link != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if it.Source != "" {
					ui.Text(c, it.Source).FontSize(10.5).TextColor(textFaint(c))
				}
				if it.Link != "" {
					link := it.Link
					if quietButton(c, "阅读原文").FontSize(11).TextColor(t.Accent).Clicked() {
						_ = openExternal(link)
					}
				}
			})
		}
	})
}
