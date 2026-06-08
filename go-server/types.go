package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------- WebSocket Messages ----------

// BrowserRequest is sent from server to browser extension via WebSocket.
type BrowserRequest struct {
	Type      string `json:"type,omitempty"`
	Source    string `json:"source,omitempty"`
	Action    string `json:"action,omitempty"`
	Command   string `json:"command"`
	URL       string `json:"url"`
	FullPage  bool   `json:"fullPage,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// BrowserResponse is returned by the browser extension.
type BrowserResponse struct {
	Type      string     `json:"type,omitempty"`
	MessageID string     `json:"message_id,omitempty"`
	Command   string     `json:"command,omitempty"`
	Success   bool       `json:"success,omitempty"`
	Error     string     `json:"error,omitempty"`
	Result    PageResult `json:"result,omitempty"`
}

// PageResult is the nested result data from the browser.
type PageResult struct {
	URL       string      `json:"url"`
	Title     string      `json:"title"`
	Timestamp string      `json:"timestamp"`
	Content   PageContent `json:"content"`
	ImageData string      `json:"imageData"`
	Format    string      `json:"format"`
	Quality   int         `json:"quality"`
}

// PageContent is the extracted page content (Markdown from defuddle).
type PageContent struct {
	Title      string `json:"title"`
	Content    string `json:"content"`  // Markdown
	Markdown   string `json:"markdown"` // Redundant field for clarity
	Author     string `json:"author"`
	Published  string `json:"published"`
	Site       string `json:"site"`
	Language   string `json:"language"`
	WordCount  int    `json:"wordCount"`
	Image      string `json:"image"`
	Favicon    string `json:"favicon"`
	Domain     string `json:"domain"`
	HTML       string `json:"html"`
	TextLength int    `json:"textLength"`
}

// Markdown returns the markdown content, preferring the Content field.
func (pc PageContent) MarkdownContent() string {
	if pc.Content != "" {
		return pc.Content
	}
	return pc.Markdown
}

// ---------- HTTP API Types ----------

// ExtractAPIRequest is the POST /api/extract request body.
type ExtractAPIRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// BrowserRegisterRequest is the POST /api/browsers/register request body.
type BrowserRegisterRequest struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

// BrowserRegisterResponse is the POST /api/browsers/register response body.
type BrowserRegisterResponse struct {
	Success bool                `json:"success"`
	Browser BrowserRegistration `json:"browser"`
}

// ExtractAPIResponse is the POST /api/extract response body.
type ExtractAPIResponse struct {
	Success  bool   `json:"success"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

// ScreenshotAPIRequest is the POST /api/screenshot request body.
type ScreenshotAPIRequest struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ScreenshotAPIResponse is the POST /api/screenshot response body.
type ScreenshotAPIResponse struct {
	Success   bool   `json:"success"`
	URL       string `json:"url"`
	ImageData string `json:"imageData"`
}

// HealthResponse is the GET /api/health response body.
type HealthResponse struct {
	Status           string    `json:"status"`
	BrowserConnected bool      `json:"browser_connected"`
	Timestamp        time.Time `json:"timestamp"`
}

// ---------- MCP Tool Parameter Types ----------

// ScreenshotParams for the "screenshot" MCP tool.
type ScreenshotParams struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ExtractParams for the "extract" MCP tool.
type ExtractParams struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// BrowserListResponse is the GET /api/browsers response body.
type BrowserListResponse struct {
	Browsers []BrowserInfo `json:"browsers"`
	Count    int           `json:"count"`
}

// AddParams for the "add" MCP tool.
type AddParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// parseArgs converts raw MCP arguments (any/map) into a typed struct.
func parseArgs[T any](raw any) (T, error) {
	var result T
	b, err := json.Marshal(raw)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return result, err
	}
	return result, nil
}

