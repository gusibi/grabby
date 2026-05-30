package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "从 GitHub 下载并安装 grabby 二进制到 ~/.local/bin",
	Long: `从 GitHub Releases 下载预编译的 grabby 二进制文件并安装。

安装位置：~/.local/bin（可通过 GRABBY_INSTALL_DIR 环境变量自定义）

仅安装 Go 二进制版本，无需 Python 或其他依赖。`,
	Run: func(_ *cobra.Command, _ []string) {
		installDir := os.Getenv("GRABBY_INSTALL_DIR")
		if installDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "获取用户目录失败: %v\n", err)
				os.Exit(1)
			}
			installDir = home + "/.local/bin"
		}

		// Determine OS and arch
		osName, archName := platformNames()
		filename := fmt.Sprintf("grabby_%s_%s.tar.gz", osName, archName)
		downloadURL := fmt.Sprintf(
			"https://github.com/gusibi/mcp-web-capture/releases/latest/download/%s",
			filename,
		)

		fmt.Fprintf(os.Stderr, "⬇️  下载 grabby (%s/%s)...\n", osName, archName)

		// Create install directory
		if err := os.MkdirAll(installDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建安装目录失败: %v\n", err)
			os.Exit(1)
		}

		// Download tarball
		tmpDir, err := os.MkdirTemp("", "grabby-install")
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tmpDir)

		tarball := filepath.Join(tmpDir, filename)
		if err := downloadFile(downloadURL, tarball); err != nil {
			fmt.Fprintf(os.Stderr, "下载失败: %v\n", err)
			fmt.Fprintf(os.Stderr, "请确认 https://github.com/gusibi/mcp-web-capture/releases 是否有 release\n")
			os.Exit(1)
		}

		// Extract
		if err := extractTarball(tarball, tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "解压失败: %v\n", err)
			os.Exit(1)
		}

		// Install binary
		src := filepath.Join(tmpDir, "grabby")
		dst := filepath.Join(installDir, "grabby")
		input, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取二进制失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(dst, input, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "写入二进制失败: %v\n", err)
			os.Exit(1)
		}

		// Init config directory
		configDir, _ := grabbyConfigDir()

		output := map[string]any{
			"message":    fmt.Sprintf("✅ 安装完成: %s", dst),
			"version":    version,
			"config_dir": configDir,
		}
		json.NewEncoder(os.Stdout).Encode(output)

		// Check PATH
		if !inPath(installDir) {
			fmt.Fprintf(os.Stderr, "⚠️  %s 不在 PATH 中，请添加到 shell 配置:\n    export PATH=\"$PATH:%s\"\n", installDir, installDir)
		}
	},
}

func platformNames() (osName, archName string) {
	switch runtime.GOOS {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	case "windows":
		osName = "Windows"
	default:
		fmt.Fprintf(os.Stderr, "不支持的操作系统: %s\n", runtime.GOOS)
		os.Exit(1)
	}
	switch runtime.GOARCH {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		fmt.Fprintf(os.Stderr, "不支持的架构: %s\n", runtime.GOARCH)
		os.Exit(1)
	}
	return
}

func downloadFile(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarball(tarball, dir string) error {
	cmd := exec.Command("tar", "-xzf", tarball, "-C", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(output))
	}
	return nil
}

func inPath(dir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), ":") {
		if p == dir {
			return true
		}
	}
	return false
}