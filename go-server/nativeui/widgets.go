package nativeui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// cardSurface is the standard white card: a hairline border and, in the light
// theme, a soft shadow that lifts it off the page the way AppKit panels sit.
func cardSurface(c *ui.Context, radius float32) ui.Element {
	t := c.Theme()
	e := ui.Column(c).Background(t.Surface).Border(1, t.Border).Radius(radius)
	if !t.Dark {
		e = e.Shadow(0, 7, 22, -8, ui.Hex("#000000").Alpha(0.10))
	}
	return e
}

// quietButton is the secondary action: no face until the pointer arrives, so
// an inspector full of actions reads as text instead of a wall of gray boxes.
func quietButton(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(5, 10).Radius(8).FontSize(12.5).FontWeight(500).
		TextColor(t.Text).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond})
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, label) })
	return b
}

// primaryButton is the one accent action on a page. It takes its disabled
// state up front: a disabled button has to lose the accent fill as it is
// built, not after.
func primaryButton(c *ui.Context, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(5, 12).Radius(8).FontSize(12.5).FontWeight(600).
		TextColor(t.AccentText).Disabled(disabled).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond})
	switch {
	case disabled:
		b.Background(t.SurfaceHover).TextColor(t.TextMuted)
	case b.Pressed():
		b.Background(t.AccentPressed)
	case b.Hovered():
		b.Background(t.AccentHover)
	default:
		b.Background(t.Accent)
	}
	b.Children(func() { ui.Text(c, label) })
	return b
}

// numberStepper pairs the arrow control with the value it edits: the stock
// stepper draws only arrows, as AppKit's does beside a field, so the number
// gets its own readout or the setting is invisible while you change it.
func numberStepper(c *ui.Context, value *float64, lo, hi, step float64, suffix string) bool {
	t := c.Theme()
	changed := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(56).Height(24).Radius(7).Border(1, t.Border).Background(t.Surface).
			Center().Children(func() {
			ui.Text(c, fmt.Sprintf("%g%s", *value, suffix)).FontSize(12.5).FontWeight(600)
		})
		if ui.Stepper(c, value, lo, hi, step).Changed() {
			changed = true
		}
	})
	return changed
}

// pill draws the round filter tag used above the feeds.
func pill(c *ui.Context, label string, selected bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(4, 11).Radius(99).FontSize(11.5).FontWeight(500)
	b.Transition(ui.ElementTransition{Duration: 120 * time.Millisecond})
	switch {
	case selected:
		b.Background(t.Accent.Alpha(0.12)).TextColor(t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover).TextColor(t.TextMuted)
	default:
		b.TextColor(t.TextMuted)
	}
	b.Children(func() { ui.Text(c, label) })
	return b
}

// flatPanel is a card without padding for lists that draw their own rows.
func flatPanel(c *ui.Context, fn func()) ui.Element {
	return cardSurface(c, 16).Clip().Children(fn)
}

func hint(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(12).TextColor(textSoft(c)).LineHeight(1.55)
}

// chip is a small rounded label, as the web UI's category pills.
func chip(c *ui.Context, label string, fg, bg ui.Color) ui.Element {
	return ui.Box(c).Padding(3, 9).Radius(99).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(fg).FontWeight(550).MaxLines(1)
	})
}

func softChip(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	return chip(c, label, t.TextMuted, bgSubtle(c))
}

func accentChip(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	return chip(c, label, t.Accent, t.Accent.Alpha(0.12))
}

func dangerChip(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	return chip(c, label, t.Danger, t.Danger.Alpha(0.12))
}

// iconButton is a compact toolbar button with an icon and an accessible label.
func iconButton(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(6, 8).Radius(8).TextColor(t.TextMuted).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond})
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, svg).Size(15, 15) })
	return b.Label(label).Tooltip(label)
}

func iconTextButton(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(5, 10).Radius(8).FontSize(12.5).FontWeight(500).TextColor(t.Text).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond})
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, svg).Size(14, 14).TextColor(t.TextMuted)
			ui.Text(c, label)
		})
	})
	return b.Label(label)
}

// statItem is one figure of statsBar: a value and its label.
type statItem struct {
	Value string
	Label string
	Size  float32
}

// statsBar gathers figures into one grouped card, separated by hairlines,
// instead of a row of individually boxed tiles.
func statsBar(c *ui.Context, items ...statItem) ui.Element {
	return cardSurface(c, 16).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			for i, it := range items {
				if i > 0 {
					ui.Divider(c).Vertical().Height(32)
				}
				size := it.Size
				if size == 0 {
					size = 26
				}
				ui.Column(c).Gap(4).Padding(15, 20).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, it.Value).FontSize(size).FontWeight(700).MaxLines(1).LetterSpacing(-0.4)
					ui.Text(c, it.Label).FontSize(10.5).FontWeight(500).TextColor(textSoft(c)).LetterSpacing(0.1).MaxLines(1)
				})
			}
		})
	})
}

