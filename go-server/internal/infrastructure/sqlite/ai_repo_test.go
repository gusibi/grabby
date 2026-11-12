package sqlite

import (
	"testing"
	"time"
)

func TestDatabase_AI_Operations(t *testing.T) {
	// Initialize in-memory database
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 1. Insert a source first (foreign key dependency)
	newSrc := Source{
		ID:              "ai-test-source",
		Name:            "AI Test Source",
		Type:            "rss",
		URL:             "https://example.com/rss",
		Schedule:        "*/5 * * * *",
		Enabled:         1,
		DefaultCategory: "auto",
		Config:          "{}",
	}
	if err := db.InsertSource(newSrc); err != nil {
		t.Fatalf("Failed to insert source: %v", err)
	}

	// 2. Insert items
	now := time.Now()
	item1 := ScrapedItem{
		SourceID:     "ai-test-source",
		OriginSource: "Test Origin 1",
		Title:        "Tech News 1",
		URL:          "https://example.com/tech-1",
		Summary:      "Summary 1",
		Content:      "Content 1",
		Category:     "article",
		PublishedAt:  &now,
		FetchedAt:    now,
	}
	item2 := ScrapedItem{
		SourceID:     "ai-test-source",
		OriginSource: "Test Origin 2",
		Title:        "Finance News 2",
		URL:          "https://example.com/finance-2",
		Summary:      "Summary 2",
		Content:      "Content 2",
		Category:     "article",
		PublishedAt:  &now,
		FetchedAt:    now,
	}

	_, err = db.InsertScrapedItem(item1)
	if err != nil {
		t.Fatalf("Failed to insert item 1: %v", err)
	}
	_, err = db.InsertScrapedItem(item2)
	if err != nil {
		t.Fatalf("Failed to insert item 2: %v", err)
	}

	// Get items to retrieve their auto-incremented IDs
	items, _, err := db.GetScrapedItems(ItemsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to query items: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("Expected at least 2 items, got %d", len(items))
	}

	id1 := items[0].ID
	id2 := items[1].ID

	// Test GetUnanalyzedItems
	unanalyzed, err := db.GetUnanalyzedItems(10)
	if err != nil {
		t.Fatalf("Failed to get unanalyzed items: %v", err)
	}
	if len(unanalyzed) != 2 {
		t.Errorf("Expected 2 unanalyzed items, got %d", len(unanalyzed))
	}

	// 3. Insert AI Analyses
	analysis1 := AIAnalysis{
		ItemID:        id1,
		AICategory:    "科技",
		AISubcategory: "AI/大模型",
		QualityScore:  9,
		AISummary:     "精炼摘要 1",
		AIComment:     "非常硬核的AI技术文章",
		AITags:        "AI,大模型,技术",
		ModelUsed:     "gemini-2.0-flash",
	}
	analysis2 := AIAnalysis{
		ItemID:        id2,
		AICategory:    "财经",
		AISubcategory: "财经/股市",
		QualityScore:  6,
		AISummary:     "精炼摘要 2",
		AIComment:     "常规股市动态报道",
		AITags:        "股市,财经",
		ModelUsed:     "gemini-2.0-flash",
	}

	if err := db.InsertAIAnalysis(analysis1); err != nil {
		t.Fatalf("Failed to insert AI analysis 1: %v", err)
	}
	if err := db.InsertAIAnalysis(analysis2); err != nil {
		t.Fatalf("Failed to insert AI analysis 2: %v", err)
	}

	// Test GetAIAnalysis
	a1, err := db.GetAIAnalysis(id1)
	if err != nil {
		t.Fatalf("Failed to get AI analysis: %v", err)
	}
	if a1 == nil || a1.AICategory != "科技" || a1.QualityScore != 9 {
		t.Errorf("GetAIAnalysis returned unexpected result: %+v", a1)
	}

	// Test GetUnanalyzedItems should now be empty (0 items)
	unanalyzed, err = db.GetUnanalyzedItems(10)
	if err != nil {
		t.Fatalf("Failed to get unanalyzed items second time: %v", err)
	}
	if len(unanalyzed) != 0 {
		t.Errorf("Expected 0 unanalyzed items, got %d", len(unanalyzed))
	}

	// 4. Test GetScrapedItemsWithAI
	// Fetch all items (no category or score limit)
	itemsWithAI, _, err := db.GetScrapedItemsWithAI(AIItemsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to get scraped items with AI: %v", err)
	}
	if len(itemsWithAI) != 2 {
		t.Errorf("Expected 2 items with AI, got %d", len(itemsWithAI))
	}

	// Filter by AI category
	techItems, _, err := db.GetScrapedItemsWithAI(AIItemsFilter{AICategory: "科技", Limit: 10})
	if err != nil {
		t.Fatalf("Failed to filter items by category: %v", err)
	}
	if len(techItems) != 1 || techItems[0].Title != items[0].Title {
		t.Errorf("Expected 1 tech item, got %+v", techItems)
	}

	// Filter by ScoreMin >= 7
	qualityItems, _, err := db.GetScrapedItemsWithAI(AIItemsFilter{ScoreMin: 7, Limit: 10})
	if err != nil {
		t.Fatalf("Failed to filter items by score: %v", err)
	}
	if len(qualityItems) != 1 || qualityItems[0].QualityScore != 9 {
		t.Errorf("Expected 1 high quality item, got %+v", qualityItems)
	}

	// 5. Test GetAICategories
	catStats, err := db.GetAICategories()
	if err != nil {
		t.Fatalf("Failed to get AI category stats: %v", err)
	}
	if len(catStats) != 2 {
		t.Errorf("Expected 2 categories stats, got %d", len(catStats))
	}
	// Verify counts and average scores
	for _, stat := range catStats {
		if stat.Name == "科技" {
			if stat.Count != 1 || stat.AvgScore != 9.0 {
				t.Errorf("Expected tech count 1, avg 9.0; got count %d, avg %f", stat.Count, stat.AvgScore)
			}
		} else if stat.Name == "财经" {
			if stat.Count != 1 || stat.AvgScore != 6.0 {
				t.Errorf("Expected finance count 1, avg 6.0; got count %d, avg %f", stat.Count, stat.AvgScore)
			}
		} else {
			t.Errorf("Unexpected category name: %s", stat.Name)
		}
	}

	// 6. Test Daily Report functions
	// Report dates are local calendar days — that is what the scheduler and
	// the generate handler pass in — so ask for the local date, not the UTC one.
	dateStr := now.Format("2006-01-02")

	// Test GetTotalItemsCountForDate
	totalCount, err := db.GetTotalItemsCountForDate(dateStr)
	if err != nil {
		t.Fatalf("Failed to get total items count for date: %v", err)
	}
	if totalCount != 2 {
		t.Errorf("Expected 2 total items for date, got %d", totalCount)
	}

	// Test GetQualityItemsForDate (threshold 7)
	qItemsForDate, err := db.GetQualityItemsForDate(dateStr, 7)
	if err != nil {
		t.Fatalf("Failed to get quality items for date: %v", err)
	}
	if len(qItemsForDate) != 1 || qItemsForDate[0].QualityScore != 9 {
		t.Errorf("Expected 1 quality item for date (score >= 7), got %d: %+v", len(qItemsForDate), qItemsForDate)
	}

	// 7. Insert and Get Daily Report
	report := AIDailyReport{
		ReportDate:        dateStr,
		ReportType:        "daily",
		Title:             "Daily Tech Report",
		Content:           "# Tech Report\nSome markdown content",
		TotalItems:        2,
		QualityItems:      1,
		CategoriesSummary: `{"科技":1}`,
		ModelUsed:         "gemini-2.0-flash",
		GeneratedAt:       now,
	}

	if err := db.InsertAIDailyReport(report); err != nil {
		t.Fatalf("Failed to insert daily report: %v", err)
	}

	// Get report
	retrievedReport, err := db.GetAIDailyReport(dateStr, "daily")
	if err != nil {
		t.Fatalf("Failed to get daily report: %v", err)
	}
	if retrievedReport == nil || retrievedReport.Title != "Daily Tech Report" || retrievedReport.QualityItems != 1 {
		t.Errorf("GetAIDailyReport returned unexpected report: %+v", retrievedReport)
	}

	legacyDateStr := now.AddDate(0, 0, 1).Format("2006-01-02")
	_, err = db.db.Exec(`
		INSERT INTO ai_daily_reports (report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used)
		VALUES (?, '', ?, ?, ?, ?, ?, ?)
	`, legacyDateStr, "Legacy Daily Report", "# Legacy report", 1, 1, `{"科技":1}`, "legacy-model")
	if err != nil {
		t.Fatalf("Failed to insert legacy daily report: %v", err)
	}

	legacyReport, err := db.GetAIDailyReport(legacyDateStr, "daily")
	if err != nil {
		t.Fatalf("Failed to get legacy daily report: %v", err)
	}
	if legacyReport == nil || legacyReport.ReportType != "daily" || legacyReport.Title != "Legacy Daily Report" {
		t.Errorf("GetAIDailyReport did not normalize legacy daily report: %+v", legacyReport)
	}

	// Get reports list
	reportsList, err := db.GetAIDailyReports(10, "")
	if err != nil {
		t.Fatalf("Failed to get daily reports list: %v", err)
	}
	if len(reportsList) != 2 || reportsList[0].ReportDate != legacyDateStr || reportsList[0].ReportType != "daily" {
		t.Errorf("Expected 2 reports with normalized daily type in list, got %+v", reportsList)
	}

	// 8. Test Settings CRUD & LoadAISettings
	err = db.SaveSetting("test_key", "test_val")
	if err != nil {
		t.Fatalf("Failed to save setting: %v", err)
	}

	val, err := db.GetSetting("test_key", "default")
	if err != nil {
		t.Fatalf("Failed to get setting: %v", err)
	}
	if val != "test_val" {
		t.Errorf("Expected 'test_val', got '%s'", val)
	}

	valDefault, err := db.GetSetting("non_existent_key", "default_val")
	if err != nil {
		t.Fatalf("Failed to get non-existent setting: %v", err)
	}
	if valDefault != "default_val" {
		t.Errorf("Expected 'default_val', got '%s'", valDefault)
	}

	// Test LoadAISettings
	envSettings := AISettings{
		Enabled:          true,
		Provider:         "openai",
		APIKey:           "env_key",
		Model:            "gpt-4o",
		BaseURL:          "https://api.openai.com",
		QualityThreshold: 8,
	}

	loaded, err := db.LoadAISettings(envSettings)
	if err != nil {
		t.Fatalf("Failed to LoadAISettings: %v", err)
	}

	if !loaded.Enabled || loaded.Provider != "openai" || loaded.APIKey != "env_key" || loaded.Model != "gpt-4o" || loaded.BaseURL != "https://api.openai.com" || loaded.QualityThreshold != 8 {
		t.Errorf("LoadAISettings loaded incorrect values first time: %+v", loaded)
	}

	// Legacy profiles (saved before multi-model support, no `models` field)
	// still honour the top-level ai_model setting.
	legacyProfiles := `[{"id":"default","name":"默认服务商","provider":"openai","api_key":"env_key","model":"gpt-4o","base_url":"https://api.openai.com"}]`
	if err := db.SaveSetting("ai_provider_profiles", legacyProfiles); err != nil {
		t.Fatalf("Failed to reset provider profiles: %v", err)
	}
	err = db.SaveSetting("ai_model", "claude-3-opus")
	if err != nil {
		t.Fatalf("Failed to update ai_model setting: %v", err)
	}

	reloaded, err := db.LoadAISettings(envSettings)
	if err != nil {
		t.Fatalf("Failed to reload settings: %v", err)
	}

	if reloaded.Model != "claude-3-opus" {
		t.Errorf("Expected loaded model 'claude-3-opus' from DB, got '%s'", reloaded.Model)
	}
	if len(reloaded.Models) != 1 || reloaded.Models[0] != "claude-3-opus" {
		t.Errorf("Expected legacy model to migrate into Models, got %v", reloaded.Models)
	}

	// Multi-model profiles are authoritative: a stale top-level ai_model must
	// not clobber the configured list.
	multiProfiles := `[{"id":"default","name":"默认服务商","provider":"openai","api_key":"env_key","model":"gpt-4o","models":["gpt-4o","gpt-4o-mini"],"base_url":"https://api.openai.com"}]`
	if err := db.SaveSetting("ai_provider_profiles", multiProfiles); err != nil {
		t.Fatalf("Failed to save multi-model profiles: %v", err)
	}
	if err := db.SaveSetting("ai_model", "claude-3-opus"); err != nil {
		t.Fatalf("Failed to update ai_model setting: %v", err)
	}

	reloaded, err = db.LoadAISettings(envSettings)
	if err != nil {
		t.Fatalf("Failed to reload multi-model settings: %v", err)
	}
	if reloaded.Model != "gpt-4o" {
		t.Errorf("Expected profile models to win, got Model '%s'", reloaded.Model)
	}
	if len(reloaded.Models) != 2 || reloaded.Models[1] != "gpt-4o-mini" {
		t.Errorf("Expected both models to survive reload, got %v", reloaded.Models)
	}
}

