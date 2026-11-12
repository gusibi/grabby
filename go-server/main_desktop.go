//go:build desktop

// Grabby Desktop: a MyGO shell around the existing Go server.
//
// Unlike the headless server, this build embeds desktop/dist (its own native
// looking UI) instead of frontend/dist. The HTTP / WebSocket / MCP server is
// unchanged and still runs in this same process, so the app is an API relay:
// external calls -> this process -> ws_browser -> Chrome extension -> page.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"go-server/internal/bootstrap"
	"go-server/internal/config"
	"go-server/internal/logging"
)

//go:embed all:desktop/dist
var frontendFS embed.FS

const desktopAppName = "Grabby"

func main() {
	mygo.App.SetName(desktopAppName)

	// A bundle started from Finder or `open` has no terminal, so mirror the
	// logs into a file (macOS: ~/Library/Logs/Grabby/desktop.log).
	if closer := logging.SetupFileLogging(desktopLogPath()); closer != nil {
		defer closer.Close()
	}

	// All of the server's data paths (grabby.db, browser_registry.json, .env,
	// raw dumps) are relative to the working directory. Point that at the
	// per-user data directory so nothing is written inside the signed bundle.
	// GRABBY_DESKTOP_DATA_DIR can override it, e.g. to reuse an existing
	// go-server directory with its database and .env already in place.
	dataDir := resolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("desktop: cannot create user data directory: %v", err)
	}
	if err := os.Chdir(dataDir); err != nil {
		log.Fatalf("desktop: cannot use user data directory: %v", err)
	}
	log.Printf("desktop: data directory %s", dataDir)
	_ = os.Setenv("GRABBY_DESKTOP_DATA_DIR", dataDir)

	// The desktop app owns its own auth preferences. Do NOT inherit
	// GRABBY_ADMIN_KEY / GRABBY_API_TOKEN from .env: by default the app needs
	// no token at all and never shows a login dialog. A token exists only when
	// the user sets one in the in-app settings page, which writes
	// desktop-settings.json next to grabby.db. Setting an empty value still
	// counts as "present" for godotenv.Load, so a stale key in .env cannot
	// silently re-enable authentication.
	desktopCfg := config.LoadDesktopSettings()
	_ = os.Setenv("GRABBY_ADMIN_KEY", "")
	_ = os.Setenv("GRABBY_API_TOKEN", desktopCfg.APIToken)

	// Loopback by default. The in-app setting (or GRABBY_DESKTOP_HOST) can
	// expose the API to the LAN, but only together with a token: /ws_command
	// and browser registration are otherwise reachable from the network.
	host := os.Getenv("GRABBY_DESKTOP_HOST")
	if host == "" {
		if desktopCfg.ExternalEnabled && desktopCfg.APIToken != "" {
			host = "0.0.0.0"
		} else {
			host = "127.0.0.1"
		}
	}
	external := !isLocalHost(host)
	if external && desktopCfg.APIToken == "" {
		log.Printf("desktop: refusing to expose an unauthenticated API on %s; falling back to 127.0.0.1", host)
		host = "127.0.0.1"
		external = false
	}
	_ = os.Setenv("HOST", host)

	settings := config.GetSettings()
	settings.Host = host
	// Enables local-loopback auth bypass and the in-app settings endpoints.
	// The headless server never sets this flag.
	settings.DesktopMode = true

	// If another Grabby (for example the headless server) owns the configured
	// port, use the next free one so the app can still serve its own UI. The
	// real port is shown in the app and the tray menu.
	chosenPort := freePort(settings.Port)
	if chosenPort != settings.Port {
		log.Printf("desktop: port %d is taken, using %d", settings.Port, chosenPort)
		_ = os.Setenv("GRABBY_DESKTOP_PORT_SHIFTED", "1")
	}
	settings.Port = chosenPort
	_ = os.Setenv("PORT", fmt.Sprint(chosenPort))

	uiBase := fmt.Sprintf("http://127.0.0.1:%d", settings.Port)
	apiAddr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)

	if !mygo.App.RequestSingleInstanceLock() {
		return // a second launch hands over to the running instance
	}

	ctx, cancel := context.WithCancel(context.Background())
	var serverWG sync.WaitGroup
	logger := logging.GetLogger()
	app, err := bootstrap.NewApp(settings, logger, frontendFS)
	if err != nil {
		log.Fatalf("desktop: cannot initialise server: %v", err)
	}
	serverWG.Add(1)
	go func() {
		defer serverWG.Done()
		if err := app.Run(ctx); err != nil {
			log.Printf("desktop: server stopped: %v", err)
		}
	}()
	if !waitForServer(uiBase, 20*time.Second) {
		log.Printf("desktop: server did not become ready at %s, opening the window anyway", uiBase)
	}

	// Platform-aware skin plus auth/bind hints for the UI. Never pass the
	// token itself through the URL.
	uiURL := uiBase + "/?platform=" + runtime.GOOS + "&desktop=1"
	if settings.APIToken != "" || settings.AdminKey != "" {
		uiURL += "&auth=1"
	}
	if external {
		uiURL += "&external=1"
	}

	var mainWindow *mygo.Window
	var quitting bool

	openMainWindow := func() {
		if mainWindow != nil {
			mainWindow.Show()
			mainWindow.Focus()
			return
		}
		opts := mygo.WindowOptions{
			Title:           desktopAppName,
			URL:             uiURL,
			Width:           1180,
			Height:          780,
			MinWidth:        900,
			MinHeight:       600,
			StateKey:        "main",
			BackgroundColor: "light-dark(#ffffff, #1c1c1e)",
			Page: mygo.PageOptions{
				TrustedOrigins: []string{uiBase},
			},
		}
		if runtime.GOOS == "darwin" {
			// Apple News style: the sidebar draws its own title bar area and
			// leaves room for the traffic lights.
			opts.TitleBarStyle = mygo.TitleBarHidden
		}
		win := mygo.NewWindow(opts)
		win.OnClose(func(e *mygo.CloseEvent) {
			if quitting {
				mainWindow = nil
				return
			}
			// Closing the window keeps the process (and the API relay) alive
			// in the tray, like a menu bar app.
			e.PreventDefault()
			win.Hide()
		})
		mainWindow = win
	}

	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) { quitting = true })
	// Registering this listener stops MyGO from quitting when the last window
	// closes; the tray is the way out.
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func(args []string, workingDir string) { openMainWindow() })
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			openMainWindow()
		}
	})

	mygo.App.WhenReady(func() {
		icon, err := trayIconPNG()
		if err != nil {
			log.Printf("desktop: cannot build tray icon: %v", err)
		}
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开 " + desktopAppName, Click: func(*mygo.MenuItem, *mygo.Window) { openMainWindow() }},
			mygo.Separator(),
			{Label: "API " + apiAddr, Disabled: true},
			mygo.Separator(),
			{Label: "退出 " + desktopAppName, Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		})
		if _, err := mygo.NewTray(mygo.TrayOptions{
			ID:             "grabby-main",
			Icon:           icon,
			IconIsTemplate: true,
			ToolTip:        desktopAppName + " · API " + apiAddr,
			Menu:           menu,
		}); err != nil {
			log.Printf("desktop: tray unavailable: %v", err)
		}
		openMainWindow()
	})

	log.Printf("desktop: API listening on %s (external=%t)", apiAddr, external)
	if err := mygo.App.Run(); err != nil {
		log.Printf("desktop: app stopped: %v", err)
	}

	cancel()
	serverWG.Wait()
}

