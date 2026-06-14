package sqlite

import (
	"database/sql"
	"encoding/base64"
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
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags
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

	// Cursor clause
	if f.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(f.Cursor)
		if err == nil {
			timeStr := cursorTime.UTC().Format("2006-01-02 15:04:05")
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
			&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags,
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
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		WHERE i.id = ?
	`, id).Scan(
		&item.ID, &item.SourceID, &item.OriginSource, &item.Title, &item.URL, &item.Summary, &item.Content,
		&item.Category, &item.SourceCategory, &pubAt, &item.FetchedAt, &item.ReadStatus, &item.Starred, &item.Tags,
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
	res, err := d.db.Exec(`
		INSERT OR IGNORE INTO scraped_items (source_id, origin_source, title, url, summary, content, category, published_at, fetched_at, read_status, starred, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 0, 0, '')
	`, item.SourceID, item.OriginSource, item.Title, item.URL, item.Summary, item.Content, item.Category, item.PublishedAt)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil || rows == 0 {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) MarkItemRead(id int64, readStatus int) error {
	_, err := d.db.Exec("UPDATE scraped_items SET read_status = ? WHERE id = ?", readStatus, id)
	return err
}

func (d *Database) ToggleItemStarred(id int64, starred int) error {
	_, err := d.db.Exec("UPDATE scraped_items SET starred = ? WHERE id = ?", starred, id)
	return err
}

func (d *Database) CleanupOldData() error {
	// Clean up read, unstarred items older than 90 days
	_, err := d.db.Exec(`
		DELETE FROM scraped_items
		WHERE fetched_at < datetime('now', '-90 days')
		AND read_status = 1
		AND starred = 0
	`)
	if err != nil {
		return err
	}

	// Clean up fetch logs older than 30 days
	_, err = d.db.Exec(`
		DELETE FROM fetch_logs
		WHERE started_at < datetime('now', '-30 days')
	`)
	if err != nil {
		return err
	}

	// Reclaim space
	_, err = d.db.Exec("VACUUM")
	return err
}

// --- Item Stats helpers ---

func (d *Database) CountItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE fetched_at >= datetime('now', '-24 hours')").Scan(&count)
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

func (d *Database) CountItemsByCategory() (map[string]int, error) {
	rows, err := d.db.Query("SELECT category, COUNT(*) FROM scraped_items WHERE fetched_at >= datetime('now', '-24 hours') GROUP BY category")
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
		WHERE si.fetched_at >= datetime('now', '-24 hours')
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
		WHERE si.fetched_at >= datetime('now', '-24 hours')
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
