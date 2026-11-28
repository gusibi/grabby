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
	for i := range base.Profiles {
		normalizeProfileModels(&base.Profiles[i])
	}
	a.aiDraft = &base
	a.promptSystem = base.SystemPrompt
	a.promptDaily = base.DailyPrompt
	if a.aiProfileIndex >= len(base.Profiles) {
		a.aiProfileIndex = 0
	}
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

// modelKey identifies a model regardless of the driver namespace the server
// stores ("custom/deepseek-chat" and "deepseek-chat" are the same choice).
func modelKey(model string) string {
	return strings.ToLower(displayModel(strings.TrimSpace(model)))
}

// hasProfileModel reports whether models already holds model, ignoring the
// driver namespace.
func hasProfileModel(models []string, model string) bool {
	key := modelKey(model)
	for _, m := range models {
		if modelKey(m) == key {
			return true
		}
	}
	return false
}

// normalizeProfileModels trims and de-dupes a profile's model list, and keeps
// the preferred model pointing at one of the selected models.
func normalizeProfileModels(p *ProviderProfile) {
	if p == nil {
		return
	}
	models := make([]string, 0, len(p.Models))
	seen := make(map[string]bool, len(p.Models))
	for _, raw := range p.Models {
		m := strings.TrimSpace(raw)
		if m == "" {
			continue
		}
		key := modelKey(m)
		if seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, m)
	}
	p.Models = models
	if len(models) == 0 {
		p.Model = ""
		return
	}
	if !hasProfileModel(models, p.Model) {
		p.Model = models[0]
	}
}

// toggleProfileModel adds or removes one model from a profile. The first
// selection becomes the preferred model; removing it falls back to the next.
func (a *App) toggleProfileModel(p *ProviderProfile, model string, on bool) {
	if p == nil {
		return
	}
	if on {
		if !hasProfileModel(p.Models, model) {
			p.Models = append(p.Models, strings.TrimSpace(model))
		}
	} else {
		kept := make([]string, 0, len(p.Models))
		key := modelKey(model)
		for _, m := range p.Models {
			if modelKey(m) != key {
				kept = append(kept, m)
			}
		}
		p.Models = kept
	}
	normalizeProfileModels(p)
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

// normalizeEndpoint makes base URLs comparable across the settings draft and
// the catalog snapshot.
func normalizeEndpoint(s string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(s)), "/")
}

// discoverKey scopes a provider-reported model list to the endpoint that
// returned it, so changing the base URL does not keep stale options.
func discoverKey(provider, baseURL string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "|" + normalizeEndpoint(baseURL)
}

// catalogProviderFor finds the catalog provider a draft profile points at.
// Custom endpoints (relay services and local gateways) are matched by base
// URL; the recognized providers are matched by their catalog id.
func (a *App) catalogProviderFor(p *ProviderProfile) *CatalogProvider {
	if p == nil || len(a.aiCatalog) == 0 {
		return nil
	}
	if base := normalizeEndpoint(p.BaseURL); base != "" {
		for i := range a.aiCatalog {
			if normalizeEndpoint(a.aiCatalog[i].API) == base {
				return &a.aiCatalog[i]
			}
		}
	}
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	switch provider {
	case "gemini", "googleai":
		provider = "google"
	case "claude":
		provider = "anthropic"
	case "", "custom", "lmstudio":
		return nil
	}
	for i := range a.aiCatalog {
		if strings.EqualFold(a.aiCatalog[i].ID, provider) {
			return &a.aiCatalog[i]
		}
	}
	return nil
}

// modelCandidates is the union of the profile's own models, the catalog list
// for its endpoint, and whatever the provider returned on the last fetch.
func (a *App) modelCandidates(p *ProviderProfile) []string {
	seen := make(map[string]bool, 32)
	out := make([]string, 0, 32)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		key := modelKey(id)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, id)
	}
	for _, m := range p.Models {
		add(m)
	}
	if cp := a.catalogProviderFor(p); cp != nil {
		for _, m := range cp.Models {
			add(m.ID)
		}
	}
	for _, m := range a.aiDiscovered[discoverKey(p.Provider, p.BaseURL)] {
		add(m)
	}
	return out
}

