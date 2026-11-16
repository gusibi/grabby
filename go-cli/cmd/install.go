package cmd

import (
	"archive/zip"
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

const releaseRepo = "gusibi/grabby"

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "从 GitHub 下载并安装最新的 grabby 二进制",
	Long: `从 GitHub Releases 下载预编译的 grabby 二进制文件并安装。

默认安装位置：macOS / Linux 为 ~/.local/bin，Windows 为 %LOCALAPPDATA%\Programs\grabby。
可通过 GRABBY_INSTALL_DIR 环境变量自定义。

仅安装 Go 二进制版本，无需 Python 或其他依赖。`,
	Run: func(_ *cobra.Command, _ []string) {
		installDir := os.Getenv("GRABBY_INSTALL_DIR")
		if installDir == "" {
			installDir = defaultInstallDir()
		}

		osName, archName := platformNames()
		ext := ".tar.gz"
		if runtime.GOOS == "windows" {
			ext = ".zip"
		}
		filename := fmt.Sprintf("grabby_%s_%s%s", osName, archName, ext)
		downloadURL := fmt.Sprintf(
			"https://github.com/%s/releases/latest/download/%s",
			releaseRepo, filename,
		)

		fmt.Fprintf(os.Stderr, "⬇️  下载 grabby (%s/%s)...\n", osName, archName)

		if err := os.MkdirAll(installDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建安装目录失败: %v\n", err)
			os.Exit(1)
		}

		tmpDir, err := os.MkdirTemp("", "grabby-install")
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tmpDir)

		archivePath := filepath.Join(tmpDir, filename)
		if err := downloadFile(downloadURL, archivePath); err != nil {
			fmt.Fprintf(os.Stderr, "下载失败: %v\n", err)
			fmt.Fprintf(os.Stderr, "请确认 https://github.com/%s/releases 是否有 release\n", releaseRepo)
			os.Exit(1)
		}

		if ext == ".zip" {
			err = extractZip(archivePath, tmpDir)
		} else {
			err = extractTarball(archivePath, tmpDir)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "解压失败: %v\n", err)
			os.Exit(1)
		}

		binary := "grabby"
		if runtime.GOOS == "windows" {
			binary = "grabby.exe"
		}
		src := filepath.Join(tmpDir, binary)
		dst := filepath.Join(installDir, binary)
		input, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取二进制失败: %v\n", err)
			os.Exit(1)
		}
		// Write to a sibling file and rename it into place. This avoids
		// "text file busy" when `grabby install` is run from the very binary
		// it is replacing (macOS/Linux allow rename over a running file).
		tmpDst := dst + ".tmp"
		if err := os.WriteFile(tmpDst, input, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "写入二进制失败: %v\n", err)
			os.Exit(1)
		}
		_ = os.Chmod(tmpDst, 0755)
		if err := os.Rename(tmpDst, dst); err != nil {
			_ = os.Remove(tmpDst)
			if err := os.WriteFile(dst, input, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "写入二进制失败: %v\n", err)
				os.Exit(1)
			}
		}
		_ = os.Chmod(dst, 0755)

		// Init config directory
		configDir, _ := grabbyConfigDir()

		output := map[string]any{
			"message":    fmt.Sprintf("✅ 安装完成: %s", dst),
			"version":    version,
			"path":       dst,
			"config_dir": configDir,
		}
		json.NewEncoder(os.Stdout).Encode(output)

		// Check PATH
		if !inPath(installDir) {
			printPathHint(installDir)
		}
	},
}

func defaultInstallDir() string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Programs", "grabby")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "bin")
}

func printPathHint(dir string) {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(os.Stderr, "⚠️  %s 不在 PATH 中，请把它加入用户 PATH，然后重新打开终端。\n", dir)
		return
	}
	fmt.Fprintf(os.Stderr, "⚠️  %s 不在 PATH 中，请添加到 shell 配置:\n    export PATH=\"$PATH:%s\"\n", dir, dir)
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

func extractZip(zipPath, dir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Release archives are flat; only extract the base name so a crafted
		// archive cannot escape the temporary directory.
		name := filepath.Base(f.Name)
		if name == "." || name == string(filepath.Separator) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outPath := filepath.Join(dir, name)
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func inPath(dir string) bool {
	target := filepath.Clean(dir)
	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if p != "" && filepath.Clean(p) == target {
			return true
		}
	}
	return false
}
