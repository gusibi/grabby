package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) initAIDraft() {
	base := AISettings{QualityThreshold: 7, Strategy: "single", ASRMode: "auto", ASRProvider: "groq", ASRBaseURL: "https://api.groq.com/openai/v1", ASRModel: "whisper-large-v3-turbo"}
	if a.aiSettings != nil {
		base = *a.aiSettings
		base.Profiles = append([]ProviderProfile(nil), a.aiSettings.Profiles...)
	}
	a.aiDraft = &base
	a.promptSystem = base.SystemPrompt
	a.promptDaily = base.DailyPrompt
	if a.aiProfileIndex >= len(base.Profiles) {
		a.aiProfileIndex = 0
	}
	a.syncModelsText()
}

func (a *App) ensureAIDraft() *AISettings {
	if a.aiDraft == nil {
		a.initAIDraft()
	}
	return a.aiDraft
}

func (a *App) currentProfile() *ProviderProfile {
	d := a.aiDraft
	if d == nil || len(d.Profiles) == 0 {
		return nil
	}
	if a.aiProfileIndex < 0 || a.aiProfileIndex >= len(d.Profiles) {
		a.aiProfileIndex = 0
	}
	return &d.Profiles[a.aiProfileIndex]
}

func (a *App) syncModelsText() {
	p := a.currentProfile()
	if p == nil {
		a.aiModelsText = ""
		return
	}
	a.aiModelsText = strings.Join(p.Models, "\n")
}

func (a *App) setProfileModels(text string) {
	p := a.currentProfile()
	if p == nil {
		return
	}
	var models []string
	for _, line := range strings.Split(text, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			models = append(models, s)
		}
	}
	p.Models = models
	if len(models) > 0 {
		p.Model = models[0]
	} else {
		p.Model = ""
	}
}

func profileTitle(p ProviderProfile, i int) string {
	name := p.Name
	if name == "" {
		name = fmt.Sprintf("配置档 %d", i+1)
	}
	if p.Provider != "" {
		return fmt.Sprintf("%s · %s", name, p.Provider)
	}
	return name
}

// readerModelsBody configures the dedicated models used by the reader's
// translate and AI-brief actions. Both may follow the analysis route.
func (a *App) readerModelsBody(c *ui.Context) {
	d := a.ensureAIDraft()
	// The AI section edits the active profile's model list as text; mirror it
	// so the pickers below offer the models the user just typed.
	a.setProfileModels(a.aiModelsText)
	ui.Column(c).Gap(14).Children(func() {
		ui.Form(c, func() {
			ui.Field(c, "翻译目标语言", func() {
				ui.TextInput(c, &d.TranslationTargetLanguage).Placeholder("简体中文").MaxWidth(240)
			})
			a.readerModelField(c, "翻译模型", d, &d.TranslationProfileID, &d.TranslationModel)
			a.readerModelField(c, "解读模型", d, &d.InterpretationProfileID, &d.InterpretationModel)
		})
		hint(c, "留空时跟随主配置里选中的配置档和首选模型；这里的模型只用于阅读页的翻译与 AI 解读，不影响自动分析。")
	})
}

func (a *App) readerModelField(c *ui.Context, label string, d *AISettings, profileID, model *string) {
	ui.Field(c, label, func() {
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			labels := []string{"跟随默认配置"}
			ids := []string{""}
			for i, p := range d.Profiles {
				labels = append(labels, profileTitle(p, i))
				ids = append(ids, p.ID)
			}
			cur := labels[0]
			profile := a.profileByID(d, *profileID)
			if profile != nil {
				for i, p := range d.Profiles {
					if p.ID == profile.ID {
						cur = labels[i+1]
						break
					}
				}
			} else {
				*profileID = ""
				*model = ""
			}
			if ui.Select(c, &cur, labels).Width(240).Changed() {
				for i, l := range labels {
					if l == cur {
						*profileID = ids[i]
						*model = ""
						break
					}
				}
				profile = a.profileByID(d, *profileID)
			}
			if profile == nil {
				return
			}
			models := profile.EffectiveModels()
			if len(models) == 0 {
				hint(c, "该配置档还没有填写模型。")
				return
			}
			modelLabels := []string{"配置档首选模型"}
			modelIDs := []string{""}
			for _, m := range models {
				modelLabels = append(modelLabels, displayModel(m))
				modelIDs = append(modelIDs, m)
			}
			curModel := "配置档首选模型"
			matchedModel := false
			for i, m := range models {
				if m == *model || (*model != "" && displayModel(m) == displayModel(*model)) {
					curModel = modelLabels[i+1]
					*model = m
					matchedModel = true
					break
				}
			}
			if !matchedModel {
				*model = ""
			}
			if ui.Select(c, &curModel, modelLabels).Width(210).Changed() {
				for i, l := range modelLabels {
					if l == curModel {
						*model = modelIDs[i]
						break
					}
				}
			}
		})
	})
}

