package cmd

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

type healthResponse struct {
	Status           string   `json:"status"`
	BrowserConnected bool     `json:"browser_connected"`
	BrowserCount     int      `json:"browser_count"`
	Browsers         []string `json:"browsers"`
	Timestamp        string   `json:"timestamp"`
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "检查 Grabby 服务状态和浏览器连接",
	Run: func(_ *cobra.Command, _ []string) {
		resp, err := http.Get(getServerURL() + "/api/health")
		if err != nil {
			output := map[string]any{"error": "服务未运行", "exit_code": 1}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var data healthResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			output := map[string]any{"error": "解析响应失败", "exit_code": 1}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(1)
		}

		if data.BrowserConnected {
			output := map[string]any{
				"status":            "ok",
				"browser_connected": true,
				"count":             data.BrowserCount,
				"browsers":          data.Browsers,
			}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(0)
		} else {
			output := map[string]any{
				"status":            "ok",
				"browser_connected": false,
				"message":           "浏览器未连接，请打开 Grabby Chrome 扩展",
			}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(2)
		}
	},
}

func init() {
	// no flags needed
}