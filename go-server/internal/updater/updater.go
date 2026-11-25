// Package updater implements self-update for the Grabby desktop app.
//
// The flow is: ask GitHub for the latest release, pick the asset that matches
// this platform, download it and verify its SHA-256 against the release's
// checksums.txt, stage it, then swap it in from a detached helper script and
// relaunch.
//
// The app is not signed with an Apple Developer ID, so asking the user to
// download and open a DMG again means another trip through Gatekeeper. The
// updater replaces the installed app in place instead: a file fetched from
// GitHub by this process carries no quarantine attribute, so the relaunched
// app starts without a security prompt.
//
// The checksum is never skipped. checksums.txt is fetched over TLS from the
// same release, and an asset that does not match it is deleted, not installed.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// RepoOwner and RepoName identify the public release feed.
	RepoOwner = "gusibi"
	RepoName  = "grabby"

	apiBase = "https://api.github.com"
	webBase = "https://github.com"
)

// Version is the running app version. Release builds set it with
//
//	go build -ldflags "-X go-server/internal/updater.Version=0.9.0"
//
// A build without the flag reports itself as a development build and refuses
// to auto-install releases over itself.
var Version = "dev"

// ReleasesURL is the public release page, for “查看发行说明”.
func ReleasesURL() string { return webBase + "/" + RepoOwner + "/" + RepoName + "/releases" }

// CurrentVersion is Version without a leading v, for display and comparison.
func CurrentVersion() string {
	return strings.TrimPrefix(strings.TrimSpace(Version), "v")
}

// DisplayVersion is what the settings page shows for the running build.
func DisplayVersion() string {
	if !IsReleaseBuild() {
		return "开发版"
	}
	return CurrentVersion()
}

var releaseVersionPattern = regexp.MustCompile(`^\d+(\.\d+)*`)

// IsReleaseBuild reports whether Version was set by a release build. Source
// builds are deliberately excluded from self-update: replacing a developer's
// own binary with a release artifact is surprising and hard to undo.
func IsReleaseBuild() bool {
	return releaseVersionPattern.MatchString(CurrentVersion())
}

// Release is the subset of the GitHub release API this package uses.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []Asset   `json:"assets"`
}

// Asset is one downloadable file of a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Version is the release tag without the leading v.
func (r *Release) Version() string {
	return strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
}

// NewerThan reports whether the release is newer than current. A source build
// ("dev") compares as 0.0.0, so any release wins — but callers still check
// IsReleaseBuild before installing.
func (r *Release) NewerThan(current string) bool {
	return CompareVersions(r.Version(), current) > 0
}

// Asset returns the release asset with the given name.
func (r *Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// AssetFor picks the asset the running platform can install.
func (r *Release) AssetFor(goos, goarch string) (Asset, error) {
	name, err := AssetName(r.Version(), goos, goarch)
	if err != nil {
		return Asset{}, err
	}
	if a, ok := r.Asset(name); ok {
		return a, nil
	}
	// Compare case-insensitively: the name is ours, but a release built by
	// hand may capitalise it differently.
	lower := strings.ToLower(name)
	for _, a := range r.Assets {
		if strings.ToLower(a.Name) == lower {
			return a, nil
		}
	}
	names := make([]string, 0, len(r.Assets))
	for _, a := range r.Assets {
		names = append(names, a.Name)
	}
	return Asset{}, fmt.Errorf("发布 v%s 里没有适用于 %s/%s 的安装包（%s）",
		r.Version(), goos, goarch, strings.Join(names, ", "))
}

// AssetName is the expected release asset name for a platform.
func AssetName(version, goos, goarch string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	switch goos {
	case "darwin":
		// The DMG is a universal binary, so Intel and Apple Silicon share it.
		return fmt.Sprintf("Grabby-%s-macOS-universal.dmg", v), nil
	case "windows":
		arch, err := windowsArch(goarch)
		if err != nil {
			return "", err
		}
		// The portable exe, not the NSIS setup: the updater swaps the
		// executable itself and never shows an installer window.
		return fmt.Sprintf("Grabby-%s-Windows-%s.exe", v, arch), nil
	case "linux":
		arch, err := linuxArch(goarch)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Grabby-%s-Linux-%s.tar.gz", v, arch), nil
	default:
		return "", fmt.Errorf("暂不支持在 %s 上自动更新", goos)
	}
}

func windowsArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("暂不支持 Windows %s", goarch)
	}
}

func linuxArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("暂不支持 Linux %s", goarch)
	}
}

// Client talks to the GitHub release API.
type Client struct {
	// Base is the API root; tests point it at an httptest server.
	Base string
	// HTTP is the client used for API calls. The zero value is fine.
	HTTP *http.Client
}

// New returns a client for the public GitHub API.
func New() *Client {
	return &Client{Base: apiBase}
}

func (c *Client) base() string {
	if strings.TrimSpace(c.Base) == "" {
		return apiBase
	}
	return strings.TrimRight(c.Base, "/")
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 45 * time.Second}
}

func userAgent() string {
	v := CurrentVersion()
	if v == "" {
		v = "dev"
	}
	return "Grabby-Updater/" + v
}

// Latest fetches the newest non-draft, non-prerelease release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	url := c.base() + "/repos/" + RepoOwner + "/" + RepoName + "/releases/latest"
	raw, err := c.getBytes(ctx, url, 4<<20)
	if err != nil {
		return nil, err
	}
	var rel Release
	if err := json.Unmarshal(raw, &rel); err != nil {
		return nil, fmt.Errorf("无法解析 GitHub 返回: %w", err)
	}
	if rel.TagName == "" {
		return nil, errors.New("GitHub 没有返回版本号")
	}
	if rel.Draft || rel.Prerelease {
		return nil, fmt.Errorf("最新发布 %s 是预发布版本", rel.TagName)
	}
	return &rel, nil
}

func (c *Client) getBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回 %s", resp.Status)
	}
	return raw, nil
}

// CompareVersions compares two dotted versions, returning -1, 0 or 1. It
// understands a leading v, build metadata and semver-style prerelease tags;
// terms it cannot parse compare as zero.
func CompareVersions(a, b string) int {
	ac, ap := parseVersion(a)
	bc, bp := parseVersion(b)
	for i := 0; i < len(ac) || i < len(bc); i++ {
		var av, bv int
		if i < len(ac) {
			av = ac[i]
		}
		if i < len(bc) {
			bv = bc[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	// A release sorts after any prerelease of the same core.
	switch {
	case ap == "" && bp == "":
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	}
	return comparePrerelease(ap, bp)
}

func parseVersion(v string) (core []int, prerelease string) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		prerelease = v[i+1:]
		v = v[:i]
	}
	if !releaseVersionPattern.MatchString(v) {
		return nil, prerelease
	}
	for _, part := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(part)
		core = append(core, n)
	}
	return core, prerelease
}

func comparePrerelease(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aerr == nil:
			// Numeric identifiers sort before alphanumeric ones.
			return -1
		case berr == nil:
			return 1
		default:
			if as[i] != bs[i] {
				if as[i] < bs[i] {
					return -1
				}
				return 1
			}
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// parseChecksums reads sha256sum output ("<hash>  ./<file>") into a map keyed
// by base name.
func parseChecksums(raw []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(fields[0])
		if len(hash) != 64 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		name = strings.TrimPrefix(name, "./")
		name = name[strings.LastIndexByte(name, '/')+1:]
		if name != "" {
			out[name] = hash
		}
	}
	return out
}
