// Package xiaohongshu is a server-side site adapter for Xiaohongshu (小红书).
// Unlike Reddit (fetchInPage .json) and partly Twitter (intercept), Xiaohongshu
// uses a mix of primitives:
//
//   - note detail: runPageScript reading the SSR window.__INITIAL_STATE__.note.
//     noteDetailMap (note data is server-rendered, not fetched via XHR feed).
//   - search / user notes: intercept capturing /api/sns/web/v1/{search/notes,
//     user_posted} XHR responses.
//
// Design: docs/prd-likes-incremental-and-reddit.md (xiaohongshu section). All
// three are atomic fetch capabilities; the service only fetches data correctly
// — login state is ensured by the user ahead of time, rate control is the
// caller's concern.
package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
)

// Note is a structured Xiaohongshu note (post).
type Note struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Desc           string    `json:"desc"`   // body text
	Type           string    `json:"type"`   // "normal" | "video"
	Author         string    `json:"author"` // nick_name
	AuthorID       string    `json:"author_id"`
	LikedCount     string    `json:"liked_count"`
	CollectedCount string    `json:"collected_count"`
	CommentCount   string    `json:"comment_count"`
	ShareCount     string    `json:"share_count"`
	Images         []string  `json:"images,omitempty"`
	URL            string    `json:"url"`
	XsecToken      string    `json:"xsec_token,omitempty"`
	Comments       []Comment `json:"comments,omitempty"`
}

// Comment is a structured Xiaohongshu note comment.
type Comment struct {
	ID              string    `json:"id"`
	NoteID          string    `json:"note_id"`
	Content         string    `json:"content"`
	Author          string    `json:"author"`
	AuthorID        string    `json:"author_id"`
	AuthorAvatar    string    `json:"author_avatar,omitempty"`
	IPLocation      string    `json:"ip_location,omitempty"`
	LikeCount       string    `json:"like_count"`
	Liked           bool      `json:"liked"`
	CreateTime      int64     `json:"create_time"`
	ShowTags        []string  `json:"show_tags,omitempty"`
	Pictures        []string  `json:"pictures,omitempty"`
	SubCommentCount string    `json:"sub_comment_count,omitempty"`
	SubComments     []Comment `json:"sub_comments,omitempty"`
}

// Sender is the subset of the WebSocket manager this adapter needs.
type Sender interface {
	ResolveBrowserConnID(name string) (string, error)
	SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
}

var _ Sender = (*browserws.WebSocketManager)(nil)

// Options controls a fetch.
type Options struct {
	Browser      string
	Limit        int
	ScrollRounds int
}

func (o Options) limit(def int) int {
	if o.Limit > 0 {
		return o.Limit
	}
	return def
}

func (o Options) scrollRounds(def int) int {
	if o.ScrollRounds > 0 {
		return o.ScrollRounds
	}
	return def
}

// FetchNote fetches one note's detail by opening its explore page and reading
// the SSR __INITIAL_STATE__ via the whitelisted xiaohongshu.initialState script.
//
// noteURL is the note's explore permalink, e.g.
// https://www.xiaohongshu.com/explore/{noteId}?xsec_token=...&xsec_source=...
func FetchNote(ctx context.Context, wm Sender, logger *zap.Logger, noteURL string, opts Options) (*Note, error) {
	noteURL = strings.TrimSpace(noteURL)
	if noteURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	noteID := noteIDFromURL(noteURL)
	if noteID == "" {
		return nil, fmt.Errorf("could not parse note id from url")
	}

	connID, err := wm.ResolveBrowserConnID(opts.Browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "runPageScript",
		URL:      noteURL,
		CloseTab: true,
		Params: map[string]any{
			"script": "xiaohongshu.initialState",
			"params": map[string]any{},
		},
	}
	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("runPageScript failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("runPageScript error: %s", resp.Error)
	}

	// xiaohongshu.initialState returns the noteDetailMap object, which the
	// extension delivers in result.json (object path). Older extensions may
	// deliver it as a JSON string in result.text; handle both, and tolerate a
	// double-encoded string.
	detailMap, err := detailMapFromResult(resp.Result.JSON, resp.Result.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse __INITIAL_STATE__ noteDetailMap: %w", err)
	}

	note, ok := noteFromDetailMap(detailMap, noteID, noteURL)
	if !ok {
		return nil, fmt.Errorf("note %s not found in __INITIAL_STATE__ — check login state or that the note is accessible", noteID)
	}
	comments, err := fetchNoteComments(ctx, wm, logger, noteURL, opts, 100)
	if err != nil {
		if logger != nil {
			logger.Warn("xiaohongshu note comments fetch failed",
				zap.String("url", noteURL),
				zap.String("note_id", noteID),
				zap.Error(err),
			)
		}
	} else {
		note.Comments = comments
	}

	if logger != nil {
		logger.Info("xiaohongshu note parsed",
			zap.String("url", noteURL),
			zap.String("note_id", note.ID),
			zap.Int("comments", len(note.Comments)),
		)
	}
	return note, nil
}

