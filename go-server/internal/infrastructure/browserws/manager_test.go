package browserws

import (
	"testing"

	"go.uber.org/zap"
)

func TestResolveBrowserConnIDByName(t *testing.T) {
	wm := NewWebSocketManager(zap.NewNop())
	wm.Connect("browser-1", nil)
	if err := wm.RegisterBrowserName("browser-1", "chrome-office"); err != nil {
		t.Fatalf("RegisterBrowserName() error = %v", err)
	}

	connID, err := wm.ResolveBrowserConnID("chrome-office")
	if err != nil {
		t.Fatalf("ResolveBrowserConnID() error = %v", err)
	}

	if connID != "browser-1" {
		t.Fatalf("ResolveBrowserConnID() = %q, want browser-1", connID)
	}
}

func TestRegisterBrowserNameRejectsDuplicateActiveName(t *testing.T) {
	wm := NewWebSocketManager(zap.NewNop())
	wm.Connect("browser-1", nil)
	wm.Connect("browser-2", nil)
	if err := wm.RegisterBrowserName("browser-1", "chrome-office"); err != nil {
		t.Fatalf("RegisterBrowserName() error = %v", err)
	}

	if err := wm.RegisterBrowserName("browser-2", "chrome-office"); err == nil {
		t.Fatal("RegisterBrowserName() error = nil, want duplicate name error")
	}
}

func TestGetBrowserListOnlyIncludesNamedBrowserConnections(t *testing.T) {
	wm := NewWebSocketManager(zap.NewNop())
	wm.Connect("command-1", nil)
	wm.Connect("browser-1", nil)
	if err := wm.RegisterBrowserName("browser-1", "chrome-office"); err != nil {
		t.Fatalf("RegisterBrowserName() error = %v", err)
	}

	browsers := wm.GetBrowserList()
	if len(browsers) != 1 {
		t.Fatalf("len(GetBrowserList()) = %d, want 1", len(browsers))
	}
	if browsers[0].ConnID != "browser-1" || browsers[0].Name != "chrome-office" {
		t.Fatalf("GetBrowserList()[0] = %+v", browsers[0])
	}
}
