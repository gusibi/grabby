// Package backup exports and restores the configuration a Grabby install is
// worth carrying between machines: AI connections/prompts/report schedule and
// the RSS/API/web source definitions.
//
// Content (scraped items, daily reports), fetch logs and AI usage history are
// deliberately excluded: they are bulk data, not configuration, and restoring
// them would fight with what the new machine has already collected.
//
// Export documents contain API keys in clear text. They are written only to a
// file the user explicitly downloads, and only the admin-authenticated API can
// produce them.
package backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	appai "go-server/internal/application/ai"
	"go-server/internal/domain/ai"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

const (
	// Version is the schema version of the exported document.
	Version = 1
	// AppName tags the document so an unrelated JSON file is rejected early.
	AppName = "grabby"
)

// SourceConfig is one importable source definition. Runtime bookkeeping
// (last_fetch_at, ETag, …) is intentionally left out.
type SourceConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Schedule        string `json:"schedule"`
	Enabled         int    `json:"enabled"`
	DefaultCategory string `json:"default_category"`
	Config          string `json:"config"`
	Category        string `json:"category"`
}

// Document is the on-disk backup file.
type Document struct {
	Version    int            `json:"version"`
	App        string         `json:"app"`
	ExportedAt time.Time      `json:"exported_at"`
	AI         *ai.AISettings `json:"ai,omitempty"`
	Sources    []SourceConfig `json:"sources"`
}

// Options selects which sections an import applies.
type Options struct {
	ImportAI      bool
	ImportSources bool
}

// Result summarises one applied import.
type Result struct {
	AIImported     bool     `json:"ai_imported"`
	SourcesCreated int      `json:"sources_created"`
	SourcesUpdated int      `json:"sources_updated"`
	Warnings       []string `json:"warnings,omitempty"`
}

// Build assembles a document from the live settings and source list.
func Build(settings ai.AISettings, sources []source.Source) Document {
	doc := Document{
		Version:    Version,
		App:        AppName,
		ExportedAt: time.Now().UTC(),
		Sources:    make([]SourceConfig, 0, len(sources)),
	}
	settingsCopy := settings
	doc.AI = &settingsCopy

	for _, src := range sources {
		doc.Sources = append(doc.Sources, SourceConfig{
			ID:              src.ID,
			Name:            src.Name,
			Type:            src.Type,
			URL:             src.URL,
			Schedule:        src.Schedule,
			Enabled:         src.Enabled,
			DefaultCategory: src.DefaultCategory,
			Config:          src.Config,
			Category:        src.Category,
		})
	}
	// Stable order keeps diffs between two backups readable.
	sort.Slice(doc.Sources, func(i, j int) bool { return doc.Sources[i].ID < doc.Sources[j].ID })
	return doc
}

// Encode renders a document as an indented JSON file.
func Encode(doc Document) ([]byte, error) {
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// Parse validates an uploaded document and normalises its source entries.
// Missing optional fields are filled with the same defaults the source form
// uses, so a hand-edited backup still imports.
func Parse(raw []byte) (Document, error) {
	var doc Document
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("无法解析配置文件：%w", err)
	}

	if app := strings.TrimSpace(doc.App); app != "" && !strings.EqualFold(app, AppName) {
		return Document{}, fmt.Errorf("这不是 Grabby 的配置文件（app=%q）", doc.App)
	}
	if doc.Version <= 0 {
		doc.Version = 1
	}
	if doc.Version > Version {
		return Document{}, fmt.Errorf("配置文件的版本为 %d，高于当前支持的最高版本 %d，请升级 Grabby 后再导入", doc.Version, Version)
	}
	if doc.AI == nil && len(doc.Sources) == 0 {
		return Document{}, fmt.Errorf("配置文件中没有 AI 设置或数据源")
	}

	// Keep the last definition for a duplicated id, matching how an upsert
	// would behave while making the counts deterministic.
	seen := make(map[string]int, len(doc.Sources))
	deduped := make([]SourceConfig, 0, len(doc.Sources))
	for i := range doc.Sources {
		entry := doc.Sources[i]
		if err := entry.normalize(i); err != nil {
			return Document{}, err
		}
		if pos, ok := seen[entry.ID]; ok {
			deduped[pos] = entry
			continue
		}
		seen[entry.ID] = len(deduped)
		deduped = append(deduped, entry)
	}
	doc.Sources = deduped
	doc.App = AppName
	return doc, nil
}