func detailMapFromResult(jsonValue map[string]any, text string) (map[string]any, error) {
	if jsonValue != nil {
		return jsonValue, nil
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, err
	}
	for i := 0; i < 3; i++ {
		switch v := value.(type) {
		case map[string]any:
			return v, nil
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, nil
			}
			if err := json.Unmarshal([]byte(v), &value); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected noteDetailMap type %T", value)
		}
	}
	return nil, fmt.Errorf("noteDetailMap is too deeply encoded")
}

// noteIDFromURL extracts the note id from an explore URL path
// (/explore/{id}, /discovery/item/{id}, or xhslink short link fallback).
func noteIDFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// The id is the segment right after a known marker. Prefer the last marker
	// so /discovery/item/{id} resolves to the id, not "item".
	markers := map[string]bool{"explore": true, "item": true}
	for i := len(parts) - 2; i >= 0; i-- {
		if markers[parts[i]] && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// fallback: last path segment
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// noteFromDetailMap pulls one note out of noteDetailMap{noteId: {note: {...}}}.
func noteFromDetailMap(detailMap map[string]any, noteID, noteURL string) (*Note, bool) {
	if detailMap == nil {
		return nil, false
	}
	entry := mapFromMaybeJSON(detailMap[noteID])
	ok := entry != nil
	if !ok {
		// If no id was given, fall back to the single entry (caller couldn't
		// resolve the id from the URL). Never fall back when an explicit id was
		// requested but not found.
		if noteID == "" && len(detailMap) == 1 {
			for _, v := range detailMap {
				if m := mapFromMaybeJSON(v); m != nil {
					entry = m
					break
				}
			}
		}
		if entry == nil {
			return nil, false
		}
	}
	nc, ok := entry["note"].(map[string]any)
	if !ok {
		return nil, false
	}
	return noteFromDetail(nc, noteURL), true
}

func mapFromMaybeJSON(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

func noteFromDetail(nc map[string]any, noteURL string) *Note {
	n := &Note{
		ID:    asString(nc["noteId"]),
		Title: asString(nc["title"]),
		Desc:  asString(nc["desc"]),
		Type:  asString(nc["type"]),
		URL:   noteURL,
	}
	if n.ID == "" {
		n.ID = asString(nc["id"])
	}
	if ii, ok := nc["interactInfo"].(map[string]any); ok {
		n.LikedCount = asString(ii["likedCount"])
		n.CollectedCount = asString(ii["collectedCount"])
		n.CommentCount = asString(ii["commentCount"])
		n.ShareCount = asString(ii["shareCount"])
	}
	if u, ok := nc["user"].(map[string]any); ok {
		n.Author = asString(u["nickName"])
		if n.Author == "" {
			n.Author = asString(u["nick_name"])
		}
		if n.Author == "" {
			n.Author = asString(u["nickname"])
		}
		n.AuthorID = asString(u["userId"])
		if n.AuthorID == "" {
			n.AuthorID = asString(u["user_id"])
		}
	}
	if imgs, ok := nc["imageList"].([]any); ok {
		for _, im := range imgs {
			if m, ok := im.(map[string]any); ok {
				if u := asString(m["urlDefault"]); u != "" {
					n.Images = append(n.Images, u)
				}
			}
		}
	}
	return n
}

// --- intercept-based: search & user notes ---

// capturedRecord mirrors one item produced by the browser intercept hook.
type capturedRecord struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	Method string `json:"method"`
	Body   string `json:"body"`
	TS     int64  `json:"ts"`
}

type interceptDiagnostic struct {
	URL        string         `json:"url"`
	Status     int            `json:"status"`
	Method     string         `json:"method"`
	BodyLen    int            `json:"body_len"`
	Notes      int            `json:"notes"`
	Shape      map[string]any `json:"shape,omitempty"`
	BodyPrefix string         `json:"body_prefix,omitempty"`
}

// Search runs a Xiaohongshu search and returns matching notes.
func Search(ctx context.Context, wm Sender, logger *zap.Logger, query string, opts Options) ([]Note, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	pageURL := "https://www.xiaohongshu.com/search_result?keyword=" + url.QueryEscape(query) + "&source=web_explore_feed"
	return runIntercept(ctx, wm, logger, opts, pageURL, []string{"/api/sns/web/v1/search/notes"}, "search/notes")
}

// UserNotes fetches a user's posted notes.
//
// userURL is the user profile URL, e.g.
// https://www.xiaohongshu.com/user/profile/{userId}
func UserNotes(ctx context.Context, wm Sender, logger *zap.Logger, userURL string, opts Options) ([]Note, error) {
	userURL = strings.TrimSpace(userURL)
	if userURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	notes, err := fetchUserNotesInitialState(ctx, wm, logger, userURL, opts)
	if err != nil {
		return nil, err
	}
	limit := opts.limit(50)
	if len(notes) > limit {
		notes = notes[:limit]
	}
	return notes, nil
}

func fetchNoteComments(ctx context.Context, wm Sender, logger *zap.Logger, noteURL string, opts Options, limit int) ([]Comment, error) {
	connID, err := wm.ResolveBrowserConnID(opts.Browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}
	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "intercept",
		URL:      noteURL,
		Visible:  true,
		CloseTab: true,
		Params: map[string]any{
			"scrollRounds": opts.scrollRounds(10),
			"maxCaptures":  200,
		},
	}
	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("intercept comments failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("intercept comments error: %s", resp.Error)
	}

	comments := make([]Comment, 0, limit)
	seen := make(map[string]struct{})
	matched := 0
	for _, raw := range resp.Result.Items {
		rec, ok := toRecord(raw)
		if !ok || !strings.Contains(rec.URL, "/api/sns/web/v2/comment/page") {
			continue
		}
		matched++
		for _, c := range extractCommentsFromBody(rec.Body) {
			if c.ID == "" {
				continue
			}
			if _, dup := seen[c.ID]; dup {
				continue
			}
			seen[c.ID] = struct{}{}
			comments = append(comments, c)
			if len(comments) >= limit {
				break
			}
		}
		if len(comments) >= limit {
			break
		}
	}
	if logger != nil {
		logger.Info("xiaohongshu comments parsed",
			zap.String("url", noteURL),
			zap.Int("captures", len(resp.Result.Items)),
			zap.Int("matched", matched),
			zap.Int("comments", len(comments)),
		)
	}
	return comments, nil
}

