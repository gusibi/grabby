package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

// --- AI Analyses CRUD ---

func (d *Database) InsertAIAnalysis(a AIAnalysis) error {
	_, err := d.db.Exec(`
		INSERT INTO ai_analyses (item_id, ai_category, ai_subcategory, quality_score, ai_summary, ai_comment, ai_tags, model_used, processed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(item_id) DO UPDATE SET
			ai_category = excluded.ai_category,
			ai_subcategory = excluded.ai_subcategory,
			quality_score = excluded.quality_score,
			ai_summary = excluded.ai_summary,
			ai_comment = excluded.ai_comment,
			ai_tags = excluded.ai_tags,
			model_used = excluded.model_used,
			processed_at = CURRENT_TIMESTAMP
	`, a.ItemID, a.AICategory, a.AISubcategory, a.QualityScore, a.AISummary, a.AIComment, a.AITags, a.ModelUsed)
	return err
}

func (d *Database) GetAIAnalysis(itemID int64) (*AIAnalysis, error) {
	var a AIAnalysis
	var processedAt time.Time
	err := d.db.QueryRow(`
		SELECT id, item_id, ai_category, ai_subcategory, quality_score, ai_summary, ai_comment, ai_tags, model_used, processed_at
		FROM ai_analyses
		WHERE item_id = ?
	`, itemID).Scan(&a.ID, &a.ItemID, &a.AICategory, &a.AISubcategory, &a.QualityScore, &a.AISummary, &a.AIComment, &a.AITags, &a.ModelUsed, &processedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.ProcessedAt = processedAt
	return &a, nil
}

func (d *Database) GetUnanalyzedItems(limit int) ([]ScrapedItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.db.Query(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		LEFT JOIN ai_analyses a ON i.id = a.item_id
		WHERE a.item_id IS NULL
		ORDER BY i.fetched_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScrapedItem
	for rows.Next() {
		var item ScrapedItem
		var pubAt sql.NullTime
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
			&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags,
		)
		if err != nil {
			return nil, err
		}
		if pubAt.Valid {
			item.PublishedAt = &pubAt.Time
		}
		list = append(list, item)
	}
	return list, nil
}

func (d *Database) GetScrapedItemsWithAI(f AIItemsFilter) ([]ScrapedItemWithAI, string, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}

	query := `
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags,
		       a.ai_category, a.ai_subcategory, a.quality_score, a.ai_summary, a.ai_comment, a.ai_tags, a.model_used, a.processed_at
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		JOIN ai_analyses a ON i.id = a.item_id
		WHERE 1=1
	`
	var args []any

	if f.AICategory != "" && f.AICategory != "all" {
		query += " AND a.ai_category = ?"
		args = append(args, f.AICategory)
	}

	if f.SourceCategory != "" && f.SourceCategory != "all" {
		query += " AND s.category = ?"
		args = append(args, f.SourceCategory)
	}

	if f.ScoreMin > 0 {
		query += " AND a.quality_score >= ?"
		args = append(args, f.ScoreMin)
	}

	if f.Days > 0 {
		query += " AND a.processed_at >= datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", f.Days))
	}

	// Cursor clause using COALESCE(i.published_at, i.fetched_at)
	if f.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(f.Cursor)
		if err == nil {
			timeStr := cursorTime.UTC().Format("2006-01-02 15:04:05")
			query += " AND (COALESCE(i.published_at, i.fetched_at) < ? OR (COALESCE(i.published_at, i.fetched_at) = ? AND i.id < ?))"
			args = append(args, timeStr, timeStr, cursorID)
		}
	}

	query += " ORDER BY COALESCE(i.published_at, i.fetched_at) DESC, i.id DESC LIMIT ?"
	args = append(args, f.Limit+1)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var list []ScrapedItemWithAI
	for rows.Next() {
		var item ScrapedItemWithAI
		var pubAt sql.NullTime
		var processedAt time.Time
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
			&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags,
			&item.AICategory, &item.AISubcategory, &item.QualityScore, &item.AISummary, &item.AIComment, &item.AITags, &item.AIModelUsed, &processedAt,
		)
		if err != nil {
			return nil, "", err
		}
		if pubAt.Valid {
			item.PublishedAt = &pubAt.Time
		}
		item.AIProcessedAt = &processedAt
		list = append(list, item)
	}

	nextCursor := ""
	if len(list) > f.Limit {
		nextItem := list[f.Limit]
		sortTime := nextItem.FetchedAt
		if nextItem.PublishedAt != nil {
			sortTime = *nextItem.PublishedAt
		}
		nextCursor = encodeCursor(sortTime, nextItem.ID)
		list = list[:f.Limit]
	}

	return list, nextCursor, nil
}

