package configimport

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestImportClaude(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{
		"env": {
			"ANTHROPIC_API_KEY": "sk-ant-test",
			"ANTHROPIC_BASE_URL": "https://relay.example.com",
			"ANTHROPIC_MODEL": "claude-sonnet-5-5"
		}
	}`)

	result := ImportAt(home, "claude")
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %+v", result.Connections)
	}
	got := result.Connections[0]
	if got.Provider != "anthropic" || got.APIKey != "sk-ant-test" || got.BaseURL != "https://relay.example.com" || got.Model != "claude-sonnet-5-5" {
		t.Errorf("connection = %+v", got)
	}
}

func TestImportCodex(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), `
model_provider = "custom"
model = "deepseek-v4-flash"

[model_providers.custom]
name = "DeepSeek"
base_url = "https://api.deepseek.com/v1"
wire_api = "chat"
`)
	writeFile(t, filepath.Join(home, ".codex", "auth.json"), `{"OPENAI_API_KEY":"sk-deepseek"}`)

	result := ImportAt(home, "codex")
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %+v", result.Connections)
	}
	got := result.Connections[0]
	if got.Provider != "custom" || got.APIKey != "sk-deepseek" || got.BaseURL != "https://api.deepseek.com/v1" || got.Model != "deepseek-v4-flash" {
		t.Errorf("connection = %+v", got)
	}
}

func TestImportOpenCodeResolvesEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MY_OPENAI_KEY", "sk-from-env")
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"), `{
		// JSONC comment
		"provider": {
			"my-relay": {
				"name": "My Relay",
				"npm": "@ai-sdk/openai-compatible",
				"options": {
					"baseURL": "https://relay.example.com/v1",
					"apiKey": "{env:MY_OPENAI_KEY}"
				},
				"models": {
					"gpt-6.1-sol": {"name": "GPT 6.1 Sol"}
				}
			}
		}
	}`)

	result := ImportAt(home, "opencode")
	if len(result.Connections) != 1 {
		t.Fatalf("connections = %+v", result.Connections)
	}
	got := result.Connections[0]
	if got.Provider != "custom" || got.APIKey != "sk-from-env" || got.BaseURL != "https://relay.example.com/v1" || got.Model != "gpt-6.1-sol" {
		t.Errorf("connection = %+v", got)
	}
}

func TestImportCCSwitch(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".cc-switch", "cc-switch.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE providers (
		id TEXT, app_type TEXT, name TEXT, settings_config TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO providers (id, app_type, name, settings_config) VALUES
		('claude-1', 'claude', 'Relay Claude', '{"env":{"ANTHROPIC_API_KEY":"sk-ant","ANTHROPIC_BASE_URL":"https://relay.example.com"}}'),
		('gemini-1', 'gemini', 'Relay Gemini', '{"env":{"GEMINI_API_KEY":"AIza-test","GEMINI_MODEL":"gemini-2.5-flash"}}')`); err != nil {
		t.Fatal(err)
	}

	result := ImportAt(home, "cc-switch")
	if len(result.Connections) != 2 {
		t.Fatalf("connections = %+v", result.Connections)
	}
	providers := map[string]Connection{}
	for _, connection := range result.Connections {
		providers[connection.Provider] = connection
	}
	if providers["anthropic"].APIKey != "sk-ant" || providers["anthropic"].BaseURL != "https://relay.example.com" {
		t.Errorf("claude connection = %+v", providers["anthropic"])
	}
	if providers["gemini"].APIKey != "AIza-test" || providers["gemini"].Model != "gemini-2.5-flash" {
		t.Errorf("gemini connection = %+v", providers["gemini"])
	}
}
