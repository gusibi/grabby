package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

type browserListResponse struct {
	Browsers []string `json:"browsers"`
	Count    int      `json:"count"`
}

type browserRegisterRequest struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

var browsersCmd = &cobra.Command{
	Use:   "browsers",
	Short: "管理浏览器连接",
}

var browsersListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已连接的浏览器",
	Run: func(_ *cobra.Command, _ []string) {
		resp, err := http.Get(getServerURL() + "/api/browsers")
		if err != nil {
			output := map[string]any{"error": "服务未运行", "exit_code": 1}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var data browserListResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			output := map[string]any{"error": "解析响应失败", "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		output := map[string]any{
			"count":    data.Count,
			"browsers": data.Browsers,
		}
		json.NewEncoder(os.Stdout).Encode(output)
	},
}

var browsersRegisterCmd = &cobra.Command{
	Use:   "register <connect_id> <name>",
	Short: "注册浏览器实例",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		payload := browserRegisterRequest{
			ConnectID: args[0],
			Name:      args[1],
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(getServerURL()+"/api/browsers/register", "application/json", bytes.NewReader(body))
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
				detail = "注册失败"
			}
			output := map[string]any{"error": detail, "exit_code": 3}
			json.NewEncoder(os.Stdout).Encode(output)
			os.Exit(3)
		}

		json.NewEncoder(os.Stdout).Encode(data)
	},
}

func init() {
	browsersCmd.AddCommand(browsersListCmd)
	browsersCmd.AddCommand(browsersRegisterCmd)
}