//go:build darwin

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeFakeDMG builds a small DMG containing a fake Grabby.app, so the staging
// path (mount, copy, version check) can be tested without downloading a
// release.
func makeFakeDMG(t *testing.T, version string) string {
	t.Helper()
	src := t.TempDir()
	app := filepath.Join(src, "Grabby.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleShortVersionString</key><string>` + version + `</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(app, "Contents", "MacOS", "Grabby")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dmg := filepath.Join(t.TempDir(), "fake.dmg")
	if out, err := exec.Command("hdiutil", "create", "-quiet", "-volname", "GrabbyTest",
		"-srcfolder", src, "-ov", "-format", "UDZO", dmg).CombinedOutput(); err != nil {
		t.Skipf("hdiutil unavailable: %v: %s", err, out)
	}
	return dmg
}

func TestStageDarwinCopiesBundle(t *testing.T) {
	dmg := makeFakeDMG(t, "0.9.0")
	dir := t.TempDir()
	staged, err := stageDarwin(&Release{TagName: "v0.9.0"}, dir, dmg)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(staged) != "Grabby.app" {
		t.Fatalf("staged %q", staged)
	}
	if _, err := os.Stat(filepath.Join(staged, "Contents", "MacOS", "Grabby")); err != nil {
		t.Fatalf("staged app is incomplete: %v", err)
	}
}

func TestStageDarwinRejectsVersionMismatch(t *testing.T) {
	dmg := makeFakeDMG(t, "0.8.0")
	dir := t.TempDir()
	_, err := stageDarwin(&Release{TagName: "v0.9.0"}, dir, dmg)
	if err == nil || !strings.Contains(err.Error(), "版本不匹配") {
		t.Fatalf("expected a version mismatch, got %v", err)
	}
}

// TestDownloadStagesRelease runs the whole pipeline against a local release:
// download, checksum, mount, copy and version check.
func TestDownloadStagesRelease(t *testing.T) {
	dmg := makeFakeDMG(t, "0.9.0")
	payload, err := os.ReadFile(dmg)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	name := "Grabby-0.9.0-macOS-universal.dmg"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			_, _ = w.Write(payload)
		case "/checksums.txt":
			fmt.Fprintf(w, "%s  ./%s\n", hex.EncodeToString(sum[:]), name)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rel := &Release{
		TagName: "v0.9.0",
		Assets: []Asset{
			{Name: name, BrowserDownloadURL: srv.URL + "/asset", Size: int64(len(payload))},
			{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
		},
	}
	staged, err := (&Client{Base: srv.URL}).Download(context.Background(), rel, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	if staged.Version != "0.9.0" {
		t.Fatalf("staged version = %q", staged.Version)
	}
	if _, err := os.Stat(filepath.Join(staged.Path, "Contents", "MacOS", "Grabby")); err != nil {
		t.Fatalf("staged app incomplete: %v", err)
	}
}

func writeBundle(t *testing.T, path, marker string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "Contents", "MacOS", marker), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDarwinScriptSwapsBundle runs the real helper script with the wait and
// the open call neutralised, then asserts the swap and the rollback backup.
func TestDarwinScriptSwapsBundle(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Grabby.app")
	writeBundle(t, dest, "old")

	newApp := filepath.Join(t.TempDir(), "Grabby.app")
	writeBundle(t, newApp, "new")

	staged := &Staged{Dir: t.TempDir(), Path: newApp}
	script := darwinScript(dest, staged)
	script = strings.ReplaceAll(script, fmt.Sprintf("pid=%d", os.Getpid()), "pid=999999")
	script = strings.ReplaceAll(script, "/usr/bin/open", "/usr/bin/true")
	script = strings.ReplaceAll(script, "/usr/bin/osascript", "/usr/bin/true")

	path := filepath.Join(t.TempDir(), "apply.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", path)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(dest, "Contents", "MacOS", "new")); err != nil {
		t.Fatalf("bundle was not replaced: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	backup := ""
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".Grabby.app.update-backup-") {
			backup = e.Name()
		}
	}
	if backup == "" {
		t.Fatal("expected the previous bundle to be kept as a backup")
	}
	if _, err := os.Stat(filepath.Join(root, backup, "Contents", "MacOS", "old")); err != nil {
		t.Fatalf("backup does not hold the previous bundle: %v", err)
	}
	if _, err := os.Stat(staged.Dir); !os.IsNotExist(err) {
		t.Fatalf("staging directory should be removed, stat err = %v", err)
	}
}

func TestDarwinScriptRollsBackOnCopyFailure(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Grabby.app")
	writeBundle(t, dest, "old")

	staged := &Staged{Dir: t.TempDir(), Path: filepath.Join(t.TempDir(), "missing.app")}
	script := darwinScript(dest, staged)
	script = strings.ReplaceAll(script, fmt.Sprintf("pid=%d", os.Getpid()), "pid=999999")
	script = strings.ReplaceAll(script, "/usr/bin/open", "/usr/bin/true")
	script = strings.ReplaceAll(script, "/usr/bin/osascript", "/usr/bin/true")

	path := filepath.Join(t.TempDir(), "apply.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", path)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	_ = cmd.Run() // exit 2 is expected

	if _, err := os.Stat(filepath.Join(dest, "Contents", "MacOS", "old")); err != nil {
		t.Fatalf("old bundle was not restored: %v", err)
	}
}
