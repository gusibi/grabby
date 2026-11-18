// Package nativeui renders Grabby's desktop interface with MyGo's native UI
// toolkit (github.com/egoist/mygo/ui) instead of a system web view.
//
// The package talks to the HTTP server that runs in the same process over
// loopback. That keeps every existing handler (auth bypass, AI, scheduler,
// browser registry) as the single source of truth while the interface itself
// is drawn natively. Rich article/report HTML is the one place where a web
// view is still used, embedded inside the native reader pane.
package nativeui

import "time"

// Item mirrors domain/item.ScrapedItem plus the AI analysis columns returned by
// /open/api/items and /open/api/ai/items.
type Item struct {
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
	ReadStatus     int        `json:"read_status"`
	Starred        int        `json:"starred"`
	Tags           string     `json:"tags"`
	Metadata       string     `json:"metadata,omitempty"`

	AICategory    string     `json:"ai_category"`
	AISubcategory string     `json:"ai_subcategory"`
	QualityScore  int        `json:"quality_score"`
	AISummary     string     `json:"ai_summary"`
	AIComment     string     `json:"ai_comment"`
	AITags        string     `json:"ai_tags"`
	AIModelUsed   string     `json:"ai_model_used"`
	AIProcessedAt *time.Time `json:"ai_processed_at"`
}

// DisplaySummary is the AI summary when there is one, else the extracted one.
func (i Item) DisplaySummary() string {
	if i.AISummary != "" {
		return i.AISummary
	}
	return i.Summary
}

// DisplaySource is the label shown above a card title.
func (i Item) DisplaySource() string {
	if i.OriginSource != "" {
		return i.OriginSource
	}
	if i.SourceCategory != "" {
		return i.SourceCategory
	}
	return "Grabby"
}

type ItemsPage struct {
	Items      []Item `json:"items"`
	Cursor     string `json:"cursor"`
	NextCursor string `json:"next_cursor"`
	Success    bool   `json:"success"`
}

func (p ItemsPage) Next() string {
	if p.Cursor != "" {
		return p.Cursor
	}
	return p.NextCursor
}

type ItemDetail struct {
	Item        Item   `json:"item"`
	HTMLContent string `json:"html_content"`
	SummaryHTML string `json:"summary_html"`
}

type AnalysisEnvelope struct {
	Analysis *Analysis `json:"analysis"`
}

type Analysis struct {
	ID            int64     `json:"id"`
	ItemID        int64     `json:"item_id"`
	AICategory    string    `json:"ai_category"`
	AISubcategory string    `json:"ai_subcategory"`
	QualityScore  int       `json:"quality_score"`
	AISummary     string    `json:"ai_summary"`
	AIComment     string    `json:"ai_comment"`
	AITags        string    `json:"ai_tags"`
	ModelUsed     string    `json:"model_used"`
	ProcessedAt   time.Time `json:"processed_at"`
}

type Health struct {
	Status           string    `json:"status"`
	BrowserConnected bool      `json:"browser_connected"`
	BrowserCount     int       `json:"browser_count"`
	Timestamp        time.Time `json:"timestamp"`
}

type Stats struct {
	TotalCount           int            `json:"total_count"`
	UnreadCount          int            `json:"unread_count"`
	StarredCount         int            `json:"starred_count"`
	TodayCount           int            `json:"today_count"`
	Categories           map[string]int `json:"categories"`
	SourceCategories     []string       `json:"source_categories"`
	SourceCategoryUnread map[string]int `json:"source_category_unread"`
}

type AICategory struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	AvgScore float64 `json:"avg_score"`
}

type CategoriesEnvelope struct {
	Categories []AICategory `json:"categories"`
}

type Source struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	URL             string     `json:"url"`
	Schedule        string     `json:"schedule"`
	Enabled         int        `json:"enabled"`
	DefaultCategory string     `json:"default_category"`
	Config          string     `json:"config"`
	LastFetchAt     *time.Time `json:"last_fetch_at"`
	LastFetchStatus *string    `json:"last_fetch_status"`
	Category        string     `json:"category"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type SourceForm struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Schedule        string `json:"schedule"`
	DefaultCategory string `json:"default_category"`
	Config          string `json:"config"`
	Category        string `json:"category"`
}

type FetchLog struct {
	ID           int64      `json:"id"`
	SourceID     string     `json:"source_id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"`
	ItemsFound   int        `json:"items_found"`
	ItemsAdded   int        `json:"items_added"`
	ErrorMessage string     `json:"error_message"`
}

type Browser struct {
	ConnID string `json:"conn_id"`
	Name   string `json:"name"`
	Online bool   `json:"online"`
	Banned bool   `json:"banned"`
}

type BrowserList struct {
	Browsers []Browser `json:"browsers"`
	Count    int       `json:"count"`
}

type ExtractRecord struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Chars     int       `json:"chars"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExtractList struct {
	Items []ExtractRecord `json:"items"`
}

