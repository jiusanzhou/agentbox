package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
)

// UsageTracker records platform usage asynchronously via a buffered channel.
type UsageTracker struct {
	store  store.Store
	logger *slog.Logger
	ch     chan model.PlatformUsageRecord
}

// NewUsageTracker creates a new usage tracker with a background worker.
func NewUsageTracker(s store.Store, logger *slog.Logger) *UsageTracker {
	if logger == nil {
		logger = slog.Default()
	}
	t := &UsageTracker{
		store:  s,
		logger: logger,
		ch:     make(chan model.PlatformUsageRecord, 1024),
	}
	go t.worker()
	return t
}

func (t *UsageTracker) worker() {
	for rec := range t.ch {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := t.store.RecordPlatformUsage(ctx, &rec); err != nil {
			t.logger.Error("failed to record usage", "type", rec.Type, "user", rec.UserID, "err", err)
		}
		cancel()
	}
}

func usageID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// TrackCompute records compute time in seconds.
func (t *UsageTracker) TrackCompute(userID, runID string, seconds float64) {
	t.ch <- model.PlatformUsageRecord{
		ID:          usageID(),
		UserID:      userID,
		Type:        "compute",
		Amount:      seconds,
		Unit:        "seconds",
		RunID:       runID,
		Description: "compute time",
		CreatedAt:   time.Now(),
	}
}

// TrackTokens records token usage.
func (t *UsageTracker) TrackTokens(userID, runID, sessionID string, tokens int64) {
	t.ch <- model.PlatformUsageRecord{
		ID:          usageID(),
		UserID:      userID,
		Type:        "tokens",
		Amount:      float64(tokens),
		Unit:        "tokens",
		RunID:       runID,
		SessionID:   sessionID,
		Description: "token usage",
		CreatedAt:   time.Now(),
	}
}

// TrackStorage records storage bytes.
func (t *UsageTracker) TrackStorage(userID, runID string, bytes int64) {
	t.ch <- model.PlatformUsageRecord{
		ID:          usageID(),
		UserID:      userID,
		Type:        "storage",
		Amount:      float64(bytes),
		Unit:        "bytes",
		RunID:       runID,
		Description: "file upload",
		CreatedAt:   time.Now(),
	}
}

// TrackAPICall records an API call.
func (t *UsageTracker) TrackAPICall(userID string) {
	t.ch <- model.PlatformUsageRecord{
		ID:          usageID(),
		UserID:      userID,
		Type:        "api_call",
		Amount:      1,
		Unit:        "calls",
		Description: "API call",
		CreatedAt:   time.Now(),
	}
}