// desktopLogPath is where the desktop build mirrors its logs. GRABBY_LOG_FILE
// overrides it; otherwise macOS uses ~/Library/Logs/Grabby/desktop.log.
func desktopLogPath() string {
	if path := os.Getenv(logging.LogFileEnv); path != "" {
		return path
	}
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Logs", desktopAppName, "desktop.log")
		}
		return ""
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfg, desktopAppName, "logs", "desktop.log")
	}
	return ""
}

func isLocalHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// resolveDataDir decides where grabby.db, .env and friends live. Order:
//
//  1. GRABBY_DESKTOP_DATA_DIR, for explicit control (scripts, tests).
//  2. A directory remembered from a previous launch, so moving the .app to
//     /Applications does not silently start from an empty database.
//  3. A go-server directory next to the executable. A bundle built inside the
//     repo (dist/Grabby.app) therefore reuses the project's own database and
//     .env on the very first double-click, with no terminal involved.
//  4. The per-user data directory, for an installed copy with no project.
func resolveDataDir() string {
	if dir := os.Getenv("GRABBY_DESKTOP_DATA_DIR"); dir != "" {
		return dir
	}
	if dir := readDataDirHint(); dir != "" && hasDatabase(dir) {
		return dir
	}
	if dir := findProjectDataDir(); dir != "" {
		writeDataDirHint(dir)
		return dir
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		if cfg, cerr := os.UserConfigDir(); cerr == nil {
			return filepath.Join(cfg, desktopAppName)
		}
		log.Fatalf("desktop: cannot resolve user data directory: %v", err)
	}
	return dir
}

func hasDatabase(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "grabby.db"))
	return err == nil
}

// findProjectDataDir looks for <repo>/go-server/grabby.db by walking up from
// the running binary (…/dist/Grabby.app/Contents/MacOS/Grabby).
func findProjectDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "go-server")
		if hasDatabase(candidate) {
			return candidate
		}
		if hasDatabase(dir) && filepath.Base(dir) == "go-server" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func dataDirHintPath() string {
	base, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		if cfg, cerr := os.UserConfigDir(); cerr == nil {
			base = filepath.Join(cfg, desktopAppName)
		} else {
			return ""
		}
	}
	return filepath.Join(base, "data-dir.txt")
}

func readDataDirHint() string {
	path := dataDirHintPath()
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeDataDirHint(dir string) {
	path := dataDirHintPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(dir+"\n"), 0o600)
}

// freePort returns start when it is available, otherwise the first free port
// in a small range above it.
func freePort(start int) int {
	for port := start; port < start+32; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return start
}

func waitForServer(base string, timeout time.Duration) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/open/api/health")
		if err == nil {
			var body struct {
				Status string `json:"status"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && body.Status == "ok" {
				return true
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// trayIconPNG draws a small black-on-transparent "G" template icon at runtime,
// so the desktop build needs no binary asset beyond the embedded frontend.
func trayIconPNG() ([]byte, error) {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	ink := color.RGBA{A: 255}
	clear := color.RGBA{}

	rounded := func(x, y int) bool {
		const x0, y0, x1, y1, r = 3, 3, 29, 29, 7
		if x < x0 || x >= x1 || y < y0 || y >= y1 {
			return false
		}
		cx, cy := x, y
		if x < x0+r {
			cx = x0 + r
		} else if x >= x1-r {
			cx = x1 - r - 1
		}
		if y < y0+r {
			cy = y0 + r
		} else if y >= y1-r {
			cy = y1 - r - 1
		}
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= r*r
	}
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if rounded(x, y) {
				img.Set(x, y, ink)
			}
		}
	}
	// Cut a letter G out of the black square.
	fill(11, 10, 22, 13, clear) // top bar
	fill(11, 10, 14, 22, clear) // left bar
	fill(11, 19, 22, 22, clear) // bottom bar
	fill(18, 15, 22, 20, clear) // inner bar
	fill(19, 15, 23, 22, clear) // right bar

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