// Regression test: time-range lookups used to compare CURRENT_TIMESTAMP's
// naive UTC text against the offset-bearing ISO string the driver binds for
// time.Time. Text comparison never matched, so every ranged daily report came
// back empty. The queries must normalise both sides before comparing.
func TestGetQualityItemsForTimeRange_NormalisesTimestampFormats(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.InsertSource(Source{
		ID: "range-src", Name: "Range Source", Type: "rss",
		URL: "https://example.com/range", Schedule: "0 * * * *",
		Enabled: 1, DefaultCategory: "auto", Config: "{}",
	}); err != nil {
		t.Fatalf("Failed to insert source: %v", err)
	}

	now := time.Now()
	itemID, err := db.InsertScrapedItem(ScrapedItem{
		SourceID:     "range-src",
		OriginSource: "Range Origin",
		Title:        "Range Item",
		URL:          "https://example.com/range-item",
		Summary:      "Range summary",
		Content:      "Range content",
		Category:     "article",
		FetchedAt:    now,
	})
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	if err := db.InsertAIAnalysis(AIAnalysis{
		ItemID:       itemID,
		AICategory:   "科技",
		QualityScore: 9,
		AISummary:    "Range summary",
		AIComment:    "Range comment",
	}); err != nil {
		t.Fatalf("Failed to insert analysis: %v", err)
	}

	// Use a non-UTC zone so the bound parameters carry an offset, matching how
	// production code builds its morning/evening windows in time.Local.
	local := time.FixedZone("UTC+8", 8*60*60)
	start := now.In(local).Add(-1 * time.Hour)
	end := now.In(local).Add(1 * time.Hour)

	items, err := db.GetQualityItemsForTimeRange(start, end, 7)
	if err != nil {
		t.Fatalf("GetQualityItemsForTimeRange failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item inside the window, got %d", len(items))
	}

	total, err := db.GetTotalItemsCountForTimeRange(start, end)
	if err != nil {
		t.Fatalf("GetTotalItemsCountForTimeRange failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 scraped item inside the window, got %d", total)
	}

	// A window that ends before the row was written must not match.
	earlyEnd := now.In(local).Add(-1 * time.Hour)
	earlyStart := earlyEnd.Add(-24 * time.Hour)
	if items, err := db.GetQualityItemsForTimeRange(earlyStart, earlyEnd, 7); err != nil {
		t.Fatalf("GetQualityItemsForTimeRange failed: %v", err)
	} else if len(items) != 0 {
		t.Errorf("expected 0 items before the window, got %d", len(items))
	}
}

