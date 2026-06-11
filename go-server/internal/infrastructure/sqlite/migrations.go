package sqlite

import (
	"fmt"
)

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
	if version < 5 {
		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		// Add report_type column to ai_daily_reports and rebuild UNIQUE constraint to (report_date, report_type).
		if _, err := tx.Exec(`
			CREATE TABLE ai_daily_reports_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				report_date TEXT NOT NULL,
				report_type TEXT NOT NULL DEFAULT 'daily',
				title TEXT NOT NULL,
				content TEXT NOT NULL,
				total_items INTEGER DEFAULT 0,
				quality_items INTEGER DEFAULT 0,
				categories_summary TEXT,
				model_used TEXT,
				generated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(report_date, report_type)
			);
		`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO ai_daily_reports_new (id, report_date, report_type, title, content, total_items, quality_items, categories_summary, model_used, generated_at)
			SELECT id, report_date, 'daily', title, content, total_items, quality_items, categories_summary, model_used, generated_at FROM ai_daily_reports`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE ai_daily_reports`); err != nil {
			return err
		}
		if _, err := tx.Exec(`ALTER TABLE ai_daily_reports_new RENAME TO ai_daily_reports`); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		if _, err := d.db.Exec("PRAGMA user_version = 5"); err != nil {
			return err
		}
		version = 5
	}
	return nil
}
