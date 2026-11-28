package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Default prompts (will be moved to domain/config later)
const DefaultSystemPrompt = `你是一位资深内容 analysis 和筛选专家。请对以下资讯进行深度分析：

【标题】：{{.Title}}
【来源】：{{.OriginSource}}
【摘要】：{{.Summary}}
【正文部分】：
{{.Content}}

请分析该条内容，并按照要求返回结构化 JSON 结果。包含以下字段：
1. category: 必须从以下核心分类中选择一个最合适的：科技、AI、财经、国际、国内、社会、娱乐、体育、教育、健康、其他。
2. subcategory: 更具体的二级细分分类，如 "AI/大模型"、"财经/股市"、"科技/半导体" 等。
3. quality_score: 1到10之间的整数评分。
   评分标准：
   - 9-10分：极其重大的行业突破、独家首发报道、极高价值的深度行业分析或评论。
   - 7-8分：高质量行业动态、有独到见解的专家观点或详实的新闻报道。
   - 5-6分：一般性的常规新闻报道、普通的行业进展。
   - 3-4分：质量较低的报道、软文性质较重、内容空洞或重复度高。
   - 1-2分：明显的广告推销、标题党、低俗八卦或垃圾内容。
4. summary: 重新用1-2句话高度精炼这篇内容的中文摘要（不超过100字），比原文更精炼。
5. comment: 对这篇内容的评价。如果分数 >= 7，解释为什么推荐，它的价值或亮点好在哪里；如果分数 < 5，指出内容有何不足或为什么分数较低。
6. tags: 提取3个相关的关键词标签。`

const DefaultDailyPrompt = `你是 Grabby AI 日报主编。你的职责不是把今天的高分资讯都列出来，而是像真正的编辑一样判断：哪些内容是读者今天「不能错过」的。其余内容宁可舍弃，也不要为了填满版面塞进日报。

【今日候选资讯（共 {{.Count}} 篇，已按初筛评分排序）】：
{{.FeedText}}

【入选标准】只收录同时满足以下条件的内容：
1. 影响面：可能改变行业格局、政策走向、市场预期或大量用户日常的内容。
2. 变化性：今天出现了实质性新进展（发布、落地、披露、突发、明确表态），而不是旧闻回顾或例行更新。
3. 信息增量：提供一手信息、独家消息、关键数据或可验证的深度分析；纯观点、复述和标题党不算。
4. 决策价值：读者据此需要调整判断或行动的，优先收录。
5. 稀缺性：同一事件只保留信息最完整、最权威的一条，其余重复报道全部舍弃。

【必须舍弃】：例行产品更新、促销与软文、无信息增量的观点、重复报道、与当天无关的旧闻、只靠标题吸引点击的内容。

候选中的「发布时间」用于判断新鲜度：优先收录当天发生的进展，旧闻即使评分高也要舍弃。

【数量与结构】：
- 正文条目总数（不含 dashboard）上限 20 条，建议 12-18 条。20 条是硬上限，绝对不能超过。
- 宁缺毋滥：如果没有足够多的必读内容，可以少于 12 条，不要为了凑数降低标准；但也不要输出空日报，至少保留 3 条当天相对最重要的内容。
- headline（今日头条）只放 3-5 条当天最重要的内容，并按重要性从高到低排序。
- 其他板块按实际内容划分（如 tech_ai、finance、world_society、other）；每个板块有多少值得收录的就放多少，不要为了凑板块硬塞。
- 除 dashboard 外，板块总数不超过 4 个。

【输出要求】：
1. 只能返回合法 JSON 对象，不能使用 Markdown 代码块，不能输出解释文字。
2. 必须包含 title、date、editor、sections 四个顶层字段。date 必须为 "{{.Date}}"。
3. 每个 section 必须包含 title 和 items；items 必须是数组。
4. 每条资讯 item 必须包含 title、summary；建议包含 source、link、score、comment。link 使用原文 URL，不要使用 Markdown 链接语法。comment 要直接说明「为什么今天不能错过」，不要写空泛的推荐语。
5. 必须包含一个 dashboard 模块，用于展示统计数据看板。dashboard 的 items 至少包含：“今日共抓取资讯”（值为 "{{.TotalItems}} 条"）和“今日优质候选”（值为 "{{.QualityItems}} 条"）。
6. 语言风格专业、克制、有洞察，不要使用夸张的营销词汇。

【JSON 结构示例】：
{
  "title": "Grabby AI 智能日报 · {{.Date}}",
  "date": "{{.Date}}",
  "editor": "Grabby AI 日报主编",
  "sections": {
    "headline": {
      "title": "📰 今日头条 / 要闻",
      "items": [
        {
          "title": "资讯标题",
          "summary": "1-2 句话摘要",
          "source": "来源",
          "link": "https://example.com/article",
          "score": "9/10",
          "comment": "为什么这条今天不能错过"
        }
      ]
    }
  }
}`