// Source represents the configuration of a data source.
type Source struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"` // "api", "rss", "web_scrape"
	URL             string     `json:"url"`
	Schedule        string     `json:"schedule"`
	Enabled         int        `json:"enabled"` // 1-enabled, 0-disabled
	DefaultCategory string     `json:"default_category"`
	Config          string     `json:"config"`
	LastETag        *string    `json:"last_etag"`
	LastModified    *string    `json:"last_modified"`
	LastFetchAt     *time.Time `json:"last_fetch_at"`
	LastFetchStatus *string    `json:"last_fetch_status"`
	Category        string     `json:"category"` // topic category: e.g. "AI", "科技新闻"
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ScrapedItem represents a single scraped content item.
type ScrapedItem struct {
	ID             int64      `json:"id"`
	SourceID       string     `json:"source_id"`
	OriginSource   string     `json:"origin_source"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	Summary        string     `json:"summary"`
	Content        string     `json:"content"`
	Category       string     `json:"category"`
	SourceCategory string     `json:"source_category"`
	PublishedAt    *time.Time `json:"published_at"`
	FetchedAt      time.Time  `json:"fetched_at"`
	ReadStatus     int        `json:"read_status"` // 0-unread, 1-read, 2-read later
	Starred        int        `json:"starred"`     // 0-unstarred, 1-starred
	Tags           string     `json:"tags"`
}

// FetchLog represents the history of a scrape execution.
type FetchLog struct {
	ID           int64      `json:"id"`
	SourceID     string     `json:"source_id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"` // "success", "partial", "error", "skipped"
	ItemsFound   int        `json:"items_found"`
	ItemsAdded   int        `json:"items_added"`
	ErrorMessage string     `json:"error_message"`
}

// SourceForm represents the form data submitted to create/update a Source.
type SourceForm struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Schedule        string `json:"schedule"`
	DefaultCategory string `json:"default_category"`
	Config          string `json:"config"`
	Category        string `json:"category"` // topic category: e.g. "AI", "科技新闻"
}

// AIAnalysis represents the AI analysis details of a scraped item.
type AIAnalysis struct {
	ID            int64     `json:"id"`
	ItemID        int64     `json:"item_id"`
	AICategory    string    `json:"ai_category"`
	AISubcategory string    `json:"ai_subcategory"`
	QualityScore  int       `json:"quality_score"`
	AISummary     string    `json:"ai_summary"`
	AIComment     string    `json:"ai_comment"`
	AITags        string    `json:"ai_tags"` // comma-separated tags
	ModelUsed     string    `json:"model_used"`
	ProcessedAt   time.Time `json:"processed_at"`
}

// AIDailyReport represents the generated daily report.
type AIDailyReport struct {
	ID                int64     `json:"id"`
	ReportDate        string    `json:"report_date"` // YYYY-MM-DD
	ReportType        string    `json:"report_type"` // "morning", "evening", "daily"
	Title             string    `json:"title"`
	Content           string    `json:"content"` // Markdown
	TotalItems        int       `json:"total_items"`
	QualityItems      int       `json:"quality_items"`
	CategoriesSummary string    `json:"categories_summary"` // JSON string of category counts
	ModelUsed         string    `json:"model_used"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// ScrapedItemWithAI represents a scraped item along with its AI analysis details.
type ScrapedItemWithAI struct {
	ScrapedItem
	AICategory    string     `json:"ai_category"`
	AISubcategory string     `json:"ai_subcategory"`
	QualityScore  int        `json:"quality_score"`
	AISummary     string     `json:"ai_summary"`
	AIComment     string     `json:"ai_comment"`
	AITags        string     `json:"ai_tags"`
	AIModelUsed   string     `json:"ai_model_used"`
	AIProcessedAt *time.Time `json:"ai_processed_at"`
}

// AICategoryStat represents statistics for an AI category.
type AICategoryStat struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	AvgScore float64 `json:"avg_score"`
}

// AISettings represents the configuration of the AI engine.
type AISettings struct {
	Enabled              bool                `json:"enabled"`
	Provider             string              `json:"provider"` // "openai", "gemini", "custom", etc.
	APIKey               string              `json:"api_key"`
	Model                string              `json:"model"`
	BaseURL              string              `json:"base_url"`
	QualityThreshold     int                 `json:"quality_threshold"`
	SystemPrompt         string              `json:"system_prompt"`
	DailyPrompt          string              `json:"daily_prompt"`
	Strategy             string              `json:"strategy"` // "single" (default), "round-robin", "failover"
	ActiveProfileID      string              `json:"active_profile_id"`
	Profiles             []AIProviderProfile `json:"profiles"`
	MorningReportTime    string              `json:"morning_report_time"`
	EveningReportTime    string              `json:"evening_report_time"`
	MorningReportEnabled bool                `json:"morning_report_enabled"`
	EveningReportEnabled bool                `json:"evening_report_enabled"`
}

// AIProviderProfile stores one selectable AI provider connection profile.
type AIProviderProfile struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Provider           string `json:"provider"`
	APIKey             string `json:"api_key"`
	Model              string `json:"model"`
	BaseURL            string `json:"base_url"`
	Disabled           bool   `json:"disabled"`             // skip this profile in multi-profile strategies
	Priority           int    `json:"priority"`             // lower = higher priority (for failover ordering)
	RequestsPerMinute  int    `json:"requests_per_minute"`  // per-profile rate limit; 0 = default (10/min)
}

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

