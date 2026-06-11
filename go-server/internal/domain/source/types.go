package source

import "time"

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
