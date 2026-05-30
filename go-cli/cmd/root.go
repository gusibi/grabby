package cmd

import (
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
	Use:     "grabby",
	Short:   "Grabby - 网页内容抓取工具",
	Long:    `通过本地 Grabby 服务抓取网页，返回干净的 Markdown 内容。`,
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