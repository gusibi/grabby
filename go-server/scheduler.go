package main

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Scheduler struct {
	mu           sync.Mutex
	cron         *cron.Cron
	db           *Database
	scraper      *Scraper
	logger       *zap.Logger
	entryMap     map[string]cron.EntryID
	dailyManager *AIDailyManager
}

func NewScheduler(db *Database, scraper *Scraper, dailyManager *AIDailyManager, logger *zap.Logger) *Scheduler {
	c := cron.New()
	return &Scheduler{
		cron:         c,
		db:           db,
		scraper:      scraper,
		logger:       logger,
		entryMap:     make(map[string]cron.EntryID),
		dailyManager: dailyManager,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load all enabled sources and schedule them
	sources, err := s.db.GetSources()
	if err != nil {
		return err
	}

	for _, src := range sources {
		if src.Enabled == 1 {
			s.scheduleSource(src)
		}
	}

	// Schedule the daily cleanup job at 3:00 AM
	_, err = s.cron.AddFunc("0 3 * * *", func() {
		s.logger.Info("Starting daily database cleanup job...")
		if err := s.db.CleanupOldData(); err != nil {
			s.logger.Error("Database cleanup failed", zap.Error(err))
		} else {
			s.logger.Info("Database cleanup finished successfully")
		}
	})
	if err != nil {
		s.logger.Error("Failed to schedule cleanup job", zap.Error(err))
	}

	// Schedule the daily AI report job at 10:00 PM (22:00)
	if s.dailyManager != nil && GetSettings().AISettings.Enabled {
		_, err = s.cron.AddFunc("0 22 * * *", func() {
			s.logger.Info("Starting daily AI report generation job...")
			runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			dateStr := time.Now().Format("2006-01-02")
			_, err := s.dailyManager.GenerateDailyReport(runCtx, dateStr)
			if err != nil {
				s.logger.Error("Scheduled daily AI report generation failed", zap.String("date", dateStr), zap.Error(err))
			} else {
				s.logger.Info("Scheduled daily AI report generation finished successfully", zap.String("date", dateStr))
			}
		})
		if err != nil {
			s.logger.Error("Failed to schedule daily AI report job", zap.Error(err))
		}
	}

	s.cron.Start()
	s.logger.Info("Scheduler started successfully")
	return nil
}

func (s *Scheduler) Stop() context.Context {
	s.logger.Info("Stopping scheduler...")
	return s.cron.Stop()
}

// AddOrUpdateSource performs incremental hot reload for a source.
func (s *Scheduler) AddOrUpdateSource(src Source) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old cron task if it exists
	if entryID, exists := s.entryMap[src.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryMap, src.ID)
		s.logger.Info("Removed old cron task for source", zap.String("id", src.ID))
	}

	// Add new cron task if enabled
	if src.Enabled == 1 {
		s.scheduleSource(src)
	}
}

// RemoveSource removes a source cron task.
func (s *Scheduler) RemoveSource(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entryMap[sourceID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryMap, sourceID)
		s.logger.Info("Removed cron task for source", zap.String("id", sourceID))
	}
}

func (s *Scheduler) scheduleSource(src Source) {
	entryID, err := s.cron.AddFunc(src.Schedule, func() {
		// Run scrape job in a background context to keep it independent
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_, _ = s.scraper.ScrapeSource(ctx, src)
	})

	if err != nil {
		s.logger.Error("Failed to schedule source cron job", zap.String("id", src.ID), zap.String("schedule", src.Schedule), zap.Error(err))
		return
	}

	s.entryMap[src.ID] = entryID
	s.logger.Info("Scheduled cron task for source", zap.String("id", src.ID), zap.String("schedule", src.Schedule))
}
