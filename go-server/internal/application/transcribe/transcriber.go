package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
)

const (
	maxAudioDownloadBytes  = 200 * 1024 * 1024 // 200 MB
	compressThresholdBytes = 18 * 1024 * 1024  // 18 MB -> compress below 25 MB Whisper limit
)

// ToolStatus describes whether a local ASR CLI dependency is available.
type ToolStatus struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
}

// EnvStatus reports local and cloud ASR readiness.
type EnvStatus struct {
	OS             string                `json:"os"`
	Arch           string                `json:"arch"`
	Tools          map[string]ToolStatus `json:"tools"`
	LocalReady     bool                  `json:"local_ready"`
	CloudReady     bool                  `json:"cloud_ready"`
	InstallCommand string                `json:"install_command"`
	ScriptPath     string                `json:"script_path,omitempty"`
}

// Transcriber converts audio or video media into text transcripts using either
// local CLI tools (mlx_whisper / whisper) or cloud OpenAI-compatible APIs.
type Transcriber struct {
	settingsProvider func() domainai.AISettings
	logger           *zap.Logger
	httpClient       *http.Client
}

// NewTranscriber constructs a Transcriber instance.
func NewTranscriber(settingsProvider func() domainai.AISettings, logger *zap.Logger) *Transcriber {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Transcriber{
		settingsProvider: settingsProvider,
		logger:           logger,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (t *Transcriber) getSettings() domainai.AISettings {
	if t.settingsProvider == nil {
		return domainai.NormalizeAISettings(domainai.AISettings{})
	}
	return domainai.NormalizeAISettings(t.settingsProvider())
}

// CheckEnvironment inspects local binary availability and cloud ASR config.
func (t *Transcriber) CheckEnvironment() EnvStatus {
	tools := map[string]ToolStatus{
		"ffmpeg":      findTool("ffmpeg"),
		"yt-dlp":      findTool("yt-dlp"),
		"mlx_whisper": findTool("mlx_whisper"),
		"whisper":     findTool("whisper"),
	}
	settings := t.getSettings()
	scriptPath := findInstallScript()
	installCmd := defaultInstallCommand(scriptPath)

	return EnvStatus{
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Tools:          tools,
		LocalReady:     tools["mlx_whisper"].Installed || tools["whisper"].Installed,
		CloudReady:     settings.ASRBaseURL != "" && (settings.ASRAPIKey != "" || settings.ASRProvider == "custom"),
		InstallCommand: installCmd,
		ScriptPath:     scriptPath,
	}
}

// InstallLocalTools runs scripts/install-asr-tools.sh (or an inline fallback)
// to install ffmpeg, yt-dlp, and mlx-whisper/openai-whisper locally.
func (t *Transcriber) InstallLocalTools(ctx context.Context) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	scriptPath := findInstallScript()
	var cmd *exec.Cmd
	if scriptPath != "" {
		cmd = exec.CommandContext(ctx, "bash", scriptPath)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", fallbackInstallScript())
	}
	cmd.Env = extendedPATH(os.Environ())

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		t.logger.Warn("Local ASR tool install failed", zap.Error(err), zap.String("output", output))
		return output, fmt.Errorf("install local ASR tools failed: %w", err)
	}
	t.logger.Info("Local ASR tool install completed")
	return output, nil
}

