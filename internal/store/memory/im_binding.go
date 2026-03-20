package memory

import (
	"context"
	"fmt"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Binding methods ---

func (s *memoryStore) CreateIMBinding(_ context.Context, binding *model.IMBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for unique platform + platform_user_id constraint.
	for _, b := range s.imBindings {
		if b.Platform == binding.Platform && b.PlatformUserID == binding.PlatformUserID {
			return fmt.Errorf("im binding for %s/%s already exists", binding.Platform, binding.PlatformUserID)
		}
	}
	s.imBindings[binding.ID] = binding
	return nil
}

func (s *memoryStore) GetIMBindingByPlatform(_ context.Context, platform, platformUserID string) (*model.IMBinding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, b := range s.imBindings {
		if b.Platform == platform && b.PlatformUserID == platformUserID {
			return b, nil
		}
	}
	return nil, fmt.Errorf("im binding not found")
}

func (s *memoryStore) ListIMBindingsByUser(_ context.Context, userID string) ([]*model.IMBinding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.IMBinding
	for _, b := range s.imBindings {
		if b.UserID == userID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (s *memoryStore) DeleteIMBinding(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.imBindings, id)
	return nil
}

func (s *memoryStore) CreateBindingCode(_ context.Context, code *model.BindingCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindingCodes[code.Code] = code
	return nil
}

func (s *memoryStore) GetBindingCode(_ context.Context, code string) (*model.BindingCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bc, ok := s.bindingCodes[code]
	if !ok {
		return nil, fmt.Errorf("binding code not found or expired")
	}
	if bc.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("binding code not found or expired")
	}
	return bc, nil
}

func (s *memoryStore) DeleteBindingCode(_ context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bindingCodes, code)
	return nil
}
