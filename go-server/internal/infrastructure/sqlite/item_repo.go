package sqlite

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// --- Scraped Items ---

func encodeCursor(t time.Time, id int64) string {
	str := fmt.Sprintf("%s|%d", t.Format(time.RFC3339Nano), id)
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func decodeCursor(cursorStr string) (time.Time, int64, error) {
	if cursorStr == "" {
		return time.Time{}, 0, nil
	}
	b, err := base64.StdEncoding.DecodeString(cursorStr)
	if err != nil {
		return time.Time{}, 0, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor ID: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor time: %w", err)
	}
	return t, id, nil
}

func (d *Database) GetScrapedItems(f ItemsFilter) ([]ScrapedItem, string, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}

	query := `
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags, COALESCE(i.metadata, '{}')
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		WHERE 1=1
	`
	var args []any

	if f.Category != "" && f.Category != "all" {
		query += " AND i.category = ?"
		args = append(args, f.Category)
	}

	if f.SourceCategory != "" && f.SourceCategory != "all" {
		query += " AND s.category = ?"
		args = append(args, f.SourceCategory)
	}

	if f.SourceID != "" {
		query += " AND i.source_id = ?"
		args = append(args, f.SourceID)
	}

	if f.ContentType != "" && f.ContentType != "all" {
		query, args = appendContentTypeFilter(query, args, f.ContentType)
	}

	if f.Origin != "" {
		query += " AND i.origin_source = ?"
		args = append(args, f.Origin)
	}

	if f.Q != "" {
		query += " AND (i.title LIKE ? OR i.summary LIKE ? OR i.content LIKE ?)"
		likeArg := "%" + f.Q + "%"
		args = append(args, likeArg, likeArg, likeArg)
	}

	if f.Starred != nil {
		query += " AND i.starred = ?"
		args = append(args, *f.Starred)
	}

	if f.ReadStatus != nil {
		query += " AND i.read_status = ?"
		args = append(args, *f.ReadStatus)
	}

	if f.Days > 0 {
		query += " AND datetime(COALESCE(i.published_at, i.fetched_at)) >= datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", f.Days))
	}

	if !f.Since.IsZero() {
		query += " AND datetime(i.fetched_at) >= datetime(?)"
		args = append(args, utcTimestamp(f.Since))
	}

	// Cursor clause
	if f.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(f.Cursor)
		if err == nil {
			timeStr := utcTimestamp(cursorTime)
			query += " AND (COALESCE(i.published_at, i.fetched_at) < ? OR (COALESCE(i.published_at, i.fetched_at) = ? AND i.id < ?))"
			args = append(args, timeStr, timeStr, cursorID)
		}
	}

	// Order by COALESCE(published_at, fetched_at) DESC, id DESC
	query += " ORDER BY COALESCE(i.published_at, i.fetched_at) DESC, i.id DESC LIMIT ?"
	args = append(args, f.Limit+1) // fetch one extra to see if there is a next page

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var list []ScrapedItem
	for rows.Next() {
		var item ScrapedItem
		var pubAt sql.NullTime
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
			&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags, &item.Metadata,
		)
		if err != nil {
			return nil, "", err
		}
		if pubAt.Valid {
			item.PublishedAt = &pubAt.Time
		}
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