type DailyReport struct {
	Success        bool   `json:"success"`
	ID             int64  `json:"id"`
	ReportDate     string `json:"report_date"`
	ReportType     string `json:"report_type"`
	Title          string `json:"title"`
	Format         string `json:"format"`
	Content        string `json:"content"`
	HTMLContent    string `json:"html_content"`
	TotalItems     int    `json:"total_items"`
	QualityItems   int    `json:"quality_items"`
	ModelUsed      string `json:"model_used"`
	GeneratedAt    string `json:"generated_at"`
	CategoriesJSON string `json:"categories_summary"`
}

type ReportListItem struct {
	ID           int64     `json:"id"`
	ReportDate   string    `json:"report_date"`
	ReportType   string    `json:"report_type"`
	Title        string    `json:"title"`
	TotalItems   int       `json:"total_items"`
	QualityItems int       `json:"quality_items"`
	ModelUsed    string    `json:"model_used"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type ReportList struct {
	Reports []ReportListItem `json:"reports"`
}

type ProviderProfile struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	APIKey            string   `json:"api_key"`
	Model             string   `json:"model"`
	Models            []string `json:"models"`
	BaseURL           string   `json:"base_url"`
	Disabled          bool     `json:"disabled"`
	Priority          int      `json:"priority"`
	RequestsPerMinute int      `json:"requests_per_minute"`
}

type AISettings struct {
	Enabled              bool              `json:"enabled"`
	Provider             string            `json:"provider"`
	APIKey               string            `json:"api_key"`
	Model                string            `json:"model"`
	Models               []string          `json:"models"`
	BaseURL              string            `json:"base_url"`
	QualityThreshold     int               `json:"quality_threshold"`
	SystemPrompt         string            `json:"system_prompt"`
	DailyPrompt          string            `json:"daily_prompt"`
	Strategy             string            `json:"strategy"`
	ActiveProfileID      string            `json:"active_profile_id"`
	Profiles             []ProviderProfile `json:"profiles"`
	MorningReportTime    string            `json:"morning_report_time"`
	EveningReportTime    string            `json:"evening_report_time"`
	MorningReportEnabled bool              `json:"morning_report_enabled"`
	EveningReportEnabled bool              `json:"evening_report_enabled"`

	ASRMode         string `json:"asr_mode"`
	ASRProvider     string `json:"asr_provider"`
	ASRBaseURL      string `json:"asr_base_url"`
	ASRAPIKey       string `json:"asr_api_key"`
	ASRModel        string `json:"asr_model"`
	ASRLocalCommand string `json:"asr_local_command"`
}

type AISettingsEnvelope struct {
	Settings *AISettings `json:"settings"`
}

type AIStats struct {
	TotalProcessed int64   `json:"total_processed"`
	TotalPending   int64   `json:"total_pending"`
	AverageScore   float64 `json:"average_score"`
	QueueLength    int     `json:"queue_length"`
}

type UsageByDay struct {
	Day        string  `json:"day"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	Cost       float64 `json:"cost"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

type AIUsage struct {
	Days           int          `json:"days"`
	TotalRequests  int64        `json:"total_requests"`
	FailedRequests int64        `json:"failed_requests"`
	TotalTokens    int64        `json:"total_tokens"`
	Cost           float64      `json:"cost"`
	AvgLatencyMS   float64      `json:"avg_latency_ms"`
	ByDay          []UsageByDay `json:"by_day"`
}

type UsageEnvelope struct {
	Summary *AIUsage `json:"summary"`
}

type DiscoverModelsResult struct {
	Success  bool     `json:"success"`
	Provider string   `json:"provider"`
	BaseURL  string   `json:"base_url"`
	Count    int      `json:"count"`
	Models   []string `json:"models"`
}

type DesktopSettings struct {
	Success           bool   `json:"success"`
	DesktopMode       bool   `json:"desktop_mode"`
	DataDir           string `json:"data_dir"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	ActiveExternal    bool   `json:"active_external"`
	EnvForcedExternal bool   `json:"env_forced_external"`
	ExternalEnabled   bool   `json:"external_enabled"`
	TokenEnabled      bool   `json:"token_enabled"`
	APIToken          string `json:"api_token"`
	AuthRequired      bool   `json:"auth_required"`
	PortShifted       bool   `json:"port_shifted"`
	RestartRequired   bool   `json:"restart_required"`
}

type ASToolStatus struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
}

type ASREnv struct {
	OS             string                  `json:"os"`
	Arch           string                  `json:"arch"`
	Tools          map[string]ASToolStatus `json:"tools"`
	LocalReady     bool                    `json:"local_ready"`
	CloudReady     bool                    `json:"cloud_ready"`
	InstallCommand string                  `json:"install_command"`
	ScriptPath     string                  `json:"script_path,omitempty"`
}

type ASREnvEnvelope struct {
	Env *ASREnv `json:"env"`
}

type TestResult struct {
	Success   bool   `json:"success"`
	Success2  bool   `json:"ok"`
	Message   string `json:"message"`
	Error     string `json:"error"`
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	LatencyMS int64  `json:"latency_ms"`
	Score     int    `json:"score"`
	Summary   string `json:"summary"`
}