func (s *SourceConfig) normalize(index int) error {
	label := fmt.Sprintf("第 %d 个数据源", index+1)
	s.ID = strings.TrimSpace(s.ID)
	if s.ID == "" {
		return fmt.Errorf("%s缺少 id", label)
	}
	label = fmt.Sprintf("数据源 %q", s.ID)
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		s.Name = s.ID
	}
	s.Type = strings.ToLower(strings.TrimSpace(s.Type))
	if s.Type == "" {
		s.Type = "rss"
	}
	s.URL = strings.TrimSpace(s.URL)
	if s.URL == "" {
		return fmt.Errorf("%s缺少 URL", label)
	}
	s.Schedule = strings.TrimSpace(s.Schedule)
	if s.Schedule == "" {
		s.Schedule = "0 */2 * * *"
	}
	if s.Enabled != 0 {
		s.Enabled = 1
	}
	s.DefaultCategory = strings.TrimSpace(s.DefaultCategory)
	if s.DefaultCategory == "" {
		s.DefaultCategory = "auto"
	}
	s.Config = strings.TrimSpace(s.Config)
	if s.Config == "" {
		s.Config = "{}"
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(s.Config), &probe); err != nil {
		return fmt.Errorf("%s的高级配置不是合法 JSON：%w", label, err)
	}
	s.Category = strings.TrimSpace(s.Category)
	if s.Category == "" {
		s.Category = "General"
	}
	return nil
}

func (s SourceConfig) toSource() source.Source {
	return source.Source{
		ID:              s.ID,
		Name:            s.Name,
		Type:            s.Type,
		URL:             s.URL,
		Schedule:        s.Schedule,
		Enabled:         s.Enabled,
		DefaultCategory: s.DefaultCategory,
		Config:          s.Config,
		Category:        s.Category,
	}
}

// Apply writes a parsed document to the database.
//
// AI settings are replaced wholesale (that is what restoring a backup means).
// Sources are upserted by id, so unrelated local sources survive and existing
// ones keep their collected items.
func Apply(db *sqlite.Database, reloader appai.SettingsReloader, doc Document, opts Options) (Result, error) {
	var result Result
	if !opts.ImportAI && !opts.ImportSources {
		return result, fmt.Errorf("没有选择要导入的内容")
	}

	if opts.ImportAI {
		if doc.AI == nil {
			result.Warnings = append(result.Warnings, "备份中没有 AI 配置，已跳过")
		} else if err := appai.PersistSettings(db, reloader, *doc.AI); err != nil {
			return result, fmt.Errorf("导入 AI 配置失败：%w", err)
		} else {
			result.AIImported = true
		}
	}

	if opts.ImportSources {
		if len(doc.Sources) == 0 {
			result.Warnings = append(result.Warnings, "备份中没有数据源，已跳过")
		} else {
			existing, err := db.GetSources()
			if err != nil {
				return result, fmt.Errorf("读取现有数据源失败：%w", err)
			}
			known := make(map[string]struct{}, len(existing))
			for _, src := range existing {
				known[src.ID] = struct{}{}
			}

			for _, entry := range doc.Sources {
				src := entry.toSource()
				_, exists := known[src.ID]
				if err := db.UpsertSource(src); err != nil {
					return result, fmt.Errorf("导入数据源 %q 失败：%w", src.ID, err)
				}
				if exists {
					result.SourcesUpdated++
				} else {
					result.SourcesCreated++
					known[src.ID] = struct{}{}
				}
			}
		}
	}

	return result, nil
}
