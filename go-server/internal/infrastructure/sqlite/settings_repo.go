package sqlite

import (
	"database/sql"
	"encoding/json"
	"strconv"
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

const DefaultDailyPrompt = `你是 Grabby AI 日报主编。请基于以下整理的今日高分优质资讯，生成一份结构化 JSON 日报。

【今日资讯汇总（共 {{.Count}} 篇）】：
{{.FeedText}}

【输出要求】：
1. 只能返回合法 JSON 对象，不能使用 Markdown 代码块，不能输出解释文字。
2. 必须包含 title、date、editor、sections 四个顶层字段。date 必须为 "{{.Date}}"。
3. sections 必须是对象，建议包含以下 key；如果某板块没有内容，可以省略：
   - headline：今日头条 / 要闻，挑选 2-3 篇最重磅内容。
   - tech_ai：前沿探索，汇总科技与 AI 相关资讯。
   - finance：财经与宏观。
   - world_society：国际与社会。
   - dashboard：今日数据看板。
4. 每个 section 必须包含 title 和 items。items 必须是数组。
5. 每条资讯 item 必须包含 title、summary；建议包含 source、link、score、comment。link 使用原文 URL，不要使用 Markdown 链接语法。
6. dashboard 的 items 也使用对象数组，例如 title 为“今日共抓取资讯”，summary 为“{{.TotalItems}} 条”；“优质内容入选”，summary 为“{{.QualityItems}} 条”。
7. 语言风格保持专业、洞察深刻、客观简练。

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
          "comment": "推荐理由或专业点评"
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

	// 9. Provider profiles
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
	} else if s.ActiveProfileID == "default" {
		for i := range s.Profiles {
			if s.Profiles[i].ID == "default" {
				s.Profiles[i].Provider = s.Provider
				s.Profiles[i].APIKey = s.APIKey
				s.Profiles[i].Model = s.Model
				s.Profiles[i].BaseURL = s.BaseURL
				break
			}
		}
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

	return s, nil
}
