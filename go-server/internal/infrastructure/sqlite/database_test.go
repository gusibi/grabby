package sqlite

import (
	"testing"
	"time"
)

func TestDatabase_CRUD(t *testing.T) {
	// Initialize in-memory database
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check if default seeds were inserted
	sources, err := db.GetSources()
	if err != nil {
		t.Fatalf("Failed to get sources: %v", err)
	}
	if len(sources) != 15 {
		t.Errorf("Expected 15 seeded sources, got %d", len(sources))
	}

	// Test InsertSource
	newSrc := Source{
		ID:              "test-source",
		Name:            "Test Source",
		Type:            "rss",
		URL:             "https://example.com/rss",
		Schedule:        "*/5 * * * *",
		Enabled:         1,
		DefaultCategory: "article",
		Config:          "{}",
	}
	if err := db.InsertSource(newSrc); err != nil {
		t.Fatalf("Failed to insert source: %v", err)
	}

	// Test GetSource
	src, err := db.GetSource("test-source")
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}
	if src == nil || src.Name != "Test Source" {
		t.Errorf("Got unexpected source: %+v", src)
	}

	// Test UpdateSource
	src.Name = "Updated Source Name"
	if err := db.UpdateSource(*src); err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}
	src, _ = db.GetSource("test-source")
	if src.Name != "Updated Source Name" {
		t.Errorf("Source update did not persist: %s", src.Name)
	}

	// Test ToggleSource
	if err := db.ToggleSource("test-source", 0); err != nil {
		t.Fatalf("Failed to toggle source: %v", err)
	}
	src, _ = db.GetSource("test-source")
	if src.Enabled != 0 {
		t.Errorf("Expected enabled = 0, got %d", src.Enabled)
	}

	// Test ScrapedItem Insertion
	pubAt := time.Now().Add(-1 * time.Hour)
	item := ScrapedItem{
		SourceID:     "test-source",
		OriginSource: "example.com",
		Title:        "Test Article Title",
		URL:          "https://example.com/test-article",
		Summary:      "This is a summary",
		Content:      "This is full content",
		Category:     "article",
		PublishedAt:  &pubAt,
	}

	rows, err := db.InsertScrapedItem(item)
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}
	if rows != 1 {
		t.Errorf("Expected 1 row affected, got %d", rows)
	}

	// Test unique constraint (INSERT OR IGNORE)
	rows, err = db.InsertScrapedItem(item)
	if err != nil {
		t.Fatalf("Failed to insert duplicate item: %v", err)
	}
	if rows != 0 {
		t.Errorf("Expected 0 rows affected for duplicate URL, got %d", rows)
	}

	// Test GetScrapedItems
	items, cursor, err := db.GetScrapedItems(ItemsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to get items: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Test Article Title" {
		t.Errorf("Unexpected item title: %s", items[0].Title)
	}
	if cursor != "" {
		t.Errorf("Expected empty next page cursor, got %s", cursor)
	}

	// Test MarkItemRead & ToggleItemStarred
	itemID := items[0].ID
	if err := db.MarkItemRead(itemID, 1); err != nil {
		t.Fatalf("Failed to mark read: %v", err)
	}
	if err := db.ToggleItemStarred(itemID, 1); err != nil {
		t.Fatalf("Failed to star item: %v", err)
	}

	items, _, _ = db.GetScrapedItems(ItemsFilter{Starred: intPtr(1)})
	if len(items) != 1 {
		t.Errorf("Expected 1 starred item, got %d", len(items))
	}

	// Test Fetch Logs
	log := FetchLog{
		SourceID:   "test-source",
		StartedAt:  time.Now().Add(-10 * time.Minute),
		FinishedAt: timePtr(time.Now()),
		Status:     "success",
		ItemsFound: 5,
		ItemsAdded: 1,
	}
	logID, err := db.InsertFetchLog(log)
	if err != nil {
		t.Fatalf("Failed to insert fetch log: %v", err)
	}
	if logID <= 0 {
		t.Errorf("Expected positive logID, got %d", logID)
	}

	logs, err := db.GetFetchLogs("test-source", 10)
	if err != nil {
		t.Fatalf("Failed to get fetch logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ItemsAdded != 1 {
		t.Errorf("Unexpected logs: %+v", logs)
	}

	// Test DeleteSource
	if err := db.DeleteSource("test-source"); err != nil {
		t.Fatalf("Failed to delete source: %v", err)
	}
	src, _ = db.GetSource("test-source")
	if src != nil {
		t.Errorf("Source should have been deleted")
	}
}

func intPtr(i int) *int {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}
