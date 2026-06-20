package sqlite

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This file holds the GORM-backed storage for on-demand fetch results: the
// extract-page cache and the tweet archive. New persistence uses GORM (see the
// project decision); the older repos keep their hand-written SQL.

// ExtractCache caches one extracted page keyed by URL so repeat /api/extract
// calls can return without re-fetching through the browser.
type ExtractCache struct {
	URL       string    `gorm:"primaryKey;column:url"`
	Title     string    `gorm:"column:title"`
	Markdown  string    `gorm:"column:markdown"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName pins the table name (GORM would otherwise pluralize the struct).
func (ExtractCache) TableName() string { return "extract_cache" }

// TweetRecord is one archived tweet. search / timeline / likes all upsert into
// this single table keyed by the tweet id, deduping across fetches.
type TweetRecord struct {
	ID             string    `gorm:"primaryKey;column:id" json:"id"`
	Text           string    `gorm:"column:text" json:"text"`
	Author         string    `gorm:"column:author" json:"author"`
	AuthorName     string    `gorm:"column:author_name" json:"author_name"`
	TweetCreatedAt string    `gorm:"column:tweet_created_at" json:"tweet_created_at"` // tweet's own timestamp (string from X)
	FavoriteCount  int       `gorm:"column:favorite_count" json:"favorite_count"`
	RetweetCount   int       `gorm:"column:retweet_count" json:"retweet_count"`
	ReplyCount     int       `gorm:"column:reply_count" json:"reply_count"`
	QuoteCount     int       `gorm:"column:quote_count" json:"quote_count"`
	URL            string    `gorm:"column:url" json:"url"`
	Media          []string  `gorm:"column:media;serializer:json" json:"media"`
	Source         string    `gorm:"column:source" json:"source"` // "search" | "timeline" | "likes"
	FetchedAt      time.Time `gorm:"column:fetched_at" json:"fetched_at"`
}

// TableName pins the table name.
func (TweetRecord) TableName() string { return "tweets" }

// RedditPost is one archived Reddit submission, keyed by post id. subreddit
// listing fetches upsert into this single table, deduping across fetches.
type RedditPost struct {
	ID          string  `gorm:"primaryKey;column:id" json:"id"`
	Title       string  `gorm:"column:title" json:"title"`
	Author      string  `gorm:"column:author" json:"author"`
	Subreddit   string  `gorm:"column:subreddit" json:"subreddit"`
	URL         string  `gorm:"column:url" json:"url"`         // permalink on reddit
	ContentURL  string  `gorm:"column:content_url" json:"content_url"`
	Body        string  `gorm:"column:body" json:"body"`
	Score       int     `gorm:"column:score" json:"score"`
	NumComments int     `gorm:"column:num_comments" json:"num_comments"`
	CreatedUTC  float64 `gorm:"column:created_utc" json:"created_utc"`
	FetchedAt   time.Time `gorm:"column:fetched_at" json:"fetched_at"`
}

// TableName pins the table name.
func (RedditPost) TableName() string { return "reddit_posts" }

// XhsNote is one archived Xiaohongshu note, keyed by note id. search / user
// notes / note detail fetches upsert into this single table, deduping across
// fetches.
type XhsNote struct {
	ID             string  `gorm:"primaryKey;column:id" json:"id"`
	Title          string  `gorm:"column:title" json:"title"`
	Desc           string  `gorm:"column:desc" json:"desc"`
	Type           string  `gorm:"column:type" json:"type"`
	Author         string  `gorm:"column:author" json:"author"`
	AuthorID       string  `gorm:"column:author_id" json:"author_id"`
	LikedCount     string  `gorm:"column:liked_count" json:"liked_count"`
	CollectedCount string  `gorm:"column:collected_count" json:"collected_count"`
	CommentCount   string  `gorm:"column:comment_count" json:"comment_count"`
	ShareCount     string  `gorm:"column:share_count" json:"share_count"`
	URL            string  `gorm:"column:url" json:"url"`
	Images         []string `gorm:"column:images;serializer:json" json:"images"`
	FetchedAt      time.Time `gorm:"column:fetched_at" json:"fetched_at"`
}

// TableName pins the table name.
func (XhsNote) TableName() string { return "xhs_notes" }

// GetExtractCache returns the cached extraction for url, or (nil, nil) on miss.
func (d *Database) GetExtractCache(url string) (*ExtractCache, error) {
	var rec ExtractCache
	err := d.gorm.Where("url = ?", url).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// SaveExtractCache upserts an extraction result keyed by URL.
func (d *Database) SaveExtractCache(url, title, markdown string) error {
	now := time.Now()
	rec := ExtractCache{URL: url, Title: title, Markdown: markdown, CreatedAt: now, UpdatedAt: now}
	return d.gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "url"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "markdown", "updated_at"}),
	}).Create(&rec).Error
}

// ListExtractCache returns cached extractions ordered by most-recently updated,
// plus the total row count for pagination.
func (d *Database) ListExtractCache(limit, offset int) ([]ExtractCache, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int64
	if err := d.gorm.Model(&ExtractCache{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ExtractCache
	err := d.gorm.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// ListTweets returns archived tweets ordered by most-recently fetched, optionally
// filtered by source ("search"/"timeline"/"likes"), plus the total count.
func (d *Database) ListTweets(source string, limit, offset int) ([]TweetRecord, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	q := d.gorm.Model(&TweetRecord{})
	if source != "" {
		q = q.Where("source = ?", source)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []TweetRecord
	err := q.Order("fetched_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveTweets upserts a batch of tweets keyed by id (latest fetch wins).
func (d *Database) SaveTweets(records []TweetRecord) error {
	if len(records) == 0 {
		return nil
	}
	return d.gorm.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"text", "author", "author_name", "tweet_created_at",
			"favorite_count", "retweet_count", "reply_count", "quote_count",
			"url", "media", "source", "fetched_at",
		}),
	}).Create(&records).Error
}

// ListRedditPosts returns archived Reddit posts ordered by most-recently fetched,
// optionally filtered by subreddit, plus the total count.
func (d *Database) ListRedditPosts(subreddit string, limit, offset int) ([]RedditPost, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	q := d.gorm.Model(&RedditPost{})
	if subreddit != "" {
		q = q.Where("subreddit = ?", subreddit)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []RedditPost
	err := q.Order("fetched_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveRedditPosts upserts a batch of Reddit posts keyed by id (latest fetch wins).
func (d *Database) SaveRedditPosts(records []RedditPost) error {
	if len(records) == 0 {
		return nil
	}
	return d.gorm.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "author", "subreddit", "url", "content_url",
			"body", "score", "num_comments", "created_utc", "fetched_at",
		}),
	}).Create(&records).Error
}

// ListXhsNotes returns archived Xiaohongshu notes ordered by most-recently
// fetched, plus the total count.
func (d *Database) ListXhsNotes(limit, offset int) ([]XhsNote, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int64
	if err := d.gorm.Model(&XhsNote{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []XhsNote
	err := d.gorm.Order("fetched_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveXhsNotes upserts a batch of Xiaohongshu notes keyed by id (latest fetch
// wins).
func (d *Database) SaveXhsNotes(records []XhsNote) error {
	if len(records) == 0 {
		return nil
	}
	return d.gorm.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "desc", "type", "author", "author_id",
			"liked_count", "collected_count", "comment_count", "share_count",
			"url", "images", "fetched_at",
		}),
	}).Create(&records).Error
}
