package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// WAL mode: concurrency reading/writing
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	// Busy timeout: wait up to 5 seconds if database is locked
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(2)

	d := &Database{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := d.seedDefaultSources(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seeding default sources failed: %w", err)
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	var version int
	err := d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query user_version: %w", err)
	}

	if version < 1 {
		createSourcesSQL := `
		CREATE TABLE IF NOT EXISTS sources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			schedule TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			default_category TEXT DEFAULT 'auto',
			config TEXT DEFAULT '{}',
			last_etag TEXT,
			last_modified TEXT,
			last_fetch_at DATETIME,
			last_fetch_status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`

		createItemsSQL := `
		CREATE TABLE IF NOT EXISTS scraped_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			origin_source TEXT,
			title TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			summary TEXT,
			content TEXT,
			category TEXT NOT NULL,
			published_at DATETIME,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			read_status INTEGER DEFAULT 0,
			starred INTEGER DEFAULT 0,
			tags TEXT DEFAULT '',
			FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
		);`

		createLogsSQL := `
		CREATE TABLE IF NOT EXISTS fetch_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME,
			status TEXT NOT NULL,
			items_found INTEGER DEFAULT 0,
			items_added INTEGER DEFAULT 0,
			error_message TEXT,
			FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
		);`

		indices := []string{
			"CREATE INDEX IF NOT EXISTS idx_items_filter ON scraped_items(category, published_at DESC);",
			"CREATE INDEX IF NOT EXISTS idx_items_origin ON scraped_items(origin_source, published_at DESC);",
			"CREATE INDEX IF NOT EXISTS idx_items_read ON scraped_items(read_status, fetched_at DESC);",
			"CREATE INDEX IF NOT EXISTS idx_items_starred ON scraped_items(starred, fetched_at DESC) WHERE starred = 1;",
			"CREATE INDEX IF NOT EXISTS idx_logs_source ON fetch_logs(source_id, started_at DESC);",
		}

		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(createSourcesSQL); err != nil {
			return err
		}
		if _, err := tx.Exec(createItemsSQL); err != nil {
			return err
		}
		if _, err := tx.Exec(createLogsSQL); err != nil {
			return err
		}
		for _, idx := range indices {
			if _, err := tx.Exec(idx); err != nil {
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		if _, err := d.db.Exec("PRAGMA user_version = 1"); err != nil {
			return err
		}
		version = 1
	}

	if version < 2 {
		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		// Add category column to sources table. We ignore errors if it already exists.
		_, _ = tx.Exec("ALTER TABLE sources ADD COLUMN category TEXT DEFAULT 'General'")

		if err := tx.Commit(); err != nil {
			return err
		}

		if _, err := d.db.Exec("PRAGMA user_version = 2"); err != nil {
			return err
		}
		version = 2
	}

	if version < 3 {
		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		createAnalysesSQL := `
		CREATE TABLE IF NOT EXISTS ai_analyses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL UNIQUE,
			ai_category TEXT NOT NULL,
			ai_subcategory TEXT,
			quality_score INTEGER NOT NULL,
			ai_summary TEXT,
			ai_comment TEXT,
			ai_tags TEXT,
			model_used TEXT,
			processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(item_id) REFERENCES scraped_items(id) ON DELETE CASCADE
		);`

		createDailyReportsSQL := `
		CREATE TABLE IF NOT EXISTS ai_daily_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_date TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			total_items INTEGER DEFAULT 0,
			quality_items INTEGER DEFAULT 0,
			categories_summary TEXT,
			model_used TEXT,
			generated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`

		indices := []string{
			"CREATE INDEX IF NOT EXISTS idx_ai_category ON ai_analyses(ai_category, quality_score DESC);",
			"CREATE INDEX IF NOT EXISTS idx_ai_score ON ai_analyses(quality_score DESC, processed_at DESC);",
			"CREATE INDEX IF NOT EXISTS idx_ai_processed ON ai_analyses(processed_at DESC);",
		}

		if _, err := tx.Exec(createAnalysesSQL); err != nil {
			return err
		}
		if _, err := tx.Exec(createDailyReportsSQL); err != nil {
			return err
		}
		for _, idx := range indices {
			if _, err := tx.Exec(idx); err != nil {
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		if _, err := d.db.Exec("PRAGMA user_version = 3"); err != nil {
			return err
		}
		version = 3
	}

	if version < 4 {
		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		createSettingsSQL := `
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`

		if _, err := tx.Exec(createSettingsSQL); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		if _, err := d.db.Exec("PRAGMA user_version = 4"); err != nil {
			return err
		}
		version = 4
	}
	return nil
}

func (d *Database) seedDefaultSources() error {
	defaultSources := []Source{
		{
			ID:              "aihot",
			Name:            "AI HOT 热点",
			Type:            "rss",
			URL:             "https://aihot.virxact.com/feed/all.xml",
			Schedule:        "0 8,12,18,22 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "AI",
			Config:          `{"full_content":false,"fetch_full_via_scrape":false}`,
		},
		{
			ID:              "hn",
			Name:            "Hacker News",
			Type:            "rss",
			URL:             "https://hnrss.org/frontpage",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "科技新闻",
			Config:          `{"full_content":false,"fetch_full_via_scrape":false}`,
		},
		{
			ID:              "hn_best",
			Name:            "Hacker News Best",
			Type:            "rss",
			URL:             "https://hnrss.org/best",
			Schedule:        "0 9 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		// 国内新闻
		{
			ID:              "chinanews_scroll",
			Name:            "国内新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/scroll-news.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_newscn",
			Name:            "国内新闻-新华网",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/newscn/whxw",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_cctv",
			Name:            "国内新闻-央视",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/weixin/cctvnewscenter",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		// 科技新闻
		{
			ID:              "rsshub_36kr",
			Name:            "科技新闻-36氪",
			Type:            "rss",
			URL:             "https://rsshub.app/36kr/newsflashes",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "ifanr",
			Name:            "科技新闻-爱范儿",
			Type:            "rss",
			URL:             "https://www.ifanr.com/feed",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "solidot",
			Name:            "科技新闻-Solidot",
			Type:            "rss",
			URL:             "https://www.solidot.org/index.rss",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		// 财经新闻
		{
			ID:              "chinanews_finance",
			Name:            "财经新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/finance.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_fortune",
			Name:            "财经新闻-财富中文",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/fortunechina",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_tmt",
			Name:            "财经新闻-钛媒体",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/tmtpost",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		// 国际新闻
		{
			ID:              "chinanews_world",
			Name:            "国际新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/world.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_bbc",
			Name:            "国际新闻-BBC",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/bbc/cn",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_zaobao",
			Name:            "国际新闻-联合早报",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/zaobao/realtime/world",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO sources (id, name, type, url, schedule, enabled, default_category, config, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET category = CASE WHEN sources.category = 'General' OR sources.category = '' THEN excluded.category ELSE sources.category END
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, src := range defaultSources {
		_, err := stmt.Exec(src.ID, src.Name, src.Type, src.URL, src.Schedule, src.Enabled, src.DefaultCategory, src.Config, src.Category)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetSources gets all sources
func (d *Database) GetSources() ([]Source, error) {
	rows, err := d.db.Query(`
		SELECT id, name, type, url, schedule, enabled, default_category, config, 
		       last_etag, last_modified, last_fetch_at, last_fetch_status, category, created_at, updated_at
		FROM sources
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Source
	for rows.Next() {
		var src Source
		var lastFetchAt sql.NullTime
		err := rows.Scan(
			&src.ID, &src.Name, &src.Type, &src.URL, &src.Schedule, &src.Enabled, &src.DefaultCategory, &src.Config,
			&src.LastETag, &src.LastModified, &lastFetchAt, &src.LastFetchStatus, &src.Category, &src.CreatedAt, &src.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastFetchAt.Valid {
			src.LastFetchAt = &lastFetchAt.Time
		}
		list = append(list, src)
	}
	return list, nil
}

// GetSource gets a source by ID
func (d *Database) GetSource(id string) (*Source, error) {
	var src Source
	var lastFetchAt sql.NullTime
	err := d.db.QueryRow(`
		SELECT id, name, type, url, schedule, enabled, default_category, config, 
		       last_etag, last_modified, last_fetch_at, last_fetch_status, category, created_at, updated_at
		FROM sources
		WHERE id = ?
	`, id).Scan(
		&src.ID, &src.Name, &src.Type, &src.URL, &src.Schedule, &src.Enabled, &src.DefaultCategory, &src.Config,
		&src.LastETag, &src.LastModified, &lastFetchAt, &src.LastFetchStatus, &src.Category, &src.CreatedAt, &src.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastFetchAt.Valid {
		src.LastFetchAt = &lastFetchAt.Time
	}
	return &src, nil
}

// InsertSource inserts a source
func (d *Database) InsertSource(src Source) error {
	category := src.Category
	if category == "" {
		category = "General"
	}
	_, err := d.db.Exec(`
		INSERT INTO sources (id, name, type, url, schedule, enabled, default_category, config, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, src.ID, src.Name, src.Type, src.URL, src.Schedule, src.Enabled, src.DefaultCategory, src.Config, category)
	return err
}

// UpdateSource updates a source config
func (d *Database) UpdateSource(src Source) error {
	category := src.Category
	if category == "" {
		category = "General"
	}
	_, err := d.db.Exec(`
		UPDATE sources
		SET name = ?, type = ?, url = ?, schedule = ?, default_category = ?, config = ?, category = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, src.Name, src.Type, src.URL, src.Schedule, src.DefaultCategory, src.Config, category, src.ID)
	return err
}

// DeleteSource deletes a source
func (d *Database) DeleteSource(id string) error {
	_, err := d.db.Exec("DELETE FROM sources WHERE id = ?", id)
	return err
}

// ToggleSource toggles enabled status
func (d *Database) ToggleSource(id string, enabled int) error {
	_, err := d.db.Exec("UPDATE sources SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", enabled, id)
	return err
}

// UpdateSourceFetchStatus updates fetch status after a scrape run
func (d *Database) UpdateSourceFetchStatus(id string, status string, etag, lastMod *string) error {
	var errQuery error
	_, errQuery = d.db.Exec(`
		UPDATE sources
		SET last_fetch_at = CURRENT_TIMESTAMP, last_fetch_status = ?, last_etag = ?, last_modified = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, etag, lastMod, id)
	return errQuery
}

// --- Scraped Items ---

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
	var tStr string
	var id int64
	_, err = fmt.Sscanf(string(b), "%[^|]|%d", &tStr, &id)
	if err != nil {
		return time.Time{}, 0, err
	}
	t, err := time.Parse(time.RFC3339Nano, tStr)
	if err != nil {
		return time.Time{}, 0, err
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
			query += " AND (i.fetched_at < ? OR (i.fetched_at = ? AND i.id < ?))"
			args = append(args, cursorTime, cursorTime, cursorID)
		}
	}

	// Order by fetched_at DESC, id DESC
	query += " ORDER BY i.fetched_at DESC, i.id DESC LIMIT ?"
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
		nextCursor = encodeCursor(nextItem.FetchedAt, nextItem.ID)
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

// --- Fetch Logs ---

func (d *Database) InsertFetchLog(log FetchLog) (int64, error) {
	res, err := d.db.Exec(`
		INSERT INTO fetch_logs (source_id, started_at, finished_at, status, items_found, items_added, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, log.SourceID, log.StartedAt, log.FinishedAt, log.Status, log.ItemsFound, log.ItemsAdded, log.ErrorMessage)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) UpdateFetchLog(log FetchLog) error {
	_, err := d.db.Exec(`
		UPDATE fetch_logs
		SET finished_at = ?, status = ?, items_found = ?, items_added = ?, error_message = ?
		WHERE id = ?
	`, log.FinishedAt, log.Status, log.ItemsFound, log.ItemsAdded, log.ErrorMessage, log.ID)
	return err
}

func (d *Database) GetFetchLogs(sourceID string, limit int) ([]FetchLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, source_id, started_at, finished_at, status, items_found, items_added, error_message
		FROM fetch_logs
		WHERE 1=1
	`
	var args []any
	if sourceID != "" {
		query += " AND source_id = ?"
		args = append(args, sourceID)
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []FetchLog
	for rows.Next() {
		var log FetchLog
		var finishedAt sql.NullTime
		err := rows.Scan(
			&log.ID, &log.SourceID, &log.StartedAt, &finishedAt, &log.Status, &log.ItemsFound, &log.ItemsAdded, &log.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			log.FinishedAt = &finishedAt.Time
		}
		list = append(list, log)
	}
	return list, nil
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

// --- AI Analyses CRUD ---

// InsertAIAnalysis inserts or updates an AI analysis.
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

// GetAIAnalysis gets an AI analysis by item ID.
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

// GetUnanalyzedItems gets scraped items that have not been analyzed by AI.
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

type AIItemsFilter struct {
	AICategory string
	ScoreMin   int
	Days       int
	Limit      int
	Cursor     string
}

// GetScrapedItemsWithAI returns scraped items that have AI analyses, matching filter.
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

	if f.ScoreMin > 0 {
		query += " AND a.quality_score >= ?"
		args = append(args, f.ScoreMin)
	}

	if f.Days > 0 {
		query += " AND a.processed_at >= datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", f.Days))
	}

	// Cursor clause using processed_at
	if f.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(f.Cursor)
		if err == nil {
			query += " AND (a.processed_at < ? OR (a.processed_at = ? AND i.id < ?))"
			args = append(args, cursorTime, cursorTime, cursorID)
		}
	}

	query += " ORDER BY a.processed_at DESC, i.id DESC LIMIT ?"
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
		nextCursor = encodeCursor(*nextItem.AIProcessedAt, nextItem.ID)
		list = list[:f.Limit]
	}

	return list, nextCursor, nil
}

// GetAICategories returns a summary of all AI-assigned categories.
func (d *Database) GetAICategories() ([]AICategoryStat, error) {
	rows, err := d.db.Query(`
		SELECT ai_category, COUNT(*), AVG(quality_score)
		FROM ai_analyses
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

// --- AI Daily Reports CRUD ---

// InsertAIDailyReport inserts or replaces a daily report.
func (d *Database) InsertAIDailyReport(r AIDailyReport) error {
	_, err := d.db.Exec(`
		INSERT INTO ai_daily_reports (report_date, title, content, total_items, quality_items, categories_summary, model_used, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(report_date) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			total_items = excluded.total_items,
			quality_items = excluded.quality_items,
			categories_summary = excluded.categories_summary,
			model_used = excluded.model_used,
			generated_at = CURRENT_TIMESTAMP
	`, r.ReportDate, r.Title, r.Content, r.TotalItems, r.QualityItems, r.CategoriesSummary, r.ModelUsed)
	return err
}

// GetAIDailyReport retrieves a daily report for a specific date (YYYY-MM-DD).
func (d *Database) GetAIDailyReport(date string) (*AIDailyReport, error) {
	var r AIDailyReport
	var generatedAt time.Time
	err := d.db.QueryRow(`
		SELECT id, report_date, title, content, total_items, quality_items, categories_summary, model_used, generated_at
		FROM ai_daily_reports
		WHERE report_date = ?
	`, date).Scan(&r.ID, &r.ReportDate, &r.Title, &r.Content, &r.TotalItems, &r.QualityItems, &r.CategoriesSummary, &r.ModelUsed, &generatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.GeneratedAt = generatedAt
	return &r, nil
}

// GetAIDailyReports retrieves the list of recent daily reports.
func (d *Database) GetAIDailyReports(limit int) ([]AIDailyReport, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.db.Query(`
		SELECT id, report_date, title, content, total_items, quality_items, categories_summary, model_used, generated_at
		FROM ai_daily_reports
		ORDER BY report_date DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AIDailyReport
	for rows.Next() {
		var r AIDailyReport
		var generatedAt time.Time
		err := rows.Scan(&r.ID, &r.ReportDate, &r.Title, &r.Content, &r.TotalItems, &r.QualityItems, &r.CategoriesSummary, &r.ModelUsed, &generatedAt)
		if err != nil {
			return nil, err
		}
		r.GeneratedAt = generatedAt
		list = append(list, r)
	}
	return list, nil
}

// GetQualityItemsForDate gets quality items analyzed on a specific date (YYYY-MM-DD).
func (d *Database) GetQualityItemsForDate(date string, scoreMin int) ([]ScrapedItemWithAI, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.source_id, i.origin_source, i.title, i.url, i.summary, i.content, i.category, s.category, i.published_at, i.fetched_at, i.read_status, i.starred, i.tags,
		       a.ai_category, a.ai_subcategory, a.quality_score, a.ai_summary, a.ai_comment, a.ai_tags, a.model_used, a.processed_at
		FROM scraped_items i
		JOIN sources s ON i.source_id = s.id
		JOIN ai_analyses a ON i.id = a.item_id
		WHERE strftime('%Y-%m-%d', a.processed_at) = ? AND a.quality_score >= ?
		ORDER BY a.quality_score DESC, i.id DESC
	`, date, scoreMin)
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

// GetTotalItemsCountForDate gets total fetched items count on a specific date (YYYY-MM-DD).
func (d *Database) GetTotalItemsCountForDate(date string) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM scraped_items
		WHERE strftime('%Y-%m-%d', fetched_at) = ?
	`, date).Scan(&count)
	return count, err
}

// GetSetting returns the value of a setting from the settings table.
// If the key does not exist, it returns the defaultVal.
func (d *Database) GetSetting(key string, defaultVal string) (string, error) {
	var val string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return defaultVal, nil
	}
	if err != nil {
		return defaultVal, err
	}
	return val, nil
}

// SaveSetting saves a key-value pair to the settings table.
func (d *Database) SaveSetting(key string, value string) error {
	_, err := d.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// LoadAISettings reads AI configurations from database, falling back to env/defaults and populating them back to DB if missing.
func (d *Database) LoadAISettings(env AISettings) (AISettings, error) {
	var s AISettings
	var err error

	// 1. Enabled
	enabledStr, err := d.GetSetting("ai_enabled", "")
	if err != nil {
		return s, err
	}
	if enabledStr == "" {
		s.Enabled = env.Enabled
	} else {
		s.Enabled = (enabledStr == "true" || enabledStr == "1")
	}

	// 2. Provider
	s.Provider, err = d.GetSetting("ai_provider", env.Provider)
	if err != nil {
		return s, err
	}

	// 3. APIKey
	s.APIKey, err = d.GetSetting("ai_api_key", env.APIKey)
	if err != nil {
		return s, err
	}

	// 4. Model
	s.Model, err = d.GetSetting("ai_model", env.Model)
	if err != nil {
		return s, err
	}

	// 5. BaseURL
	s.BaseURL, err = d.GetSetting("ai_base_url", env.BaseURL)
	if err != nil {
		return s, err
	}

	// 6. QualityThreshold
	thresholdStr, err := d.GetSetting("ai_quality_threshold", "")
	if err != nil {
		return s, err
	}
	if thresholdStr == "" {
		s.QualityThreshold = env.QualityThreshold
		if s.QualityThreshold == 0 {
			s.QualityThreshold = 7
		}
	} else {
		if val, err := strconv.Atoi(thresholdStr); err == nil {
			s.QualityThreshold = val
		} else {
			s.QualityThreshold = 7
		}
	}

	// 7. SystemPrompt
	s.SystemPrompt, err = d.GetSetting("ai_system_prompt", DefaultSystemPrompt)
	if err != nil {
		return s, err
	}

	// 8. DailyPrompt
	s.DailyPrompt, err = d.GetSetting("ai_daily_prompt", DefaultDailyPrompt)
	if err != nil {
		return s, err
	}

	// 9. Provider profiles
	profilesJSON, err := d.GetSetting("ai_provider_profiles", "")
	if err != nil {
		return s, err
	}
	if profilesJSON != "" {
		_ = json.Unmarshal([]byte(profilesJSON), &s.Profiles)
	}
	s.ActiveProfileID, err = d.GetSetting("ai_active_profile_id", "")
	if err != nil {
		return s, err
	}
	if len(s.Profiles) == 0 {
		s.Profiles = []AIProviderProfile{
			{
				ID:       "default",
				Name:     "默认服务商",
				Provider: s.Provider,
				APIKey:   s.APIKey,
				Model:    s.Model,
				BaseURL:  s.BaseURL,
			},
		}
		s.ActiveProfileID = "default"
	} else if s.ActiveProfileID == "default" {
		for i := range s.Profiles {
			if s.Profiles[i].ID == "default" {
				s.Profiles[i].Provider = s.Provider
				s.Profiles[i].APIKey = s.APIKey
				s.Profiles[i].Model = s.Model
				s.Profiles[i].BaseURL = s.BaseURL
				break
			}
		}
	}

	// Normalize settings (trim spaces and fix custom model prefix)
	s = NormalizeAISettings(s)

	// Save clean/normalized settings to database
	enabledDBVal := "false"
	if s.Enabled {
		enabledDBVal = "true"
	}
	_ = d.SaveSetting("ai_enabled", enabledDBVal)
	_ = d.SaveSetting("ai_provider", s.Provider)
	_ = d.SaveSetting("ai_api_key", s.APIKey)
	_ = d.SaveSetting("ai_model", s.Model)
	_ = d.SaveSetting("ai_base_url", s.BaseURL)
	_ = d.SaveSetting("ai_quality_threshold", strconv.Itoa(s.QualityThreshold))
	_ = d.SaveSetting("ai_system_prompt", s.SystemPrompt)
	_ = d.SaveSetting("ai_daily_prompt", s.DailyPrompt)
	_ = d.SaveSetting("ai_active_profile_id", s.ActiveProfileID)
	if profilesBytes, err := json.Marshal(s.Profiles); err == nil {
		_ = d.SaveSetting("ai_provider_profiles", string(profilesBytes))
	}

	return s, nil
}