func fetchUserNotesInitialState(ctx context.Context, wm Sender, logger *zap.Logger, userURL string, opts Options) ([]Note, error) {
	connID, err := wm.ResolveBrowserConnID(opts.Browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "runPageScript",
		URL:      userURL,
		CloseTab: true,
		Params: map[string]any{
			"script": "xiaohongshu.userNotesInitialState",
			"params": map[string]any{},
		},
	}
	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("runPageScript failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("runPageScript error: %s", resp.Error)
	}

	if logger != nil {
		logger.Info("xiaohongshu user notes initial state received",
			zap.String("url", userURL),
			zap.Any("shape", userNotesInitialStateShape(resp.Result.JSON, resp.Result.Text)),
		)
	}

	notes, err := userNotesFromInitialStateResult(resp.Result.JSON, resp.Result.Text)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Info("xiaohongshu user notes initial state parsed",
			zap.String("url", userURL),
			zap.Int("notes", len(notes)),
		)
	}
	return notes, nil
}

func userNotesInitialStateShape(jsonValue map[string]any, text string) map[string]any {
	shape := map[string]any{}
	if jsonValue != nil {
		shape["result"] = "json"
		shape["keys"] = sortedKeys(jsonValue)
		notes := asSlice(jsonValue["notes"])
		shape["groups"] = len(notes)
		if len(notes) > 0 {
			shape["first_group_len"] = len(asSlice(notes[0]))
		}
		if debug := asMap(jsonValue["__debug"]); debug != nil {
			shape["debug"] = debug
		}
		return shape
	}
	shape["result"] = "text"
	shape["text_len"] = len(text)
	shape["text_prefix"] = compactPrefix(text, 300)
	return shape
}