func (a *App) profileByID(d *AISettings, id string) *ProviderProfile {
	if d == nil || id == "" {
		return nil
	}
	for i := range d.Profiles {
		if d.Profiles[i].ID == id {
			return &d.Profiles[i]
		}
	}
	return nil
}

// displayModel trims a driver namespace for the picker label.
func displayModel(model string) string {
	for _, prefix := range []string{"openai/", "anthropic/", "googleai/", "vertexai/", "ollama/", "custom/"} {
		if strings.HasPrefix(model, prefix) {
			return strings.TrimPrefix(model, prefix)
		}
	}
	return model
}

func (a *App) aiView(c *ui.Context) ui.Element {
	return ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Grow(1).Padding(24, 30).Gap(16).Children(func() {
			a.errorBanner(c)
			a.aiStatsPanel(c)
			a.aiUsagePanel(c)
			a.aiCategoriesPanel(c)

			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "AI 优质内容").FontSize(17).FontWeight(650).LetterSpacing(-0.3).Grow(1)
				ui.Text(c, "按质量评分 ≥ 7").FontSize(11.5).TextColor(textSoft(c))
			})
			a.searchBar(c, "ai")
			a.filter(c, "ai")
			items := a.currentItems()
			if len(items) == 0 {
				emptyState(c, "还没有 AI 优质内容", "开启 AI 分析后，达到质量阈值的内容会出现在这里。")
			} else {
				a.feedGrid(c, items, a.cardColumns(c))
				a.loadMoreRow(c, "ai")
			}
		})
	})
}

func (a *App) aiStatsPanel(c *ui.Context) {
	avg := "—"
	if a.aiStats.AverageScore > 0 {
		avg = fmt.Sprintf("%.1f", a.aiStats.AverageScore)
	}
	statsBar(c,
		statItem{Value: fmt.Sprint(a.aiStats.TotalProcessed), Label: "已分析"},
		statItem{Value: fmt.Sprint(a.aiStats.TotalPending), Label: "待分析"},
		statItem{Value: fmt.Sprint(a.aiStats.QueueLength), Label: "队列中"},
		statItem{Value: avg, Label: "平均分"},
	)
}

