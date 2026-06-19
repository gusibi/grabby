package browserregistry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go-server/internal/domain/browser"
)

var ErrBrowserRegistryConflict = errors.New("browser registration conflict")

type browserRegistryFile struct {
	Browsers []browser.BrowserRegistration `json:"browsers"`
}

type BrowserRegistry struct {
	mu       sync.RWMutex
	path     string
	browsers map[string]*browser.BrowserRegistration
}

func NewBrowserRegistry(path string) (*BrowserRegistry, error) {
	if path == "" {
		path = filepath.Join(".", "browser_registry.json")
	}

	registry := &BrowserRegistry{
		path:     path,
		browsers: make(map[string]*browser.BrowserRegistration),
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

	for _, b := range file.Browsers {
		connectID := strings.TrimSpace(b.ConnectID)
		name := strings.TrimSpace(b.Name)
		if connectID != "" && name != "" {
			br.browsers[connectID] = &browser.BrowserRegistration{
				ConnectID: connectID,
				Name:      name,
				Banned:    b.Banned,
			}
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

	file := browserRegistryFile{Browsers: make([]browser.BrowserRegistration, 0, len(ids))}
	for _, connectID := range ids {
		file.Browsers = append(file.Browsers, *br.browsers[connectID])
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

func (br *BrowserRegistry) Register(connectID, name string) (browser.BrowserRegistration, error) {
	connectID = strings.TrimSpace(connectID)
	name = strings.TrimSpace(name)
	if connectID == "" {
		return browser.BrowserRegistration{}, errors.New("connect_id is required")
	}
	if name == "" {
		return browser.BrowserRegistration{}, errors.New("name is required")
	}

	br.mu.Lock()
	defer br.mu.Unlock()

	if reg, ok := br.browsers[connectID]; ok {
		if reg.Name == name {
			return *reg, nil
		}
		return browser.BrowserRegistration{}, ErrBrowserRegistryConflict
	}

	for existingID, reg := range br.browsers {
		if existingID != connectID && reg.Name == name {
			return browser.BrowserRegistration{}, ErrBrowserRegistryConflict
		}
	}

	reg := &browser.BrowserRegistration{ConnectID: connectID, Name: name}
	br.browsers[connectID] = reg
	if err := br.save(); err != nil {
		delete(br.browsers, connectID)
		return browser.BrowserRegistration{}, err
	}

	return *reg, nil
}

func (br *BrowserRegistry) Ban(connectID string) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	reg, ok := br.browsers[connectID]
	if !ok {
		return errors.New("browser registration not found")
	}

	reg.Banned = true
	return br.save()
}

func (br *BrowserRegistry) Unban(connectID string) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	reg, ok := br.browsers[connectID]
	if !ok {
		return errors.New("browser registration not found")
	}

	reg.Banned = false
	return br.save()
}

func (br *BrowserRegistry) List() []browser.BrowserRegistration {
	br.mu.RLock()
	defer br.mu.RUnlock()

	list := make([]browser.BrowserRegistration, 0, len(br.browsers))
	for _, reg := range br.browsers {
		list = append(list, *reg)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (br *BrowserRegistry) Validate(connectID, name string) bool {
	br.mu.RLock()
	defer br.mu.RUnlock()
	reg, ok := br.browsers[connectID]
	if !ok {
		return false
	}
	return reg.Name == name && !reg.Banned
}

func (br *BrowserRegistry) GetName(connectID string) (string, bool) {
	br.mu.RLock()
	defer br.mu.RUnlock()
	reg, ok := br.browsers[connectID]
	if !ok {
		return "", false
	}
	return reg.Name, true
}