func (d *Database) GetAICategories() ([]AICategoryStat, error) {
	rows, err := d.db.Query(`
		SELECT ai_category, COUNT(*), AVG(quality_score)
		FROM ai_analyses
		WHERE processed_at >= datetime('now', '-24 hours')
		GROUP BY ai_category
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AICategoryStat
	for rows.Next() {
		var stat AICategoryStat
		err := rows.Scan(&stat.Name, &stat.Count, &stat.AvgScore)
		if err != nil {
			return nil, err
		}
		list = append(list, stat)
	}
	return list, nil
}

func (d *Database) CountAIAnalyses() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM ai_analyses").Scan(&count)
	return count, err
}

func (d *Database) CountUnprocessedAIItems() (int, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM scraped_items si LEFT JOIN ai_analyses aa ON si.id = aa.item_id WHERE aa.id IS NULL`).Scan(&count)
	return count, err
}

func (d *Database) AverageAIQualityScore() (float64, error) {
	var avg float64
	err := d.db.QueryRow("SELECT COALESCE(AVG(quality_score), 0) FROM ai_analyses").Scan(&avg)
	return avg, err
}

// --- AI Daily Reports CRUD ---

func normalizeReportType(reportType string) string {
	if reportType == "" {
		return "daily"
	}
	return reportType
}

func (d *Database) InsertAIDailyReport(r AIDailyReport) error {
	r.ReportType = normalizeReportType(r.ReportType)
	_, err := d.db.Exec(`
		INSERT INTO ai_daily_reports (report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(report_date, report_type) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			total_items = excluded.total_items,
			quality_items = excluded.quality_items,
			categories_summary = excluded.categories_summary,
			model_used = excluded.model_used,
			generated_at = CURRENT_TIMESTAMP
	`, r.ReportDate, r.ReportType, r.Title, r.Content, r.TotalItems, r.QualityItems, r.CategoriesSummary, r.ModelUsed)
	return err
}

