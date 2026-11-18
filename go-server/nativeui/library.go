package nativeui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// libraryStats is the archive at a glance. It is global, not filtered, so it
// sits above the results rather than inside them.
func (a *App) libraryStats(c *ui.Context) {
	connected := "—"
	if a.health.BrowserConnected || len(a.browsers) > 0 {
		connected = fmt.Sprint(len(a.browsers))
	}
	statsBar(c,
		statItem{Value: formatCount(a.stats.TotalCount), Label: "已归档"},
		statItem{Value: formatCount(a.stats.UnreadCount), Label: "未读"},
		statItem{Value: formatCount(a.stats.StarredCount), Label: "已收藏"},
		statItem{Value: connected, Label: "浏览器"},
	)
}

func (a *App) library(c *ui.Context) ui.Element {
	return ui.Column(c).Grow(1).Children(func() {
		ui.Column(c).Padding(20, 30, 12, 30).Gap(12).Shrink(0).Children(func() {
			a.errorBanner(c)
			a.searchBar(c, "library")
			a.filter(c, "library")
			a.contentTypeRow(c, "library")
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 30, 32, 30).Gap(16).Children(func() {
				a.libraryStats(c)
				items := a.currentItems()
				if len(items) == 0 {
					emptyState(c, "没有符合条件的内容", "换个来源或类型，或点「清除筛选」。")
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
				a.loadMoreRow(c, "library")
			})
		})
	})
}

// contentTypeRow filters a feed by content type and source category. Today
// and the library keep independent filters, so the row reads whichever view
// asks for it.
func (a *App) contentTypeRow(c *ui.Context, view string) {
	f := a.filters[view]
	if f == nil {
		return
	}
	types := []struct{ id, label string }{
		{"", "全部类型"}, {"article", "文章"}, {"podcast", "播客"}, {"youtube", "视频"}, {"tweet", "推文"},
	}
	ui.Row(c).Gap(5).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "类型").FontSize(10.5).TextColor(textFaint(c)).LetterSpacing(0.3).Padding(0, 5, 0, 3)
		for _, ty := range types {
			ty := ty
			on := f.ContentType == ty.id
			pill(c, ty.label, on).OnClick(func() {
				if f.ContentType != ty.id {
					f.ContentType = ty.id
					a.reloadItems(view)
				}
			})
		}
		if len(a.stats.SourceCategories) > 0 {
			ui.Divider(c).Vertical().Height(14)
			ui.Text(c, "来源").FontSize(10.5).TextColor(textFaint(c)).LetterSpacing(0.3).Padding(0, 8, 0, 3)
			for _, name := range a.stats.SourceCategories {
				if name == "" {
					continue
				}
				name := name
				on := f.SourceCategory == name
				pill(c, name, on).OnClick(func() {
					if f.SourceCategory == name {
						f.SourceCategory = ""
					} else {
						f.SourceCategory = name
					}
					a.reloadItems(view)
				})
			}
		}
	})
}

func (a *App) feedGrid(c *ui.Context, items []Item, cols int) {
	ui.Grid(c).Columns(cols).Gap(16).Children(func() {
		for _, it := range items {
			a.itemCard(c, it)
		}
	})
}

func (a *App) itemCard(c *ui.Context, it Item) {
	t := c.Theme()
	bg := t.Surface
	if it.ReadStatus == 1 {
		bg = t.Surface.Mix(t.Background, 0.45)
	}
	titleColor := t.Text
	if it.ReadStatus == 1 {
		titleColor = t.TextMuted
	}
	card := ui.ButtonBase(c).Padding(0).Background(bg).Border(1, t.Border).Radius(16).
		Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond})
	if !t.Dark {
		card = card.Shadow(0, 6, 18, -8, ui.Hex("#000000").Alpha(0.09))
	}
	if card.Hovered() {
		card.BorderColor(t.Text.Alpha(0.16))
	}
	card.Children(func() {
		// Grow keeps the text flush left: a button centers its children.
		ui.Column(c).Grow(1).Padding(16, 17).Gap(9).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				dot := t.Accent.Alpha(0)
				if it.ReadStatus == 0 {
					dot = t.Accent
				}
				// The dot keeps its room when read, so sources line up
				// across the cards of a row.
				ui.Box(c).Size(6, 6).Radius(3).Background(dot).Shrink(0)
				ui.Text(c, shortSourceLabel(it)).FontSize(10.5).FontWeight(600).TextColor(t.Accent).LetterSpacing(0.2).Grow(1).MaxLines(1)
				ui.Text(c, timeAgo(itemTime(it))).FontSize(10.5).TextColor(textFaint(c))
				star := ui.ButtonBase(c).Children(func() {
					ui.Icon(c, starIcon(c, it.Starred == 1)).Size(13, 13)
				}).Label("收藏").Padding(1, 4).Radius(6).TextColor(textFaint(c))
				if star.Hovered() {
					star.Background(t.SurfaceHover)
				}
				if it.Starred == 1 {
					star.TextColor(ui.Hex("#e5a83b"))
				}
				if star.Clicked() {
					a.toggleStar(it)
				}
			})
			ui.Text(c, orDefault(it.Title, "未命名内容")).FontSize(15).FontWeight(600).TextColor(titleColor).MaxLines(2).LineHeight(1.42).LetterSpacing(-0.2)
			if s := cleanSummary(it.DisplaySummary()); s != "" {
				ui.Text(c, truncate(s, 150)).FontSize(12.5).TextColor(textSoft(c)).MaxLines(2).LineHeight(1.6)
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if it.AICategory != "" {
					accentChip(c, it.AICategory)
				} else if it.Category != "" && it.Category != "article" {
					softChip(c, categoryLabel(it.Category))
				}
				if it.ContentTypeLabel() != "" {
					softChip(c, it.ContentTypeLabel())
				}
				ui.Spacer(c).Grow(1)
				if it.QualityScore > 0 {
					scoreMark(c, it.QualityScore)
				}
			})
		})
	})
	if card.Clicked() {
		a.openReader(it)
	}
}

