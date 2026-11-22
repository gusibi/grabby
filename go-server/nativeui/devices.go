package nativeui

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
)

func (a *App) devicesView(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 30).Gap(16).Children(func() {
			a.errorBanner(c)
			t := c.Theme()

			ui.Row(c).Padding(15, 17).Background(t.Surface).Border(1, t.Border).Radius(14).Gap(12).AlignItems(ui.Center).Children(func() {
				online := 0
				for _, b := range a.browsers {
					if b.Online {
						online++
					}
				}
				dot := t.Danger
				if a.health.BrowserConnected {
					dot = t.Success
				}
				ui.Box(c).Size(9, 9).Radius(5).Background(dot)
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					label := "扩展未连接"
					if a.health.BrowserConnected {
						label = "扩展已连接"
					}
					ui.Text(c, label).FontSize(14).FontWeight(650)
					ui.Text(c, fmt.Sprintf("%d 个已注册 · %d 个在线 · ws://localhost:%d/ws_browser", len(a.browsers), online, a.port())).
						FontSize(11).TextColor(textSoft(c)).Font("monospace")
				})
			})

			ui.Text(c, "已注册设备").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
			if len(a.browsers) == 0 {
				flatPanel(c, func() {
					a.connectionHelp(c)
				})
			} else {
				flatPanel(c, func() {
					for i, b := range a.browsers {
						if i > 0 {
							ui.Divider(c)
						}
						a.deviceRow(c, b)
					}
				})
				ui.Column(c).Padding(18, 20).Background(t.Surface).Border(1, t.Border).Radius(14).Gap(12).Children(func() {
					ui.Text(c, "连接与路由").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
					hint(c, "扩展按连接名称选择。多个设备同时在线时，抓取任务默认使用第一个在线设备，也可以在 API 请求里通过 browser 字段指定。")
					keyValue(c, "WebSocket", fmt.Sprintf("ws://localhost:%d/ws_browser", a.port()))
					keyValue(c, "扩展", "加载 chrome-extension 目录，选项里填写同一地址。")
					if a.desktop.APIToken != "" {
						keyValue(c, "Token", "已启用（扩展与命令行需要带上）")
					}
				})
			}
		})
	})
}

func (a *App) deviceRow(c *ui.Context, b Browser) {
	t := c.Theme()
	ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		dot := t.Danger
		state := "离线"
		if b.Online {
			dot = t.Success
			state = "在线"
		}
		ui.Box(c).Size(8, 8).Radius(4).Background(dot)
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Text(c, orDefault(b.Name, "未命名设备")).FontSize(13.5).FontWeight(650)
				if state == "离线" {
					softChip(c, state)
				}
				if b.Banned {
					dangerChip(c, "已封禁")
				}
			})
			ui.Text(c, b.ConnID).FontSize(10.5).TextColor(textSoft(c)).Font("monospace").MaxLines(1)
		})
		if b.Online {
			quietButton(c, "踢下线").FontSize(11).OnClick(func() {
				a.runAsync("已踢下线", func() error { return a.client.browserAction("kick", b.ConnID) })
			})
		}
		if b.Banned {
			quietButton(c, "解除封禁").FontSize(11).OnClick(func() {
				a.runAsync("已解除封禁", func() error { return a.client.browserAction("unban", b.ConnID) })
			})
		} else {
			quietButton(c, "封禁").FontSize(11).TextColor(t.Danger).OnClick(func() {
				a.ask("封禁设备", "「"+orDefault(b.Name, b.ConnID)+"」将不能再次注册，直到解除封禁。", "封禁", func() error {
					return a.client.browserAction("ban", b.ConnID)
				})
			})
		}
		if !b.Online {
			quietButton(c, "删除").FontSize(11).TextColor(t.Danger).OnClick(func() {
				name := orDefault(b.Name, b.ConnID)
				a.ask("删除设备", "将删除「"+name+"」的注册记录和封禁状态。扩展再次注册时，设备会重新出现。", "删除", func() error {
					if err := a.client.browserAction("delete", b.ConnID); err != nil {
						return err
					}
					a.win.Update(func() {
						a.browsers = slices.DeleteFunc(a.browsers, func(device Browser) bool {
							return device.ConnID == b.ConnID
						})
						switch a.loading {
						case "devices", "today", "library", "apihub":
							a.seq++
							a.loading = ""
							a.ensureData()
						}
						a.toasts = append(a.toasts, toastMsg{Text: "设备已删除"})
					})
					return nil
				})
			})
		}
	})
}

func (a *App) connectionHelp(c *ui.Context) {
	ui.Column(c).Padding(20, 22).Gap(10).Children(func() {
		ui.Text(c, "还没有浏览器连接").FontSize(16).FontWeight(650)
		hint(c, "Grabby 的抓取由 Chrome 扩展在真实浏览器里带登录态执行。装好扩展并连接后，这里会显示设备。")
		for i, step := range []string{
			"打开 Chrome 的「扩展程序」页面，开启开发者模式。",
			"选择「加载已解压的扩展程序」，指向仓库里的 chrome-extension 目录。",
			"点 Grabby 图标 → 设置，填写 ws://localhost:" + fmt.Sprint(a.port()) + "/ws_browser。",
			"勾选「启动时自动连接服务器」并保存。",
		} {
			ui.Row(c).Gap(8).Children(func() {
				ui.Box(c).Size(18, 18).Radius(9).Background(c.Theme().Accent.Alpha(0.12)).Center().Children(func() {
					ui.Text(c, fmt.Sprint(i+1)).FontSize(9.5).FontWeight(700).TextColor(c.Theme().Accent)
				})
				ui.Text(c, step).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
			})
		}
	})
}
