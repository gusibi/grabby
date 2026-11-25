//go:build darwin || linux

package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func apply(s *Staged) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前程序: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	var script string
	switch runtime.GOOS {
	case "darwin":
		bundle, err := appBundle(exe)
		if err != nil {
			return err
		}
		// Running straight from a mounted DMG: it is read-only, and replacing
		// it would only lose the update when the image is ejected.
		if strings.HasPrefix(bundle, "/Volumes/") {
			return errors.New("Grabby 正在从磁盘映像运行，无法自动更新；请先把它拖到「应用程序」文件夹，再检查更新")
		}
		if err := checkWritableDir(filepath.Dir(bundle)); err != nil {
			return fmt.Errorf("没有权限更新 %s: %w", bundle, err)
		}
		script = darwinScript(bundle, s)
	default:
		if err := checkWritableDir(filepath.Dir(exe)); err != nil {
			return fmt.Errorf("没有权限更新 %s（如果通过系统包安装，请用包管理器更新）: %w", exe, err)
		}
		script = linuxScript(exe, s)
	}

	path, err := writeHelper(s.Dir, "apply-update.sh", script, 0o700)
	if err != nil {
		return err
	}
	return spawnDetached("/bin/sh", path)
}

// appBundle walks up from the executable to the enclosing .app directory.
func appBundle(exe string) (string, error) {
	dir := filepath.Dir(exe)
	for {
		if strings.HasSuffix(dir, ".app") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("当前不是 .app 安装，无法自动替换；请下载新版本手动安装")
}

func spawnDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	// A new session keeps the helper alive after the app's process group is
	// torn down; its output is dropped because there is no terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法启动更新脚本: %w", err)
	}
	return cmd.Process.Release()
}

// darwinScript replaces the app bundle and reopens it once the running process
// has exited. If the copy fails it rolls the old bundle back and reopens that,
// so a bad update cannot leave the user with no app. The previous bundle stays
// as a hidden backup until CleanupBackups runs on a later launch.
func darwinScript(bundle string, s *Staged) string {
	backup := filepath.Join(filepath.Dir(bundle), "."+filepath.Base(bundle)+".update-backup-"+strconv.Itoa(os.Getpid()))
	stage := shQuote(s.Dir)
	return fmt.Sprintf(`#!/bin/sh
set +e
log="${HOME:-/tmp}/Library/Logs/Grabby/update.log"
mkdir -p "$(dirname "$log")" 2>/dev/null
exec >>"$log" 2>&1
echo "--- $(date) update pid=%d"

pid=%d
dest=%s
new=%s
backup=%s

i=0
while kill -0 "$pid" 2>/dev/null; do
	i=$((i+1))
	[ "$i" -gt 900 ] && break
	sleep 0.2
done
sleep 0.3

rm -rf "$backup"
if ! mv "$dest" "$backup"; then
	echo "cannot move $dest; reopening the current version"
	/usr/bin/osascript -e 'display notification "无法替换当前版本，请手动下载新版本" with title "Grabby"' >/dev/null 2>&1
	/usr/bin/open "$dest" >/dev/null 2>&1
	rm -rf %s
	exit 1
fi
if ! /usr/bin/ditto "$new" "$dest"; then
	echo "copy failed; rolling back"
	rm -rf "$dest"
	mv "$backup" "$dest"
	/usr/bin/osascript -e 'display notification "更新安装失败，已恢复到原版本" with title "Grabby"' >/dev/null 2>&1
	/usr/bin/open "$dest" >/dev/null 2>&1
	rm -rf %s
	exit 2
fi
/usr/bin/xattr -dr com.apple.quarantine "$dest" >/dev/null 2>&1
echo "opening updated $dest"
/usr/bin/open "$dest"
sleep 2
rm -rf %s
echo "done"
`, os.Getpid(), os.Getpid(), shQuote(bundle), shQuote(s.Path), shQuote(backup), stage, stage, stage)
}

// linuxScript copies the new binary next to the old one, renames it into place
// and starts it. Renaming over a running executable is safe on Linux; the wait
// above only makes the restart a restart rather than a second instance.
func linuxScript(exe string, s *Staged) string {
	stage := shQuote(s.Dir)
	return fmt.Sprintf(`#!/bin/sh
set +e
pid=%d
dest=%s
new=%s

i=0
while kill -0 "$pid" 2>/dev/null; do
	i=$((i+1))
	[ "$i" -gt 900 ] && break
	sleep 0.2
done
sleep 0.3

tmp="$dest.grabby-update"
if ! cp "$new" "$tmp"; then
	rm -rf %s
	nohup "$dest" >/dev/null 2>&1 &
	exit 1
fi
chmod 0755 "$tmp"
if ! mv -f "$tmp" "$dest"; then
	rm -f "$tmp"
	rm -rf %s
	nohup "$dest" >/dev/null 2>&1 &
	exit 2
fi
nohup "$dest" >/dev/null 2>&1 &
sleep 1
rm -rf %s
`, os.Getpid(), shQuote(exe), shQuote(s.Path), stage, stage, stage)
}

// CleanupBackups removes bundles left behind by earlier updates. It is called
// at startup, when the app is safely running from the replacement.
func CleanupBackups() {
	if runtime.GOOS != "darwin" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	bundle, err := appBundle(exe)
	if err != nil {
		return
	}
	parent := filepath.Dir(bundle)
	prefix := "." + filepath.Base(bundle) + ".update-backup-"
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < 10*time.Minute {
			continue
		}
		_ = os.RemoveAll(filepath.Join(parent, e.Name()))
	}
}
