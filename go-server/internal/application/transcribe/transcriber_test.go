package transcribe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
)

func TestCheckEnvironment(t *testing.T) {
	tr := NewTranscriber(func() domainai.AISettings {
		return domainai.AISettings{
			ASRMode:     "auto",
			ASRProvider: "groq",
			ASRAPIKey:   "gsk_test",
		}
	}, zap.NewNop())

	env := tr.CheckEnvironment()
	if !env.CloudReady {
		t.Fatalf("expected CloudReady=true when Groq key is configured")
	}
	if env.InstallCommand == "" {
		t.Fatalf("expected non-empty InstallCommand")
	}
	for _, k := range []string{"ffmpeg", "yt-dlp", "mlx_whisper", "whisper"} {
		if _, ok := env.Tools[k]; !ok {
			t.Errorf("missing tool key %q in EnvStatus.Tools", k)
		}
	}
}

func TestTranscribeMediaCloud(t *testing.T) {
	audioPayload := []byte("fake-mp3-audio-bytes")
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(audioPayload)
	}))
	defer audioSrv.Close()

	var gotAuth, gotModel string
	var gotFile []byte
	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			b, _ := io.ReadAll(part)
			switch part.FormName() {
			case "model":
				gotModel = string(b)
			case "file":
				gotFile = b
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"text": "Hello from cloud whisper transcript.",
		})
	}))
	defer asrSrv.Close()

	tr := NewTranscriber(func() domainai.AISettings {
		return domainai.AISettings{
			ASRMode:     "cloud",
			ASRProvider: "groq",
			ASRBaseURL:  asrSrv.URL,
			ASRAPIKey:   "secret-key",
			ASRModel:    "whisper-large-v3-turbo",
		}
	}, zap.NewNop())

	txt, err := tr.TranscribeMedia(context.Background(), audioSrv.URL+"/episode.mp3", false)
	if err != nil {
		t.Fatalf("TranscribeMedia failed: %v", err)
	}
	if txt != "Hello from cloud whisper transcript." {
		t.Fatalf("unexpected transcript: %q", txt)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q, want Bearer secret-key", gotAuth)
	}
	if gotModel != "whisper-large-v3-turbo" {
		t.Errorf("model = %q, want whisper-large-v3-turbo", gotModel)
	}
	if string(gotFile) != string(audioPayload) {
		t.Errorf("uploaded bytes mismatch: got %q", string(gotFile))
	}
}

func TestTranscribeLocalWithMockScript(t *testing.T) {
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-mp3"))
	}))
	defer audioSrv.Close()

	binDir := t.TempDir()
	mockBin := filepath.Join(binDir, "mock-whisper.sh")
	script := `#!/usr/bin/env bash
out_dir=""
audio_file="$1"
shift
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output-dir" ]]; then
    out_dir="$2"
    shift 2
  else
    shift
  fi
done
base="$(basename "$audio_file")"
stem="${base%.*}"
echo "Local whisper output text." > "${out_dir}/${stem}.txt"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatalf("write mock script: %v", err)
	}

	tr := NewTranscriber(func() domainai.AISettings {
		return domainai.AISettings{
			ASRMode:         "local",
			ASRLocalCommand: mockBin,
		}
	}, zap.NewNop())

	txt, err := tr.TranscribeMedia(context.Background(), audioSrv.URL+"/show.m4a", false)
	if err != nil {
		t.Fatalf("TranscribeMedia local failed: %v", err)
	}
	if !strings.Contains(txt, "Local whisper output text.") {
		t.Fatalf("unexpected local transcript: %q", txt)
	}
}

func TestTranscribeMediaDisabled(t *testing.T) {
	tr := NewTranscriber(func() domainai.AISettings {
		return domainai.AISettings{ASRMode: "disabled"}
	}, zap.NewNop())
	if _, err := tr.TranscribeMedia(context.Background(), "https://example.com/a.mp3", false); err == nil {
		t.Fatalf("expected error when ASRMode is disabled")
	}
}