// aiSettingsBody is the model and connection editor. It lives on the Settings
// page, not the AI page, which stays a view over the analysis results.
func (a *App) aiSettingsBody(c *ui.Context) {
	t := c.Theme()
	d := a.ensureAIDraft()
	ui.Column(c).Gap(14).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			enabled := d.Enabled
			if ui.Switch(c, &enabled).Changed() {
				d.Enabled = enabled
			}
			ui.Text(c, "自动分析新内容").FontSize(12.5)
			ui.Text(c, map[bool]string{true: "已启用", false: "未启用"}[enabled]).FontSize(11).TextColor(textSoft(c))
			ui.Spacer(c).Grow(1)
		})
		hint(c, "同一条分析任务会按配置档与模型的组合轮询；失败时按策略回退。")

		if len(d.Profiles) == 0 {
			ui.Row(c).Gap(8).Children(func() {
				quietButton(c, "添加配置档").OnClick(func() { a.addProfile() })
				quietButton(c, "导入已有配置").OnClick(func() { a.importAIConfig() })
			})
			return
		}

		p := a.currentProfile()
		if p == nil {
			return
		}
		ui.Form(c, func() {
			ui.Field(c, "配置档", func() {
				ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
					labels := make([]string, len(d.Profiles))
					for i, prof := range d.Profiles {
						labels[i] = profileTitle(prof, i)
					}
					sel := labels[a.aiProfileIndex]
					if ui.Select(c, &sel, labels).Width(260).Changed() {
						for i, l := range labels {
							if l == sel {
								a.aiProfileIndex = i
								a.syncModelsText()
								break
							}
						}
					}
					if quietButton(c, "添加").Clicked() {
						a.addProfile()
					}
					if quietButton(c, "导入").Clicked() {
						a.importAIConfig()
					}
					if quietButton(c, "删除").Clicked() {
						a.removeProfile()
					}
				})
			})
			ui.Field(c, "名称", func() { ui.TextInput(c, &p.Name).MaxWidth(420) })
			ui.Field(c, "Provider", func() {
				providers := []string{"openai", "anthropic", "gemini", "custom", "ollama", "lmstudio"}
				cur := p.Provider
				if ui.Select(c, &cur, providers).MaxWidth(240).Changed() {
					p.Provider = cur
				}
			})
			ui.Field(c, "Base URL", func() {
				ui.TextInput(c, &p.BaseURL).Placeholder("留空使用官方端点").Font("monospace").MaxWidth(420)
			})
			ui.Field(c, "API Key", func() {
				ui.TextInput(c, &p.APIKey).Password().Placeholder("sk-…").MaxWidth(420)
			})
			ui.Field(c, "模型", func() {
				ui.Column(c).Gap(5).Children(func() {
					ui.TextArea(c, &a.aiModelsText).Height(86).Font("monospace").Placeholder("一行一个，例如：gpt-4o-mini").MaxWidth(560)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						busy := a.aiDiscoverBusy
						if quietButton(c, "获取模型").FontSize(11).Disabled(busy).Clicked() && !busy {
							a.discoverModels()
						}
						if busy {
							ui.Spinner(c)
						}
						if a.aiDiscoverMsg != "" {
							color := t.Success
							if a.aiDiscoverErr {
								color = t.Danger
							}
							ui.Text(c, a.aiDiscoverMsg).FontSize(11).TextColor(color).Grow(1).MaxLines(2)
						} else {
							hint(c, "从服务商拉取可用模型列表，替换上面的内容。")
						}
					})
				})
			})
			ui.Field(c, "每分钟请求", func() {
				rpm := float64(p.RequestsPerMinute)
				if numberStepper(c, &rpm, 0, 120, 1, " 次") {
					p.RequestsPerMinute = int(rpm)
				}
			})
			ui.Field(c, "禁用", func() {
				disabled := p.Disabled
				if ui.Switch(c, &disabled).Changed() {
					p.Disabled = disabled
				}
			})
			ui.Field(c, "质量阈值", func() {
				threshold := float64(d.QualityThreshold)
				if numberStepper(c, &threshold, 1, 10, 1, " 分") {
					d.QualityThreshold = int(threshold)
				}
			})
			ui.Field(c, "策略", func() {
				strategyLabels := []string{"单模型", "轮询", "故障转移"}
				strategyIDs := []string{"single", "round-robin", "failover"}
				cur := strategyLabels[0]
				for i, id := range strategyIDs {
					if d.Strategy == id {
						cur = strategyLabels[i]
						break
					}
				}
				if ui.Select(c, &cur, strategyLabels).MaxWidth(220).Changed() {
					for i, label := range strategyLabels {
						if label == cur {
							d.Strategy = strategyIDs[i]
							break
						}
					}
				}
			})
		})

		// The page has one primary save at the bottom; this section only offers
		// the connection test so there is a single commit point for the form.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			busy := a.aiBusy
			test := quietButton(c, "测试连接").Disabled(busy)
			if test.Clicked() && !busy {
				a.testAIDraft()
			}
			if a.aiTestMsg != "" {
				color := t.Success
				if a.aiTestErr {
					color = t.Danger
				}
				ui.Text(c, a.aiTestMsg).FontSize(11).TextColor(color).Grow(1).MaxLines(2)
			} else {
				hint(c, "用当前配置发一次请求，验证密钥与模型是否可用。")
			}
		})
	})
}