// usageChart draws a week of requests as horizontal bars. Rows read the same
// way with one busy day as with seven, where vertical bars leave a lone
// block floating over an empty plot.
func usageChart(c *ui.Context, days []UsageByDay) ui.Element {
	t := c.Theme()
	maxReq := int64(1)
	for _, d := range days {
		if d.Requests > maxReq {
			maxReq = d.Requests
		}
	}
	return ui.Column(c).Gap(7).MaxWidth(560).Children(func() {
		for _, d := range days {
			d := d
			frac := float32(d.Requests) / float32(maxReq)
			count := fmt.Sprint(d.Requests)
			if d.Requests == 0 {
				count = "—"
			}
			row := ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, shortDay(d.Day)).FontSize(10.5).TextColor(textFaint(c)).Width(34).Shrink(0)
				ui.Row(c).Grow(1).Height(6).Radius(3).Background(bgSubtle(c)).Children(func() {
					if frac > 0 {
						ui.Box(c).Grow(frac).Height(6).Radius(3).Background(t.Accent.Alpha(0.5))
					}
					if frac < 1 {
						ui.Box(c).Grow(1 - frac).Height(6)
					}
				})
				ui.Text(c, count).FontSize(10.5).TextColor(textSoft(c)).Width(40).TextAlign(ui.End).Shrink(0)
			})
			row.Tooltip(fmt.Sprintf("%s · %d 请求 · %s tokens", d.Day, d.Requests, humanTokens(d.Tokens)))
		}
	})
}

func shortDay(day string) string {
	if len(day) >= 10 {
		return day[5:10]
	}
	return day
}

func emptyState(c *ui.Context, title, description string) {
	ui.Column(c).Fill().Center().Gap(10).Padding(48).Children(func() {
		ui.Box(c).Size(52, 52).Radius(26).Background(bgSubtle(c)).Center().Children(func() {
			ui.Icon(c, iconInbox).Size(24, 24).TextColor(textSoft(c))
		})
		ui.Text(c, title).FontSize(15).FontWeight(600)
		if description != "" {
			ui.Text(c, description).FontSize(12.5).TextColor(textSoft(c)).TextAlign(ui.Center).MaxWidth(380).LineHeight(1.5)
		}
	})
}

func keyValue(c *ui.Context, key, value string) {
	ui.Row(c).Gap(12).Children(func() {
		ui.Text(c, key).FontSize(12).TextColor(textSoft(c)).Width(96).Shrink(0)
		ui.Text(c, value).FontSize(12.5).Grow(1)
	})
}

// itemMonogram draws the deterministic gradient cover used by cards and rows.
func itemMonogram(c *ui.Context, label string, size float32) ui.Element {
	t := c.Theme()
	from := ui.Hex("#fa2d48").Alpha(0.16)
	to := ui.Hex("#fa2d48").Alpha(0.06)
	initial := "G"
	if s := strings.TrimSpace(label); s != "" {
		initial = strings.ToUpper(string([]rune(s)[0]))
	}
	if t.Dark {
		from = ui.Hex("#ff453a").Alpha(0.22)
		to = ui.Hex("#ff453a").Alpha(0.08)
	}
	return ui.Box(c).Size(size, size).Radius(size*0.24).Gradient(from, to, 135).Center().Children(func() {
		ui.Text(c, initial).FontSize(size * 0.4).FontWeight(700).TextColor(t.Accent)
	})
}

func hueFor(s string) float32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return float32(h.Sum32()%360) - 180
}

func scoreMark(c *ui.Context, score int) ui.Element {
	return scoreChip(c, float64(score))
}

// scoreChip shows a quality score, tinted by how high it is.
func scoreChip(c *ui.Context, score float64) ui.Element {
	t := c.Theme()
	fg, bg := t.TextMuted, bgSubtle(c)
	if score >= 8 {
		fg, bg = t.Success, t.Success.Alpha(0.13)
	} else if score >= 6 {
		fg, bg = t.Warning, t.Warning.Alpha(0.13)
	}
	return chip(c, fmt.Sprintf("%.1f", score), fg, bg)
}

func starIcon(c *ui.Context, starred bool) *ui.SVG {
	if starred {
		return iconStar
	}
	return iconStarLine
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("01-02")
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	default:
		return t.Local().Format("2006-01-02")
	}
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func cleanSummary(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// shortSourceLabel trims a feed's display name at its subtitle separator, so
// "钛媒体: 引领未来商业与生活新知" reads as "钛媒体" on a card.
func shortSourceLabel(it Item) string {
	s := strings.TrimSpace(sourceLabel(it))
	for _, sep := range []string{"：", ":", " — ", " - ", " | ", "｜"} {
		if i := strings.Index(s, sep); i >= 2 {
			return strings.TrimSpace(s[:i])
		}
	}
	return truncate(s, 18)
}

func sourceLabel(it Item) string {
	if it.OriginSource != "" {
		return it.OriginSource
	}
	if it.SourceCategory != "" {
		return it.SourceCategory
	}
	return "Grabby"
}
