package sqlite

import (
	"testing"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestExtractCacheMissThenHitAndRefresh(t *testing.T) {
	db := newTestDB(t)

	// Miss on unknown URL.
	if got, err := db.GetExtractCache("https://example.com"); err != nil || got != nil {
		t.Fatalf("expected miss, got rec=%v err=%v", got, err)
	}

	// Save then hit.
	if err := db.SaveExtractCache("https://example.com", "Example", "# Example"); err != nil {
		t.Fatalf("SaveExtractCache: %v", err)
	}
	got, err := db.GetExtractCache("https://example.com")
	if err != nil || got == nil {
		t.Fatalf("expected hit, got rec=%v err=%v", got, err)
	}
	if got.Title != "Example" || got.Markdown != "# Example" {
		t.Fatalf("unexpected cached content: %+v", got)
	}
	created := got.CreatedAt

	// Upsert same URL updates content but preserves created_at.
	if err := db.SaveExtractCache("https://example.com", "Example v2", "# Example v2"); err != nil {
		t.Fatalf("SaveExtractCache update: %v", err)
	}
	got2, _ := db.GetExtractCache("https://example.com")
	if got2.Markdown != "# Example v2" || got2.Title != "Example v2" {
		t.Fatalf("expected updated content, got %+v", got2)
	}
	if !got2.CreatedAt.Equal(created) {
		t.Fatalf("created_at should be preserved on upsert: was %v now %v", created, got2.CreatedAt)
	}
}

func TestSaveTweetsUpsertsByID(t *testing.T) {
	db := newTestDB(t)

	first := []TweetRecord{{
		ID: "1", Text: "hello", Author: "jack", FavoriteCount: 1,
		Media: []string{"https://img/1.jpg"}, Source: "search",
	}}
	if err := db.SaveTweets(first); err != nil {
		t.Fatalf("SaveTweets first: %v", err)
	}

	// Same id from a different source with updated counts: upsert, not duplicate.
	second := []TweetRecord{{
		ID: "1", Text: "hello", Author: "jack", FavoriteCount: 5, Source: "likes",
	}}
	if err := db.SaveTweets(second); err != nil {
		t.Fatalf("SaveTweets second: %v", err)
	}

	var count int64
	if err := db.gorm.Model(&TweetRecord{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", count)
	}

	var rec TweetRecord
	if err := db.gorm.First(&rec, "id = ?", "1").Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.FavoriteCount != 5 || rec.Source != "likes" {
		t.Fatalf("expected upserted counts/source, got %+v", rec)
	}
}

// SaveTweets on an empty slice must be a no-op (GORM errors on empty Create).
func TestSaveTweetsEmpty(t *testing.T) {
	db := newTestDB(t)
	if err := db.SaveTweets(nil); err != nil {
		t.Fatalf("SaveTweets(nil) should be no-op, got %v", err)
	}
}
