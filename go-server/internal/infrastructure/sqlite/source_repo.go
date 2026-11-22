package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// --- Source CRUD ---

func (d *Database) GetSources() ([]Source, error) {
	rows, err := d.db.Query(`
		SELECT id, name, type, url, schedule, enabled, default_category, config,
		       last_etag, last_modified, last_fetch_at, last_fetch_status, category, created_at, updated_at
		FROM sources
		WHERE id != '__realtime__'
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

// UpsertSource inserts a source or overwrites its user-editable fields when the
// id already exists. Runtime bookkeeping columns (ETag, last fetch status) are
// left untouched so a config restore does not erase the local sync state.
func (d *Database) UpsertSource(src Source) error {
	category := src.Category
	if category == "" {
		category = "General"
	}
	enabled := src.Enabled
	if enabled != 0 {
		enabled = 1
	}
	_, err := d.db.Exec(`
		INSERT INTO sources (id, name, type, url, schedule, enabled, default_category, config, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			url = excluded.url,
			schedule = excluded.schedule,
			enabled = excluded.enabled,
			default_category = excluded.default_category,
			config = excluded.config,
			category = excluded.category,
			updated_at = CURRENT_TIMESTAMP
	`, src.ID, src.Name, src.Type, src.URL, src.Schedule, enabled, src.DefaultCategory, src.Config, category)
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

// UpdateSourceStatusFromLatestLog updates async results without overwriting
// validators or status from a more recent fetch.
func (d *Database) UpdateSourceStatusFromLatestLog(log FetchLog) error {
	_, err := d.db.Exec(`
		UPDATE sources SET last_fetch_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND ? = (SELECT MAX(id) FROM fetch_logs WHERE source_id = ?)
	`, log.Status, log.SourceID, log.ID, log.SourceID)
	return err
}

func (d *Database) InsertFetchLog(log FetchLog) (int64, error) {
	trigger := log.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	res, err := d.db.Exec(`
		INSERT INTO fetch_logs (source_id, started_at, finished_at, status, items_found, items_added, error_message, trigger_type, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.SourceID, utcTimestamp(log.StartedAt), utcTimestampArg(log.FinishedAt), log.Status, log.ItemsFound, log.ItemsAdded, log.ErrorMessage, trigger, log.Summary)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) UpdateFetchLog(log FetchLog) error {
	_, err := d.db.Exec(`
		UPDATE fetch_logs
		SET finished_at = ?, status = ?, items_found = ?, items_added = ?, items_updated = ?,
		    items_skipped = ?, items_failed = ?, summary = ?, error_message = ?
		WHERE id = ?
	`, utcTimestampArg(log.FinishedAt), log.Status, log.ItemsFound, log.ItemsAdded, log.ItemsUpdated,
		log.ItemsSkipped, log.ItemsFailed, log.Summary, log.ErrorMessage, log.ID)
	return err
}

const fetchLogColumns = `id, source_id, started_at, finished_at, status, COALESCE(items_found, 0), COALESCE(items_added, 0),
	COALESCE(items_updated, 0), COALESCE(items_skipped, 0), COALESCE(items_failed, 0),
	COALESCE(trigger_type, 'manual'), COALESCE(summary, ''), COALESCE(error_message, '')`

func scanFetchLog(s rowScanner) (FetchLog, error) {
	var log FetchLog
	var finishedAt sql.NullTime
	err := s.Scan(
		&log.ID, &log.SourceID, &log.StartedAt, &finishedAt, &log.Status,
		&log.ItemsFound, &log.ItemsAdded, &log.ItemsUpdated, &log.ItemsSkipped, &log.ItemsFailed,
		&log.Trigger, &log.Summary, &log.ErrorMessage,
	)
	if finishedAt.Valid {
		log.FinishedAt = &finishedAt.Time
	}
	return log, err
}

func (d *Database) GetFetchLogs(sourceID string, limit int) ([]FetchLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT ` + fetchLogColumns + ` FROM fetch_logs WHERE 1=1`
	var args []any
	if sourceID != "" {
		query += " AND source_id = ?"
		args = append(args, sourceID)
	}
	query += " ORDER BY started_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []FetchLog{}
	for rows.Next() {
		log, err := scanFetchLog(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, log)
	}
	return list, rows.Err()
}

// GetFetchLog loads a single run so the detail API can return one record with
// its per-item entries.
func (d *Database) GetFetchLog(id int64) (*FetchLog, error) {
	row := d.db.QueryRow(`SELECT `+fetchLogColumns+` FROM fetch_logs WHERE id = ?`, id)
	log, err := scanFetchLog(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// --- Fetch log item details ---

func (d *Database) InsertFetchLogItem(entry FetchLogItem) (int64, error) {
	texts := "[]"
	if len(entry.Texts) > 0 {
		if b, err := json.Marshal(entry.Texts); err == nil {
			texts = string(b)
		}
	}
	res, err := d.db.Exec(`
		INSERT INTO fetch_log_items
			(log_id, source_id, item_id, title, url, category, status, reason, error_stage, texts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.LogID, entry.SourceID, entry.ItemID, entry.Title, entry.URL, entry.Category,
		entry.Status, entry.Reason, entry.ErrorStage, texts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) UpdateFetchLogItem(entry FetchLogItem) error {
	texts := "[]"
	if len(entry.Texts) > 0 {
		if b, err := json.Marshal(entry.Texts); err == nil {
			texts = string(b)
		}
	}
	_, err := d.db.Exec(`
		UPDATE fetch_log_items
		SET item_id = ?, title = ?, url = ?, category = ?, status = ?, reason = ?,
		    error_stage = ?, texts = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, entry.ItemID, entry.Title, entry.URL, entry.Category, entry.Status, entry.Reason,
		entry.ErrorStage, texts, entry.ID)
	return err
}

func (d *Database) GetFetchLogItems(logID int64) ([]FetchLogItem, error) {
	rows, err := d.db.Query(`
		SELECT id, log_id, source_id, COALESCE(item_id, 0), COALESCE(title, ''), COALESCE(url, ''),
		       COALESCE(category, ''), status, COALESCE(reason, ''), COALESCE(error_stage, ''),
		       COALESCE(texts, '[]'), created_at, updated_at
		FROM fetch_log_items
		WHERE log_id = ?
		ORDER BY id ASC
	`, logID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []FetchLogItem{}
	for rows.Next() {
		var e FetchLogItem
		var texts string
		if err := rows.Scan(&e.ID, &e.LogID, &e.SourceID, &e.ItemID, &e.Title, &e.URL,
			&e.Category, &e.Status, &e.Reason, &e.ErrorStage, &texts, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Texts = decodeTextResults(texts)
		list = append(list, e)
	}
	return list, rows.Err()
}

func decodeTextResults(raw string) []FetchTextResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []FetchTextResult{}
	}
	var texts []FetchTextResult
	if err := json.Unmarshal([]byte(raw), &texts); err != nil {
		return []FetchTextResult{}
	}
	if texts == nil {
		return []FetchTextResult{}
	}
	return texts
}

func (d *Database) DeleteFetchLogItems(logID int64) error {
	_, err := d.db.Exec("DELETE FROM fetch_log_items WHERE log_id = ?", logID)
	return err
}

// CountPendingFetchLogItems reports how many entries are still waiting on an
// asynchronous task, so a run is not marked finished early.
func (d *Database) CountPendingFetchLogItems(logID int64) (int, error) {
	var n int
	err := d.db.QueryRow("SELECT COUNT(*) FROM fetch_log_items WHERE log_id = ? AND status = 'pending'", logID).Scan(&n)
	return n, err
}

// FinalizeFetchLog recomputes the run's counters from its item rows and stores
// the final state. When overrideStatus is empty the status is derived from the
// per-item outcomes; explicit values such as not_modified are preserved.
func (d *Database) FinalizeFetchLog(logID int64, overrideStatus, summary, errMsg string) (FetchLog, error) {
	log, err := d.GetFetchLog(logID)
	if err != nil || log == nil {
		return FetchLog{}, err
	}
	items, err := d.GetFetchLogItems(logID)
	if err != nil {
		return FetchLog{}, err
	}

	added, updated, skipped, failed, pending, textFailed := 0, 0, 0, 0, 0, 0
	for _, e := range items {
		switch e.Status {
		case "stored":
			added++
		case "updated":
			updated++
		case "failed":
			failed++
		case "pending":
			pending++
		case "skipped", "kept_existing", "no_public_text", "not_attempted":
			skipped++
		}
		// A stored item can still have one failed text field (for example an
		// RSS item kept as a summary because the original page 404'd, or a
		// YouTube video whose description loaded but caption track failed).
		if e.Status != "failed" && e.Status != "pending" && hasFailedText(e.Texts) {
			textFailed++
		}
	}

	status := overrideStatus
	if status == "" {
		switch {
		case pending > 0:
			status = "running"
		case failed+textFailed > 0 && (added+updated) > 0:
			status = "partial"
		case failed+textFailed > 0:
			status = "error"
		case added+updated > 0:
			status = "success"
		default:
			status = "no_new_items"
		}
	}
	if summary == "" {
		summary = formatRunSummary(len(items), added, updated, skipped, failed, textFailed, pending)
	}
	if status == "running" {
		log.Status = status
		log.ItemsFound = len(items)
		log.ItemsAdded = added
		log.ItemsUpdated = updated
		log.ItemsSkipped = skipped
		log.ItemsFailed = failed
		log.Summary = summary
		log.ErrorMessage = errMsg
		if log.FinishedAt != nil {
			log.FinishedAt = nil
		}
		_, err = d.db.Exec(`
			UPDATE fetch_logs SET status = ?, items_found = ?, items_added = ?, items_updated = ?,
			    items_skipped = ?, items_failed = ?, summary = ?, error_message = ?, finished_at = NULL
			WHERE id = ?
		`, log.Status, log.ItemsFound, log.ItemsAdded, log.ItemsUpdated, log.ItemsSkipped,
			log.ItemsFailed, summary, errMsg, logID)
		return *log, err
	}

	now := time.Now().UTC()
	log.Status = status
	log.FinishedAt = &now
	log.ItemsFound = len(items)
	log.ItemsAdded = added
	log.ItemsUpdated = updated
	log.ItemsSkipped = skipped
	log.ItemsFailed = failed
	log.Summary = summary
	log.ErrorMessage = errMsg
	if err := d.UpdateFetchLog(*log); err != nil {
		return FetchLog{}, err
	}
	return *log, nil
}

func (d *Database) MarkFetchLogSkipped(logID int64, message string) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET status = 'skipped', error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?", message, logID)
	return err
}

func (d *Database) MarkFetchLogProgress(logID int64, added int) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET items_found = items_found + 1, items_added = items_added + ? WHERE id = ?", added, logID)
	return err
}

func (d *Database) MarkFetchLogSkippedSimple(logID int64) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET status = 'skipped', finished_at = CURRENT_TIMESTAMP WHERE id = ?", logID)
	return err
}

func hasFailedText(texts []FetchTextResult) bool {
	for _, t := range texts {
		if t.Status == "failed" {
			return true
		}
	}
	return false
}

func formatRunSummary(found, added, updated, skipped, failed, textFailed, pending int) string {
	s := fmt.Sprintf("发现 %d 条，新增 %d，补全 %d，跳过 %d，失败 %d", found, added, updated, skipped, failed)
	if textFailed > 0 {
		s += fmt.Sprintf("，文本失败 %d", textFailed)
	}
	if pending > 0 {
		s += fmt.Sprintf("，处理中 %d", pending)
	}
	return s
}
