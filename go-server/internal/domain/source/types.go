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
//
// Status values:
//   - running: a fetch (or its asynchronous browser work) is still in flight
//   - success: every processed item completed without a recorded failure
//   - partial: some items succeeded and some failed
//   - error: the run itself failed (feed/list unreachable, parser error)
//   - skipped: no work was attempted (for example no browser connected)
//   - not_modified: the feed answered 304 Not Modified
//   - no_new_items: the source was fetched but every discovered entry already existed
//
// Trigger values: "manual", "scheduled" (cron), or "retry".
type FetchLog struct {
	ID           int64      `json:"id"`
	SourceID     string     `json:"source_id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"`
	ItemsFound   int        `json:"items_found"`
	ItemsAdded   int        `json:"items_added"`
	ItemsUpdated int        `json:"items_updated"`
	ItemsSkipped int        `json:"items_skipped"`
	ItemsFailed  int        `json:"items_failed"`
	Trigger      string     `json:"trigger"`
	Summary      string     `json:"summary"`
	ErrorMessage string     `json:"error_message"`
}

// FetchTextResult records how one kind of text (feed body, article body,
// YouTube description, caption track, podcast transcript, ...) was obtained
// for a single entry.
//
// Status values: stored, updated, kept_existing, skipped, not_attempted,
// no_public_text, failed, pending.
// Stage values (only for failures): access, extract, persist.
type FetchTextResult struct {
	Kind     string `json:"kind"` // summary, content, description, transcript, audio
	Status   string `json:"status"`
	Source   string `json:"source,omitempty"` // feed, web_page, blog_article, youtube_caption, transcript_link, audio_enclosure, ...
	URL      string `json:"url,omitempty"`
	Language string `json:"language,omitempty"`
	Chars    int    `json:"chars,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Stage    string `json:"stage,omitempty"`
}

// FetchLogItem is one entry discovered/processed during a fetch run. It stores
// only status and pointers; the text itself always lives in scraped_items.
type FetchLogItem struct {
	ID         int64             `json:"id"`
	LogID      int64             `json:"log_id"`
	SourceID   string            `json:"source_id"`
	ItemID     int64             `json:"item_id,omitempty"`
	Title      string            `json:"title"`
	URL        string            `json:"url"`
	Category   string            `json:"category,omitempty"`
	Status     string            `json:"status"` // stored, updated, kept_existing, skipped, no_public_text, failed, pending
	Reason     string            `json:"reason,omitempty"`
	ErrorStage string            `json:"error_stage,omitempty"`
	Texts      []FetchTextResult `json:"texts"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
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
