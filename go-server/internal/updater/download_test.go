package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func platformRelease(base, version string, payloadSize int) *Release {
	name, _ := AssetName(version, runtime.GOOS, runtime.GOARCH)
	return &Release{
		TagName: "v" + version,
		Assets: []Asset{
			{Name: name, BrowserDownloadURL: base + "/asset", Size: int64(payloadSize)},
			{Name: "checksums.txt", BrowserDownloadURL: base + "/checksums.txt"},
		},
	}
}

func TestDownloadRejectsBadChecksum(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write([]byte("staged update"))
		case "/checksums.txt":
			name, _ := AssetName("0.9.0", runtime.GOOS, runtime.GOARCH)
			wrong := strings.Repeat("0", 64)
			_, _ = w.Write([]byte(wrong + "  ./" + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rel := platformRelease(srv.URL, "0.9.0", 13)
	c := &Client{Base: srv.URL}
	staged, err := c.Download(context.Background(), rel, nil)
	if err == nil {
		staged.Close()
		t.Fatal("expected a checksum mismatch to abort the update")
	}
	if !strings.Contains(err.Error(), "校验失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadRequiresChecksums(t *testing.T) {
	name, _ := AssetName("0.9.0", runtime.GOOS, runtime.GOARCH)
	rel := &Release{
		TagName: "v0.9.0",
		Assets:  []Asset{{Name: name, BrowserDownloadURL: "http://127.0.0.1:1/asset"}},
	}
	c := &Client{Base: "http://127.0.0.1:1"}
	if _, err := c.Download(context.Background(), rel, nil); err == nil {
		t.Fatal("expected a release without checksums.txt to be rejected")
	}
}

func TestDownloadFileHashesAndReportsProgress(t *testing.T) {
	payload := []byte(strings.Repeat("grabby", 5000))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "30000")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "download.bin")
	var lastDone, lastTotal int64
	c := &Client{}
	sum, err := c.downloadFile(context.Background(), srv.URL, dest, func(done, total int64) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded %d bytes, want %d", len(data), len(payload))
	}
	want := sha256.Sum256(payload)
	if sum != hex.EncodeToString(want[:]) {
		t.Fatalf("hash = %s, want %s", sum, hex.EncodeToString(want[:]))
	}
	if lastDone != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("progress ended at %d/%d", lastDone, lastTotal)
	}
}

func TestDownloadFileStallIsCutOff(t *testing.T) {
	old := stallTimeout
	stallTimeout = 200 * time.Millisecond
	defer func() { stallTimeout = old }()

	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("x"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	start := time.Now()
	c := &Client{}
	_, err := c.downloadFile(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), nil)
	if err == nil || !strings.Contains(err.Error(), "下载超时") {
		t.Fatalf("expected a stall timeout, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("stall was not cut off promptly: %v", elapsed)
	}
}

func TestDownloadFileTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	c := &Client{}
	_, err := c.downloadFile(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), nil)
	if err == nil {
		t.Fatal("expected a truncated download to fail")
	}
	if !strings.Contains(err.Error(), "下载不完整") && !strings.Contains(err.Error(), "下载中断") {
		t.Fatalf("unexpected error: %v", err)
	}
}