func (d *Database) GetScrapedItem(id int64) (*ScrapedItem, error) {
	var item ScrapedItem
	var pubAt sql.NullTime
	err := d.db.QueryRow(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags, COALESCE(i.metadata, '{}')
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		WHERE i.id = ?
	`, id).Scan(
		&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
		&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags, &item.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if pubAt.Valid {
		item.PublishedAt = &pubAt.Time
	}
	return &item, nil
}

func (d *Database) InsertScrapedItem(item ScrapedItem) (int64, error) {
	metadata := item.Metadata
	if metadata == "" {
		metadata = "{}"
	}
	res, err := d.db.Exec(`
		INSERT OR IGNORE INTO scraped_items (source_id, origin_source, title, url, summary, content, category, published_at, fetched_at, read_status, starred, tags, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 0, 0, ?, ?)
	`, item.SourceID, item.OriginSource, item.Title, item.URL, item.Summary, item.Content, item.Category, utcTimestampArg(item.PublishedAt), item.Tags, metadata)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil || rows == 0 {
		return 0, err
	}
	return res.LastInsertId()
}

// UpsertScrapedItem inserts a new scraped item or updates an existing item with the
// same URL when the incoming content is richer (longer). It returns (id, created, err).
func (d *Database) UpsertScrapedItem(item ScrapedItem) (int64, bool, error) {
	id, err := d.InsertScrapedItem(item)
	if err != nil {
		return 0, false, err
	}
	if id > 0 {
		return id, true, nil
	}

	var existingID int64
	var existingContent string
	err = d.db.QueryRow("SELECT id, COALESCE(content, '') FROM scraped_items WHERE url = ?", item.URL).Scan(&existingID, &existingContent)
	if err != nil {
		return 0, false, err
	}

	if len([]rune(strings.TrimSpace(item.Content))) > len([]rune(strings.TrimSpace(existingContent))) {
		metadata := item.Metadata
		if metadata == "" {
			metadata = "{}"
		}
		_, err = d.db.Exec(`
			UPDATE scraped_items
			SET title = CASE WHEN ? != '' THEN ? ELSE title END,
			    summary = CASE WHEN ? != '' THEN ? ELSE summary END,
			    content = ?,
			    category = CASE WHEN ? != '' THEN ? ELSE category END,
			    metadata = CASE WHEN ? != '{}' AND ? != '' THEN ? ELSE metadata END
			WHERE id = ?
		`, item.Title, item.Title, item.Summary, item.Summary, item.Content, item.Category, item.Category, metadata, metadata, metadata, existingID)
		if err != nil {
			return existingID, false, err
		}
	}
	return existingID, false, nil
}

// UpdateSourceItemMedia upgrades an existing item with audio metadata captured
// from its feed enclosure, without disturbing the fetched content or other
// metadata keys (full-text status, transcripts, ...).
func (d *Database) UpdateSourceItemMedia(sourceID, url, category, metadata string) error {
	if metadata == "" || metadata == "{}" {
		return nil
	}
	var existingRaw string
	err := d.db.QueryRow(
		`SELECT COALESCE(metadata, '{}') FROM scraped_items WHERE source_id = ? AND url = ? LIMIT 1`,
		sourceID, url,
	).Scan(&existingRaw)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	merged := map[string]any{}
	if existingRaw != "" {
		_ = json.Unmarshal([]byte(existingRaw), &merged)
	}
	incoming := map[string]any{}
	_ = json.Unmarshal([]byte(metadata), &incoming)
	for k, v := range incoming {
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		merged[k] = v
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		raw = []byte(metadata)
	}
	_, err = d.db.Exec(
		`UPDATE scraped_items SET category = ?, metadata = ? WHERE source_id = ? AND url = ?`,
		category, string(raw), sourceID, url,
	)
	return err
}

// SetSourceItemsCategory rewrites the content type of every item that belongs
// to a source. A source type or default content type change must be reflected
// immediately, not only for items scraped after the change.
func (d *Database) SetSourceItemsCategory(sourceID, category string) error {
	category = strings.TrimSpace(category)
	if sourceID == "" || category == "" || category == "auto" {
		return nil
	}
	_, err := d.db.Exec(
		`UPDATE scraped_items SET category = ? WHERE source_id = ?`,
		category, sourceID,
	)
	return err
}

// UpdateScrapedItemContent backfills full extracted content and metadata for an existing item.
func (d *Database) UpdateScrapedItemContent(id int64, content string, metadata string) error {
	if metadata != "" && metadata != "{}" {
		_, err := d.db.Exec("UPDATE scraped_items SET content = ?, metadata = ? WHERE id = ?", content, metadata, id)
		return err
	}
	_, err := d.db.Exec("UPDATE scraped_items SET content = ? WHERE id = ?", content, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanScrapedItemWithAI(s rowScanner) (ScrapedItemWithAI, error) {
	var it ScrapedItemWithAI
	var pubAt sql.NullTime
	var aiCat, aiSub, aiSummary, aiComment, aiTags, aiModel sql.NullString
	var aiScore sql.NullInt64
	var aiProcessed sql.NullTime

	err := s.Scan(
		&it.ID, &it.SourceID, &it.OriginSource, &it.Title, &it.URL, &it.Summary, &it.Content,
		&it.Category, &it.SourceCategory, &pubAt, &it.FetchedAt, &it.ReadStatus, &it.Starred, &it.Tags, &it.Metadata,
		&aiCat, &aiSub, &aiScore, &aiSummary, &aiComment, &aiTags, &aiModel, &aiProcessed,
	)
	if err != nil {
		return it, err
	}
	if pubAt.Valid {
		it.PublishedAt = &pubAt.Time
	}
	if aiCat.Valid {
		it.AICategory = aiCat.String
	}
	if aiSub.Valid {
		it.AISubcategory = aiSub.String
	}
	if aiScore.Valid {
		it.QualityScore = int(aiScore.Int64)
	}
	if aiSummary.Valid {
		it.AISummary = aiSummary.String
	}
	if aiComment.Valid {
		it.AIComment = aiComment.String
	}
	if aiTags.Valid {
		it.AITags = aiTags.String
	}
	if aiModel.Valid {
		it.AIModelUsed = aiModel.String
	}
	if aiProcessed.Valid {
		t := aiProcessed.Time
		it.AIProcessedAt = &t
	}
	return it, nil
}

func appendContentTypeFilter(query string, args []any, contentType string) (string, []any) {
	if contentType == "" || strings.EqualFold(contentType, "all") {
		return query, args
	}
	switch strings.ToLower(contentType) {
	case "rss":
		query += " AND i.category IN ('article', 'news', 'blog') AND s.type NOT IN ('podcast', 'youtube')"
	case "podcast":
		query += " AND (i.category = 'podcast' OR s.type = 'podcast')"
	case "youtube":
		query += " AND (i.category = 'youtube' OR s.type = 'youtube')"
	case "news":
		query += " AND i.category IN ('article', 'news')"
	case "blog":
		query += " AND i.category IN ('blog', 'article')"
	default:
		query += " AND i.category = ?"
		args = append(args, strings.ToLower(contentType))
	}
	return query, args
}

// QueryUnifiedFeed queries items across all source types with optional AI analysis joined.
func (d *Database) QueryUnifiedFeed(f ItemsFilter) ([]ScrapedItemWithAI, string, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}

	query := `
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags, COALESCE(i.metadata, '{}'),
		       a.ai_category, a.ai_subcategory, a.quality_score, a.ai_summary, a.ai_comment, a.ai_tags, a.model_used, a.processed_at
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		LEFT JOIN ai_analyses a ON i.id = a.item_id
		WHERE 1=1
	`
	var args []any

	if f.ContentType != "" && f.ContentType != "all" {
		query, args = appendContentTypeFilter(query, args, f.ContentType)
	}

	if f.Category != "" && f.Category != "all" {
		query += " AND (s.category = ? OR a.ai_category = ? OR i.category = ?)"
		args = append(args, f.Category, f.Category, f.Category)
	}

	if f.SourceCategory != "" && f.SourceCategory != "all" {
		query += " AND s.category = ?"
		args = append(args, f.SourceCategory)
	}

	if f.SourceID != "" {
		query += " AND i.source_id = ?"
		args = append(args, f.SourceID)
	}

	if f.Origin != "" {
		query += " AND i.origin_source = ?"
		args = append(args, f.Origin)
	}

	if f.ScoreMin > 0 {
		query += " AND COALESCE(a.quality_score, 0) >= ?"
		args = append(args, f.ScoreMin)
	}

	if f.Days > 0 {
		query += " AND datetime(COALESCE(i.published_at, i.fetched_at)) >= datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", f.Days))
	}

	if f.Q != "" {
		query += " AND (i.title LIKE ? OR i.summary LIKE ? OR i.content LIKE ?)"
		likeArg := "%" + f.Q + "%"
		args = append(args, likeArg, likeArg, likeArg)
	}

	if f.Starred != nil {
		query += " AND i.starred = ?"
		args = append(args, *f.Starred)
	}

	if f.ReadStatus != nil {
		query += " AND i.read_status = ?"
		args = append(args, *f.ReadStatus)
	}

	if f.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(f.Cursor)
		if err == nil {
			timeStr := utcTimestamp(cursorTime)
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
		it, err := scanScrapedItemWithAI(rows)
		if err != nil {
			return nil, "", err
		}
		list = append(list, it)
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

// GetScrapedItemWithAI fetches a single item along with optional AI analysis.
func (d *Database) GetScrapedItemWithAI(id int64) (*ScrapedItemWithAI, error) {
	row := d.db.QueryRow(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags, COALESCE(i.metadata, '{}'),
		       a.ai_category, a.ai_subcategory, a.quality_score, a.ai_summary, a.ai_comment, a.ai_tags, a.model_used, a.processed_at
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		LEFT JOIN ai_analyses a ON i.id = a.item_id
		WHERE i.id = ?
	`, id)
	it, err := scanScrapedItemWithAI(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (d *Database) MarkItemRead(id int64, readStatus int) error {
	_, err := d.db.Exec("UPDATE scraped_items SET read_status = ? WHERE id = ?", readStatus, id)
	return err
}

func (d *Database) ToggleItemStarred(id int64, starred int) error {
	_, err := d.db.Exec("UPDATE scraped_items SET starred = ? WHERE id = ?", starred, id)
	return err
}

// CleanupOldData enforces the retention policy for ingested content.
//
//   - scraped_items fetched more than 30 days ago are deleted, except items the
//     user starred.
//   - the AI analyses of those deleted items are removed alongside them (done
//     explicitly rather than relying only on ON DELETE CASCADE).
//   - fetch_logs older than 30 days are deleted.
//
// AI daily reports (ai_daily_reports) and webpage extract captures
// (extract_cache) are derived history, not raw ingested data, so they are
// deliberately kept.
func (d *Database) CleanupOldData() error {
	const staleItemCond = `
		fetched_at < datetime('now', '-30 days')
		AND starred = 0`

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear AI scores first so no orphaned analyses survive even if foreign
	// key cascades are disabled.
	if _, err := tx.Exec(`
		DELETE FROM ai_analyses
		WHERE item_id IN (SELECT id FROM scraped_items WHERE ` + staleItemCond + `)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		DELETE FROM scraped_items
		WHERE ` + staleItemCond + `
	`); err != nil {
		return err
	}

	// Fetch logs are operational history with no long-term value.
	if _, err := tx.Exec(`
		DELETE FROM fetch_logs
		WHERE started_at < datetime('now', '-30 days')
	`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Reclaim space (VACUUM cannot run inside a transaction).
	_, err = d.db.Exec("VACUUM")
	return err
}

// --- Item Stats helpers ---

func (d *Database) CountItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items").Scan(&count)
	return count, err
}

func (d *Database) CountUnreadItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE read_status = 0").Scan(&count)
	return count, err
}

func (d *Database) CountStarredItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE starred = 1").Scan(&count)
	return count, err
}

