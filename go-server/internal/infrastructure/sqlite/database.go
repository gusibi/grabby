package sqlite

import (
	"database/sql"
	"fmt"

	glebarez "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

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
	db   *sql.DB
	gorm *gorm.DB
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

	// GORM shares the same *sql.DB connection pool so the new repos and the
	// existing hand-written SQL hit one sqlite file without extra lock contention.
	// Silence GORM's own SQL logger; the app handles returned errors via zap and
	// does not want per-query / record-not-found noise on stdout.
	gormDB, err := gorm.Open(glebarez.Dialector{Conn: db}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to open gorm: %w", err)
	}

	d := &Database{db: db, gorm: gormDB}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	// AutoMigrate only the GORM-managed tables (extract cache + tweet archive).
	if err := gormDB.AutoMigrate(&ExtractCache{}, &TweetRecord{}); err != nil {
		db.Close()
		return nil, fmt.Errorf("gorm auto-migrate failed: %w", err)
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
