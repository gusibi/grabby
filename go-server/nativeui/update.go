package nativeui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"go-server/internal/updater"
)

// updateState is the settings page's self-update state. It lives on App so
// progress survives the rebuild MyGo makes on every frame.
type updateState struct {
	busy       bool
	phase      string // checking, downloading, installing
	message    string
	err        string
	latest     string
	progress   float64 // 0..1; negative draws an indeterminate bar
	upToDate   bool
	releaseURL string
}

// updateSection is the “关于与更新” card. Checking, downloading, verifying and
// installing all happen without further prompts: that is the point of the
// feature, because the alternative is another unsigned DMG the user has to
// authorise in System Settings.
func (a *App) updateSection(c *ui.Context) {
	a.settingsSection(c, "关于与更新", "检查到新版本后会下载并校验更新包，然后自动替换当前版本并重启；不需要再到系统安全设置里授权。", func() {
		keyValue(c, "当前版本", updater.DisplayVersion())

		switch {
		case a.update.busy:
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, a.update.message).FontSize(12.5).TextColor(textSoft(c))
			})
			if a.update.phase == "downloading" {
				ui.Progress(c, a.update.progress).Width(320)
			}
		case a.update.err != "":
			t := c.Theme()
			ui.Text(c, a.update.err).FontSize(12).TextColor(t.Danger).LineHeight(1.5)
		case a.update.message != "":
			ui.Text(c, a.update.message).FontSize(12.5).TextColor(textSoft(c)).LineHeight(1.5)
		}

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			label := "检查更新"
			if a.update.err != "" {
				label = "重试"
			}
			quietButtonDisabled(c, label, a.update.busy).OnClick(a.checkForUpdates)
			if a.update.releaseURL != "" {
				quietButton(c, "查看发行说明").FontSize(12).OnClick(func() {
					_ = openExternal(a.update.releaseURL)
				})
			}
		})
		hint(c, "更新包会与发布页 checksums.txt 里的 SHA-256 比对；校验失败时不会替换当前版本。")
	})
}

func (a *App) checkForUpdates() {
	if a.update.busy {
		return
	}
	a.update = updateState{
		busy:     true,
		phase:    "checking",
		message:  "正在检查更新…",
		progress: -1,
	}
	go a.runUpdate()
}

// setUpdate changes update state on the UI thread. All writes to a.update
// after checkForUpdates go through here, including those from the download
// goroutine.
func (a *App) setUpdate(fn func(*updateState)) {
	if a.win == nil {
		fn(&a.update)
		return
	}
	a.win.Update(func() { fn(&a.update) })
}

func (a *App) updateFailed(msg string) {
	a.setUpdate(func(u *updateState) {
		u.busy = false
		u.phase = ""
		u.err = msg
	})
}

func (a *App) runUpdate() {
	// The overall limit is generous because international downloads can be
	// slow; a dead connection is cut off by the updater's stall timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	client := updater.New()
	rel, err := client.Latest(ctx)
	if err != nil {
		a.updateFailed("检查更新失败: " + err.Error())
		return
	}

	latest := rel.Version()
	if !rel.NewerThan(updater.Version) {
		a.setUpdate(func(u *updateState) {
			u.busy = false
			u.phase = ""
			u.upToDate = true
			u.latest = latest
			u.releaseURL = rel.HTMLURL
			u.message = "已是最新版本（v" + updater.CurrentVersion() + "）"
		})
		return
	}

	// A source build must not replace itself with a release binary.
	if !updater.IsReleaseBuild() {
		a.setUpdate(func(u *updateState) {
			u.busy = false
			u.phase = ""
			u.latest = latest
			u.releaseURL = rel.HTMLURL
			u.message = fmt.Sprintf("发现新版本 v%s；当前是开发版，不自动替换，请手动下载安装。", latest)
		})
		return
	}

	a.setUpdate(func(u *updateState) {
		u.phase = "downloading"
		u.latest = latest
		u.releaseURL = rel.HTMLURL
		u.progress = -1
		u.message = "正在下载 v" + latest + "…"
	})

	var last time.Time
	staged, err := client.Download(ctx, rel, func(done, total int64) {
		if total <= 0 {
			return
		}
		if done != total && time.Since(last) < 250*time.Millisecond {
			return
		}
		last = time.Now()
		p := float64(done) / float64(total)
		a.setUpdate(func(u *updateState) {
			u.progress = p
			u.message = fmt.Sprintf("正在下载 v%s… %s / %s（%d%%）",
				latest, formatBytes(done), formatBytes(total), int(p*100))
		})
	})
	if err != nil {
		a.updateFailed("更新失败: " + err.Error())
		return
	}

	a.setUpdate(func(u *updateState) {
		u.phase = "installing"
		u.progress = 1
		u.message = "下载完成，正在重启并安装 v" + latest + "…"
	})
	// Let the window draw the final state before the process exits. The
	// detached helper waits for this process regardless, so this is purely
	// cosmetic.
	time.Sleep(1200 * time.Millisecond)

	if err := staged.Apply(); err != nil {
		_ = staged.Close()
		a.updateFailed("无法安装更新: " + err.Error())
		return
	}
	// Quit is posted to the UI thread by MyGo; the helper swaps the files once
	// this process is gone, then reopens the app.
	mygo.App.Quit()
}

// formatBytes renders a download size for the progress line.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
