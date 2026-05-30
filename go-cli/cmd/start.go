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
	Use:   "start <python|go>",
	Short: "启动 Grabby 服务 (python 或 go)",
	Long: `启动 Grabby 后端服务。

需要项目源码目录存在（python-server/ 或 go-server/）。
支持通过以下方式指定项目目录：
1. GRABBY_PROJECT_DIR 环境变量
2. 从当前目录向上查找 python-server/ 或 go-server/
3. 二进制文件在 go-cli/ 目录下时自动取父目录

配置文件 .env 统一放在 ~/.grabby/ 目录下。`,
	Args: cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		serverType := args[0]
		dir := findProjectDir()
		if dir == "" {
			fmt.Fprintf(os.Stderr, "错误: 找不到项目目录 (需要包含 python-server/ 或 go-server/)\n")
			fmt.Fprintf(os.Stderr, "      请设置 GRABBY_PROJECT_DIR 环境变量指定路径\n")
			os.Exit(1)
		}

		switch serverType {
		case "python":
			startPython(dir)
		case "go":
			startGo(dir)
		default:
			fmt.Fprintf(os.Stderr, "未知的服务类型: %s (可用: python, go)\n", serverType)
			os.Exit(1)
		}
	},
}

// findProjectDir searches for the project root containing python-server/ or go-server/.
func findProjectDir() string {
	if v := os.Getenv("GRABBY_PROJECT_DIR"); v != "" {
		return v
	}

	// Try upward search from cwd
	dir, _ := os.Getwd()
	for {
		if hasDir(dir, "python-server") || hasDir(dir, "go-server") {
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
			if hasDir(parent, "python-server") {
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

func startPython(projectDir string) {
	serverDir := filepath.Join(projectDir, "python-server")
	mainPy := filepath.Join(serverDir, "main.py")
	if _, err := os.Stat(mainPy); os.IsNotExist(err) {
		output := map[string]any{"error": fmt.Sprintf("找不到 Python 服务: %s", mainPy), "exit_code": 3}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(3)
	}

	configDir, _ := grabbyConfigDir()

	output := map[string]any{
		"message":    "正在启动 Python Grabby 服务...",
		"port":       5040,
		"config_dir": configDir,
	}
	json.NewEncoder(os.Stdout).Encode(output)

	cmd := exec.Command("python3", mainPy)
	// Set working directory to ~/.grabby/ so pydantic-settings loads .env from there
	cmd.Dir = configDir
	// Also pass the env vars explicitly
	cmd.Env = mergeEnv(loadEnvFile())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Python 服务启动失败: %v\n", err)
		os.Exit(1)
	}
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

	// For Go, we set CWD to project dir so go run works,
	// but pass config vars from ~/.grabby/.env as env vars
	cmd := exec.Command("go", "run", "./go-server/...")
	cmd.Dir = projectDir
	cmd.Env = mergeEnv(loadEnvFile())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Go 服务启动失败: %v\n", err)
		os.Exit(1)
	}
}