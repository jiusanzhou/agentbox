package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
)

const tickInterval = 1 * time.Minute

// Scheduler checks for due schedules every minute and creates pending runs.
type Scheduler struct {
	store  store.Store
	logger *slog.Logger
}

// New creates a Scheduler that persists runs via the given store.
// The logger is optional; slog.Default() is used when nil.
func New(s store.Store, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:  s,
		logger: logger,
	}
}

// Start launches the scheduling loop in a background goroutine.
// It returns immediately. The loop stops when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
	s.logger.Info("scheduler started", "interval", tickInterval)
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	// Run once immediately on startup so we don't wait a full minute.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()
	schedules, err := s.store.ListDueSchedules(ctx, now)
	if err != nil {
		s.logger.Error("failed to list due schedules", "err", err)
		return
	}
	if len(schedules) == 0 {
		return
	}

	s.logger.Info("processing due schedules", "count", len(schedules))

	for _, sched := range schedules {
		if err := s.processSchedule(ctx, sched, now); err != nil {
			s.logger.Error("failed to process schedule",
				"schedule_id", sched.ID,
				"name", sched.Name,
				"err", err,
			)
		}
	}
}

func (s *Scheduler) processSchedule(ctx context.Context, sched *model.Schedule, now time.Time) error {
	runID, err := generateID()
	if err != nil {
		return fmt.Errorf("generate run id: %w", err)
	}

	run := &model.Run{
		ID:        runID,
		UserID:    sched.UserID,
		Name:      fmt.Sprintf("scheduled: %s", sched.Name),
		Mode:      model.RunModeRun,
		Status:    model.RunStatusPending,
		Runtime:   sched.Runtime,
		AgentFile: sched.AgentID,
		CreatedAt: now,
	}

	if err := s.store.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	// Advance schedule timestamps.
	sched.LastRunAt = &now

	next, err := computeNextRun(sched.CronExpr, sched.Timezone, now)
	if err != nil {
		s.logger.Warn("failed to compute next run time, disabling schedule",
			"schedule_id", sched.ID,
			"err", err,
		)
		sched.Enabled = false
	} else {
		sched.NextRunAt = &next
	}

	if err := s.store.UpdateSchedule(ctx, sched); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}

	s.logger.Info("created scheduled run",
		"run_id", runID,
		"schedule_id", sched.ID,
		"schedule_name", sched.Name,
	)
	return nil
}

// computeNextRun parses a cron expression with an optional IANA timezone and
// returns the next fire time after the reference time.
func computeNextRun(cronExpr, timezone string, after time.Time) (time.Time, error) {
	loc := time.UTC
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("load timezone %q: %w", timezone, err)
		}
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron expression %q: %w", cronExpr, err)
	}

	next := schedule.Next(after.In(loc))
	return next.UTC(), nil
}

// generateID returns a 16-character hex string from 8 random bytes.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ComputeNextRun parses a cron expression with timezone and returns the next fire time.
func (s *Scheduler) ComputeNextRun(cronExpr, timezone string) (time.Time, error) {
	return computeNextRun(cronExpr, timezone, time.Now())
}

// TriggerSchedule manually triggers a schedule, creating a run immediately.
func (s *Scheduler) TriggerSchedule(ctx context.Context, sched *model.Schedule) (*model.Run, error) {
	now := time.Now()
	runID, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate run id: %w", err)
	}

	run := &model.Run{
		ID:        runID,
		UserID:    sched.UserID,
		Name:      fmt.Sprintf("manual: %s", sched.Name),
		Mode:      model.RunModeRun,
		Status:    model.RunStatusPending,
		Runtime:   sched.Runtime,
		AgentFile: sched.AgentID,
		CreatedAt: now,
	}

	if err := s.store.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	s.logger.Info("manually triggered schedule", "run_id", runID, "schedule_id", sched.ID)
	return run, nil
}