func (a *App) addProfile() {
	d := a.ensureAIDraft()
	p := ProviderProfile{
		ID:                fmt.Sprintf("profile-%d", len(d.Profiles)+1),
		Name:              fmt.Sprintf("配置档 %d", len(d.Profiles)+1),
		Provider:          "openai",
		RequestsPerMinute: 10,
	}
	d.Profiles = append(d.Profiles, p)
	a.aiProfileIndex = len(d.Profiles) - 1
	if d.ActiveProfileID == "" {
		d.ActiveProfileID = p.ID
	}
	a.syncModelsText()
}

func (a *App) removeProfile() {
	d := a.ensureAIDraft()
	if len(d.Profiles) == 0 {
		return
	}
	idx := a.aiProfileIndex
	removed := d.Profiles[idx]
	a.ask("删除配置档", "「"+profileTitle(removed, idx)+"」将被删除。", "删除", func() error {
		a.win.Update(func() {
			d.Profiles = append(d.Profiles[:idx], d.Profiles[idx+1:]...)
			if a.aiProfileIndex >= len(d.Profiles) {
				a.aiProfileIndex = max(0, len(d.Profiles)-1)
			}
			if d.ActiveProfileID == removed.ID {
				d.ActiveProfileID = ""
				if len(d.Profiles) > 0 {
					d.ActiveProfileID = d.Profiles[0].ID
				}
			}
			a.syncModelsText()
		})
		return nil
	})
}

func (a *App) saveAIDraft() {
	d := a.ensureAIDraft()
	a.setProfileModels(a.aiModelsText)
	d.SystemPrompt = a.promptSystem
	d.DailyPrompt = a.promptDaily
	if d.ActiveProfileID == "" && len(d.Profiles) > 0 {
		d.ActiveProfileID = d.Profiles[0].ID
	}
	cp := *d
	cp.Profiles = append([]ProviderProfile(nil), d.Profiles...)
	a.aiBusy = true
	go func() {
		err := a.client.saveAISettings(cp)
		a.win.Update(func() {
			a.aiBusy = false
			if err != nil {
				a.toasts = append(a.toasts, toastMsg{Text: err.Error(), Error: true})
				return
			}
			a.aiDraft = nil
			a.aiTestMsg = ""
			a.toasts = append(a.toasts, toastMsg{Text: "AI 设置已保存"})
			a.reloadView("ai")
			a.reloadView("settings")
		})
	}()
}

func (a *App) testAIDraft() {
	a.ensureAIDraft()
	p := a.currentProfile()
	if p == nil {
		a.notifyErr("请先添加一个配置档")
		return
	}
	a.setProfileModels(a.aiModelsText)
	prof := *p
	prompt := a.promptSystem
	a.aiBusy = true
	a.aiTestMsg = "正在测试连接…"
	a.aiTestErr = false
	go func() {
		res, err := a.client.testAI(prof, prompt)
		a.win.Update(func() {
			a.aiBusy = false
			if err != nil {
				a.aiTestErr = true
				a.aiTestMsg = "测试失败: " + err.Error()
				return
			}
			if res.Error != "" {
				a.aiTestErr = true
				a.aiTestMsg = res.Error
				return
			}
			if res.Message != "" {
				a.aiTestMsg = res.Message
			} else if res.Summary != "" {
				a.aiTestMsg = "连接成功 · " + truncate(res.Summary, 80)
			} else {
				a.aiTestMsg = "连接成功"
			}
			if !res.Success && res.Success2 {
				a.aiTestErr = false
			}
		})
	}()
}

func (a *App) importAIConfig() {
	a.runAsync("已扫描本地 AI 配置", func() error {
		var out map[string]any
		err := a.client.post("/api/ai/import", map[string]any{"source": "auto", "apply": true}, &out)
		if err != nil {
			return err
		}
		a.win.Update(func() {
			a.aiDraft = nil
			a.reloadView("ai")
			a.reloadView("settings")
		})
		return nil
	})
}