const DefaultDailyPrompt = `你是 Grabby AI 日报主编。请基于以下整理的今日高分优质资讯，撰写一份排版精美、结构清晰的 Markdown 格式日报。

【今日资讯汇总（共 {{.Count}} 篇）】：
{{.FeedText}}

【写作要求】：
1. 必须使用 Markdown 格式，包含各板块标题。
2. 日报结构应包含：
   - 📰 **今日头条 / 要闻**：挑选 2-3 篇最重磅（评分最高）的内容进行详细介绍和专业点评。
   - 🔬 **前沿探索 (科技与 AI)**：汇总所有科技、AI 相关的资讯。
   - 💰 **财经与宏观**：汇总所有财经相关的资讯。
   - 🌐 **国际与社会**：汇总其他分类（国际、社会、体育等）相关的资讯。
   - 📊 **今日数据看板**：以简洁的列表形式列出：今日共抓取 {{.TotalItems}} 条资讯，其中有 {{.QualityItems}} 条优质内容入选日报。
3. 请为每条资讯保留指向原文的 Markdown 链接（例如 [阅读原文](链接)）。
4. 语言风格应保持专业、洞察深刻、客观简练。如果没有某个板块的内容，可以直接忽略该板块。`

// NormalizeAISettings cleans up user inputs (trimming whitespace) and handles
// specific model name normalization for custom OpenAI-compatible endpoints.
func NormalizeAISettings(settings AISettings) AISettings {
	settings.Provider = strings.TrimSpace(settings.Provider)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.SystemPrompt = strings.TrimSpace(settings.SystemPrompt)
	settings.DailyPrompt = strings.TrimSpace(settings.DailyPrompt)
	settings.ActiveProfileID = strings.TrimSpace(settings.ActiveProfileID)
	settings.Strategy = strings.TrimSpace(settings.Strategy)
	if settings.Strategy == "" {
		settings.Strategy = "single"
	}
	settings.MorningReportTime = strings.TrimSpace(settings.MorningReportTime)
	if settings.MorningReportTime == "" {
		settings.MorningReportTime = "08:30"
	}
	settings.EveningReportTime = strings.TrimSpace(settings.EveningReportTime)
	if settings.EveningReportTime == "" {
		settings.EveningReportTime = "22:30"
	}

	for i := range settings.Profiles {
		settings.Profiles[i].ID = strings.TrimSpace(settings.Profiles[i].ID)
		settings.Profiles[i].Name = strings.TrimSpace(settings.Profiles[i].Name)
		settings.Profiles[i].Provider = strings.TrimSpace(settings.Profiles[i].Provider)
		settings.Profiles[i].Model = strings.TrimSpace(settings.Profiles[i].Model)
		settings.Profiles[i].BaseURL = strings.TrimSpace(settings.Profiles[i].BaseURL)
		settings.Profiles[i].APIKey = strings.TrimSpace(settings.Profiles[i].APIKey)
		if settings.Profiles[i].ID == "" {
			settings.Profiles[i].ID = fmt.Sprintf("profile-%d", i+1)
		}
		if settings.Profiles[i].Name == "" {
			settings.Profiles[i].Name = settings.Profiles[i].Provider
		}
		// Assign default priority if unset (0 means unset, assign i+1)
		if settings.Profiles[i].Priority <= 0 {
			settings.Profiles[i].Priority = i + 1
		}
		// Assign default requests_per_minute if unset (0 means use default 10)
		if settings.Profiles[i].RequestsPerMinute <= 0 {
			settings.Profiles[i].RequestsPerMinute = 10
		}
		if strings.ToLower(settings.Profiles[i].Provider) == "custom" && settings.Profiles[i].Model != "" {
			if !strings.HasPrefix(strings.ToLower(settings.Profiles[i].Model), "custom/") {
				settings.Profiles[i].Model = "custom/" + settings.Profiles[i].Model
			}
		}
	}

	if len(settings.Profiles) > 0 {
		if settings.ActiveProfileID == "" {
			settings.ActiveProfileID = settings.Profiles[0].ID
		}
		for _, profile := range settings.Profiles {
			if profile.ID == settings.ActiveProfileID {
				settings.Provider = profile.Provider
				settings.Model = profile.Model
				settings.BaseURL = profile.BaseURL
				settings.APIKey = profile.APIKey
				break
			}
		}
	}

	if strings.ToLower(settings.Provider) == "custom" && settings.Model != "" {
		// If model does not start with "custom/", prepend it
		if !strings.HasPrefix(strings.ToLower(settings.Model), "custom/") {
			settings.Model = "custom/" + settings.Model
		}
	}
	return settings
}
