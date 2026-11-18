package nativeui

import (
	"github.com/egoist/mygo/ui"
)

// palette restores the desktop app's original identity: white paper, ink text,
// a red signal, and the same grays as the previous UI's stylesheet.
func themeFor(dark bool) *ui.Theme {
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
		t.Dark = true
		t.Background = ui.Hex("#1c1c1e")
		t.Surface = ui.Hex("#2c2c2e")
		t.SurfaceHover = ui.Hex("#333335")
		t.SurfacePressed = ui.Hex("#3a3a3c")
		t.Border = ui.Hex("#3a3a3d")
		t.Text = ui.Hex("#f5f5f7")
		t.TextMuted = ui.Hex("#a1a1a6")
		t.Accent = ui.Hex("#ff453a")
		t.AccentHover = ui.Hex("#ff6b61")
		t.AccentPressed = ui.Hex("#e0352b")
		t.AccentText = ui.Hex("#ffffff")
		t.Selection = ui.Hex("#ff453a").Alpha(0.25)
		t.Danger = ui.Hex("#ff6961")
		t.Warning = ui.Hex("#e9be68")
		t.Success = ui.Hex("#6fcf97")
		t.Scrollbar = ui.Hex("#48484a")
	} else {
		t.Background = ui.Hex("#ffffff")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f4f4f6")
		t.SurfacePressed = ui.Hex("#eaeaee")
		t.Border = ui.Hex("#e7e7ec")
		t.Text = ui.Hex("#1d1d1f")
		t.TextMuted = ui.Hex("#6e6e73")
		t.Accent = ui.Hex("#fa2d48")
		t.AccentHover = ui.Hex("#ff4a61")
		t.AccentPressed = ui.Hex("#e01f39")
		t.AccentText = ui.Hex("#ffffff")
		t.Selection = ui.Hex("#fa2d48").Alpha(0.18)
		t.Danger = ui.Hex("#d70015")
		t.Warning = ui.Hex("#b26a00")
		t.Success = ui.Hex("#30a46c")
		t.Scrollbar = ui.Hex("#d6d6db")
	}
	t.Radius = 10
	t.Spacing = 4
	t.FontSize = 13
	t.InactiveAccent = t.Accent
	// A focus ring the color of the accent at full strength turns into a red
	// box around whatever is focused; the softer ring reads as a hint.
	t.Focus = t.Accent.Alpha(0.45)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 1
	t.TooltipDelay = 0
	return t
}

func (a *App) applyTheme(c *ui.Context) {
	dark := false
	if cur := c.Theme(); cur != nil {
		dark = cur.Dark
	}
	c.SetTheme(themeFor(dark))
}

// textSoft is the muted color, used for metadata lines.
func textSoft(c *ui.Context) ui.Color { return c.Theme().TextMuted }

// textFaint is the tertiary gray of sidebar section labels and footers.
func textFaint(c *ui.Context) ui.Color {
	_ = c
	return ui.Hex("#8e8e93")
}

// bgSubtle is a faint panel fill derived from the theme.
func bgSubtle(c *ui.Context) ui.Color {
	t := c.Theme()
	if t.Dark {
		return t.SurfaceHover
	}
	return ui.Hex("#f5f5f7")
}

// sidebarBg is the translucent-gray sidebar the desktop app always had.
func sidebarBg(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#262629")
	}
	return ui.Hex("#f3f3f6")
}

// canvasBg is the page behind the white cards: a paper tone just off white,
// so cards sit on something instead of dissolving into the window.
func canvasBg(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#19191b")
	}
	return ui.Hex("#f7f7f9")
}

// navActive is the gray pill behind the chosen navigation item.
func navActive(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#ffffff").Alpha(0.10)
	}
	return ui.Hex("#000000").Alpha(0.07)
}
