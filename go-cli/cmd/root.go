package cmd

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// These are set by goreleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const defaultServerURL = "http://localhost:5040"

func getServerURL() string {
	if v := os.Getenv("GRABBY_SERVER_URL"); v != "" {
		return v
	}
	return defaultServerURL
}

// doAPI builds a request against the Grabby server. GRABBY_API_TOKEN, when set,
// is sent as X-API-Key; local desktop connections never require it.
func doAPI(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, getServerURL()+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := os.Getenv("GRABBY_API_TOKEN"); token != "" {
		req.Header.Set("X-API-Key", token)
	}
	return http.DefaultClient.Do(req)
}

// grabbyConfigDir returns ~/.grabby/. Creates the directory if it doesn't exist.
func grabbyConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".grabby")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

var rootCmd = &cobra.Command{
	Use:   "grabby",
	Short: "Grabby - 网页内容抓取工具",
	Long: `通过本地 Grabby 服务抓取网页，返回干净的 Markdown 内容。

服务地址默认 http://localhost:5040，可用 GRABBY_SERVER_URL 覆盖。
仅当服务端开启局域网访问并设置 Token 时才需要 GRABBY_API_TOKEN（本机连接免密）。`,
	Version: version,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(browsersCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(installCmd)
}
