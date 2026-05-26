package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var ErrBrowserRegistryConflict = errors.New("browser registration conflict")

type BrowserRegistration struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

type browserRegistryFile struct {
	Browsers []BrowserRegistration `json:"browsers"`
}

type BrowserRegistry struct {
	mu       sync.RWMutex
	path     string
	browsers map[string]string
}

func NewBrowserRegistry(path string) (*BrowserRegistry, error) {
	if path == "" {
		path = filepath.Join(".", "browser_registry.json")
	}

	registry := &BrowserRegistry{
		path:     path,
		browsers: make(map[string]string),
	}
	if err := registry.load(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (br *BrowserRegistry) load() error {
	data, err := os.ReadFile(br.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var file browserRegistryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	for _, browser := range file.Browsers {
		connectID := strings.TrimSpace(browser.ConnectID)
		name := strings.TrimSpace(browser.Name)
		if connectID != "" && name != "" {
			br.browsers[connectID] = name
		}
	}
	return nil
}

func (br *BrowserRegistry) save() error {
	if err := os.MkdirAll(filepath.Dir(br.path), 0o755); err != nil {
		return err
	}

	ids := make([]string, 0, len(br.browsers))
	for connectID := range br.browsers {
		ids = append(ids, connectID)
	}
	sort.Strings(ids)

	file := browserRegistryFile{Browsers: make([]BrowserRegistration, 0, len(ids))}
	for _, connectID := range ids {
		file.Browsers = append(file.Browsers, BrowserRegistration{
			ConnectID: connectID,
			Name:      br.browsers[connectID],
		})
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := br.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, br.path)
}

func (br *BrowserRegistry) Register(connectID, name string) (BrowserRegistration, error) {
	connectID = strings.TrimSpace(connectID)
	name = strings.TrimSpace(name)
	if connectID == "" {
		return BrowserRegistration{}, errors.New("connect_id is required")
	}
	if name == "" {
		return BrowserRegistration{}, errors.New("name is required")
	}

	br.mu.Lock()
	defer br.mu.Unlock()

	if existingName, ok := br.browsers[connectID]; ok {
		if existingName == name {
			return BrowserRegistration{ConnectID: connectID, Name: name}, nil
		}
		return BrowserRegistration{}, ErrBrowserRegistryConflict
	}

	for existingID, existingName := range br.browsers {
		if existingID != connectID && existingName == name {
			return BrowserRegistration{}, ErrBrowserRegistryConflict
		}
	}

	br.browsers[connectID] = name
	if err := br.save(); err != nil {
		delete(br.browsers, connectID)
		return BrowserRegistration{}, err
	}

	return BrowserRegistration{ConnectID: connectID, Name: name}, nil
}

func (br *BrowserRegistry) Validate(connectID, name string) bool {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.browsers[connectID] == name
}

func (br *BrowserRegistry) GetName(connectID string) (string, bool) {
	br.mu.RLock()
	defer br.mu.RUnlock()
	name, ok := br.browsers[connectID]
	return name, ok
}