// TranscribeMedia downloads (and if needed compresses) media from mediaURL and
// converts speech to text. When isPageURL is true, yt-dlp extracts audio first.
func (t *Transcriber) TranscribeMedia(ctx context.Context, mediaURL string, isPageURL bool) (string, error) {
	settings := t.getSettings()
	if settings.ASRMode == "disabled" {
		return "", errors.New("ASR transcription is disabled")
	}

	tmpDir, err := os.MkdirTemp("", "grabby-asr-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var audioPath string
	if isPageURL {
		audioPath, err = t.downloadPageAudioWithYTDLP(ctx, mediaURL, tmpDir)
	} else {
		audioPath, err = t.downloadDirectAudio(ctx, mediaURL, tmpDir)
	}
	if err != nil {
		return "", err
	}

	audioPath = t.maybeCompressAudio(ctx, audioPath, tmpDir)

	switch settings.ASRMode {
	case "local":
		return t.transcribeLocal(ctx, settings, audioPath, tmpDir)
	case "cloud":
		return t.transcribeCloud(ctx, settings, audioPath)
	default: // "auto": prefer local when installed, otherwise cloud API
		if t.hasLocalWhisper(settings) {
			txt, localErr := t.transcribeLocal(ctx, settings, audioPath, tmpDir)
			if localErr == nil && strings.TrimSpace(txt) != "" {
				return txt, nil
			}
			t.logger.Warn("Local ASR failed, falling back to cloud ASR", zap.Error(localErr))
			if settings.ASRBaseURL != "" && (settings.ASRAPIKey != "" || settings.ASRProvider == "custom") {
				return t.transcribeCloud(ctx, settings, audioPath)
			}
			return "", localErr
		}
		if settings.ASRBaseURL != "" && (settings.ASRAPIKey != "" || settings.ASRProvider == "custom") {
			return t.transcribeCloud(ctx, settings, audioPath)
		}
		return "", errors.New("no ASR backend available: install mlx_whisper/whisper locally or configure cloud ASR API key")
	}
}

func (t *Transcriber) hasLocalWhisper(settings domainai.AISettings) bool {
	if settings.ASRLocalCommand != "" {
		return true
	}
	return findTool("mlx_whisper").Installed || findTool("whisper").Installed
}

func (t *Transcriber) downloadPageAudioWithYTDLP(ctx context.Context, pageURL, tmpDir string) (string, error) {
	ytdlp := findTool("yt-dlp")
	if !ytdlp.Installed {
		return "", errors.New("yt-dlp is not installed; install local ASR tools or provide a video with captions")
	}

	outTemplate := filepath.Join(tmpDir, "audio.%(ext)s")
	cmd := exec.CommandContext(ctx, ytdlp.Path,
		"--no-playlist",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "9",
		"-o", outTemplate,
		pageURL,
	)
	cmd.Env = extendedPATH(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	matches, _ := filepath.Glob(filepath.Join(tmpDir, "audio.*"))
	for _, m := range matches {
		if !strings.HasSuffix(m, ".part") {
			return m, nil
		}
	}
	return "", errors.New("yt-dlp completed but no audio file was produced")
}

func (t *Transcriber) downloadDirectAudio(ctx context.Context, audioURL, tmpDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return "", fmt.Errorf("build audio download request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download audio HTTP %d", resp.StatusCode)
	}

	ext := audioExtFromURL(audioURL)
	destPath := filepath.Join(tmpDir, "audio"+ext)
	f, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(resp.Body, maxAudioDownloadBytes+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		return "", fmt.Errorf("write audio file: %w", err)
	}
	if n > maxAudioDownloadBytes {
		return "", fmt.Errorf("audio file exceeds %d MB limit", maxAudioDownloadBytes/(1024*1024))
	}
	return destPath, nil
}

func (t *Transcriber) maybeCompressAudio(ctx context.Context, audioPath, tmpDir string) string {
	info, err := os.Stat(audioPath)
	if err != nil || info.Size() <= compressThresholdBytes {
		return audioPath
	}

	ffmpeg := findTool("ffmpeg")
	if !ffmpeg.Installed {
		return audioPath
	}

	outPath := filepath.Join(tmpDir, "compressed.mp3")
	cmd := exec.CommandContext(ctx, ffmpeg.Path,
		"-y", "-i", audioPath,
		"-vn", "-ac", "1", "-ar", "16000", "-b:a", "32k",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.logger.Warn("ffmpeg compression failed, using original audio", zap.Error(err), zap.String("output", string(out)))
		return audioPath
	}
	return outPath
}

func (t *Transcriber) transcribeLocal(ctx context.Context, settings domainai.AISettings, audioPath, tmpDir string) (string, error) {
	var cmdPath string
	var args []string

	if custom := strings.TrimSpace(settings.ASRLocalCommand); custom != "" {
		parts := strings.Fields(custom)
		cmdPath = parts[0]
		args = append(args, parts[1:]...)
		args = append(args, audioPath, "--output-format", "txt", "--output-dir", tmpDir)
	} else if mlx := findTool("mlx_whisper"); mlx.Installed {
		cmdPath = mlx.Path
		args = []string{
			audioPath,
			"--model", "mlx-community/whisper-large-v3-turbo",
			"--output-format", "txt",
			"--output-dir", tmpDir,
		}
	} else if wh := findTool("whisper"); wh.Installed {
		cmdPath = wh.Path
		args = []string{
			audioPath,
			"--model", "base",
			"--output-format", "txt",
			"--output-dir", tmpDir,
		}
	} else {
		return "", errors.New("neither mlx_whisper nor whisper is installed locally")
	}

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = extendedPATH(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("local ASR command failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	txtMatches, _ := filepath.Glob(filepath.Join(tmpDir, "*.txt"))
	for _, txtPath := range txtMatches {
		b, readErr := os.ReadFile(txtPath)
		if readErr == nil && strings.TrimSpace(string(b)) != "" {
			return strings.TrimSpace(string(b)), nil
		}
	}

	stdoutText := strings.TrimSpace(string(out))
	if stdoutText != "" {
		return stdoutText, nil
	}
	return "", errors.New("local ASR produced empty transcript")
}

func (t *Transcriber) transcribeCloud(ctx context.Context, settings domainai.AISettings, audioPath string) (string, error) {
	if settings.ASRBaseURL == "" {
		return "", errors.New("cloud ASR base_url is not configured")
	}
	if settings.ASRAPIKey == "" && settings.ASRProvider != "custom" {
		return "", fmt.Errorf("cloud ASR api_key is required for provider %s", settings.ASRProvider)
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("open audio for upload: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}

	model := settings.ASRModel
	if model == "" {
		model = "whisper-large-v3-turbo"
	}
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("response_format", "text")
	if err := writer.Close(); err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(settings.ASRBaseURL, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if settings.ASRAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+settings.ASRAPIKey)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cloud ASR request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read cloud ASR response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cloud ASR HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	raw := strings.TrimSpace(string(respBytes))
	if strings.HasPrefix(raw, "{") {
		var parsed struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(respBytes, &parsed) == nil && strings.TrimSpace(parsed.Text) != "" {
			return strings.TrimSpace(parsed.Text), nil
		}
	}
	if raw == "" {
		return "", errors.New("cloud ASR returned empty transcript")
	}
	return raw, nil
}

func audioExtFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(u.Path))
		switch ext {
		case ".mp3", ".m4a", ".wav", ".ogg", ".opus", ".aac", ".webm", ".flac", ".mp4":
			return ext
		}
	}
	return ".mp3"
}

func findTool(name string) ToolStatus {
	if p, err := exec.LookPath(name); err == nil {
		return ToolStatus{Installed: true, Path: p}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/usr/local/bin", name),
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".local/bin", name))
		if pyBins, err := filepath.Glob(filepath.Join(home, "Library/Python/*/bin", name)); err == nil {
			candidates = append(candidates, pyBins...)
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return ToolStatus{Installed: true, Path: c}
		}
	}
	return ToolStatus{Installed: false}
}

