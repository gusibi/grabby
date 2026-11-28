package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settingsTabs splits the settings page into the kinds of setting it holds.
// Six cards in one scroll became hard to scan, and the model editor in
// particular deserves the whole pane.
var settingsTabs = []struct{ ID, Label string }{
	{"general", "常规"},
	{"models", "AI 模型"},
	{"reading", "阅读与转写"},
	{"prompts", "提示词与日报"},
	{"security", "安全"},
	{"about", "关于"},
}

func (a *App) settingsView(c *ui.Context) ui.Element {
	d := a.ensureAIDraft()
	if a.settingsTab == "" {
		a.settingsTab = settingsTabs[0].ID
	}
	// The model editor needs the catalog; loading it is a no-op once cached.
	if a.settingsTab == "models" {
		a.ensureAIModelCatalog()
	}
	labels := make([]string, len(settingsTabs))
	selected := 0
	for i, tab := range settingsTabs {
		labels[i] = tab.Label
		if tab.ID == a.settingsTab {
			selected = i
		}
	}
	return ui.Column(c).Grow(1).Children(func() {
		ui.Row(c).Padding(20, 30, 4, 30).AlignItems(ui.Center).Children(func() {
			if ui.Segmented(c, &selected, labels...).Changed() {
				a.settingsTab = settingsTabs[selected].ID
				a.aiModelSearch = ""
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Grow(1).Padding(16, 30, 32, 30).Gap(16).Children(func() {
				a.errorBanner(c)

				switch a.settingsTab {
				case "models":
					a.settingsSection(c, "AI 模型", "配置服务商、模型与轮询策略；保存后立即生效。调用用量与分析结果在「AI 分析」页。", func() {
						a.aiSettingsBody(c)
					})

				case "reading":
					a.settingsSection(c, "阅读增强", "阅读页的全文翻译与 AI 对话（解读与追问）可以使用独立模型；不选则跟随主配置。", func() {
						a.readerModelsBody(c)
					})
					a.asrSection(c, d)

				case "prompts":
					a.settingsSection(c, "提示词", "决定单条内容分析与日报生成的输出风格；留空则使用内置提示词。", func() {
						ui.Column(c).Gap(6).Children(func() {
							ui.Text(c, "单条内容分析").FontSize(12).TextColor(textSoft(c))
							ui.TextArea(c, &a.promptSystem).Height(120).Font("monospace").Placeholder("留空使用内置提示词").MaxWidth(640)
							ui.Text(c, "AI 日报").FontSize(12).TextColor(textSoft(c))
							ui.TextArea(c, &a.promptDaily).Height(120).Font("monospace").Placeholder("留空使用内置提示词").MaxWidth(640)
						})
					})
					a.settingsSection(c, "日报定时", "到点自动生成早报 / 晚报；定时改动需重启 APP 后才生效。", func() {
						autoSave := func() { a.saveAIDraft() }
						ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
							m := d.MorningReportEnabled
							if ui.Switch(c, &m).Changed() {
								d.MorningReportEnabled = m
								autoSave()
							}
							ui.Text(c, "早报").FontSize(12).Width(50)
							ui.TextInput(c, &d.MorningReportTime).Width(90).Font("monospace")
						})
						ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
							e := d.EveningReportEnabled
							if ui.Switch(c, &e).Changed() {
								d.EveningReportEnabled = e
								autoSave()
							}
							ui.Text(c, "晚报").FontSize(12).Width(50)
							ui.TextInput(c, &d.EveningReportTime).Width(90).Font("monospace")
						})
					})

				case "security":
					a.settingsSection(c, "API 与安全", "默认只监听本机且不需要 Token；对外提供 API 时才需要设置 Token。", func() {
						a.tokenPanel(c, fmt.Sprintf("http://localhost:%d", a.port()))
					})

				case "about":
					a.settingsSection(c, "数据与文件", "订阅源、内容库与设置文件都保存在同一个数据目录里。", func() {
						keyValue(c, "数据目录", orDefault(a.desktop.DataDir, "—"))
						ui.Row(c).Gap(8).Children(func() {
							quietButton(c, "导出配置备份").FontSize(12).OnClick(func() { go a.exportConfig() })
							quietButton(c, "导入配置备份").FontSize(12).OnClick(func() { go a.pickImportConfig() })
						})
						hint(c, "备份文件包含明文 API Key，请妥善保管。导入时 AI 配置整体覆盖并立即生效；数据源按 ID 新增或更新。")
					})
					a.updateSection(c)

				default:
					a.settingsSection(c, "服务状态", "HTTP / WebSocket / MCP 服务随 APP 启动；关窗后留在托盘继续运行，从托盘「退出」才真正结束。", func() {
						status := "运行中"
						if a.health.Status != "" && a.health.Status != "ok" {
							status = a.health.Status
						}
						keyValue(c, "服务", status)
						keyValue(c, "监听", fmt.Sprintf("%s:%d", orDefault(a.desktop.Host, "127.0.0.1"), a.port()))
						keyValue(c, "浏览器", fmt.Sprintf("%d 个已注册 · %s", len(a.browsers), map[bool]string{true: "扩展已连接", false: "扩展未连接"}[a.health.BrowserConnected]))
						keyValue(c, "数据目录", orDefault(a.desktop.DataDir, "—"))
						keyValue(c, "日志", defaultLogPath())
						ui.Row(c).Gap(8).Children(func() {
							quietButton(c, "打开数据目录").FontSize(12).OnClick(func() {
								if a.desktop.DataDir != "" {
									_ = openPath(a.desktop.DataDir)
								}
							})
							quietButton(c, "打开日志目录").FontSize(12).OnClick(func() {
								_ = openPath(filepath.Dir(defaultLogPath()))
							})
						})
					})
				}
			})
		})
	})
}

