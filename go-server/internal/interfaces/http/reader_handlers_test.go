package httpiface

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func TestHandleTranslateRejectsEmptyDocument(t *testing.T) {
	h := NewAIHandlers(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/ai/translate", strings.NewReader(`{"item_id":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.HandleTranslate(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleInterpretRequiresItemID(t *testing.T) {
	h := NewAIHandlers(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/ai/interpret", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.HandleInterpret(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleChatRequiresItemID(t *testing.T) {
	h := NewAIHandlers(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.HandleChat(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