// CountItemsSince counts the items fetched at or after t.
func (d *Database) CountItemsSince(t time.Time) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE datetime(fetched_at) >= datetime(?)", utcTimestamp(t)).Scan(&count)
	return count, err
}

func (d *Database) CountItemsByCategory() (map[string]int, error) {
	rows, err := d.db.Query("SELECT category, COUNT(*) FROM scraped_items GROUP BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		result[category] = count
	}
	return result, rows.Err()
}

func (d *Database) CountItemsBySourceCategory() (map[string]int, error) {
	rows, err := d.db.Query(`
		SELECT s.category, COUNT(*) 
		FROM scraped_items si 
		JOIN sources s ON si.source_id = s.id 
		WHERE si.read_status = 0
		GROUP BY s.category
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		result[category] = count
	}
	return result, rows.Err()
}

func (d *Database) GetDistinctSourceCategories() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT s.category 
		FROM scraped_items si 
		JOIN sources s ON si.source_id = s.id 
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (d *Database) GetLatestScrapedItem() (int64, string, error) {
	var id int64
	var title string
	err := d.db.QueryRow("SELECT id, title FROM scraped_items ORDER BY id DESC LIMIT 1").Scan(&id, &title)
	return id, title, err
}

func (d *Database) GetScrapedItemByURL(url string) (int64, string, error) {
	var id int64
	var title string
	err := d.db.QueryRow("SELECT id, title FROM scraped_items WHERE url = ?", url).Scan(&id, &title)
	return id, title, err
}

func (d *Database) ItemExistsByURL(url string) (bool, error) {
	var exists bool
	err := d.db.QueryRow("SELECT EXISTS(SELECT 1 FROM scraped_items WHERE url = ?)", url).Scan(&exists)
	return exists, err
}

// GetScrapedItemFullByURL loads an item by its stored URL, including source
// category and metadata. Unlike GetScrapedItemByURL it returns the whole row.
func (d *Database) GetScrapedItemFullByURL(url string) (*ScrapedItem, error) {
	var it ScrapedItem
	var pubAt sql.NullTime
	err := d.db.QueryRow(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category,
		       COALESCE(s.category, ''), i.published_at, i.fetched_at, i.read_status, i.starred, i.tags,
		       COALESCE(i.metadata, '{}')
		FROM scraped_items i
		LEFT JOIN sources s ON i.source_id = s.id
		WHERE i.url = ?
		LIMIT 1
	`, url).Scan(
		&it.ID, &it.SourceID, &it.OriginSource, &it.Title, &it.URL, &it.Summary, &it.Content, &it.Category,
		&it.SourceCategory, &pubAt, &it.FetchedAt, &it.ReadStatus, &it.Starred, &it.Tags, &it.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if pubAt.Valid {
		it.PublishedAt = &pubAt.Time
	}
	return &it, nil
}

// FindScrapedItemByVideoID resolves a YouTube video by identity, not just by
// URL spelling: youtu.be/ID, watch?v=ID&t=..., shorts/ID and the canonical watch
// URL all land on the same item. Older rows carried video_id in metadata, which
// keeps them reachable too.
func (d *Database) FindScrapedItemByVideoID(videoID string) (*ScrapedItem, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, nil
	}
	canonical := "https://www.youtube.com/watch?v=" + videoID
	var id int64
	err := d.db.QueryRow(`
		SELECT id FROM scraped_items
		WHERE url = ? OR url = ? OR (COALESCE(metadata, '') LIKE ?)
		ORDER BY id ASC LIMIT 1
	`, canonical, "https://youtu.be/"+videoID, `%"video_id":"`+videoID+`"%`).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d.GetScrapedItem(id)
}

