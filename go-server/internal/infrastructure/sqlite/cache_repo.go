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
	ID             string    `gorm:"primaryKey;column:id"`
	Text           string    `gorm:"column:text"`
	Author         string    `gorm:"column:author"`
	AuthorName     string    `gorm:"column:author_name"`
	TweetCreatedAt string    `gorm:"column:tweet_created_at"` // tweet's own timestamp (string from X)
	FavoriteCount  int       `gorm:"column:favorite_count"`
	RetweetCount   int       `gorm:"column:retweet_count"`
	ReplyCount     int       `gorm:"column:reply_count"`
	QuoteCount     int       `gorm:"column:quote_count"`
	URL            string    `gorm:"column:url"`
	Media          []string  `gorm:"column:media;serializer:json"`
	Source         string    `gorm:"column:source"` // "search" | "timeline" | "likes"
	FetchedAt      time.Time `gorm:"column:fetched_at"`
}

// TableName pins the table name.
func (TweetRecord) TableName() string { return "tweets" }

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
