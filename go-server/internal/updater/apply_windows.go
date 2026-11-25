//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const createNoWindow = 0x08000000

func apply(s *Staged) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前程序: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if err := checkWritableDir(filepath.Dir(exe)); err != nil {
		return fmt.Errorf("没有权限更新 %s: %w", exe, err)
	}

	script := windowsScript(exe, s)
	path, err := writeHelper(s.Dir, "apply-update.ps1", script, 0o600)
	if err != nil {
		return err
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法启动更新脚本: %w", err)
	}
	return cmd.Process.Release()
}

// windowsScript waits for the app to exit, copies the new executable over the
// old one and restarts it. The copy is retried because antivirus and the
// just-exited process can briefly hold the file.
func windowsScript(exe string, s *Staged) string {
	return fmt.Sprintf(`$ErrorActionPreference = 'SilentlyContinue'
$targetPid = %d
try { Wait-Process -Id $targetPid -Timeout 60 -ErrorAction SilentlyContinue } catch {}
Start-Sleep -Milliseconds 500

$dest = %s
$new = %s
$stage = %s

for ($i = 0; $i -lt 60; $i++) {
    try {
        Copy-Item -LiteralPath $new -Destination $dest -Force -ErrorAction Stop
        break
    } catch {
        Start-Sleep -Milliseconds 500
    }
}
Start-Process -FilePath $dest
Start-Sleep -Milliseconds 500
Remove-Item -LiteralPath $stage -Recurse -Force
`, os.Getpid(), psQuote(exe), psQuote(s.Path), psQuote(s.Dir))
}

// CleanupBackups is a no-op on Windows; the update script removes its own
// staging directory.
func CleanupBackups() {}
