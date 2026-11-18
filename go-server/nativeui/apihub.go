package nativeui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/egoist/mygo/ui"
)

type endpointInfo struct {
	Method string
	Path   string
	Desc   string
}

var publicEndpoints = []endpointInfo{
	{"GET", "/open/api/health", "服务与浏览器连接状态"},
	{"GET", "/open/api/stats", "归档、未读、收藏统计"},
	{"GET", "/open/api/items", "内容列表（limit / cursor / q / starred）"},
	{"GET", "/open/api/items/:id", "单条内容与渲染后的正文"},
	{"GET", "/open/api/feed", "统一多平台 Feed（新闻 / YouTube / 播客）"},
	{"GET", "/open/api/ai/items", "按质量分筛选 AI 优质内容"},
	{"GET", "/open/api/ai/daily", "按日期与类型读取 AI 日报"},
	{"GET", "/open/api/ai/daily/rss", "日报 RSS 订阅"},
}

var adminEndpoints = []endpointInfo{
	{"POST", "/api/extract", "在真实浏览器里提取网页为 Markdown"},
	{"POST", "/api/screenshot", "页面截图"},
	{"POST", "/api/twitter/search", "X 搜索"},
	{"POST", "/api/reddit/subreddit", "Reddit 板块"},
	{"POST", "/api/xiaohongshu/search", "小红书搜索"},
	{"GET", "/api/sources", "订阅源列表"},
	{"POST", "/api/sources/:id/run", "立即抓取一个订阅源"},
	{"POST", "/api/ai/settings", "保存 AI 配置"},
}

func (a *App) apiHub(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 30).Gap(16).Children(func() {
			a.errorBanner(c)
			t := c.Theme()
			base := fmt.Sprintf("http://localhost:%d", a.port())

			auth := "免密"
			if a.desktop.TokenEnabled {
				auth = "Token"
			}
			statsBar(c,
				statItem{Value: base, Label: "本地 API", Size: 17},
				statItem{Value: fmt.Sprint(a.port()), Label: "端口"},
				statItem{Value: auth, Label: "鉴权"},
				statItem{Value: fmt.Sprint(len(a.browsers)), Label: "浏览器"},
			)

			if a.desktop.PortShifted {
				ui.Row(c).Padding(10, 14).Background(t.Warning.Alpha(0.1)).Radius(10).Children(func() {
					ui.Text(c, "默认端口被占用，已自动切换到 "+fmt.Sprint(a.port())+"。").FontSize(11).TextColor(t.Warning)
				})
			}

			a.tokenPanel(c, base)

			ui.Text(c, "公开端点（无需 Token）").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
			a.endpointTable(c, base, publicEndpoints)

			ui.Text(c, "管理端点（本机免密，远程需 Token）").FontSize(17).FontWeight(650).LetterSpacing(-0.3)
			a.endpointTable(c, base, adminEndpoints)

			a.mcpPanel(c, base)
			a.quickStart(c, base)
		})
	})
}

func (a *App) tokenPanel(c *ui.Context, base string) {
	t := c.Theme()
	ui.Column(c).Padding(18, 20).Background(t.Surface).Border(1, t.Border).Radius(14).Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "API Token").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2).Grow(1)
			external := a.externalDraft
			if ui.Switch(c, &external).Changed() {
				a.externalDraft = external
			}
			ui.Text(c, "开放局域网").FontSize(12).TextColor(textSoft(c))
		})
		hint(c, "默认只监听 127.0.0.1，且本机请求始终免密。需要给局域网其他设备调用时，开启「开放局域网」并设置 Token；保存后部分设置需要重启 APP。")
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "Token").FontSize(11).TextColor(textSoft(c))
				tok := a.tokenDraft
				if !a.tokenReveal {
					tok = maskToken(tok)
				}
				ui.TextInput(c, &tok).Font("monospace").ReadOnly(!a.tokenReveal)
				if a.tokenReveal {
					a.tokenDraft = tok
				}
			})
			if iconButton(c, iconCopy, "复制 Token").Clicked() {
				a.copyText(a.tokenDraft)
			}
			label := "显示"
			if a.tokenReveal {
				label = "隐藏"
			}
			quietButton(c, label).FontSize(11).OnClick(func() { a.tokenReveal = !a.tokenReveal })
			quietButton(c, "生成").FontSize(11).OnClick(func() {
				a.tokenDraft = randomToken()
				a.tokenReveal = true
			})
			if a.tokenDraft != "" {
				quietButton(c, "清除").FontSize(11).OnClick(func() {
					a.tokenDraft = ""
					a.externalDraft = false
					a.tokenReveal = false
				})
			}
			save := primaryButton(c, "保存", a.settingsBusy || (a.externalDraft && a.tokenDraft == ""))
			if save.Clicked() && !a.settingsBusy {
				a.saveDesktop()
			}
		})
		if a.externalDraft && a.tokenDraft == "" {
			ui.Text(c, "开放局域网访问前必须设置 Token。").FontSize(10.5).TextColor(t.Danger)
		}
		if a.desktop.RestartRequired {
			ui.Text(c, "设置已保存，重启 APP 后生效。").FontSize(10.5).TextColor(t.Warning)
		}
	})
}