// runIntercept sends one intercept command, filters captures by URL substring,
// parses notes, dedupes, and truncates to limit.
func runIntercept(ctx context.Context, wm Sender, logger *zap.Logger, opts Options, pageURL string, urlSubs []string, label string) ([]Note, error) {
	connID, err := wm.ResolveBrowserConnID(opts.Browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "intercept",
		URL:      pageURL,
		Visible:  true, // xhs infinite-scroll needs a visible tab
		CloseTab: true,
		Params: map[string]any{
			"scrollRounds": opts.scrollRounds(6),
			"maxCaptures":  200,
		},
	}

	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("intercept failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("intercept error: %s", resp.Error)
	}

	limit := opts.limit(50)
	notes := make([]Note, 0, limit)
	seen := make(map[string]struct{})
	matched := 0
	diagnostics := make([]interceptDiagnostic, 0, 10)

	for _, raw := range resp.Result.Items {
		rec, ok := toRecord(raw)
		if !ok || !matchesAny(rec.URL, urlSubs) {
			continue
		}
		matched++
		extracted := extractNotesFromListing(rec.Body, label)
		if len(diagnostics) < 10 {
			diagnostics = append(diagnostics, diagnoseInterceptRecord(rec, len(extracted)))
		}
		for _, n := range extracted {
			if n.ID == "" {
				continue
			}
			if _, dup := seen[n.ID]; dup {
				continue
			}
			seen[n.ID] = struct{}{}
			notes = append(notes, n)
			if len(notes) >= limit {
				break
			}
		}
		if len(notes) >= limit {
			break
		}
	}

	if logger != nil {
		logger.Info("xiaohongshu intercept parsed",
			zap.String("url", pageURL),
			zap.String("label", label),
			zap.Int("captures", len(resp.Result.Items)),
			zap.Int("matched", matched),
			zap.Int("notes", len(notes)),
		)
	}

	if len(notes) == 0 {
		if logger != nil {
			logger.Warn("xiaohongshu intercept captured no parseable notes",
				zap.String("url", pageURL),
				zap.String("label", label),
				zap.Int("captures", len(resp.Result.Items)),
				zap.Int("matched", matched),
				zap.Any("matched_samples", diagnostics),
			)
		}
		return notes, fmt.Errorf("no notes captured (matched %d/%d responses) — check login state or that xiaohongshu page structure still matches; matched_samples=%s", matched, len(resp.Result.Items), diagnosticsJSON(diagnostics))
	}
	return notes, nil
}

