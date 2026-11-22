package scraping

import (
	"strings"
	"testing"

	"go-server/internal/domain/source"
)

func TestAcceptanceFetchLogRedactsCredentials(t *testing.T) {
	if got := sanitizeURL("https://alice:s3cret@example.com/post?token=abc123&x=1"); strings.Contains(got, "s3cret") ||
		strings.Contains(got, "abc123") || !strings.Contains(got, "***@example.com") {
		t.Fatalf("sanitizeURL = %q", got)
	}
	if got := sanitizeText("fetch https://bob:hunter2@example.com/a?api_key=xyz failed"); strings.Contains(got, "hunter2") ||
		strings.Contains(got, "xyz") {
		t.Fatalf("sanitizeText = %q", got)
	}

	db, _ := newBenchmarkDB(t)
	insertTestSource(t, db, source.Source{ID: "redact-src", Name: "Redact", Type: "rss",
		URL: "https://example.com/feed", Schedule: "0 * * * *", Enabled: 1, Config: "{}"})
	run := newScrapeRun(db, 0, "redact-src")
	run.reset()
	run.add(ItemReport{Title: "Secret item", URL: "https://user:pass@example.com/a?signature=sig",
		Status: "failed", Reason: "HTTP 401 for https://user:pass@example.com/a?signature=sig",
		ErrorStage: "access", Texts: []source.FetchTextResult{{
			Kind: "content", Status: "failed", URL: "https://user:pass@example.com/a?signature=sig",
			Reason: "token=leaked", Stage: "access",
		}}})
	data, err := db.GetFetchLogItems(0)
	if err == nil && len(data) > 0 {
		t.Fatalf("unexpected rows without a log")
	}
	// Persist through a real log row to prove the DB never sees the secret.
	logID, err := db.InsertFetchLog(source.FetchLog{SourceID: "redact-src", Status: "running", Trigger: "manual"})
	if err != nil {
		t.Fatalf("InsertFetchLog: %v", err)
	}
	run.logID = logID
	run.flush()
	rows, err := db.GetFetchLogItems(logID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v err=%v", rows, err)
	}
	raw := rows[0].URL + rows[0].Reason + rows[0].Texts[0].URL + rows[0].Texts[0].Reason
	for _, secret := range []string{"user:pass", "hunter2", "leaked", "abc123"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("log leaked %q in %+v", secret, rows[0])
		}
	}
}
