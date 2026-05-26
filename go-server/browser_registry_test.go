package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestRegistry(t *testing.T) *BrowserRegistry {
	t.Helper()
	registry, err := NewBrowserRegistry(filepath.Join(t.TempDir(), "browser_registry.json"))
	if err != nil {
		t.Fatalf("NewBrowserRegistry() error = %v", err)
	}
	return registry
}

func TestBrowserRegistryRegister(t *testing.T) {
	registry := newTestRegistry(t)

	browser, err := registry.Register("browser-1", "chrome-office")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if browser.ConnectID != "browser-1" || browser.Name != "chrome-office" {
		t.Fatalf("Register() = %+v", browser)
	}
	if !registry.Validate("browser-1", "chrome-office") {
		t.Fatal("Validate() = false")
	}
}

func TestBrowserRegistryRegisterIsIdempotent(t *testing.T) {
	registry := newTestRegistry(t)
	if _, err := registry.Register("browser-1", "chrome-office"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	browser, err := registry.Register("browser-1", "chrome-office")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if browser.ConnectID != "browser-1" || browser.Name != "chrome-office" {
		t.Fatalf("Register() = %+v", browser)
	}
}

func TestBrowserRegistryRejectsDuplicateName(t *testing.T) {
	registry := newTestRegistry(t)
	if _, err := registry.Register("browser-1", "chrome-office"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := registry.Register("browser-2", "chrome-office")
	if !errors.Is(err, ErrBrowserRegistryConflict) {
		t.Fatalf("Register() error = %v, want ErrBrowserRegistryConflict", err)
	}
}

func TestBrowserRegistryRejectsSameIDWithDifferentName(t *testing.T) {
	registry := newTestRegistry(t)
	if _, err := registry.Register("browser-1", "chrome-office"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := registry.Register("browser-1", "chrome-home")
	if !errors.Is(err, ErrBrowserRegistryConflict) {
		t.Fatalf("Register() error = %v, want ErrBrowserRegistryConflict", err)
	}
}

func TestBrowserRegistryReloadsFromFile(t *testing.T) {
	registry := newTestRegistry(t)
	if _, err := registry.Register("browser-1", "chrome-office"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	reloaded, err := NewBrowserRegistry(registry.path)
	if err != nil {
		t.Fatalf("NewBrowserRegistry() error = %v", err)
	}

	name, ok := reloaded.GetName("browser-1")
	if !ok || name != "chrome-office" {
		t.Fatalf("GetName() = %q, %v", name, ok)
	}
	if _, err := os.Stat(registry.path); err != nil {
		t.Fatalf("registry file was not written: %v", err)
	}
}
