package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

type Scheduler struct {
	mu           sync.Mutex
	cron         *cron.Cron
	db           *sqlite.Database
	scraper      *scraping.Scraper
	logger       *zap.Logger
	entryMap     map[string]cron.EntryID
	dailyManager *appai.AIDailyManager
}

func NewScheduler(db *sqlite.Database, scraper *scraping.Scraper, dailyManager *appai.AIDailyManager, logger *zap.Logger) *Scheduler {
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

	sources, err := s.db.GetSources()
	if err != nil {
		return err
	}

	for _, src := range sources {
		if src.Enabled == 1 {
			s.scheduleSource(src)
		}
	}

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

	if s.dailyManager != nil && config.GetSettings().AISettings.Enabled {
		aiSettings := config.GetSettings().AISettings

		if aiSettings.MorningReportEnabled {
			morningCron := timeToCronExpr(aiSettings.MorningReportTime)
			_, err = s.cron.AddFunc(morningCron, func() {
				s.logger.Info("Starting morning AI report generation job...")
				runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				now := time.Now()
				dateStr := now.Format("2006-01-02")
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

func (s *Scheduler) AddOrUpdateSource(src source.Source) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entryMap[src.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryMap, src.ID)
		s.logger.Info("Removed old cron task for source", zap.String("id", src.ID))
	}

	if src.Enabled == 1 {
		s.scheduleSource(src)
	}
}

func (s *Scheduler) RemoveSource(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entryMap[sourceID]; exists {
		s.cron.Remove(entryID)
		delete(s.entryMap, sourceID)
		s.logger.Info("Removed cron task for source", zap.String("id", sourceID))
	}
}

func (s *Scheduler) scheduleSource(src source.Source) {
	entryID, err := s.cron.AddFunc(src.Schedule, func() {
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

func timeToCronExpr(hhmm string) string {
	parts := splitTime(hhmm)
	return fmt.Sprintf("%d %d * * *", parts[1], parts[0])
}

func splitTime(hhmm string) [2]int {
	hour, min := 0, 0
	if len(hhmm) >= 5 {
		fmt.Sscanf(hhmm[:2], "%d", &hour)
		fmt.Sscanf(hhmm[3:5], "%d", &min)
	}
	return [2]int{hour, min}
}
