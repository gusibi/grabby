package main

import (
	"context"
	"fmt"
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

	// Schedule the daily AI report jobs (morning/evening)
	if s.dailyManager != nil && GetSettings().AISettings.Enabled {
		aiSettings := GetSettings().AISettings

		if aiSettings.MorningReportEnabled {
			morningCron := timeToCronExpr(aiSettings.MorningReportTime)
			_, err = s.cron.AddFunc(morningCron, func() {
				s.logger.Info("Starting morning AI report generation job...")
				runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				now := time.Now()
				dateStr := now.Format("2006-01-02")
				// Parse morning time (HH:MM)
				mt := aiSettings.MorningReportTime
				parts := splitTime(mt)
				morningHour, morningMin := parts[0], parts[1]
				end := time.Date(now.Year(), now.Month(), now.Day(), morningHour, morningMin, 0, 0, now.Location())
				start := end.Add(-24 * time.Hour)
				_, err := s.dailyManager.GenerateRangedReport(runCtx, dateStr, "morning", start, end)
				if err != nil {
					s.logger.Error("Scheduled morning AI report generation failed", zap.String("date", dateStr), zap.Error(err))
				} else {
					s.logger.Info("Scheduled morning AI report generation finished successfully", zap.String("date", dateStr))
				}
			})
			if err != nil {
				s.logger.Error("Failed to schedule morning AI report job", zap.Error(err))
			} else {
				s.logger.Info("Scheduled morning AI report job", zap.String("cron", morningCron))
			}
		}

		if aiSettings.EveningReportEnabled {
			eveningCron := timeToCronExpr(aiSettings.EveningReportTime)
			_, err = s.cron.AddFunc(eveningCron, func() {
				s.logger.Info("Starting evening AI report generation job...")
				runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				now := time.Now()
				dateStr := now.Format("2006-01-02")
				// Evening range: from morning time today to evening time today
				mt := aiSettings.MorningReportTime
				mparts := splitTime(mt)
				morningHour, morningMin := mparts[0], mparts[1]
				et := aiSettings.EveningReportTime
				eparts := splitTime(et)
				eveningHour, eveningMin := eparts[0], eparts[1]
				start := time.Date(now.Year(), now.Month(), now.Day(), morningHour, morningMin, 0, 0, now.Location())
				end := time.Date(now.Year(), now.Month(), now.Day(), eveningHour, eveningMin, 0, 0, now.Location())
				_, err := s.dailyManager.GenerateRangedReport(runCtx, dateStr, "evening", start, end)
				if err != nil {
					s.logger.Error("Scheduled evening AI report generation failed", zap.String("date", dateStr), zap.Error(err))
				} else {
					s.logger.Info("Scheduled evening AI report generation finished successfully", zap.String("date", dateStr))
				}
			})
			if err != nil {
				s.logger.Error("Failed to schedule evening AI report job", zap.Error(err))
			} else {
				s.logger.Info("Scheduled evening AI report job", zap.String("cron", eveningCron))
			}
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

// timeToCronExpr converts a "HH:MM" time string to a cron expression "MM HH * * *".
func timeToCronExpr(hhmm string) string {
	parts := splitTime(hhmm)
	return fmt.Sprintf("%d %d * * *", parts[1], parts[0])
}

// splitTime parses "HH:MM" into [hour, minute]. Falls back to [0, 0] on error.
func splitTime(hhmm string) [2]int {
	hour, min := 0, 0
	if len(hhmm) >= 5 {
		fmt.Sscanf(hhmm[:2], "%d", &hour)
		fmt.Sscanf(hhmm[3:5], "%d", &min)
	}
	return [2]int{hour, min}
}
