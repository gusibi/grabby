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
	dateStr := now.UTC().Format("2006-01-02")
	
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

	// Get reports list
	reportsList, err := db.GetAIDailyReports(10, "")
	if err != nil {
		t.Fatalf("Failed to get daily reports list: %v", err)
	}
	if len(reportsList) != 1 || reportsList[0].ReportDate != dateStr {
		t.Errorf("Expected 1 report in list, got %+v", reportsList)
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

	// Verify it saved them to DB by updating DB and reloading
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
}