// Regression test: date-based lookups compared strftime() of the UTC-stored
// timestamp against a local calendar date, so rows written near midnight sat in
// the wrong day. Both date lookups must use the same local-day window as the
// range-based queries.
func TestGetQualityItemsForDate_UsesLocalDayWindow(t *testing.T) {
	origLoc := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = origLoc })

	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.InsertSource(Source{
		ID: "day-src", Name: "Day Source", Type: "rss",
		URL: "https://example.com/day", Schedule: "0 * * * *",
		Enabled: 1, DefaultCategory: "auto", Config: "{}",
	}); err != nil {
		t.Fatalf("Failed to insert source: %v", err)
	}

	itemID, err := db.InsertScrapedItem(ScrapedItem{
		SourceID:     "day-src",
		OriginSource: "Day Origin",
		Title:        "Day Item",
		URL:          "https://example.com/day-item",
		Summary:      "Day summary",
		Content:      "Day content",
		Category:     "article",
		FetchedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	if err := db.InsertAIAnalysis(AIAnalysis{
		ItemID:       itemID,
		AICategory:   "科技",
		QualityScore: 9,
		AISummary:    "Day summary",
		AIComment:    "Day comment",
	}); err != nil {
		t.Fatalf("Failed to insert analysis: %v", err)
	}

	// 2026-06-13 16:30 UTC is 2026-06-14 00:30 in UTC+8: the row belongs to
	// local day 2026-06-14 even though its UTC date is 2026-06-13.
	const boundaryUTC = "2026-06-13 16:30:00"
	for _, table := range []string{"ai_analyses", "scraped_items"} {
		if _, err := db.db.Exec("UPDATE "+table+" SET "+map[string]string{
			"ai_analyses":   "processed_at",
			"scraped_items": "fetched_at",
		}[table]+" = ?", boundaryUTC); err != nil {
			t.Fatalf("Failed to pin %s timestamp: %v", table, err)
		}
	}

	items, err := db.GetQualityItemsForDate("2026-06-14", 7)
	if err != nil {
		t.Fatalf("GetQualityItemsForDate returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item on local day 2026-06-14, got %d", len(items))
	}

	if items, err := db.GetQualityItemsForDate("2026-06-13", 7); err != nil {
		t.Fatalf("GetQualityItemsForDate returned error: %v", err)
	} else if len(items) != 0 {
		t.Errorf("expected 0 items on local day 2026-06-13, got %d", len(items))
	}

	if total, err := db.GetTotalItemsCountForDate("2026-06-14"); err != nil {
		t.Fatalf("GetTotalItemsCountForDate returned error: %v", err)
	} else if total != 1 {
		t.Errorf("expected 1 fetched item on local day 2026-06-14, got %d", total)
	}

	if total, err := db.GetTotalItemsCountForDate("2026-06-13"); err != nil {
		t.Fatalf("GetTotalItemsCountForDate returned error: %v", err)
	} else if total != 0 {
		t.Errorf("expected 0 fetched items on local day 2026-06-13, got %d", total)
	}

	if _, err := db.GetQualityItemsForDate("not-a-date", 7); err == nil {
		t.Error("expected an error for an unparseable date, got nil")
	}
}