// ListSourceItems returns every item that belongs to a source. Used for
// backfilling entries stored before full-text/transcript capture was enabled.
func (d *Database) ListSourceItems(sourceID string) ([]ScrapedItem, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category,
		       COALESCE(s.category, ''), i.published_at, i.fetched_at, i.read_status, i.starred, i.tags,
		       COALESCE(i.metadata, '{}')
		FROM scraped_items i
		LEFT JOIN sources s ON i.source_id = s.id
		WHERE i.source_id = ?
		ORDER BY COALESCE(i.published_at, i.fetched_at) DESC, i.id DESC
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScrapedItem
	for rows.Next() {
		var it ScrapedItem
		var pubAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.SourceID, &it.OriginSource, &it.Title, &it.URL, &it.Summary,
			&it.Content, &it.Category, &it.SourceCategory, &pubAt, &it.FetchedAt, &it.ReadStatus,
			&it.Starred, &it.Tags, &it.Metadata); err != nil {
			return nil, err
		}
		if pubAt.Valid {
			it.PublishedAt = &pubAt.Time
		}
		list = append(list, it)
	}
	return list, rows.Err()
}

// UpdateScrapedItemFromScrape writes refreshed scrape fields without touching
// user state (read_status, starred, tags). The item ID is stable, so bookmarks,
// read markers and AI analyses keep pointing at the same row.
func (d *Database) UpdateScrapedItemFromScrape(id int64, title, summary, content, category, metadata string, publishedAt *time.Time) error {
	if metadata == "" {
		metadata = "{}"
	}
	_, err := d.db.Exec(`
		UPDATE scraped_items
		SET title = CASE WHEN ? != '' THEN ? ELSE title END,
		    summary = CASE WHEN ? != '' THEN ? ELSE summary END,
		    content = CASE WHEN ? != '' THEN ? ELSE content END,
		    category = CASE WHEN ? != '' THEN ? ELSE category END,
		    metadata = ?,
		    published_at = COALESCE(?, published_at)
		WHERE id = ?
	`, title, title, summary, summary, content, content, category, category, metadata,
		utcTimestampArg(publishedAt), id)
	return err
}

// ExecRaw is a narrow escape hatch for tests and one-off maintenance commands.
func (d *Database) ExecRaw(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}
