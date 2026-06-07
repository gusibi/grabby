package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

type extractRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

type extractResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Markdown string `json:"markdown"`
}

var extractBrowser string

var extractCmd = &cobra.Command{
	Use:   "extract <url>",
	Short: "抓取指定 URL 的网页内容为 Markdown",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		url := args[0]

		payload := extractRequest{URL: url, Browser: extractBrowser}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(getServerURL()+"/api/extract", "application/json", bytes.NewReader(body))
		if err != nil {
			output := map[string]any{"error": "服务未运行", "exit_code": 1}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(1)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var data extractResponse
		if err := json.Unmarshal(respBody, &data); err != nil {
			output := map[string]any{"error": "解析响应失败", "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		if !data.Success {
			var errResp map[string]any
			json.Unmarshal(respBody, &errResp)
			detail, _ := errResp["detail"].(string)
			if detail == "" {
				detail, _ = errResp["error"].(string)
			}
			if detail == "" {
				detail = "提取失败"
			}
			output := map[string]any{"error": detail, "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		result := map[string]any{
			"title":    data.Title,
			"url":      data.URL,
			"markdown": data.Markdown,
		}
		json.NewEncoder(os.Stdout).Encode(result)
	},
}

func init() {
	extractCmd.Flags().StringVarP(&extractBrowser, "browser", "b", "", "浏览器名称")
}