package sqlite

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AICatalogSnapshot caches the normalized models.dev catalog. It is a single
// row (id = 1) because the catalog is a global, read-mostly dataset; keeping
// the JSON blob intact makes refreshes atomic and lets the frontend receive
// exactly what the backend validated.
type AICatalogSnapshot struct {
	ID        int       `gorm:"primaryKey;column:id"`
	Version   int       `gorm:"column:version"`
	Payload   string    `gorm:"column:payload"`
	FetchedAt time.Time `gorm:"column:fetched_at"`
	ETag      string    `gorm:"column:etag"`
	Source    string    `gorm:"column:source"`
}

// TableName pins the table name (GORM would otherwise pluralize the struct).
func (AICatalogSnapshot) TableName() string { return "ai_catalog_snapshot" }

// GetAICatalogSnapshot returns the cached catalog or (nil, nil) when the cache
// has not been populated yet.
func (d *Database) GetAICatalogSnapshot() (*AICatalogSnapshot, error) {
	var rec AICatalogSnapshot
	err := d.gorm.Where("id = ?", 1).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// SaveAICatalogSnapshot upserts the single cached catalog row.
func (d *Database) SaveAICatalogSnapshot(payload, etag, source string, version int, fetchedAt time.Time) error {
	rec := AICatalogSnapshot{
		ID:        1,
		Version:   version,
		Payload:   payload,
		FetchedAt: fetchedAt,
		ETag:      etag,
		Source:    source,
	}
	return d.gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"version", "payload", "fetched_at", "etag", "source"}),
	}).Create(&rec).Error
}