// modelPicker is the model field: a preferred-model dropdown over the chosen
// models, a checkbox per available model, and a manual ID entry as the escape
// hatch for endpoints neither the catalog nor discovery knows.
func (a *App) modelPicker(c *ui.Context, p *ProviderProfile) {
	t := c.Theme()
	candidates := a.modelCandidates(p)
	meta := map[string]CatalogModel{}
	if cp := a.catalogProviderFor(p); cp != nil {
		for _, m := range cp.Models {
			meta[modelKey(m.ID)] = m
		}
	}

	ui.Column(c).Gap(10).Children(func() {
		// Preferred model: the one used first by the single-model strategy.
		ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			if len(p.Models) > 0 {
				labels := make([]string, len(p.Models))
				idx := 0
				for i, m := range p.Models {
					labels[i] = displayModel(m)
					if modelKey(m) == modelKey(p.Model) {
						idx = i
					}
				}
				cur := labels[idx]
				ui.Text(c, "首选模型").FontSize(11).TextColor(textSoft(c))
				if ui.Select(c, &cur, labels).Width(240).Changed() {
					for i, label := range labels {
						if label == cur {
							p.Model = p.Models[i]
							break
						}
					}
				}
			}
			ui.Text(c, fmt.Sprintf("已选 %d 个", len(p.Models))).FontSize(11).TextColor(textFaint(c))
		})

		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.TextInput(c, &a.aiModelSearch).Placeholder("搜索模型").Width(180)
			busy := a.aiDiscoverBusy
			if quietButtonDisabled(c, "从服务商获取", busy).Clicked() && !busy {
				a.discoverModels()
			}
			if quietButtonDisabled(c, "刷新在线目录", a.aiCatalogBusy).Clicked() && !a.aiCatalogBusy {
				a.loadAIModelCatalog(true)
			}
			if busy || a.aiCatalogBusy {
				ui.Spinner(c)
			}
		})

		// One status line for both sources; the provider fetch wins because
		// it is the more specific answer for the current endpoint.
		msg, isErr := a.aiDiscoverMsg, a.aiDiscoverErr
		if msg == "" {
			msg, isErr = a.aiCatalogMsg, a.aiCatalogErr
		}
		ui.Row(c).Children(func() {
			if msg != "" {
				color := t.TextMuted
				if isErr {
					color = t.Danger
				}
				ui.Text(c, msg).FontSize(11).TextColor(color).MaxWidth(560).MaxLines(2)
			} else if len(candidates) == 0 {
				hint(c, "还没有可选的模型：点击「从服务商获取」，或在下方手动填写模型 ID。")
			} else {
				hint(c, "勾选要参与分析的模型；首选模型用于单模型策略。")
			}
		})

		if len(candidates) > 0 {
			query := strings.ToLower(strings.TrimSpace(a.aiModelSearch))
			matches := make([]string, 0, len(candidates))
			for _, id := range candidates {
				if query != "" && !strings.Contains(strings.ToLower(id), query) &&
					!strings.Contains(strings.ToLower(displayModel(id)), query) {
					continue
				}
				matches = append(matches, id)
			}
			const maxShown = 24
			ui.Column(c).Gap(1).MaxWidth(560).Children(func() {
				for i, id := range matches {
					if i >= maxShown {
						return
					}
					id := id
					checked := hasProfileModel(p.Models, id)
					box := ui.Checkbox(c, &checked, displayModel(id))
					if cm, ok := meta[modelKey(id)]; ok && cm.Context > 0 {
						box.Tooltip(fmt.Sprintf("%s · 上下文 %s", cm.ID, humanTokens(cm.Context)))
					}
					if box.Changed() {
						a.toggleProfileModel(p, id, checked)
					}
				}
			})
			if len(matches) > maxShown {
				hint(c, fmt.Sprintf("还有 %d 个模型，用上面的搜索缩小范围。", len(matches)-maxShown))
			} else if len(matches) == 0 {
				hint(c, "没有匹配的模型。")
			}
		}

		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			manual := a.aiManualOpen
			if ui.Checkbox(c, &manual, "手动填写模型 ID").Changed() {
				a.aiManualOpen = manual
			}
			if !a.aiManualOpen {
				return
			}
			ui.TextInput(c, &a.aiManualModel).Font("monospace").Placeholder("例如 gpt-4o-mini").Width(260)
			if quietButton(c, "添加此模型").Clicked() {
				if id := strings.TrimSpace(a.aiManualModel); id != "" {
					a.toggleProfileModel(p, id, true)
					a.aiManualModel = ""
				}
			}
		})
	})
}

