package nativeui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func dailyStubServer(t *testing.T, reports func(call int) string) *httptest.Server {
	t.Helper()
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"format":       "markdown",
			"content":      "# 日报",
			"generated_at": reports(calls),
		})
	}))
}

func TestWaitForDailyReturnsWhenReportIsReplaced(t *testing.T) {
	srv := dailyStubServer(t, func(call int) string {
		if call >= 2 {
			return time.Now().Add(time.Second).Format(time.RFC3339Nano)
		}
		return time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
	})
	defer srv.Close()

	client := &Client{base: srv.URL, hc: http.DefaultClient}
	report, err := client.waitForDaily("2026-11-28", "daily", time.Now(), 10*time.Second)
	if err != nil {
		t.Fatalf("waitForDaily returned error: %v", err)
	}
	if generated, err := time.Parse(time.RFC3339Nano, report.GeneratedAt); err != nil || !generated.After(time.Now()) {
		t.Fatalf("generated_at = %q, want a time in the future of the request", report.GeneratedAt)
	}
}

func TestWaitForDailyTimesOutWithOldReport(t *testing.T) {
	srv := dailyStubServer(t, func(int) string {
		return time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
	})
	defer srv.Close()

	client := &Client{base: srv.URL, hc: http.DefaultClient}
	_, err := client.waitForDaily("2026-11-28", "daily", time.Now(), time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
}
