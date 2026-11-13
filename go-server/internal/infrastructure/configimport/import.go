// Package configimport discovers AI provider configurations from the tools a
// user already has installed (Claude Code, Codex, OpenCode and cc-switch).
//
// The goal is to remove the "type the same key and endpoint again" step: the
// settings UI can import the connections it finds, then the user only needs to
// pick a model. Import is read-only; the source tools' files are never changed.
package configimport

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

// Connection is one importable provider configuration.
type Connection struct {
	Source     string   `json:"source"`
	SourceName string   `json:"source_name"`
	Name       string   `json:"name"`
	Provider   string   `json:"provider"`
	BaseURL    string   `json:"base_url"`
	APIKey     string   `json:"api_key,omitempty"`
	HasKey     bool     `json:"has_key"`
	Model      string   `json:"model,omitempty"`
	Models     []string `json:"models,omitempty"`
	Path       string   `json:"path"`
	Warning    string   `json:"warning,omitempty"`
}

// Source reports whether one importer found a configuration file.
type Source struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	Found bool   `json:"found"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
}

// Result is the full import preview.
type Result struct {
	Sources     []Source     `json:"sources"`
	Connections []Connection `json:"connections"`
}

// Import scans one source ("auto", "claude", "codex", "opencode",
// "cc-switch") and returns the preview.
func Import(source string) Result {
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{Sources: []Source{{ID: source, Name: source, Error: err.Error()}}}
	}
	return ImportAt(home, source)
}

// ImportAt is Import with an explicit home directory (used by tests and by
// callers that keep a portable config directory).
func ImportAt(home, source string) Result {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "auto"
	}

	var result Result
	run := func(id, name string, fn func(string) ([]Connection, Source)) {
		if source != "auto" && source != id {
			return
		}
		connections, src := fn(home)
		src.ID = id
		if src.Name == "" {
			src.Name = name
		}
		src.Count = len(connections)
		src.Found = src.Found || len(connections) > 0
		result.Sources = append(result.Sources, src)
		result.Connections = append(result.Connections, connections...)
	}

	run("claude", "Claude Code", importClaude)
	run("codex", "Codex", importCodex)
	run("opencode", "OpenCode", importOpenCode)
	run("cc-switch", "CC Switch", importCCSwitch)
	result.Connections = dedupe(result.Connections)
	return result
}

// ---------------------------------------------------------------------------
// Claude Code
// ---------------------------------------------------------------------------

func importClaude(home string) ([]Connection, Source) {
	paths := []string{
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "settings.local.json"),
	}
	src := Source{Path: strings.Join(paths, ", ")}
	var out []Connection
	var lastErr error
	for _, path := range paths {
		root, err := readJSONMap(path)
		if err != nil {
			if !os.IsNotExist(err) {
				lastErr = err
			}
			continue
		}
		src.Found = true
		env, _ := root["env"].(map[string]any)
		if env == nil {
			continue
		}
		key := firstString(env, "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY")
		baseURL := firstString(env, "ANTHROPIC_BASE_URL")
		model := firstString(env, "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL")
		if key == "" && baseURL == "" {
			continue
		}
		out = append(out, Connection{
			Source:     "claude",
			SourceName: "Claude Code",
			Name:       connectionName("Claude Code", baseURL),
			Provider:   "anthropic",
			BaseURL:    baseURL,
			APIKey:     key,
			HasKey:     key != "",
			Model:      model,
			Path:       path,
		})
	}
	if len(out) == 0 && lastErr != nil {
		src.Error = lastErr.Error()
	}
	return out, src
}

// ---------------------------------------------------------------------------
// Codex
// ---------------------------------------------------------------------------

type codexConfig struct {
	ModelProvider  string                   `toml:"model_provider"`
	Model          string                   `toml:"model"`
	ModelProviders map[string]codexProvider `toml:"model_providers"`
}

type codexProvider struct {
	Name                    string `toml:"name"`
	BaseURL                 string `toml:"base_url"`
	WireAPI                 string `toml:"wire_api"`
	EnvKey                  string `toml:"env_key"`
	ExperimentalBearerToken string `toml:"experimental_bearer_token"`
}

func importCodex(home string) ([]Connection, Source) {
	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	src := Source{Path: configPath}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			src.Error = err.Error()
		}
		return nil, src
	}
	src.Found = true
	var cfg codexConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		src.Error = err.Error()
		return nil, src
	}

	authKey := ""
	if auth, err := readJSONMap(authPath); err == nil {
		authKey = firstString(auth, "OPENAI_API_KEY")
	}

	providerID := strings.TrimSpace(cfg.ModelProvider)
	provider := cfg.ModelProviders[providerID]
	baseURL := strings.TrimSpace(provider.BaseURL)
	key := strings.TrimSpace(provider.ExperimentalBearerToken)
	if key == "" {
		key = authKey
	}
	if key == "" && provider.EnvKey != "" {
		key = os.Getenv(provider.EnvKey)
	}
	if baseURL == "" && providerID != "" && providerID != "openai" {
		return nil, src
	}

	driver := "custom"
	if providerID == "openai" || (providerID == "" && baseURL == "") {
		driver = "openai"
	}
	warning := ""
	if strings.EqualFold(provider.WireAPI, "responses") {
		warning = "Codex 使用 Responses API；Grabby 走 Chat Completions，若该端点不兼容请手动确认"
	}
	name := strings.TrimSpace(provider.Name)
	if name == "" {
		name = connectionName("Codex", baseURL)
	}
	models := []string{}
	if strings.TrimSpace(cfg.Model) != "" {
		models = append(models, strings.TrimSpace(cfg.Model))
	}
	return []Connection{{
		Source:     "codex",
		SourceName: "Codex",
		Name:       name,
		Provider:   driver,
		BaseURL:    baseURL,
		APIKey:     key,
		HasKey:     key != "",
		Model:      strings.TrimSpace(cfg.Model),
		Models:     models,
		Path:       configPath,
		Warning:    warning,
	}}, src
}

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

type openCodeConfig struct {
	Provider map[string]openCodeProvider `json:"provider"`
}

type openCodeProvider struct {
	Name    string                     `json:"name"`
	NPM     string                     `json:"npm"`
	Options openCodeProviderOptions    `json:"options"`
	Models  map[string]json.RawMessage `json:"models"`
}

type openCodeProviderOptions struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
}

func importOpenCode(home string) ([]Connection, Source) {
	paths := []string{
		filepath.Join(home, ".config", "opencode", "opencode.json"),
		filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
		filepath.Join(home, ".opencode.json"),
	}
	src := Source{Path: strings.Join(paths, ", ")}
	var out []Connection
	var lastErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				lastErr = err
			}
			continue
		}
		src.Found = true
		std, err := hujson.Standardize(data)
		if err != nil {
			lastErr = err
			continue
		}
		var cfg openCodeConfig
		if err := json.Unmarshal(std, &cfg); err != nil {
			lastErr = err
			continue
		}
		for id, provider := range cfg.Provider {
			baseURL := resolveEnv(provider.Options.BaseURL)
			apiKey := resolveEnv(provider.Options.APIKey)
			if baseURL == "" && apiKey == "" {
				continue
			}
			driver := driverForNPM(provider.NPM, id)
			models := make([]string, 0, len(provider.Models))
			for modelID := range provider.Models {
				models = append(models, modelID)
			}
			name := strings.TrimSpace(provider.Name)
			if name == "" {
				name = id
			}
			out = append(out, Connection{
				Source:     "opencode",
				SourceName: "OpenCode",
				Name:       name,
				Provider:   driver,
				BaseURL:    baseURL,
				APIKey:     apiKey,
				HasKey:     apiKey != "",
				Model:      firstStringSlice(models),
				Models:     models,
				Path:       path,
			})
		}
	}
	if len(out) == 0 && lastErr != nil {
		src.Error = lastErr.Error()
	}
	return out, src
}

func driverForNPM(npm, id string) string {
	npm = strings.ToLower(npm)
	id = strings.ToLower(id)
	switch {
	case strings.Contains(npm, "anthropic") || strings.Contains(id, "anthropic") || strings.Contains(id, "claude"):
		return "anthropic"
	case strings.Contains(npm, "google") || id == "google" || strings.Contains(id, "gemini"):
		return "gemini"
	case id == "openai" || strings.HasSuffix(npm, "@ai-sdk/openai"):
		return "openai"
	case strings.Contains(id, "ollama"):
		return "ollama"
	default:
		return "custom"
	}
}

// ---------------------------------------------------------------------------
// CC Switch
// ---------------------------------------------------------------------------

func importCCSwitch(home string) ([]Connection, Source) {
	path := filepath.Join(home, ".cc-switch", "cc-switch.db")
	src := Source{Path: path}
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			src.Error = err.Error()
		}
		return nil, src
	}
	src.Found = true

	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		src.Error = err.Error()
		return nil, src
	}
	defer db.Close()

	rows, err := db.Query(`SELECT app_type, name, settings_config FROM providers`)
	if err != nil {
		src.Error = err.Error()
		return nil, src
	}
	defer rows.Close()

	var out []Connection
	for rows.Next() {
		var appType, name, raw string
		if err := rows.Scan(&appType, &name, &raw); err != nil {
			continue
		}
		connection, ok := ccSwitchConnection(path, appType, name, raw)
		if ok {
			out = append(out, connection)
		}
	}
	if err := rows.Err(); err != nil {
		src.Error = err.Error()
	}
	return out, src
}

func ccSwitchConnection(path, appType, name, raw string) (Connection, bool) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Connection{}, false
	}
	base := Connection{
		Source:     "cc-switch",
		SourceName: "CC Switch",
		Name:       strings.TrimSpace(name),
		Path:       path,
	}
	switch strings.ToLower(appType) {
	case "claude":
		env, _ := cfg["env"].(map[string]any)
		key := firstString(env, "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY")
		if key == "" {
			key = firstString(cfg, "anthropicApiKey", "apiKey")
		}
		base.Provider = "anthropic"
		base.APIKey = key
		base.HasKey = key != ""
		base.BaseURL = firstString(env, "ANTHROPIC_BASE_URL")
		base.Model = firstString(env, "ANTHROPIC_MODEL")
		if base.Model != "" {
			base.Models = []string{base.Model}
		}
		return base, base.APIKey != "" || base.BaseURL != ""
	case "codex":
		auth, _ := cfg["auth"].(map[string]any)
		key := firstString(auth, "OPENAI_API_KEY")
		base.Provider = "custom"
		base.APIKey = key
		base.HasKey = key != ""
		if text, ok := cfg["config"].(string); ok {
			var doc codexConfig
			if err := toml.Unmarshal([]byte(text), &doc); err == nil {
				provider := doc.ModelProviders[doc.ModelProvider]
				base.BaseURL = strings.TrimSpace(provider.BaseURL)
				base.Model = strings.TrimSpace(doc.Model)
				if base.Model != "" {
					base.Models = []string{base.Model}
				}
				if doc.ModelProvider == "openai" {
					base.Provider = "openai"
				}
			}
		}
		return base, base.APIKey != "" || base.BaseURL != ""
	case "gemini":
		env, _ := cfg["env"].(map[string]any)
		key := firstString(env, "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENAI_API_KEY")
		base.Provider = "gemini"
		base.APIKey = key
		base.HasKey = key != ""
		base.BaseURL = firstString(env, "GOOGLE_GEMINI_BASE_URL", "GOOGLE_GENAI_BASE_URL")
		base.Model = firstString(env, "GEMINI_MODEL")
		if base.Model != "" {
			base.Models = []string{base.Model}
		}
		return base, base.APIKey != "" || base.BaseURL != ""
	case "opencode":
		provider := openCodeProvider{
			Name: firstString(cfg, "name"),
			NPM:  firstString(cfg, "npm"),
		}
		if options, ok := cfg["options"].(map[string]any); ok {
			provider.Options.BaseURL = firstString(options, "baseURL", "baseUrl")
			provider.Options.APIKey = firstString(options, "apiKey")
		}
		base.Provider = driverForNPM(provider.NPM, firstString(cfg, "id"))
		base.BaseURL = resolveEnv(provider.Options.BaseURL)
		base.APIKey = resolveEnv(provider.Options.APIKey)
		base.HasKey = base.APIKey != ""
		if models, ok := cfg["models"].(map[string]any); ok {
			for modelID := range models {
				base.Models = append(base.Models, modelID)
			}
			base.Model = firstStringSlice(base.Models)
		}
		return base, base.APIKey != "" || base.BaseURL != ""
	}
	return Connection{}, false
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func readJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	std, err := hujson.Standardize(data)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(std, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func firstString(values map[string]any, keys ...string) string {
	if values == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return resolveEnv(strings.TrimSpace(value))
		}
	}
	return ""
}

func firstStringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func resolveEnv(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, prefix := range []string{"{env:", "${env:", "$ENV:"} {
		if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
			return os.Getenv(name)
		}
	}
	if strings.HasPrefix(value, "$") && !strings.ContainsAny(value[1:], "/:") {
		return os.Getenv(strings.TrimPrefix(value, "$"))
	}
	return value
}

func connectionName(prefix, baseURL string) string {
	if baseURL == "" {
		return prefix
	}
	host := baseURL
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	if idx := strings.IndexAny(host, "/:"); idx >= 0 {
		host = host[:idx]
	}
	if host == "" {
		return prefix
	}
	return fmt.Sprintf("%s · %s", prefix, host)
}

func dedupe(connections []Connection) []Connection {
	seen := map[string]struct{}{}
	out := make([]Connection, 0, len(connections))
	for _, connection := range connections {
		key := strings.ToLower(strings.Join([]string{
			connection.Provider, strings.TrimRight(connection.BaseURL, "/"),
			connection.Model, connection.APIKey,
		}, "|"))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, connection)
	}
	return out
}
