package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 Grabby 服务",
	Long: `启动 Grabby 后端服务（go-server）。

需要项目源码目录存在（go-server/）。
支持通过以下方式指定项目目录：
1. GRABBY_PROJECT_DIR 环境变量
2. 从当前目录向上查找 go-server/
3. 二进制文件在 go-cli/ 目录下时自动取父目录

配置文件 .env 统一放在 ~/.grabby/ 目录下。`,
	Args: cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		// 兼容旧用法 `grabby start go`；Python 后端已移除。
		if len(args) > 0 && args[0] != "go" {
			fmt.Fprintf(os.Stderr, "未知的服务类型: %s (Grabby 现在只有 go-server，直接运行 `grabby start` 即可)\n", args[0])
			os.Exit(1)
		}

		dir := findProjectDir()
		if dir == "" {
			fmt.Fprintf(os.Stderr, "错误: 找不到项目目录 (需要包含 go-server/)\n")
			fmt.Fprintf(os.Stderr, "      请设置 GRABBY_PROJECT_DIR 环境变量指定路径\n")
			os.Exit(1)
		}

		startGo(dir)
	},
}

// findProjectDir searches for the project root containing go-server/.
func findProjectDir() string {
	if v := os.Getenv("GRABBY_PROJECT_DIR"); v != "" {
		return v
	}

	// Try upward search from cwd
	dir, _ := os.Getwd()
	for {
		if hasDir(dir, "go-server") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// If grabby binary is in go-cli/, parent is the project dir
	if selfDir, err := selfDir(); err == nil {
		if filepath.Base(selfDir) == "go-cli" {
			parent := filepath.Dir(selfDir)
			if hasDir(parent, "go-server") {
				return parent
			}
		}
	}

	return ""
}

func hasDir(parent, name string) bool {
	info, err := os.Stat(filepath.Join(parent, name))
	return err == nil && info.IsDir()
}

func selfDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// loadEnvFile reads ~/.grabby/.env and returns a map of key=value.
// Returns empty map if file doesn't exist.
func loadEnvFile() map[string]string {
	configDir, err := grabbyConfigDir()
	if err != nil {
		return nil
	}
	envFile := filepath.Join(configDir, ".env")
	f, err := os.Open(envFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return vars
}

// mergeEnv returns current env plus additional vars overlaid.
func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func startGo(projectDir string) {
	goDir := filepath.Join(projectDir, "go-server")
	if _, err := os.Stat(filepath.Join(goDir, "main.go")); os.IsNotExist(err) {
		output := map[string]any{"error": fmt.Sprintf("找不到 Go 服务: %s", goDir), "exit_code": 3}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(3)
	}

	configDir, _ := grabbyConfigDir()

	output := map[string]any{
		"message":    "正在启动 Go Grabby 服务...",
		"port":       5040,
		"config_dir": configDir,
	}
	json.NewEncoder(os.Stdout).Encode(output)

	// go-server 是独立的 Go module，必须在它自己的目录里 go run（跟 start.sh 一致），
	// 从仓库根目录跑 ./go-server/... 会报 "directory prefix ... does not contain main module"。
	// 配置从 ~/.grabby/.env 以环境变量传入。
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = goDir
	cmd.Env = mergeEnv(loadEnvFile())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Go 服务启动失败: %v\n", err)
		os.Exit(1)
	}
}