// ensureAIModelCatalog loads the shared model catalog once per session. It is
// called when the model tab is shown so the first network call never blocks
// the page.
func (a *App) ensureAIModelCatalog() {
	if a.aiCatalogLoaded || a.aiCatalogTried || a.aiCatalogBusy || a.win == nil {
		return
	}
	a.loadAIModelCatalog(false)
}

func (a *App) loadAIModelCatalog(refresh bool) {
	if a.aiCatalogBusy || a.win == nil {
		return
	}
	a.aiCatalogBusy = true
	a.aiCatalogTried = true
	a.aiCatalogErr = false
	a.aiCatalogMsg = "正在加载在线模型目录…"
	go func() {
		providers, err := a.client.modelCatalog(refresh)
		a.win.Update(func() {
			a.aiCatalogBusy = false
			if err != nil {
				a.aiCatalogErr = true
				a.aiCatalogMsg = "在线模型目录不可用：" + err.Error()
				return
			}
			a.aiCatalog = providers
			a.aiCatalogLoaded = true
			total := 0
			for _, p := range providers {
				total += len(p.Models)
			}
			a.aiCatalogMsg = fmt.Sprintf("在线目录：%d 个服务商 · %d 个模型", len(providers), total)
		})
	}()
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
								normalizeProfileModels(&d.Profiles[i])
								a.aiModelSearch = ""
								a.aiDiscoverMsg = ""
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
					a.aiModelSearch = ""
					a.aiDiscoverMsg = ""
				}
			})
			ui.Field(c, "Base URL", func() {
				placeholder := "留空使用官方端点"
				if cp := a.catalogProviderFor(p); cp != nil && cp.API != "" {
					placeholder = cp.API
				}
				ui.TextInput(c, &p.BaseURL).Placeholder(placeholder).Font("monospace").MaxWidth(420)
			})
			ui.Field(c, "API Key", func() {
				ui.TextInput(c, &p.APIKey).Password().Placeholder("sk-…").MaxWidth(420)
			})
			ui.Field(c, "模型", func() {
				a.modelPicker(c, p)
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
	a.aiModelSearch = ""
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
			a.aiModelSearch = ""
		})
		return nil
	})
}

func (a *App) saveAIDraft() {
	d := a.ensureAIDraft()
	for i := range d.Profiles {
		normalizeProfileModels(&d.Profiles[i])
	}
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
	normalizeProfileModels(p)
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

// discoverModels asks the provider for its model list. The result becomes the
// checkbox candidates for the current endpoint; the selection stays untouched.
func (a *App) discoverModels() {
	p := a.currentProfile()
	if p == nil {
		a.notifyErr("请先添加一个配置档")
		return
	}
	provider, baseURL, apiKey, profileID := p.Provider, p.BaseURL, p.APIKey, p.ID
	key := discoverKey(provider, baseURL)
	a.aiDiscoverBusy = true
	a.aiDiscoverErr = false
	a.aiDiscoverMsg = "正在从服务商获取模型…"
	go func() {
		models, err := a.client.discoverModels(provider, baseURL, apiKey, profileID)
		a.win.Update(func() {
			a.aiDiscoverBusy = false
			if err != nil {
				a.aiDiscoverErr = true
				a.aiDiscoverMsg = "获取模型失败：" + err.Error()
				return
			}
			if len(models) == 0 {
				a.aiDiscoverErr = true
				a.aiDiscoverMsg = "服务商没有返回可用模型"
				return
			}
			a.aiDiscovered[key] = models
			a.aiDiscoverErr = false
			a.aiDiscoverMsg = fmt.Sprintf("服务商返回 %d 个模型，勾选后保存生效", len(models))
		})
	}()
}
