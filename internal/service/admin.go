package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/runtime"
	"go.zoe.im/x"
)

// SetConfigPath sets the path for config file persistence.
func (s *Service) SetConfigPath(path string) {
	s.configPath = path
}

// GetConfig returns the sanitized server configuration.
func (s *Service) GetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.Sanitize(s.cfg))
}

// UpdateConfig applies a partial config update.
func (s *Service) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Apply known fields
	if v, ok := patch["session_ttl"]; ok {
		var val string
		if err := json.Unmarshal(v, &val); err == nil {
			s.cfg.SessionTTL = val
		}
	}
	if v, ok := patch["cleanup_interval"]; ok {
		var val string
		if err := json.Unmarshal(v, &val); err == nil {
			s.cfg.CleanupInterval = val
		}
	}
	if v, ok := patch["rate_limit"]; ok {
		var rl config.RateLimitConfig
		if err := json.Unmarshal(v, &rl); err == nil {
			s.cfg.RateLimit = rl
			if s.limiter != nil {
				s.limiter.UpdateConfig(rl)
			}
		}
	}
	if v, ok := patch["debug"]; ok {
		var val bool
		if err := json.Unmarshal(v, &val); err == nil {
			s.cfg.Debug = val
		}
	}

	// Persist to file
	if s.configPath != "" {
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			s.logger.Error("failed to save config", "err", err)
			http.Error(w, fmt.Sprintf(`{"error":"save failed: %s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.Sanitize(s.cfg))
}

// GetChannels returns the configured IM channels (sanitized).
func (s *Service) GetChannels(w http.ResponseWriter, r *http.Request) {
	sanitized := config.Sanitize(s.cfg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitized.Channels)
}

// AddChannel adds a new channel to the config and hot-reloads.
func (s *Service) AddChannel(w http.ResponseWriter, r *http.Request) {
	var ch x.TypedLazyConfig
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if ch.Type == "" {
		http.Error(w, `{"error":"type is required"}`, http.StatusBadRequest)
		return
	}

	s.cfg.Channels = append(s.cfg.Channels, ch)

	if s.configPath != "" {
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			s.logger.Error("failed to save config", "err", err)
		}
	}

	if err := s.ReloadChannels(r.Context()); err != nil {
		s.logger.Error("failed to reload channels", "err", err)
		http.Error(w, fmt.Sprintf(`{"error":"reload failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RemoveChannel removes a channel by index and hot-reloads.
func (s *Service) RemoveChannel(w http.ResponseWriter, r *http.Request) {
	idxStr := r.PathValue("index")
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx >= len(s.cfg.Channels) {
		http.Error(w, `{"error":"invalid channel index"}`, http.StatusBadRequest)
		return
	}

	s.cfg.Channels = append(s.cfg.Channels[:idx], s.cfg.Channels[idx+1:]...)

	if s.configPath != "" {
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			s.logger.Error("failed to save config", "err", err)
		}
	}

	if err := s.ReloadChannels(r.Context()); err != nil {
		s.logger.Error("failed to reload channels", "err", err)
		http.Error(w, fmt.Sprintf(`{"error":"reload failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ReloadChannels stops existing channels and starts new ones from config.
func (s *Service) ReloadChannels(ctx context.Context) error {
	if s.router != nil {
		_ = s.router.Stop(ctx)
	}

	if len(s.cfg.Channels) == 0 {
		s.router = nil
		return nil
	}

	router := channel.NewRouter(s.engine, s.logger)
	for _, chCfg := range s.cfg.Channels {
		ch, err := channel.New(chCfg, s.mux)
		if err != nil {
			s.logger.Error("init channel failed", "type", chCfg.Type, "err", err)
			continue
		}
		if wh, ok := ch.(interface {
			Path() string
			HandleIncoming(http.ResponseWriter, *http.Request)
		}); ok {
			s.mux.HandleFunc("POST "+wh.Path(), wh.HandleIncoming)
		}
		router.Add(ch)
	}

	if err := router.Start(ctx); err != nil {
		return fmt.Errorf("start channel router: %w", err)
	}
	s.router = router
	return nil
}

// ListRuntimes returns all registered agent runtimes.
func (s *Service) ListRuntimes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runtime.List())
}