func diagnosticsJSON(diagnostics []interceptDiagnostic) string {
	if len(diagnostics) == 0 {
		return "[]"
	}
	b, err := json.Marshal(diagnostics)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func diagnoseInterceptRecord(rec capturedRecord, notes int) interceptDiagnostic {
	return interceptDiagnostic{
		URL:        summarizeURL(rec.URL),
		Status:     rec.Status,
		Method:     rec.Method,
		BodyLen:    len(rec.Body),
		Notes:      notes,
		Shape:      jsonShape(rec.Body),
		BodyPrefix: compactPrefix(rec.Body, 300),
	}
}

func summarizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery == "" {
		return u.Host + u.Path
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return u.Host + u.Path + "?<query>"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return u.Host + u.Path + "?" + strings.Join(keys, ",")
}

func jsonShape(body string) map[string]any {
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return map[string]any{"parse_error": err.Error()}
	}
	shape := map[string]any{
		"keys": sortedKeys(root),
	}
	for _, key := range []string{"code", "success", "msg"} {
		if v, ok := root[key]; ok {
			shape[key] = v
		}
	}
	if data, ok := root["data"].(map[string]any); ok {
		dataShape := map[string]any{"keys": sortedKeys(data)}
		for _, key := range []string{"items", "notes", "note_list", "feeds", "list"} {
			if arr, ok := data[key].([]any); ok {
				dataShape[key+"_len"] = len(arr)
				if len(arr) > 0 {
					dataShape[key+"_first_keys"] = sortedKeys(asMap(arr[0]))
				}
			}
		}
		shape["data"] = dataShape
	}
	return shape
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func compactPrefix(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

func toRecord(raw any) (capturedRecord, bool) {
	b, err := json.Marshal(raw)
	if err != nil {
		return capturedRecord{}, false
	}
	var rec capturedRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return capturedRecord{}, false
	}
	return rec, true
}

func matchesAny(u string, subs []string) bool {
	for _, s := range subs {
		if strings.Contains(u, s) {
			return true
		}
	}
	return false
}

// extractNotesFromListing parses a search/notes or user_posted response body.
//
// search/notes:  data.items[].note_card  (each item also has id + xsec_token)
// user_posted:   data.notes[]           (each note is the card itself: note_id, xsec_token, display_title, ...)
func extractNotesFromListing(body, label string) []Note {
	if body == "" {
		return nil
	}
	var root struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
			Notes []json.RawMessage `json:"notes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil
	}
	var out []Note
	if label == "user_posted" {
		for _, raw := range root.Data.Notes {
			var card struct {
				NoteID       string         `json:"note_id"`
				XsecToken    string         `json:"xsec_token"`
				Type         string         `json:"type"`
				DisplayTitle string         `json:"display_title"`
				User         map[string]any `json:"user"`
				Interact     map[string]any `json:"interact_info"`
				Cover        map[string]any `json:"cover"`
			}
			if err := json.Unmarshal(raw, &card); err != nil {
				continue
			}
			out = append(out, noteFromSearchCard(card.NoteID, card.XsecToken, card.Type, card.DisplayTitle, card.User, card.Interact))
		}
		if len(out) > 0 {
			return out
		}
	}
	for _, raw := range root.Data.Items {
		var item struct {
			ID        string         `json:"id"`
			XsecToken string         `json:"xsec_token"`
			NoteCard  map[string]any `json:"note_card"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		nc := item.NoteCard
		if nc == nil {
			continue
		}
		out = append(out, noteFromSearchCard(
			item.ID, item.XsecToken,
			asString(nc["type"]), asString(nc["display_title"]),
			asMap(nc["user"]), asMap(nc["interact_info"]),
		))
	}
	return out
}

