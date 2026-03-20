package memory

import (
	"context"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Session methods ---

func (s *memoryStore) CreateIMSession(_ context.Context, session *model.IMSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.imSessions[session.ID]; exists {
		return fmt.Errorf("im session %s already exists", session.ID)
	}
	s.imSessions[session.ID] = session
	return nil
}

func (s *memoryStore) GetIMSession(_ context.Context, id string) (*model.IMSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.imSessions[id]
	if !ok {
		return nil, fmt.Errorf("im session %s not found", id)
	}
	return session, nil
}

func (s *memoryStore) GetIMSessionByChat(_ context.Context, platform, chatID string) (*model.IMSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sess := range s.imSessions {
		if sess.Platform == platform && sess.PlatformChatID == chatID && sess.Active {
			return sess, nil
		}
	}
	return nil, fmt.Errorf("im session not found for %s/%s", platform, chatID)
}

func (s *memoryStore) ListIMSessionsByUser(_ context.Context, userID string) ([]*model.IMSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.IMSession
	for _, sess := range s.imSessions {
		if sess.UserID == userID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateIMSession(_ context.Context, session *model.IMSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.imSessions[session.ID]; !exists {
		return fmt.Errorf("im session %s not found", session.ID)
	}
	s.imSessions[session.ID] = session
	return nil
}

func (s *memoryStore) DeleteIMSession(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.imSessions, id)
	return nil
}
