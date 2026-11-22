package scraping

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

// ItemReport is the in-memory outcome for one discovered/processed entry. It
// is translated into a persisted FetchLogItem at the end of a run (or earlier
// when a browser task is still pending).
type ItemReport struct {
	Title      string
	URL        string
	Category   string
	ItemID     int64
	Status     string // stored, updated, kept_existing, skipped, no_public_text, failed, pending
	Reason     string
	ErrorStage string // access, extract, persist
	Texts      []source.FetchTextResult
}

// scrapeRun accumulates per-entry reports for one fetch_logs row. It is safe
// for the sequential scrape paths and exposes explicit helpers for the
// asynchronous browser queue.
type scrapeRun struct {
	db       *sqlite.Database
	logID    int64
	sourceID string

	mu           sync.Mutex
	reports      []*persistedReport
	browserTasks []ScrapeTask

	notModified    bool
	overrideStatus string
	summaryHint    string
}

type persistedReport struct {
	report ItemReport
	rowID  int64
}

func newScrapeRun(db *sqlite.Database, logID int64, sourceID string) *scrapeRun {
	return &scrapeRun{db: db, logID: logID, sourceID: sourceID}
}

func (r *scrapeRun) add(rep ItemReport) {
	if r == nil {
		return
	}
	sanitizeReport(&rep)
	r.mu.Lock()
	r.reports = append(r.reports, &persistedReport{report: rep})
	r.mu.Unlock()
}

// queueBrowser stages work until the initial run and its pending rows are saved.
func (r *scrapeRun) queueBrowser(task ScrapeTask, rep ItemReport) {
	sanitizeReport(&rep)
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := &persistedReport{report: rep}
	r.reports = append(r.reports, pending)
	task.LogID = r.logID
	task.notModified = r.notModified
	task.pending = pending
	r.browserTasks = append(r.browserTasks, task)
}

func (r *scrapeRun) dispatchBrowser(queue *TaskQueue) {
	for i := range r.browserTasks {
		task := &r.browserTasks[i]
		task.LogItemID = task.pending.rowID
	}
	if len(r.browserTasks) > 0 {
		queue.Enqueue(r.browserTasks)
	}
}

// reset clears both the in-memory reports and persisted rows before a retry so
// a failed attempt does not leave duplicate detail lines.
func (r *scrapeRun) reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.reports = nil
	r.browserTasks = nil
	r.mu.Unlock()
	if r.logID > 0 {
		_ = r.db.DeleteFetchLogItems(r.logID)
	}
}

// flush writes any report that was not already persisted for an async task.
func (r *scrapeRun) flush() {
	if r == nil || r.logID <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, pr := range r.reports {
		if pr.rowID > 0 {
			continue
		}
		id, err := r.db.InsertFetchLogItem(toFetchLogItem(r.logID, r.sourceID, pr.report, 0))
		if err == nil {
			pr.rowID = id
		}
	}
}

func (r *scrapeRun) added() int {
	n := 0
	for _, pr := range r.snapshot() {
		if pr.report.Status == "stored" {
			n++
		}
	}
	return n
}

func (r *scrapeRun) snapshot() []*persistedReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*persistedReport, len(r.reports))
	copy(out, r.reports)
	return out
}

func (r *scrapeRun) counts() (stored, updated, skipped, failed, pending int) {
	for _, pr := range r.snapshot() {
		switch pr.report.Status {
		case "stored":
			stored++
		case "updated":
			updated++
		case "failed":
			failed++
		case "pending":
			pending++
		default:
			skipped++
		}
	}
	return
}

// textFailures counts entries with at least one failed text field, without
// treating the stored item itself as failed.
func (r *scrapeRun) textFailures() int {
	n := 0
	for _, pr := range r.snapshot() {
		if pr.report.Status == "failed" || pr.report.Status == "pending" {
			continue
		}
		for _, t := range pr.report.Texts {
			if t.Status == "failed" {
				n++
				break
			}
		}
	}
	return n
}

// whenPending counts how many async entries are still in flight. The task queue
// refreshes this from the database, which is the source of truth.
func (r *scrapeRun) whenPending() int {
	if r == nil || r.logID <= 0 {
		return 0
	}
	n, err := r.db.CountPendingFetchLogItems(r.logID)
	if err != nil {
		return 0
	}
	return n
}

func toFetchLogItem(logID int64, sourceID string, rep ItemReport, itemID int64) sqlite.FetchLogItem {
	if itemID == 0 {
		itemID = rep.ItemID
	}
	texts := rep.Texts
	if texts == nil {
		texts = []source.FetchTextResult{}
	}
	return sqlite.FetchLogItem{
		LogID:      logID,
		SourceID:   sourceID,
		ItemID:     itemID,
		Title:      rep.Title,
		URL:        rep.URL,
		Category:   rep.Category,
		Status:     rep.Status,
		Reason:     sanitizeText(rep.Reason),
		ErrorStage: rep.ErrorStage,
		Texts:      texts,
	}
}

func sanitizeReport(rep *ItemReport) {
	rep.Title = sanitizeText(rep.Title)
	rep.URL = sanitizeURL(rep.URL)
	rep.Reason = sanitizeText(rep.Reason)
	for i := range rep.Texts {
		rep.Texts[i].URL = sanitizeURL(rep.Texts[i].URL)
		rep.Texts[i].Reason = sanitizeText(rep.Texts[i].Reason)
	}
}

var (
	urlUserinfoRegex = regexp.MustCompile(`(?i)(https?://)[^/@\s]+@`)
	secretParamRegex = regexp.MustCompile(`(?i)(^|[?&\s])((?:token|access_token|api_key|apikey|key|auth|authorization|signature|sig|password|passwd|secret|code)=)[^&#\s]+`)
)

// sanitizeURL removes embedded credentials and secret query parameters before
// a URL reaches a persisted log or the API.
func sanitizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = urlUserinfoRegex.ReplaceAllString(s, "${1}***@")
	s = secretParamRegex.ReplaceAllString(s, "${1}${2}***")
	return s
}

// sanitizeText applies URL redaction to free-form text (errors, reasons) and
// caps its length so one pathological error cannot bloat the log.
func sanitizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = urlUserinfoRegex.ReplaceAllString(s, "${1}***@")
	s = secretParamRegex.ReplaceAllString(s, "${1}${2}***")
	if utf8.RuneCountInString(s) > 500 {
		runes := []rune(s)
		s = string(runes[:500]) + "…"
	}
	return s
}

// --- metadata helpers ---

func parseMetadata(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func marshalMetadata(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func metaString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

// preferLonger keeps the richer of two text values. Used so an incomplete
// second fetch never replaces a previously stored description or transcript.
func preferLonger(existing, incoming string) string {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return strings.TrimSpace(existing)
	}
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return incoming
	}
	if utf8.RuneCountInString(incoming) >= utf8.RuneCountInString(existing) {
		return incoming
	}
	return existing
}

// mergeMetadata copies non-empty incoming keys onto the existing map. Keys in
// protected are only written when the current value is empty, so a failed or
// partial fetch cannot erase a good description/transcript.
func mergeMetadata(existing map[string]any, incoming map[string]any, protected ...string) map[string]any {
	if existing == nil {
		existing = map[string]any{}
	}
	protectedSet := map[string]bool{}
	for _, k := range protected {
		protectedSet[k] = true
	}
	for k, v := range incoming {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		if protectedSet[k] {
			if cur, ok := existing[k].(string); ok && strings.TrimSpace(cur) != "" {
				continue
			}
		}
		existing[k] = v
	}
	return existing
}