func (a *App) itemRow(c *ui.Context, it Item) {
	t := c.Theme()
	bg := t.Surface
	if it.ReadStatus == 1 {
		bg = t.Surface.Mix(t.Background, 0.45)
	}
	row := ui.ButtonBase(c).Padding(0).Background(bg).Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Duration: 150 * time.Millisecond})
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		// Grow keeps the content flush left: a button centers its children, so
		// without it rows sized by their text are indented differently.
		ui.Row(c).Grow(1).Padding(12, 16).Gap(13).AlignItems(ui.Center).Children(func() {
			itemMonogram(c, sourceLabel(it), 38)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					dot := t.Accent.Alpha(0)
					if it.ReadStatus == 0 {
						dot = t.Accent
					}
					ui.Box(c).Size(5, 5).Radius(3).Background(dot)
					ui.Text(c, shortSourceLabel(it)).FontSize(10.5).TextColor(t.Accent).FontWeight(600).LetterSpacing(0.2).MaxLines(1)
					if it.AICategory != "" {
						ui.Text(c, "· "+it.AICategory).FontSize(10.5).TextColor(textSoft(c)).MaxLines(1)
					}
				})
				ui.Text(c, orDefault(it.Title, "未命名内容")).FontSize(13.5).FontWeight(600).MaxLines(1).LetterSpacing(-0.1)
				if s := cleanSummary(it.DisplaySummary()); s != "" {
					ui.Text(c, truncate(s, 140)).FontSize(12).TextColor(textSoft(c)).MaxLines(1)
				}
			})
			ui.Column(c).Gap(4).AlignItems(ui.End).Shrink(0).Children(func() {
				ui.Text(c, formatDate(itemTime(it))).FontSize(10.5).TextColor(textFaint(c))
				if it.QualityScore > 0 {
					scoreMark(c, it.QualityScore)
				}
			})
			star := ui.ButtonBase(c).Children(func() {
				ui.Icon(c, starIcon(c, it.Starred == 1)).Size(13, 13)
			}).Label("收藏").Padding(3, 5).Radius(6).TextColor(textFaint(c))
			if star.Hovered() {
				star.Background(t.SurfacePressed)
			}
			if it.Starred == 1 {
				star.TextColor(ui.Hex("#e5a83b"))
			}
			if star.Clicked() {
				a.toggleStar(it)
			}
		})
	})
	if row.Clicked() {
		a.openReader(it)
	}
}

func (a *App) loadMoreRow(c *ui.Context, view string) {
	if a.viewCursors[view] == "" {
		return
	}
	ui.Row(c).AlignItems(ui.Center).Padding(4, 0, 12, 0).Children(func() {
		quietButton(c, "加载更多").FontSize(12).OnClick(func() { a.loadMore(view) })
	})
}

func itemTime(it Item) time.Time {
	if it.PublishedAt != nil && !it.PublishedAt.IsZero() {
		return *it.PublishedAt
	}
	return it.FetchedAt
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func (it Item) ContentTypeLabel() string {
	switch it.Category {
	case "podcast":
		return "播客"
	case "youtube":
		return "视频"
	case "tweet":
		return "推文"
	}
	return ""
}

// categoryLabel is the human label for an item's stored category.
func categoryLabel(cat string) string {
	switch cat {
	case "article":
		return "文章"
	case "news":
		return "新闻"
	case "podcast":
		return "播客"
	case "youtube":
		return "视频"
	case "tweet":
		return "推文"
	}
	return cat
}
