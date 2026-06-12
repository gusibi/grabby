package item

import "time"

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

// ItemsFilter represents filters when querying scraped items.
type ItemsFilter struct {
	Category       string
	SourceCategory string
	Origin         string
	Q              string
	Starred        *int
	ReadStatus     *int
	Cursor         string
	Limit          int
}

// AIItemsFilter represents filters when querying scraped items with AI analysis.
type AIItemsFilter struct {
	AICategory     string
	SourceCategory string
	ScoreMin       int
	Days           int
	Limit          int
	Cursor         string
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
