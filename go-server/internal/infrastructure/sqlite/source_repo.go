package sqlite

import (
	"database/sql"
)

// --- Source CRUD ---

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

func (d *Database) DeleteSource(id string) error {
	_, err := d.db.Exec("DELETE FROM sources WHERE id = ?", id)
	return err
}

func (d *Database) ToggleSource(id string, enabled int) error {
	_, err := d.db.Exec("UPDATE sources SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", enabled, id)
	return err
}

func (d *Database) UpdateSourceFetchStatus(id string, status string, etag, lastMod *string) error {
	var errQuery error
	_, errQuery = d.db.Exec(`
		UPDATE sources
		SET last_fetch_at = CURRENT_TIMESTAMP, last_fetch_status = ?, last_etag = ?, last_modified = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, etag, lastMod, id)
	return errQuery
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

func (d *Database) MarkFetchLogSkipped(logID int64, message string) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET status = 'skipped', error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?", message, logID)
	return err
}

func (d *Database) MarkFetchLogProgress(logID int64, added int) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET items_found = items_found + 1, items_added = items_added + ?, status = 'success', finished_at = CURRENT_TIMESTAMP WHERE id = ?", added, logID)
	return err
}

func (d *Database) MarkFetchLogSkippedSimple(logID int64) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET status = 'skipped', finished_at = CURRENT_TIMESTAMP WHERE id = ?", logID)
	return err
}
