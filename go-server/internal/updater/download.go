package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// Progress reports download progress. total is 0 when the server did not send
// a Content-Length.
type Progress func(done, total int64)

// stallTimeout is how long a download may go without receiving any bytes
// before it is treated as dead. Slow-but-moving connections are never cut off.
var stallTimeout = 90 * time.Second

// Staged is a downloaded, checksum-verified update waiting to be installed.
type Staged struct {
	// Dir is the temporary directory holding the update. Removing it after a
	// successful install is the helper's job.
	Dir string
	// Path is the artifact that replaces the app: the .app bundle on macOS,
	// the executable elsewhere.
	Path string
	// Version is the release version the artifact was built from.
	Version string
}

// Close discards the staged update. Call it when installing fails.
func (s *Staged) Close() error {
	if s == nil || s.Dir == "" {
		return nil
	}
	return os.RemoveAll(s.Dir)
}

// Download fetches the release asset for this platform, verifies its SHA-256
// against the release's checksums.txt and prepares it for Apply.
//
// The returned Staged owns a temporary directory; the caller must either
// Apply it (the helper removes the directory after relaunching) or Close it.
func (c *Client) Download(ctx context.Context, rel *Release, progress Progress) (*Staged, error) {
	if rel == nil {
		return nil, errors.New("没有可用的发布版本")
	}
	version := rel.Version()
	asset, err := rel.AssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	checksums, err := c.fetchChecksums(ctx, rel)
	if err != nil {
		return nil, err
	}
	want, ok := checksums[asset.Name]
	if !ok {
		return nil, fmt.Errorf("校验文件 checksums.txt 里没有 %s", asset.Name)
	}

	dir, err := os.MkdirTemp("", "grabby-update-"+version+"-")
	if err != nil {
		return nil, fmt.Errorf("无法创建临时目录: %w", err)
	}
	cleanup := func(err error) (*Staged, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	archive := filepath.Join(dir, asset.Name)
	got, err := c.downloadFile(ctx, asset.BrowserDownloadURL, archive, progress)
	if err != nil {
		return cleanup(err)
	}
	if !strings.EqualFold(got, want) {
		return cleanup(fmt.Errorf("更新包校验失败：期望 %s，实际 %s；已取消安装", want, got))
	}

	path, err := stage(rel, dir, archive)
	if err != nil {
		return cleanup(err)
	}
	return &Staged{Dir: dir, Path: path, Version: version}, nil
}

func (c *Client) fetchChecksums(ctx context.Context, rel *Release) (map[string]string, error) {
	asset, ok := rel.Asset("checksums.txt")
	if !ok {
		return nil, errors.New("该版本没有 checksums.txt，无法验证更新包；已取消更新")
	}
	raw, err := c.getBytes(ctx, asset.BrowserDownloadURL, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("下载校验文件失败: %w", err)
	}
	sums := parseChecksums(raw)
	if len(sums) == 0 {
		return nil, errors.New("校验文件为空，无法验证更新包；已取消更新")
	}
	return sums, nil
}

// downloadFile streams url into dest and returns the SHA-256 of what it wrote.
// progress is throttled so a 100 MB download does not flood the UI.
func (c *Client) downloadFile(ctx context.Context, url, dest string, progress Progress) (string, error) {
	dlCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent())
	// A download is much larger than an API response, so it gets its own
	// client without the API's short timeout; ctx still bounds it.
	hc := &http.Client{}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载更新包失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载更新包失败: 服务器返回 %s", resp.Status)
	}

	// Watch for a stalled connection (common on slow international links) and
	// cut it off instead of leaving the progress bar frozen forever.
	stalled := &atomic.Bool{}
	progressCh := make(chan struct{}, 1)
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		timer := time.NewTimer(stallTimeout)
		defer timer.Stop()
		for {
			select {
			case <-watchDone:
				return
			case <-progressCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(stallTimeout)
			case <-timer.C:
				stalled.Store(true)
				cancel()
				return
			case <-dlCtx.Done():
				return
			}
		}
	}()

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("无法写入临时文件: %w", err)
	}
	defer f.Close()

	total := resp.ContentLength
	hasher := sha256.New()
	buf := make([]byte, 64<<10)
	var done int64
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			select {
			case progressCh <- struct{}{}:
			default:
			}
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("写入临时文件失败: %w", werr)
			}
			hasher.Write(buf[:n])
			done += int64(n)
			if progress != nil && time.Since(last) >= 200*time.Millisecond {
				last = time.Now()
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if stalled.Load() {
				return "", fmt.Errorf("下载超时：%s 没有收到新数据，请检查网络后重试", stallTimeout)
			}
			return "", fmt.Errorf("下载中断: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	if total > 0 && done != total {
		return "", fmt.Errorf("下载不完整：期望 %d 字节，收到 %d", total, done)
	}
	if progress != nil {
		progress(done, total)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// stage turns the verified archive into the artifact Apply installs.
func stage(rel *Release, dir, archive string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return stageDarwin(rel, dir, archive)
	case "linux":
		return stageLinux(dir, archive)
	case "windows":
		// The release asset is already the executable.
		return archive, nil
	default:
		return "", fmt.Errorf("暂不支持在 %s 上自动更新", runtime.GOOS)
	}
}

// stageDarwin mounts the DMG, copies Grabby.app out of it and checks the
// bundle's Info.plist version, so a mismatched asset never replaces the app.
func stageDarwin(rel *Release, dir, archive string) (string, error) {
	mnt := filepath.Join(dir, "mnt")
	if err := os.MkdirAll(mnt, 0o755); err != nil {
		return "", fmt.Errorf("无法创建挂载点: %w", err)
	}
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-noverify",
		"-quiet", "-mountpoint", mnt, archive).CombinedOutput(); err != nil {
		return "", fmt.Errorf("无法打开 DMG: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", "-quiet", mnt).Run()
	}()

	entries, err := os.ReadDir(mnt)
	if err != nil {
		return "", fmt.Errorf("无法读取 DMG: %w", err)
	}
	var src string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			src = filepath.Join(mnt, e.Name())
			break
		}
	}
	if src == "" {
		return "", errors.New("DMG 里没有找到 .app")
	}

	dst := filepath.Join(dir, "Grabby.app")
	// ditto preserves symlinks, permissions and the code signature, which a
	// plain cp -R does not fully guarantee for app bundles.
	if out, err := exec.Command("ditto", src, dst).CombinedOutput(); err != nil {
		return "", fmt.Errorf("无法复制新版本: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	if err := verifyBundleVersion(dst, rel.Version()); err != nil {
		return "", err
	}
	// No quarantine is expected on a file this process downloaded, but a DMG
	// mounted from a copied file can carry the attribute through. Clear it so
	// the relaunched app never hits Gatekeeper.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", dst).Run()
	return dst, nil
}

