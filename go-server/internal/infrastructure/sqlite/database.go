package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"go-server/internal/domain/ai"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
)

// Type aliases for domain types — keeps method signatures unchanged during migration.
type Source = source.Source
type SourceForm = source.SourceForm
type FetchLog = source.FetchLog
type ScrapedItem = item.ScrapedItem
type ScrapedItemWithAI = item.ScrapedItemWithAI
type ItemsFilter = item.ItemsFilter
type AIItemsFilter = item.AIItemsFilter
type AIAnalysis = ai.AIAnalysis
type AIDailyReport = ai.AIDailyReport
type AICategoryStat = ai.AICategoryStat
type AISettings = ai.AISettings
type AIProviderProfile = ai.AIProviderProfile

// NormalizeAISettings delegates to the domain package implementation.
func NormalizeAISettings(settings AISettings) AISettings {
	return ai.NormalizeAISettings(settings)
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// WAL mode: concurrency reading/writing
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	// Busy timeout: wait up to 5 seconds if database is locked
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(2)

	d := &Database{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := d.seedDefaultSources(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seeding default sources failed: %w", err)
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