func (a *App) aiUsagePanel(c *ui.Context) {
	if a.aiUsage == nil {
		return
	}
	u := a.aiUsage
	cardSurface(c, 16).Children(func() {
		ui.Column(c).Padding(20, 22).Gap(14).Children(func() {
			ui.Text(c, "近 7 天调用").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
			// Five figures need the full width; a narrow pane shows them two
			// rows deep instead of wrapping half a label.
			figures := []struct{ value, label string }{
				{fmt.Sprint(u.TotalRequests), "请求"},
				{fmt.Sprint(u.FailedRequests), "失败"},
				{humanTokens(u.TotalTokens), "Token"},
				{fmt.Sprintf("$%.4f", u.Cost), "成本"},
				{fmt.Sprintf("%.0f ms", u.AvgLatencyMS), "平均延迟"},
			}
			if a.contentWidth(c) < 720 {
				ui.Row(c).Gap(16).Children(func() {
					for _, f := range figures[:3] {
						usageStat(c, f.value, f.label)
					}
				})
				ui.Row(c).Gap(16).Children(func() {
					for _, f := range figures[3:] {
						usageStat(c, f.value, f.label)
					}
				})
			} else {
				ui.Row(c).Gap(16).Children(func() {
					for _, f := range figures {
						usageStat(c, f.value, f.label)
					}
				})
			}
			if len(u.ByDay) > 0 {
				usageChart(c, u.ByDay)
			}
		})
	})
}

func (a *App) aiCategoriesPanel(c *ui.Context) {
	if len(a.aiCategories) == 0 {
		return
	}
	cardSurface(c, 16).Children(func() {
		ui.Column(c).Padding(20, 22).Gap(12).Children(func() {
			ui.Text(c, "AI 分类").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, cat := range a.aiCategories {
					if cat.Count == 0 {
						continue
					}
					b := softChip(c, fmt.Sprintf("%s · %d", cat.Name, cat.Count))
					cat := cat
					if a.filters["ai"].AICategory == cat.Name {
						b = accentChip(c, fmt.Sprintf("%s · %d", cat.Name, cat.Count))
					}
					b.OnClick(func() {
						if a.filters["ai"].AICategory == cat.Name {
							a.filters["ai"].AICategory = ""
						} else {
							a.filters["ai"].AICategory = cat.Name
						}
						a.reloadItems("ai")
					})
				}
			})
		})
	})
}

func usageStat(c *ui.Context, value, label string) {
	ui.Column(c).Gap(2).Grow(1).Children(func() {
		ui.Text(c, value).FontSize(17).FontWeight(650).LetterSpacing(-0.2)
		ui.Text(c, label).FontSize(11).TextColor(textSoft(c))
	})
}

func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprint(n)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// discoverModels asks the provider for its model list and fills the editor.
func (a *App) discoverModels() {
	p := a.currentProfile()
	if p == nil {
		a.notifyErr("请先添加一个配置档")
		return
	}
	provider, baseURL, apiKey, profileID := p.Provider, p.BaseURL, p.APIKey, p.ID
	a.aiDiscoverBusy = true
	a.aiDiscoverErr = false
	a.aiDiscoverMsg = "正在获取模型…"
	go func() {
		models, err := a.client.discoverModels(provider, baseURL, apiKey, profileID)
		a.win.Update(func() {
			a.aiDiscoverBusy = false
			if err != nil {
				a.aiDiscoverErr = true
				a.aiDiscoverMsg = err.Error()
				return
			}
			if len(models) == 0 {
				a.aiDiscoverErr = true
				a.aiDiscoverMsg = "服务商没有返回可用模型"
				return
			}
			a.aiModelsText = strings.Join(models, "\n")
			a.setProfileModels(a.aiModelsText)
			a.aiDiscoverMsg = fmt.Sprintf("获取到 %d 个模型，保存后生效", len(models))
		})
	}()
}
