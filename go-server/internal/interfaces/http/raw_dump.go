package httpiface

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Raw response dumps are a development aid: adapters for private/undocumented
// channels (GSC batchexecute, Bing webmaster pages) must be written against
// real payloads, not guesses. The dump dir is configurable so it can point
// outside the repo in production.
//
// Default: ./testdata/raw relative to the server's working directory
// (start.sh runs from go-server/).
func rawDumpDir() string {
	if dir := strings.TrimSpace(os.Getenv("GRABBY_RAW_DIR")); dir != "" {
		return dir
	}
	return filepath.Join("testdata", "raw")
}

var unsafeDumpChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// sanitizeDumpName keeps file names predictable and prevents path traversal.
func sanitizeDumpName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = unsafeDumpChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-._")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

// writeRawDump writes payload as indented JSON and returns the file path.
func writeRawDump(name string, payload any) (string, error) {
	dir := rawDumpDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create raw dump dir: %w", err)
	}

	base := sanitizeDumpName(name)
	if base == "" {
		base = "capture"
	}
	file := filepath.Join(dir, fmt.Sprintf("%s-%s.json", base, time.Now().Format("20060102-150405")))

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode raw dump: %w", err)
	}
	if err := os.WriteFile(file, body, 0o644); err != nil {
		return "", fmt.Errorf("write raw dump: %w", err)
	}
	return file, nil
}
