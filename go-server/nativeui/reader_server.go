package nativeui

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// readerServer serves reader documents over http://127.0.0.1. The embedded web
// view needs a real origin for its page: YouTube's player refuses to start on
// a document loaded with LoadHTML (an opaque origin), showing "video
// unavailable". A loopback origin is enough for the iframe to play.
type readerServer struct {
	mu    sync.Mutex
	docs  map[string]string
	order []string
	base  string
}

// readerDocsKept bounds the documents kept for reloads and back/forward.
const readerDocsKept = 8

// newReaderServer listens on a random loopback port. It returns nil when the
// port cannot be opened; the reader then falls back to LoadHTML.
func newReaderServer() *readerServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil
	}
	s := &readerServer{docs: map[string]string{}, base: "http://" + ln.Addr().String()}
	mux := http.NewServeMux()
	mux.HandleFunc("/r/", s.serve)
	go func() { _ = http.Serve(ln, mux) }()
	return s
}

func (s *readerServer) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/r/")
	s.mu.Lock()
	doc, ok := s.docs[key]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, doc)
}

// publish makes doc the page at a fresh URL and returns it.
func (s *readerServer) publish(doc string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	key := hex.EncodeToString(raw[:])
	s.mu.Lock()
	s.docs[key] = doc
	s.order = append(s.order, key)
	for len(s.order) > readerDocsKept {
		delete(s.docs, s.order[0])
		s.order = s.order[1:]
	}
	s.mu.Unlock()
	return s.base + "/r/" + key
}

// loadReader shows doc in the reader web view. Documents are served over
// loopback so YouTube embeds play; without that server the document still
// renders through LoadHTML.
func (a *App) loadReader(doc, baseURL string) {
	if a.readerWV == nil {
		return
	}
	if a.readerDocs != nil {
		if url := a.readerDocs.publish(doc); url != "" {
			a.readerWV.Page().LoadURL(url)
			return
		}
	}
	a.readerWV.Page().LoadHTML(doc, baseURL)
}