func verifyBundleVersion(appDir, want string) error {
	exe := filepath.Join(appDir, "Contents", "MacOS", "Grabby")
	if info, err := os.Stat(exe); err != nil || info.IsDir() {
		return errors.New("新版本里没有可执行文件，已取消更新")
	}
	plist := filepath.Join(appDir, "Contents", "Info.plist")
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", plist).Output()
	if err != nil {
		return fmt.Errorf("无法读取新版本号: %w", err)
	}
	if got := strings.TrimSpace(string(out)); got != want {
		return fmt.Errorf("更新包版本不匹配：期望 %s，实际 %s", want, got)
	}
	return nil
}

// stageLinux extracts the tarball and returns the grabby binary inside it.
func stageLinux(dir, archive string) (string, error) {
	if out, err := exec.Command("tar", "-xzf", archive, "-C", dir).CombinedOutput(); err != nil {
		return "", fmt.Errorf("无法解压更新包: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "grabby" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("更新包里没有找到 grabby 可执行文件")
	}
	dst := filepath.Join(dir, "grabby-update")
	if found != dst {
		if err := os.Rename(found, dst); err != nil {
			return "", fmt.Errorf("无法准备新版本: %w", err)
		}
	}
	if err := os.Chmod(dst, 0o755); err != nil {
		return "", fmt.Errorf("无法设置可执行权限: %w", err)
	}
	return dst, nil
}
