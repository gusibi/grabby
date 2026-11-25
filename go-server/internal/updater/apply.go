package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Apply installs the staged update over the running app. It starts a detached
// helper that waits for this process to exit, swaps the files and relaunches
// the app; Apply itself returns as soon as that helper is running, so the
// caller must quit the app right after.
func (s *Staged) Apply() error {
	if s == nil || s.Path == "" {
		return errors.New("没有可安装的更新包")
	}
	if _, err := os.Stat(s.Path); err != nil {
		return fmt.Errorf("更新包不存在: %w", err)
	}
	return apply(s)
}

// checkWritableDir proves the updater can create files in the directory it is
// about to replace, and fails early with a readable error (a .deb install in
// /opt, for example, is owned by root).
func checkWritableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".grabby-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

func writeHelper(dir, name, body string, mode os.FileMode) (string, error) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		return "", fmt.Errorf("无法写入更新脚本: %w", err)
	}
	return path, nil
}

// shQuote quotes s for /bin/sh.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote quotes s for PowerShell single-quoted strings.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
