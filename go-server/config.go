package main

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// Settings holds all server configuration.
// Values are read from environment variables (and .env file if present),
// falling back to sensible defaults.
type Settings struct {
	Host              string
	Port              int
	ConnectID         string
	Debug             bool
	WebsocketTimeout  float64
	APIExtractTimeout float64
	DefaultBrowser    string // default browser name for routing; empty = first active connection
	AISettings        AISettings
}

var (
	settings     *Settings
	settingsOnce sync.Once
)

// GetSettings returns the singleton Settings instance.
func GetSettings() *Settings {
	settingsOnce.Do(func() {
		settings = loadSettings()
	})
	return settings
}

func loadSettings() *Settings {
	// Load .env file if it exists; ignore errors when missing.
	_ = loadDotEnv()

	return &Settings{
		Host:              getEnv("HOST", "0.0.0.0"),
		Port:              getEnvInt("PORT", 5040),
		ConnectID:         getEnv("CONNECT_ID", "browser-tools"),
		Debug:             getEnvBool("DEBUG", false),
		WebsocketTimeout:  getEnvFloat("WEBSOCKET_TIMEOUT", 5.0),
		APIExtractTimeout: getEnvFloat("API_EXTRACT_TIMEOUT", 60.0),
		DefaultBrowser:    getEnv("DEFAULT_BROWSER", ""),
		AISettings: AISettings{
			Enabled:          getEnvBool("AI_ENABLED", false),
			Provider:         getEnv("AI_PROVIDER", "gemini"),
			APIKey:           getEnv("AI_API_KEY", ""),
			Model:            getEnv("AI_MODEL", "googleai/gemini-2.0-flash"),
			BaseURL:          getEnv("AI_BASE_URL", ""),
			QualityThreshold: getEnvInt("AI_QUALITY_THRESHOLD", 7),
		},
	}
}

// loadDotEnv tries to read .env from the working directory.
// Matches the Python version's behavior where .env is loaded before
// environment variables are read.
func loadDotEnv() error {
	return godotenv.Load(".env")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvBool(key string, defaultVal bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	return v == "true" || v == "1" || v == "yes"
}

func getEnvFloat(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}