func (d *Database) GetAIDailyReport(date string, reportType string) (*AIDailyReport, error) {
	reportType = normalizeReportType(reportType)
	var r AIDailyReport
	var generatedAt time.Time
	err := d.db.QueryRow(`
		SELECT id, report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used, generated_at
		FROM ai_daily_reports
		WHERE report_date = ? AND (report_type = ? OR (? = 'daily' AND report_type = ''))
	`, date, reportType, reportType).Scan(&r.ID, &r.ReportDate, &r.ReportType, &r.Title, &r.Content, &r.TotalItems, &r.QualityItems, &r.CategoriesSummary, &r.ModelUsed, &generatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.ReportType = normalizeReportType(r.ReportType)
	r.GeneratedAt = generatedAt
	return &r, nil
}

func (d *Database) GetAIDailyReports(limit int, reportType string) ([]AIDailyReport, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows *sql.Rows
	var err error
	if reportType != "" {
		rows, err = d.db.Query(`
			SELECT id, report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used, generated_at
			FROM ai_daily_reports
			WHERE report_type = ?
			ORDER BY report_date DESC, generated_at DESC
			LIMIT ?
		`, reportType, limit)
	} else {
		rows, err = d.db.Query(`
			SELECT id, report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used, generated_at
			FROM ai_daily_reports
			ORDER BY report_date DESC, generated_at DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AIDailyReport
	for rows.Next() {
		var r AIDailyReport
		var generatedAt time.Time
		err := rows.Scan(&r.ID, &r.ReportDate, &r.ReportType, &r.Title, &r.Content, &r.TotalItems, &r.QualityItems, &r.CategoriesSummary, &r.ModelUsed, &generatedAt)
		if err != nil {
			return nil, err
		}
		r.ReportType = normalizeReportType(r.ReportType)
		r.GeneratedAt = generatedAt
		list = append(list, r)
	}
	return list, nil
}

// localDayWindow converts a YYYY-MM-DD string into the half-open [start, end)
// interval covering that calendar day in the server's local timezone.
func localDayWindow(date string) (time.Time, time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date %q: %w", date, err)
	}
	return day, day.AddDate(0, 0, 1), nil
}

// GetQualityItemsForDate returns AI-analysed items scoring at least scoreMin
// whose analysis ran during the local calendar day named by date (YYYY-MM-DD).
//
// The day is interpreted in the server's local timezone and delegated to the
// windowed query so date-based and range-based reports share one 口径.
func (d *Database) GetQualityItemsForDate(date string, scoreMin int) ([]ScrapedItemWithAI, error) {
	start, end, err := localDayWindow(date)
	if err != nil {
		return nil, err
	}
	return d.GetQualityItemsForTimeRange(start, end, scoreMin)
}

// GetTotalItemsCountForDate counts items fetched during the local calendar day
// named by date (YYYY-MM-DD).
func (d *Database) GetTotalItemsCountForDate(date string) (int, error) {
	start, end, err := localDayWindow(date)
	if err != nil {
		return 0, err
	}
	return d.GetTotalItemsCountForTimeRange(start, end)
}

// GetQualityItemsForTimeRange returns AI-analysed items whose score meets the
// threshold and whose analysis ran inside [start, end).
//
// Timestamps written through CURRENT_TIMESTAMP are naive UTC strings while the
// driver binds time.Time as an offset-bearing ISO string. Comparing those
// directly as text never matches, so both sides are normalised through
// SQLite's datetime() before comparing.
func (d *Database) GetQualityItemsForTimeRange(start, end time.Time, scoreMin int) ([]ScrapedItemWithAI, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags,
		       a.ai_category, a.ai_subcategory, a.quality_score, a.ai_summary, a.ai_comment, a.ai_tags, a.model_used, a.processed_at
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		JOIN ai_analyses a ON i.id = a.item_id
		WHERE datetime(a.processed_at) >= datetime(?) AND datetime(a.processed_at) < datetime(?) AND a.quality_score >= ?
		ORDER BY a.quality_score DESC, i.id DESC
	`, start, end, scoreMin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScrapedItemWithAI
	for rows.Next() {
		var item ScrapedItemWithAI
		var pubAt sql.NullTime
		var processedAt time.Time
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
			&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags,
			&item.AICategory, &item.AISubcategory, &item.QualityScore, &item.AISummary, &item.AIComment, &item.AITags, &item.AIModelUsed, &processedAt,
		)
		if err != nil {
			return nil, err
		}
		if pubAt.Valid {
			item.PublishedAt = &pubAt.Time
		}
		item.AIProcessedAt = &processedAt
		list = append(list, item)
	}
	return list, nil
}

// GetTotalItemsCountForTimeRange counts scraped items fetched inside
// [start, end). See GetQualityItemsForTimeRange for the datetime() rationale.
func (d *Database) GetTotalItemsCountForTimeRange(start, end time.Time) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM scraped_items
		WHERE datetime(fetched_at) >= datetime(?) AND datetime(fetched_at) < datetime(?)
	`, start, end).Scan(&count)
	return count, err
}
