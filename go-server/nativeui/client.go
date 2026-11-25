package nativeui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is the loopback HTTP client the native UI uses. Every request comes
// from 127.0.0.1, which the desktop server trusts without a token.
type Client struct {
	base   string
	hc     *http.Client
	hcLong *http.Client
}

func NewClient(base string) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		hc:     &http.Client{Timeout: 30 * time.Second},
		hcLong: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) get(path string, out any) error {
	return c.do(http.MethodGet, path, nil, out)
}

func (c *Client) post(path string, body any, out any) error {
	return c.do(http.MethodPost, path, body, out)
}

func (c *Client) put(path string, body any, out any) error {
	return c.do(http.MethodPut, path, body, out)
}

func (c *Client) del(path string, out any) error {
	return c.do(http.MethodDelete, path, nil, out)
}

// postLong sends a request that may run for minutes, as AI translation of a
// long article does.
func (c *Client) postLong(path string, body any, out any) error {
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return c.doWith(c.hcLong, http.MethodPost, path, raw, out)
}

func (c *Client) do(method, path string, body any, out any) error {
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return c.doWith(c.hc, method, path, raw, out)
}

func (c *Client) doWith(hc *http.Client, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error
		if msg == "" {
			msg = e.Detail
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s", msg)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}

// rawGet returns the response body untouched, for the config export download.
func (c *Client) rawGet(path string) ([]byte, http.Header, error) {
	resp, err := c.hc.Get(c.base + path)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("%s", strings.TrimSpace(string(raw)))
	}
	return raw, resp.Header, nil
}

// rawPost sends an already encoded document, as the config importer takes.
func (c *Client) rawPost(path string, body []byte, out any) error {
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error
		if msg == "" {
			msg = e.Detail
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return fmt.Errorf("%s", msg)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func q(s string) string { return url.QueryEscape(s) }

// port is the local port the client talks to, used by the UI before the
// desktop settings have loaded.
func (c *Client) port() int {
	u, err := url.Parse(c.base)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(u.Port())
	return p
}

// --- endpoint helpers ---

func (c *Client) health() (Health, error) {
	var h Health
	err := c.get("/open/api/health", &h)
	return h, err
}

func (c *Client) stats() (Stats, error) {
	var s Stats
	err := c.get("/open/api/stats", &s)
	return s, err
}

func (c *Client) browsers() (BrowserList, error) {
	var b BrowserList
	err := c.get("/api/browsers", &b)
	return b, err
}

func (c *Client) aiCategories() ([]AICategory, error) {
	var e CategoriesEnvelope
	err := c.get("/open/api/ai/categories", &e)
	return e.Categories, err
}

func (c *Client) dailyList(limit int) ([]ReportListItem, error) {
	var e ReportList
	err := c.get(fmt.Sprintf("/open/api/ai/daily/list?limit=%d", limit), &e)
	return e.Reports, err
}

func (c *Client) sources() ([]Source, error) {
	var s []Source
	err := c.get("/api/sources", &s)
	return s, err
}

func (c *Client) logs(sourceID string) ([]FetchLog, error) {
	var l []FetchLog
	path := "/api/logs"
	if sourceID != "" {
		path += "?source_id=" + q(sourceID)
	}
	err := c.get(path, &l)
	return l, err
}

func (c *Client) logDetail(id int64) (FetchLogDetail, error) {
	var d FetchLogDetail
	err := c.get(fmt.Sprintf("/api/logs/%d", id), &d)
	return d, err
}

func (c *Client) captures() ([]ExtractRecord, error) {
	var e ExtractList
	err := c.get("/api/captures/extract?limit=20", &e)
	return e.Items, err
}

func (c *Client) aiSettings() (*AISettings, error) {
	var e AISettingsEnvelope
	if err := c.get("/api/ai/settings", &e); err != nil {
		return nil, err
	}
	return e.Settings, nil
}

func (c *Client) aiStats() (AIStats, error) {
	var s AIStats
	err := c.get("/open/api/ai/stats", &s)
	return s, err
}

func (c *Client) aiUsage() (*AIUsage, error) {
	var e UsageEnvelope
	if err := c.get("/api/ai/usage?days=7", &e); err != nil {
		return nil, err
	}
	return e.Summary, nil
}

func (c *Client) desktopSettings() (DesktopSettings, error) {
	var d DesktopSettings
	err := c.get("/api/desktop/settings", &d)
	return d, err
}

func (c *Client) asrEnv() (*ASREnv, error) {
	var e ASREnvEnvelope
	if err := c.get("/open/api/asr/env", &e); err != nil {
		return nil, err
	}
	return e.Env, nil
}

func (c *Client) item(id int64) (ItemDetail, error) {
	var d ItemDetail
	err := c.get(fmt.Sprintf("/open/api/items/%d", id), &d)
	return d, err
}

func (c *Client) analysis(id int64) (*Analysis, error) {
	var e AnalysisEnvelope
	if err := c.get(fmt.Sprintf("/open/api/ai/analysis/%d", id), &e); err != nil {
		return nil, err
	}
	return e.Analysis, nil
}

func (c *Client) daily(date, typ string) (DailyReport, error) {
	path := "/open/api/ai/daily?date=" + q(date) + "&type=" + q(typ)
	var d DailyReport
	if err := c.get(path, &d); err != nil {
		return d, err
	}
	// JSON reports are a document at the top level, not a markdown string in
	// "content"; keep the raw body so the view can pretty-print it.
	if d.Format == "json" && d.Content == "" {
		if raw, _, err := c.rawGet(path); err == nil {
			d.Content = string(raw)
		}
	}
	return d, nil
}

func (c *Client) items(filter Filter, cursor string) (ItemsPage, error) {
	path := "/open/api/items?limit=" + fmt.Sprint(filter.limit())
	if cursor != "" {
		path += "&cursor=" + q(cursor)
	}
	switch filter.Mode {
	case "unread":
		path += "&read_status=0"
	case "starred":
		path += "&starred=1"
	}
	if filter.Query != "" {
		path += "&q=" + q(filter.Query)
	}
	if filter.Category != "" {
		path += "&category=" + q(filter.Category)
	}
	if filter.ContentType != "" {
		path += "&content_type=" + q(filter.ContentType)
	}
	if filter.SourceCategory != "" {
		path += "&source_category=" + q(filter.SourceCategory)
	}
	if filter.SourceID != "" {
		path += "&source_id=" + q(filter.SourceID)
	}
	if filter.Since != "" {
		path += "&since=" + q(filter.Since)
	}
	var p ItemsPage
	err := c.get(path, &p)
	return p, err
}

func (c *Client) qualityItems(filter Filter, cursor string) (ItemsPage, error) {
	path := "/open/api/ai/items?limit=" + fmt.Sprint(filter.limit()) + "&score_min=" + fmt.Sprint(max(1, filter.ScoreMin))
	if cursor != "" {
		path += "&cursor=" + q(cursor)
	}
	if filter.AICategory != "" {
		path += "&category=" + q(filter.AICategory)
	}
	if filter.SourceCategory != "" {
		path += "&source_category=" + q(filter.SourceCategory)
	}
	if filter.SourceID != "" {
		path += "&source_id=" + q(filter.SourceID)
	}
	if filter.ContentType != "" {
		path += "&content_type=" + q(filter.ContentType)
	}
	if filter.Since != "" {
		path += "&since=" + q(filter.Since)
	}
	var p ItemsPage
	err := c.get(path, &p)
	return p, err
}

func (c *Client) markRead(id int64, read bool) error {
	v := 0
	if read {
		v = 1
	}
	return c.post(fmt.Sprintf("/api/items/%d/read", id), map[string]int{"read_status": v}, nil)
}

func (c *Client) star(id int64, starred bool) error {
	v := 0
	if starred {
		v = 1
	}
	return c.post(fmt.Sprintf("/api/items/%d/star", id), map[string]int{"starred": v}, nil)
}

func (c *Client) reanalyze(id int64) error {
	return c.post(fmt.Sprintf("/api/ai/reanalyze/%d", id), map[string]any{}, nil)
}

func (c *Client) translate(id int64, title, target string, blocks []string) (TranslationResult, error) {
	var r struct {
		Success      bool     `json:"success"`
		Title        string   `json:"title"`
		Translations []string `json:"translations"`
		ModelUsed    string   `json:"model_used"`
	}
	err := c.postLong("/api/ai/translate", map[string]any{
		"item_id":         id,
		"title":           title,
		"target_language": target,
		"blocks":          blocks,
	}, &r)
	return TranslationResult{Title: r.Title, Translations: r.Translations, ModelUsed: r.ModelUsed}, err
}

// streamChat reads the SSE answer of /api/ai/chat, calling onDelta for each
// text chunk and returning the model that answered. It does not use the
// default client timeout because a long answer can take minutes.
func (c *Client) streamChat(ctx context.Context, id int64, messages []ChatMessage, onDelta func(string)) (string, error) {
	body, err := json.Marshal(map[string]any{"item_id": id, "messages": messages})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/ai/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	hc := &http.Client{Timeout: 10 * time.Minute}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var e struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error
		if msg == "" {
			msg = e.Detail
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("%s", msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var model string
	gotDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var ev struct {
			Delta     string `json:"delta"`
			Error     string `json:"error"`
			Done      bool   `json:"done"`
			ModelUsed string `json:"model_used"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if ev.Error != "" {
			return "", fmt.Errorf("%s", ev.Error)
		}
		if ev.ModelUsed != "" {
			model = ev.ModelUsed
		}
		if ev.Delta != "" {
			onDelta(ev.Delta)
		}
		if ev.Done {
			gotDone = true
			return model, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if !gotDone {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("连接中断，未收到完整响应")
	}
	return model, nil
}

func (c *Client) toggleSource(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	return c.post("/api/sources/"+q(id)+"/toggle", map[string]int{"enabled": v}, nil)
}

func (c *Client) runSource(id string) error {
	return c.post("/api/sources/"+q(id)+"/run", map[string]any{}, nil)
}

func (c *Client) deleteSource(id string) error {
	return c.del("/api/sources/"+q(id), nil)
}

func (c *Client) saveSource(form SourceForm, editing bool) error {
	if editing {
		return c.put("/api/sources/"+q(form.ID), form, nil)
	}
	return c.post("/api/sources", form, nil)
}

func (c *Client) saveAISettings(s AISettings) error {
	return c.post("/api/ai/settings", s, nil)
}

func (c *Client) testAI(profile ProviderProfile, systemPrompt string) (TestResult, error) {
	var r TestResult
	err := c.post("/api/ai/test", map[string]any{
		"profile":       profile,
		"system_prompt": systemPrompt,
	}, &r)
	return r, err
}

func (c *Client) discoverModels(provider, baseURL, apiKey, profileID string) ([]string, error) {
	var r DiscoverModelsResult
	err := c.post("/api/ai/models/discover", map[string]string{
		"provider":   provider,
		"base_url":   baseURL,
		"api_key":    apiKey,
		"profile_id": profileID,
	}, &r)
	return r.Models, err
}

func (c *Client) generateDaily(date, typ string) error {
	return c.post("/api/ai/daily/generate", map[string]string{
		"date": date, "report_type": typ,
	}, nil)
}

func (c *Client) startEvaluation() error {
	return c.post("/api/ai/start_eval", map[string]any{}, nil)
}

func (c *Client) browserAction(action, connID string) error {
	return c.post("/api/browsers/"+action, map[string]string{"conn_id": connID}, nil)
}

func (c *Client) saveDesktopSettings(external bool, token string) (DesktopSettings, error) {
	var d DesktopSettings
	err := c.post("/api/desktop/settings", map[string]any{
		"external_enabled": external,
		"api_token":        token,
	}, &d)
	return d, err
}

func (c *Client) installASR() error {
	return c.post("/api/asr/install", map[string]any{}, nil)
}

func (c *Client) extract(url, adapter string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.post("/api/extract", map[string]string{"url": url}, &out)
	return out, err
}

func (c *Client) platformCapture(adapter, value string) (json.RawMessage, error) {
	var out json.RawMessage
	var path string
	var body map[string]string
	switch adapter {
	case "twitter-search":
		path, body = "/api/twitter/search", map[string]string{"query": value}
	case "twitter-timeline":
		path, body = "/api/twitter/timeline", map[string]string{"kind": value}
	case "twitter-likes":
		path, body = "/api/twitter/likes", map[string]string{}
	case "reddit-subreddit":
		path, body = "/api/reddit/subreddit", map[string]string{"subreddit": value}
	case "reddit-search":
		path, body = "/api/reddit/search", map[string]string{"query": value}
	case "reddit-thread":
		path, body = "/api/reddit/thread", map[string]string{"url": value}
	case "xhs-search":
		path, body = "/api/xiaohongshu/search", map[string]string{"query": value}
	case "xhs-note":
		path, body = "/api/xiaohongshu/note", map[string]string{"url": value}
	case "gsc":
		path, body = "/api/gsc/record", map[string]string{"kind": value}
	default:
		return nil, fmt.Errorf("未知的采集动作: %s", adapter)
	}
	err := c.post(path, body, &out)
	return out, err
}

func (c *Client) configExport() ([]byte, error) {
	raw, _, err := c.rawGet("/api/config/export")
	return raw, err
}

func (c *Client) configImport(raw []byte) (map[string]any, error) {
	var out map[string]any
	err := c.rawPost("/api/config/import", raw, &out)
	return out, err
}