// asrSection draws the speech-to-text card: mode, endpoint, and the local
// toolchain status.
func (a *App) asrSection(c *ui.Context, d *AISettings) {
	a.settingsSection(c, "音视频转文字 (ASR)", "用于无字幕 YouTube 视频和播客音频自动生成文字版；支持本地 CLI 与云端 Whisper 接口。", func() {
		ui.Form(c, func() {
			ui.Field(c, "模式", func() {
				labels := []string{"自动", "云端", "本地", "关闭"}
				ids := []string{"auto", "cloud", "local", "disabled"}
				cur := labels[0]
				for i, id := range ids {
					if d.ASRMode == id {
						cur = labels[i]
					}
				}
				if ui.Select(c, &cur, labels).MaxWidth(220).Changed() {
					for i, label := range labels {
						if label == cur {
							d.ASRMode = ids[i]
						}
					}
				}
			})
			ui.Field(c, "Provider", func() { ui.TextInput(c, &d.ASRProvider).Placeholder("groq / openai / custom").MaxWidth(420) })
			ui.Field(c, "Base URL", func() { ui.TextInput(c, &d.ASRBaseURL).Font("monospace").MaxWidth(420) })
			ui.Field(c, "API Key", func() { ui.TextInput(c, &d.ASRAPIKey).Password().MaxWidth(420) })
			ui.Field(c, "模型", func() { ui.TextInput(c, &d.ASRModel).Font("monospace").MaxWidth(420) })
		})
		if a.asrEnv != nil {
			ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				state := "本地未就绪"
				if a.asrEnv.LocalReady {
					state = "本地可用"
				}
				if a.asrEnv.CloudReady {
					state += " · 云端可用"
				}
				softChip(c, state)
				for _, name := range []string{"ffmpeg", "yt-dlp", "mlx_whisper", "whisper"} {
					ok := false
					if a.asrEnv.Tools != nil {
						ok = a.asrEnv.Tools[name].Installed
					}
					label := name + " 未安装"
					if ok {
						label = name + " 已安装"
					}
					if ok {
						softChip(c, label)
					} else {
						dangerChip(c, label)
					}
				}
			})
			if cmd := a.asrEnv.InstallCommand; cmd != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, cmd).Font("monospace").FontSize(10).TextColor(textSoft(c)).Grow(1).MaxLines(1)
					quietButton(c, "复制命令").FontSize(12).OnClick(func() { a.copyText(cmd) })
				})
			}
		}
		ui.Row(c).Gap(8).Children(func() {
			quietButton(c, "一键安装本地转写工具").FontSize(12).OnClick(func() {
				a.runAsync("已开始安装本地转写工具", a.client.installASR)
			})
		})
	})
}

func (a *App) settingsSection(c *ui.Context, title, desc string, fn func()) {
	t := c.Theme()
	ui.Column(c).Children(func() {
		cardSurface(c, 14).Children(func() {
			ui.Column(c).Padding(18, 20, 14, 20).Gap(3).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, title).FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
				if desc != "" {
					ui.Text(c, desc).FontSize(11.5).TextColor(textSoft(c)).LineHeight(1.55).MaxWidth(720)
				}
			})
			ui.Column(c).Padding(18, 20, 20, 20).Gap(14).Children(fn)
		})
	})
}

func defaultLogPath() string {
	if p := os.Getenv("GRABBY_LOG_FILE"); p != "" {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		// Matches main_desktop.go's logging.SetupFileLogging path.
		return filepath.Join(home, "Library", "Logs", "Grabby", "desktop.log")
	}
	return ""
}

func (a *App) exportConfig() {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.win,
		Title:       "导出配置备份",
		DefaultPath: fmt.Sprintf("grabby-config-%s.json", time.Now().Format("20060102-150405")),
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return
	}
	raw, err := a.client.configExport()
	if err != nil {
		a.notifyErr(err.Error())
		return
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		a.notifyErr("写入失败: " + err.Error())
		return
	}
	a.notify("已导出到 " + path)
}

func (a *App) pickImportConfig() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:  a.win,
		Title:   "导入配置备份",
		Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		a.notifyErr("读取失败: " + err.Error())
		return
	}
	if len(raw) > 16<<20 {
		a.notifyErr("备份文件过大（上限 16 MB）")
		return
	}
	a.pendingImport = raw
	a.ask("导入配置备份", "导入会覆盖当前 AI 配置并立即生效；数据源按 ID 新增或更新，不影响已抓取内容。备份文件包含明文 API Key。", "确认导入", func() error {
		raw := a.pendingImport
		a.pendingImport = nil
		out, err := a.client.configImport(raw)
		if err != nil {
			return err
		}
		a.win.Update(func() {
			a.aiDraft = nil
			a.reloadView("settings")
			a.reloadView("ai")
			a.reloadView("sources")
		})
		_ = out
		return nil
	})
}
