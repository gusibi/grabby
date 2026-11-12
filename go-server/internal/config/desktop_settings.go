package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DesktopSettings holds preferences owned by the desktop app rather than the
// shared .env file. It lives in the per-user data directory, next to grabby.db,
// so the desktop build can default to "no token" without changing how the
// headless server or the web frontend read their credentials.
type DesktopSettings struct {
	Version         int    `json:"version"`
	ExternalEnabled bool   `json:"external_enabled"`
	APIToken        string `json:"api_token"`
}

const desktopSettingsFile = "desktop-settings.json"

// DesktopSettingsPath resolves the config file. The desktop shell always chdirs
// into the data directory, so the environment variable is only needed by the
// HTTP handler that may run on a different goroutine.
func DesktopSettingsPath() string {
	dir := os.Getenv("GRABBY_DESKTOP_DATA_DIR")
	if dir == "" {
		return desktopSettingsFile
	}
	return filepath.Join(dir, desktopSettingsFile)
}

// DesktopDataDir reports the directory that holds grabby.db and
// desktop-settings.json. The desktop shell chdirs into it before the server
// starts, so the working directory is authoritative.
func DesktopDataDir() string {
	if dir := os.Getenv("GRABBY_DESKTOP_DATA_DIR"); dir != "" {
		return dir
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func LoadDesktopSettings() DesktopSettings {
	b, err := os.ReadFile(DesktopSettingsPath())
	if err != nil {
		return DesktopSettings{Version: 1}
	}
	var s DesktopSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return DesktopSettings{Version: 1}
	}
	if s.Version == 0 {
		s.Version = 1
	}
	return s
}

func SaveDesktopSettings(s DesktopSettings) error {
	s.Version = 1
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := DesktopSettingsPath()
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// The file may contain an API token, so keep it user-only and write it
	// atomically to avoid a truncated config after a crash.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