func extractCommentsFromBody(body string) []Comment {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	var root struct {
		Data struct {
			Comments []json.RawMessage `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil
	}
	out := make([]Comment, 0, len(root.Data.Comments))
	for _, raw := range root.Data.Comments {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		c := commentFromMap(m)
		if c.ID != "" {
			out = append(out, c)
		}
	}
	return out
}

func commentFromMap(m map[string]any) Comment {
	user := asMap(m["user_info"])
	c := Comment{
		ID:              asString(m["id"]),
		NoteID:          asString(m["note_id"]),
		Content:         asString(m["content"]),
		Author:          asString(user["nickname"]),
		AuthorID:        asString(user["user_id"]),
		AuthorAvatar:    asString(user["image"]),
		IPLocation:      asString(m["ip_location"]),
		LikeCount:       asString(m["like_count"]),
		Liked:           asBool(m["liked"]),
		CreateTime:      asInt64(m["create_time"]),
		ShowTags:        asStringSlice(m["show_tags"]),
		Pictures:        picturesFromAny(m["pictures"]),
		SubCommentCount: asString(m["sub_comment_count"]),
	}
	for _, raw := range asSlice(m["sub_comments"]) {
		if sub := asMap(raw); sub != nil {
			sc := commentFromMap(sub)
			if sc.ID != "" {
				c.SubComments = append(c.SubComments, sc)
			}
		}
	}
	return c
}

func userNotesFromInitialStateResult(jsonValue map[string]any, text string) ([]Note, error) {
	var root map[string]any
	if jsonValue != nil {
		root = jsonValue
	} else {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		if err := json.Unmarshal([]byte(text), &root); err != nil {
			return nil, err
		}
	}
	return userNotesFromInitialState(root), nil
}

func userNotesFromInitialState(root map[string]any) []Note {
	var rawNotes any
	if user := asMap(root["user"]); user != nil {
		rawNotes = user["notes"]
	} else {
		rawNotes = root["notes"]
	}
	var out []Note
	appendNote := func(v any) {
		item := asMap(v)
		if item == nil {
			return
		}
		n := noteFromUserInitialStateItem(item)
		if n.ID != "" {
			out = append(out, n)
		}
	}
	for _, v := range asSlice(rawNotes) {
		if group := asSlice(v); group != nil {
			for _, item := range group {
				appendNote(item)
			}
			continue
		}
		appendNote(v)
	}
	return out
}

func noteFromUserInitialStateItem(item map[string]any) Note {
	card := asMap(item["noteCard"])
	if card == nil {
		card = asMap(item["note_card"])
	}
	if card == nil {
		card = item
	}
	id := firstString(item["id"], card["noteId"], card["note_id"], card["id"])
	token := firstString(item["xsecToken"], item["xsec_token"], card["xsecToken"], card["xsec_token"])
	n := noteFromSearchCard(
		id,
		token,
		firstString(card["type"]),
		firstString(card["displayTitle"], card["display_title"], card["title"]),
		asMap(card["user"]),
		asMap(card["interactInfo"]),
	)
	if n.LikedCount == "" {
		interact := asMap(card["interact_info"])
		n.LikedCount = firstString(interact["liked_count"], interact["likedCount"])
		n.CollectedCount = firstString(interact["collected_count"], interact["collectedCount"])
		n.CommentCount = firstString(interact["comment_count"], interact["commentCount"])
		n.ShareCount = firstString(interact["shared_count"], interact["shareCount"])
	}
	if cover := asMap(card["cover"]); cover != nil {
		if imageURL := firstString(cover["urlDefault"], cover["url_default"], cover["urlPre"], cover["url"]); imageURL != "" {
			n.Images = append(n.Images, imageURL)
		}
	}
	return n
}

func noteFromSearchCard(id, token, typ, title string, user, interact map[string]any) Note {
	n := Note{
		ID:        id,
		XsecToken: token,
		Type:      typ,
		Title:     title,
	}
	if user != nil {
		n.Author = asString(user["nick_name"])
		if n.Author == "" {
			n.Author = asString(user["nickname"])
		}
		if n.Author == "" {
			n.Author = asString(user["nickName"])
		}
		n.AuthorID = asString(user["user_id"])
		if n.AuthorID == "" {
			n.AuthorID = asString(user["userId"])
		}
	}
	if interact != nil {
		n.LikedCount = asString(interact["liked_count"])
		if n.LikedCount == "" {
			n.LikedCount = asString(interact["likedCount"])
		}
		n.CollectedCount = asString(interact["collected_count"])
		if n.CollectedCount == "" {
			n.CollectedCount = asString(interact["collectedCount"])
		}
		n.CommentCount = asString(interact["comment_count"])
		if n.CommentCount == "" {
			n.CommentCount = asString(interact["commentCount"])
		}
		n.ShareCount = asString(interact["shared_count"])
		if n.ShareCount == "" {
			n.ShareCount = asString(interact["shareCount"])
		}
	}
	if n.ID != "" {
		n.URL = "https://www.xiaohongshu.com/explore/" + n.ID
		if token != "" {
			n.URL += "?xsec_token=" + url.QueryEscape(token) + "&xsec_source=pc_search"
		}
	}
	return n
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func asStringSlice(v any) []string {
	items := asSlice(v)
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := asString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func picturesFromAny(v any) []string {
	items := asSlice(v)
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		pic := asMap(item)
		if pic == nil {
			continue
		}
		if u := firstString(pic["url_default"], pic["urlDefault"], pic["url_pre"], pic["urlPre"]); u != "" {
			out = append(out, u)
		}
	}
	return out
}

func firstString(values ...any) string {
	for _, v := range values {
		if s := asString(v); s != "" {
			return s
		}
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