// --- Settings CRUD ---

func (d *Database) GetSetting(key string, defaultVal string) (string, error) {
	var val string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return defaultVal, nil
	}
	if err != nil {
		return defaultVal, err
	}
	return val, nil
}

func (d *Database) SaveSetting(key string, value string) error {
	_, err := d.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

const (
	// dailyPromptVersionKey records which generation of the built-in daily
	// prompt has been applied. It lets LoadAISettings upgrade our own defaults
	// without ever overwriting a prompt the user wrote.
	dailyPromptVersionKey = "ai_daily_prompt_version"
	dailyPromptVersion    = "2"
)

// legacyDailyPromptMarkers identify prompts shipped in earlier versions.
// Matching phrases rather than whole texts keeps the check stable when
// unrelated formatting changes.
var legacyDailyPromptMarkers = []string{
	"Markdown 格式日报",   // earliest version: no item budget, listed every candidate
	"挑选 2-3 篇最重磅",     // same version, described in prose
	"汇总所有科技、AI 相关的资讯", // same version, per-section catch-all wording
	"如果某板块没有内容，可以省略",  // a later default that still had no item budget
	"12 条到 20 条",      // forced every section to 3-4 items, padding the report
}

// upgradeDailyPrompt moves a stored built-in prompt to the current default.
// A prompt the user edited is preserved: only strings still recognisable as
// one of our shipped defaults are replaced.
func (d *Database) upgradeDailyPrompt(prompt *string) error {
	version, _ := d.GetSetting(dailyPromptVersionKey, "")
	if version == dailyPromptVersion {
		return nil
	}
	stored := strings.TrimSpace(*prompt)
	if stored == "" || isLegacyBuiltInDailyPrompt(stored) {
		*prompt = DefaultDailyPrompt
		if err := d.SaveSetting("ai_daily_prompt", DefaultDailyPrompt); err != nil {
			return err
		}
	}
	return d.SaveSetting(dailyPromptVersionKey, dailyPromptVersion)
}

// isLegacyBuiltInDailyPrompt reports whether prompt is one of our old defaults
// rather than text the user wrote.
func isLegacyBuiltInDailyPrompt(prompt string) bool {
	if !strings.Contains(prompt, "Grabby AI 日报主编") {
		return false
	}
	for _, marker := range legacyDailyPromptMarkers {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

// profileIDExists reports whether a profile with the given id is present.
func profileIDExists(profiles []AIProviderProfile, id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

// ensureProfileIDs fills missing ids and de-duplicates them before rows are
// written to the normalized tables (which use the id as a primary key).
func ensureProfileIDs(profiles []AIProviderProfile) {
	seen := make(map[string]struct{}, len(profiles))
	for i := range profiles {
		id := strings.TrimSpace(profiles[i].ID)
		if id == "" {
			id = fmt.Sprintf("profile-%d", i+1)
		}
		base := id
		for suffix := 2; ; suffix++ {
			if _, exists := seen[id]; !exists {
				break
			}
			id = fmt.Sprintf("%s-%d", base, suffix)
		}
		profiles[i].ID = id
		seen[id] = struct{}{}
		if profiles[i].Priority <= 0 {
			profiles[i].Priority = i + 1
		}
		if profiles[i].RequestsPerMinute <= 0 {
			profiles[i].RequestsPerMinute = 10
		}
	}
}

func (d *Database) LoadAISettings(env AISettings) (AISettings, error) {
	var s AISettings
	var err error

	// 1. Enabled
	enabledStr, err := d.GetSetting("ai_enabled", "")
	if err != nil {
		return s, err
	}
	if enabledStr == "" {
		s.Enabled = env.Enabled
	} else {
		s.Enabled = (enabledStr == "true" || enabledStr == "1")
	}

	// 2. Provider
	s.Provider, err = d.GetSetting("ai_provider", env.Provider)
	if err != nil {
		return s, err
	}

	// 3. APIKey
	s.APIKey, err = d.GetSetting("ai_api_key", env.APIKey)
	if err != nil {
		return s, err
	}

	// 4. Model
	s.Model, err = d.GetSetting("ai_model", env.Model)
	if err != nil {
		return s, err
	}

	// 5. BaseURL
	s.BaseURL, err = d.GetSetting("ai_base_url", env.BaseURL)
	if err != nil {
		return s, err
	}

	// 6. QualityThreshold
	thresholdStr, err := d.GetSetting("ai_quality_threshold", "")
	if err != nil {
		return s, err
	}
	if thresholdStr == "" {
		s.QualityThreshold = env.QualityThreshold
		if s.QualityThreshold == 0 {
			s.QualityThreshold = 7
		}
	} else {
		if val, err := strconv.Atoi(thresholdStr); err == nil {
			s.QualityThreshold = val
		} else {
			s.QualityThreshold = 7
		}
	}

	// 7. SystemPrompt
	s.SystemPrompt, err = d.GetSetting("ai_system_prompt", DefaultSystemPrompt)
	if err != nil {
		return s, err
	}

	// 8. DailyPrompt
	s.DailyPrompt, err = d.GetSetting("ai_daily_prompt", DefaultDailyPrompt)
	if err != nil {
		return s, err
	}
	if err := d.upgradeDailyPrompt(&s.DailyPrompt); err != nil {
		return s, err
	}

	// 9. Provider profiles. The normalized ai_connections/ai_models tables are
	// authoritative once they contain data; the legacy JSON blob is kept as a
	// mirror and is migrated on first load.
	profilesJSON, err := d.GetSetting("ai_provider_profiles", "")
	if err != nil {
		return s, err
	}
	if profilesJSON != "" {
		_ = json.Unmarshal([]byte(profilesJSON), &s.Profiles)
	}
	s.ActiveProfileID, err = d.GetSetting("ai_active_profile_id", "")
	if err != nil {
		return s, err
	}

	usingTable := false
	if hasTable, tableErr := d.HasAIConnections(); tableErr == nil && hasTable {
		if tableProfiles, err := d.LoadAIConnections(); err == nil && len(tableProfiles) > 0 {
			s.Profiles = tableProfiles
			usingTable = true
		}
	}

	if len(s.Profiles) == 0 {
		s.Profiles = []AIProviderProfile{
			{
				ID:       "default",
				Name:     "默认服务商",
				Provider: s.Provider,
				APIKey:   s.APIKey,
				Model:    s.Model,
				BaseURL:  s.BaseURL,
			},
		}
		s.ActiveProfileID = "default"
	} else if !usingTable && s.ActiveProfileID == "default" {
		for i := range s.Profiles {
			if s.Profiles[i].ID == "default" {
				s.Profiles[i].Provider = s.Provider
				s.Profiles[i].APIKey = s.APIKey
				s.Profiles[i].BaseURL = s.BaseURL
				// Models is authoritative for multi-model profiles. Only adopt
				// the legacy single-model setting when the profile has none.
				if len(s.Profiles[i].Models) == 0 {
					s.Profiles[i].Model = s.Model
				}
				break
			}
		}
	}
	if len(s.Profiles) > 0 && !profileIDExists(s.Profiles, s.ActiveProfileID) {
		s.ActiveProfileID = s.Profiles[0].ID
	}

	// Migrate the effective profile set (including the generated default) into
	// the normalized tables on first load, then load the persisted strategy.
	if !usingTable && len(s.Profiles) > 0 {
		ensureProfileIDs(s.Profiles)
		if err := d.ReplaceAIConnections(s.Profiles); err != nil {
			return s, err
		}
		strategy := s.Strategy
		if strategy == "" {
			strategy = "single"
		}
		if err := d.SaveAIRoute(strategy, s.Profiles); err != nil {
			return s, err
		}
	}
	if strategy, err := d.LoadAIRoute(); err == nil && strategy != "" {
		s.Strategy = strategy
	}

	// 10. Morning/Evening report settings
	morningEnabledStr, err := d.GetSetting("ai_morning_report_enabled", "")
	if err == nil && morningEnabledStr != "" {
		s.MorningReportEnabled = (morningEnabledStr == "true" || morningEnabledStr == "1")
	} else {
		s.MorningReportEnabled = env.MorningReportEnabled
	}
	eveningEnabledStr, err := d.GetSetting("ai_evening_report_enabled", "")
	if err == nil && eveningEnabledStr != "" {
		s.EveningReportEnabled = (eveningEnabledStr == "true" || eveningEnabledStr == "1")
	} else {
		s.EveningReportEnabled = env.EveningReportEnabled
	}
	s.MorningReportTime, err = d.GetSetting("ai_morning_report_time", env.MorningReportTime)
	if err != nil || s.MorningReportTime == "" {
		s.MorningReportTime = "08:30"
	}
	s.EveningReportTime, err = d.GetSetting("ai_evening_report_time", env.EveningReportTime)
	if err != nil || s.EveningReportTime == "" {
		s.EveningReportTime = "22:30"
	}

	// 10b. Reader connection models (translation / interpretation). Empty
	// values mean "follow the active analysis profile".
	s.TranslationProfileID, _ = d.GetSetting("ai_translation_profile_id", env.TranslationProfileID)
	s.TranslationModel, _ = d.GetSetting("ai_translation_model", env.TranslationModel)
	s.TranslationTargetLanguage, _ = d.GetSetting("ai_translation_target_language", env.TranslationTargetLanguage)
	s.InterpretationProfileID, _ = d.GetSetting("ai_interpretation_profile_id", env.InterpretationProfileID)
	s.InterpretationModel, _ = d.GetSetting("ai_interpretation_model", env.InterpretationModel)

	// 11. ASR (Audio/Video Speech-to-Text) settings
	s.ASRMode, _ = d.GetSetting("asr_mode", env.ASRMode)
	s.ASRProvider, _ = d.GetSetting("asr_provider", env.ASRProvider)
	s.ASRBaseURL, _ = d.GetSetting("asr_base_url", env.ASRBaseURL)
	s.ASRAPIKey, _ = d.GetSetting("asr_api_key", env.ASRAPIKey)
	s.ASRModel, _ = d.GetSetting("asr_model", env.ASRModel)
	s.ASRLocalCommand, _ = d.GetSetting("asr_local_command", env.ASRLocalCommand)

	// Normalize settings (trim spaces and fix custom model prefix)
	s = NormalizeAISettings(s)

	// Save clean/normalized settings to database
	enabledDBVal := "false"
	if s.Enabled {
		enabledDBVal = "true"
	}
	_ = d.SaveSetting("ai_enabled", enabledDBVal)
	_ = d.SaveSetting("ai_provider", s.Provider)
	_ = d.SaveSetting("ai_api_key", s.APIKey)
	_ = d.SaveSetting("ai_model", s.Model)
	_ = d.SaveSetting("ai_base_url", s.BaseURL)
	_ = d.SaveSetting("ai_quality_threshold", strconv.Itoa(s.QualityThreshold))
	_ = d.SaveSetting("ai_system_prompt", s.SystemPrompt)
	_ = d.SaveSetting("ai_daily_prompt", s.DailyPrompt)
	_ = d.SaveSetting("ai_active_profile_id", s.ActiveProfileID)
	if profilesBytes, err := json.Marshal(s.Profiles); err == nil {
		_ = d.SaveSetting("ai_provider_profiles", string(profilesBytes))
	}

	morningEnabledDB := "false"
	if s.MorningReportEnabled {
		morningEnabledDB = "true"
	}
	eveningEnabledDB := "false"
	if s.EveningReportEnabled {
		eveningEnabledDB = "true"
	}
	_ = d.SaveSetting("ai_morning_report_enabled", morningEnabledDB)
	_ = d.SaveSetting("ai_evening_report_enabled", eveningEnabledDB)
	_ = d.SaveSetting("ai_morning_report_time", s.MorningReportTime)
	_ = d.SaveSetting("ai_evening_report_time", s.EveningReportTime)
	_ = d.SaveSetting("ai_translation_profile_id", s.TranslationProfileID)
	_ = d.SaveSetting("ai_translation_model", s.TranslationModel)
	_ = d.SaveSetting("ai_translation_target_language", s.TranslationTargetLanguage)
	_ = d.SaveSetting("ai_interpretation_profile_id", s.InterpretationProfileID)
	_ = d.SaveSetting("ai_interpretation_model", s.InterpretationModel)
	_ = d.SaveSetting("asr_mode", s.ASRMode)
	_ = d.SaveSetting("asr_provider", s.ASRProvider)
	_ = d.SaveSetting("asr_base_url", s.ASRBaseURL)
	_ = d.SaveSetting("asr_api_key", s.ASRAPIKey)
	_ = d.SaveSetting("asr_model", s.ASRModel)
	_ = d.SaveSetting("asr_local_command", s.ASRLocalCommand)

	return s, nil
}
