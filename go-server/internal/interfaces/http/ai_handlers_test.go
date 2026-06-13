package httpiface

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"go-server/internal/infrastructure/sqlite"
)

func TestHandleDailyReturnsStructuredContentWithoutReportWrapper(t *testing.T) {
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	generatedAt := time.Date(2026, 6, 13, 1, 0, 0, 0, time.UTC)
	report := sqlite.AIDailyReport{
		ReportDate:        "2026-06-13",
		ReportType:        "morning",
		Title:             "Morning Report",
		Content:           `{"title":"Grabby AI 日报","date":"2026-06-13","editor":"Agnes","sections":{"headline":{"title":"今日头条","items":[{"title":"News","summary":"Summary","source":"Source","link":"https://example.com"}]}}}`,
		TotalItems:        10,
		QualityItems:      3,
		CategoriesSummary: `{"AI":3}`,
		ModelUsed:         "custom/agnes",
		GeneratedAt:       generatedAt,
	}
	if err := db.InsertAIDailyReport(report); err != nil {
		t.Fatalf("InsertAIDailyReport returned error: %v", err)
	}

	handler := NewAIHandlers(db, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/ai/daily?date=2026-06-13&type=morning", nil)
	rec := httptest.NewRecorder()

	handler.HandleDaily(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, key := range []string{"report", "html_content", "content"} {
		if _, ok := body[key]; ok {
			t.Fatalf("response should not include %q: %+v", key, body)
		}
	}
	if body["success"] != true {
		t.Fatalf("expected success=true, got %+v", body["success"])
	}
	if body["report_type"] != "morning" || body["model_used"] != "custom/agnes" {
		t.Fatalf("missing report metadata: %+v", body)
	}
	if generatedAtValue, ok := body["generated_at"].(string); !ok || generatedAtValue == "" {
		t.Fatalf("expected generated_at metadata, got %+v", body["generated_at"])
	}
	sections, ok := body["sections"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level sections object, got %+v", body["sections"])
	}
	headline, ok := sections["headline"].(map[string]any)
	if !ok || headline["title"] != "今日头条" {
		t.Fatalf("expected headline section, got %+v", sections["headline"])
	}
}
