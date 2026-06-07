package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

type screenshotRequest struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

var screenshotBrowser string

var screenshotCmd = &cobra.Command{
	Use:   "screenshot <url>",
	Short: "捕获指定 URL 的网页截图",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		url := args[0]

		payload := screenshotRequest{URL: url, Browser: screenshotBrowser}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(getServerURL()+"/api/screenshot", "application/json", bytes.NewReader(body))
		if err != nil {
			output := map[string]any{"error": "服务未运行", "exit_code": 1}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(1)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var data map[string]any
		if err := json.Unmarshal(respBody, &data); err != nil {
			output := map[string]any{"error": "解析响应失败", "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		if success, _ := data["success"].(bool); !success {
			detail, _ := data["detail"].(string)
			if detail == "" {
				detail, _ = data["error"].(string)
			}
			if detail == "" {
				detail = "截图失败"
			}
			output := map[string]any{"error": detail, "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		json.NewEncoder(os.Stdout).Encode(data)
	},
}

func init() {
	screenshotCmd.Flags().StringVarP(&screenshotBrowser, "browser", "b", "", "浏览器名称")
}