func extendedPATH(env []string) []string {
	home, _ := os.UserHomeDir()
	extra := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home != "" {
		extra = append(extra, filepath.Join(home, ".local/bin"))
	}
	prefix := strings.Join(extra, string(os.PathListSeparator))

	out := make([]string, 0, len(env)+1)
	foundPath := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH="+prefix+string(os.PathListSeparator)+strings.TrimPrefix(kv, "PATH="))
			foundPath = true
		} else {
			out = append(out, kv)
		}
	}
	if !foundPath {
		out = append(out, "PATH="+prefix)
	}
	return out
}

func findInstallScript() string {
	candidates := []string{
		filepath.Join("scripts", "install-asr-tools.sh"),
		filepath.Join("..", "scripts", "install-asr-tools.sh"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "scripts", "install-asr-tools.sh"),
			filepath.Join(dir, "..", "scripts", "install-asr-tools.sh"),
		)
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

func defaultInstallCommand(scriptPath string) string {
	if scriptPath != "" {
		return "bash " + scriptPath
	}
	if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			return "brew install ffmpeg yt-dlp && (uv tool install mlx-whisper || pipx install mlx-whisper || pip3 install --user mlx-whisper)"
		}
		return "brew install ffmpeg yt-dlp && (uv tool install openai-whisper || pipx install openai-whisper || pip3 install --user openai-whisper)"
	}
	return "pip3 install --user yt-dlp openai-whisper"
}

func fallbackInstallScript() string {
	return `set -euo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/.local/bin:$PATH"
if command -v brew >/dev/null 2>&1; then
  command -v ffmpeg >/dev/null 2>&1 || brew install ffmpeg
  command -v yt-dlp >/dev/null 2>&1 || brew install yt-dlp
fi
PKG="openai-whisper"
if [[ "$(uname -s)" == "Darwin" && "$(uname -m)" == "arm64" ]]; then
  PKG="mlx-whisper"
fi
if command -v uv >/dev/null 2>&1; then
  uv tool install "$PKG"
elif command -v pipx >/dev/null 2>&1; then
  pipx install "$PKG"
elif command -v pip3 >/dev/null 2>&1; then
  pip3 install --user --break-system-packages "$PKG" 2>/dev/null || pip3 install --user "$PKG"
else
  echo "Need uv, pipx, or pip3 to install $PKG" >&2
  exit 1
fi`
}