func maskToken(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••••••"
	}
	return s[:4] + "••••••••" + s[len(s)-4:]
}

func randomToken() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func (a *App) saveDesktop() {
	token := a.tokenDraft
	external := a.externalDraft
	a.settingsBusy = true
	a.runAsync("", func() error {
		d, err := a.client.saveDesktopSettings(external, token)
		if err != nil {
			return err
		}
		a.win.Update(func() {
			a.settingsBusy = false
			a.desktop = d
			a.tokenDraft = d.APIToken
			a.externalDraft = d.ExternalEnabled
			a.toasts = append(a.toasts, toastMsg{Text: "桌面设置已保存"})
		})
		return nil
	})
}

func (a *App) endpointTable(c *ui.Context, base string, list []endpointInfo) {
	t := c.Theme()
	flatPanel(c, func() {
		for i, ep := range list {
			if i > 0 {
				ui.Divider(c)
			}
			ep := ep
			row := ui.Row(c).Padding(10, 15).Gap(12).AlignItems(ui.Center)
			hovered := row.Hovered()
			row.Children(func() {
				methodColor := textFaint(c)
				if ep.Method != "GET" {
					methodColor = t.Accent
				}
				ui.Text(c, ep.Method).FontSize(10.5).FontWeight(700).TextColor(methodColor).Width(42).Font("monospace")
				ui.Text(c, ep.Path).FontSize(11).Font("monospace").Width(260).Shrink(0)
				ui.Text(c, ep.Desc).FontSize(11.5).TextColor(textSoft(c)).Grow(1).MaxLines(1)
				copy := iconButton(c, iconCopy, "复制 "+ep.Path)
				if !hovered {
					copy.Opacity(0.45)
				}
				if copy.Clicked() {
					a.copyText(base + ep.Path)
				}
			})
		}
	})
}

func (a *App) mcpPanel(c *ui.Context, base string) {
	t := c.Theme()
	ui.Column(c).Padding(18, 20).Background(t.Surface).Border(1, t.Border).Radius(14).Gap(12).Children(func() {
		ui.Text(c, "MCP 接入").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
		hint(c, "同一进程也提供 MCP，Claude Code / Codex 等客户端可以直接把 Grabby 当作浏览器自动化与内容库工具。")
		keyValue(c, "Streamable", base+"/mcp")
		keyValue(c, "SSE", base+"/mcp/sse")
		keyValue(c, "鉴权", "远程访问时通过 Authorization: Bearer <API Token>")
	})
}

func (a *App) quickStart(c *ui.Context, base string) {
	t := c.Theme()
	sample := "curl -s " + base + "/open/api/health\n\n" +
		"curl -s -X POST " + base + "/api/extract \\\n" +
		"  -H 'Content-Type: application/json' \\\n" +
		"  -d '{\"url\":\"https://example.com/article\"}'"
	if a.desktop.TokenEnabled {
		sample = "curl -s " + base + "/open/api/health\n\n" +
			"curl -s -X POST " + base + "/api/extract \\\n" +
			"  -H 'Authorization: Bearer $GRABBY_API_TOKEN' \\\n" +
			"  -H 'Content-Type: application/json' \\\n" +
			"  -d '{\"url\":\"https://example.com/article\"}'"
	}
	ui.Column(c).Padding(18, 20).Background(t.Surface).Border(1, t.Border).Radius(14).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "快速开始").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2).Grow(1)
			quietButton(c, "复制").FontSize(11).OnClick(func() { a.copyText(sample) })
			quietButton(c, "打开数据目录").FontSize(11).OnClick(func() {
				if a.desktop.DataDir != "" {
					_ = openPath(a.desktop.DataDir)
				}
			})
		})
		ui.Text(c, sample).Font("monospace").FontSize(11).Selectable()
	})
}
