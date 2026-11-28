package nativeui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// today is the day's desk: how much came in today, the day's report if one
// was generated, and every item fetched since midnight. Global figures and
// the browser connection live in the content library and the sidebar, so the
// page does not repeat them.
func (a *App) today(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 30).Gap(18).Children(func() {
			a.errorBanner(c)
			a.todayReport(c)

			items := a.currentItems()
			ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
				ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, "今天的内容").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
					count := a.stats.TodayCount
					if count == 0 && len(items) > 0 {
						count = len(items)
					}
					ui.Text(c, fmt.Sprintf("今日收录 %s 条", formatCount(count))).FontSize(11.5).TextColor(textSoft(c))
				})
				quietButton(c, "打开内容库").OnClick(func() { a.navigate("library") })
			})
			a.searchBar(c, "today")
			a.filter(c, "today")
			a.contentTypeRow(c, "today")

			if len(items) == 0 {
				emptyState(c, "今天还没有符合条件的内容", "换个类型或来源，或清除筛选；今天的全部内容都在内容库。")
				return
			}
			if a.layout == 1 {
				flatPanel(c, func() {
					for i, it := range items {
						if i > 0 {
							ui.Divider(c)
						}
						a.itemRow(c, it)
					}
				})
			} else {
				a.feedGrid(c, items, a.cardColumns(c))
			}
			a.loadMoreRow(c, "today")
		})
	})
}

// formatCount groups thousands, so a day with many items stays readable.
func formatCount(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	head := len(s) % 3
	if head > 0 {
		b.WriteString(s[:head])
	}
	for i := head; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteString(",")
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// latestReportFor returns the newest report whose date is today.
func (a *App) latestReportForToday() *ReportListItem {
	today := time.Now().Format("2006-01-02")
	for i := range a.dailyList {
		if a.dailyList[i].ReportDate == today {
			return &a.dailyList[i]
		}
	}
	return nil
}

func (a *App) todayReport(c *ui.Context) {
	r := a.latestReportForToday()
	if r == nil {
		return
	}
	cardSurface(c, 16).Children(func() {
		ui.Column(c).Padding(20, 22).Gap(10).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				accentChip(c, "今日日报")
				if r.ReportType != "" && r.ReportType != "daily" {
					ui.Text(c, reportTypeLabel(r.ReportType)).FontSize(11.5).TextColor(textSoft(c))
				}
				ui.Spacer(c).Grow(1)
				ui.Text(c, timeAgo(r.GeneratedAt)).FontSize(11.5).TextColor(textSoft(c))
			})
			title := r.Title
			if title == "" {
				title = "Grabby 日报"
			}
			ui.Text(c, title).FontSize(19).FontWeight(650).MaxLines(2).LetterSpacing(-0.3).LineHeight(1.4)
			if r.TotalItems > 0 {
				ui.Text(c, joinNonEmpty(
					fmt.Sprintf("收录 %d 条", r.TotalItems),
					fmt.Sprintf("优质 %d 条", r.QualityItems),
					r.ModelUsed,
				)).FontSize(11).TextColor(textSoft(c))
			}
			ui.Row(c).Gap(8).Children(func() {
				if primaryButton(c, "查看日报", false).Clicked() {
					a.openDaily(r.ReportDate, r.ReportType)
				}
				if quietButton(c, "重新生成").Clicked() {
					a.generateReport(r.ReportDate, r.ReportType)
				}
			})
		})
	})
}

func reportTypeLabel(t string) string {
	switch t {
	case "morning":
		return "早报"
	case "evening":
		return "晚报"
	case "daily":
		return "日报"
	}
	if t == "" {
		return "日报"
	}
	return t
}

func (a *App) openDaily(date, typ string) {
	a.dailyPicked = true
	a.chooseDaily(date, typ)
	a.navigate("daily")
}

func joinNonEmpty(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " · ")
}
