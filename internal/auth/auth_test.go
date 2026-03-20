package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go.zoe.im/agentbox/internal/model"
)

// ---------------------------------------------------------------------------
// Mock store -- implements only the Store methods auth.go calls.
// Lives in package auth so it accesses unexported helpers directly.
// ---------------------------------------------------------------------------

var errNotFound = errors.New("not found")

type mockStore struct {
	users       map[string]*model.User
	emailIndex  map[string]*model.User
	apiKeyIndex map[string]*model.User
}

func newMockStore() *mockStore {
	return &mockStore{
		users:       make(map[string]*model.User),
		emailIndex:  make(map[string]*model.User),
		apiKeyIndex: make(map[string]*model.User),
	}
}

func (m *mockStore) CreateUser(_ context.Context, u *model.User) error {
	if _, ok := m.emailIndex[u.Email]; ok {
		return errors.New("duplicate email")
	}
	m.users[u.ID] = u
	m.emailIndex[u.Email] = u
	if u.APIKey != "" {
		m.apiKeyIndex[u.APIKey] = u
	}
	return nil
}

func (m *mockStore) GetUser(_ context.Context, id string) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (m *mockStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := m.emailIndex[email]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (m *mockStore) GetUserByAPIKey(_ context.Context, hash string) (*model.User, error) {
	u, ok := m.apiKeyIndex[hash]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (m *mockStore) UpdateUser(_ context.Context, u *model.User) error {
	old, ok := m.users[u.ID]
	if !ok {
		return errNotFound
	}
	if old.APIKey != "" {
		delete(m.apiKeyIndex, old.APIKey)
	}
	m.users[u.ID] = u
	m.emailIndex[u.Email] = u
	if u.APIKey != "" {
		m.apiKeyIndex[u.APIKey] = u
	}
	return nil
}

// --- Stubs for the remaining store.Store methods (unused by auth) ---

func (m *mockStore) CreateRun(context.Context, *model.Run) error   { return nil }
func (m *mockStore) GetRun(context.Context, string) (*model.Run, error) {
	return nil, errNotFound
}
func (m *mockStore) UpdateRun(context.Context, *model.Run) error                  { return nil }
func (m *mockStore) ListRuns(context.Context, int, int) ([]*model.Run, error)     { return nil, nil }
func (m *mockStore) DeleteRun(context.Context, string) error                      { return nil }
func (m *mockStore) CreateIntegration(context.Context, *model.Integration) error  { return nil }
func (m *mockStore) GetIntegration(context.Context, string) (*model.Integration, error) {
	return nil, errNotFound
}
func (m *mockStore) ListIntegrations(context.Context, string) ([]*model.Integration, error) {
	return nil, nil
}
func (m *mockStore) UpdateIntegration(context.Context, *model.Integration) error { return nil }
func (m *mockStore) DeleteIntegration(context.Context, string) error             { return nil }
func (m *mockStore) ListAllEnabledIntegrations(context.Context) ([]*model.Integration, error) {
	return nil, nil
}
func (m *mockStore) CreateAgentDNA(context.Context, *model.AgentDNA) error { return nil }
func (m *mockStore) GetAgentDNA(context.Context, string) (*model.AgentDNA, error) {
	return nil, errNotFound
}
func (m *mockStore) GetAgentDNABySlug(context.Context, string) (*model.AgentDNA, error) {
	return nil, errNotFound
}
func (m *mockStore) UpdateAgentDNA(context.Context, *model.AgentDNA) error { return nil }
func (m *mockStore) DeleteAgentDNA(context.Context, string) error          { return nil }
func (m *mockStore) ListAgentDNAs(context.Context, model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	return nil, nil
}
func (m *mockStore) IncrementAgentDNADownloads(context.Context, string) error { return nil }
func (m *mockStore) CreateSubscription(context.Context, *model.Subscription) error { return nil }
func (m *mockStore) GetSubscription(context.Context, string) (*model.Subscription, error) {
	return nil, errNotFound
}
func (m *mockStore) GetActiveSubscription(context.Context, string, string) (*model.Subscription, error) {
	return nil, errNotFound
}
func (m *mockStore) GetSubscriptionByStripeSubID(context.Context, string) (*model.Subscription, error) {
	return nil, errNotFound
}
func (m *mockStore) UpdateSubscription(context.Context, *model.Subscription) error { return nil }
func (m *mockStore) ListSubscriptions(context.Context, string) ([]*model.Subscription, error) {
	return nil, nil
}
func (m *mockStore) CreateUsageRecord(context.Context, *model.UsageRecord) error { return nil }
func (m *mockStore) GetUsageSummary(context.Context, string, string, string) (*model.UsageSummary, error) {
	return nil, errNotFound
}
func (m *mockStore) ListUsageRecords(context.Context, model.BillingListOptions) ([]*model.UsageRecord, error) {
	return nil, nil
}
func (m *mockStore) CreateAuthorPayout(context.Context, *model.AuthorPayout) error { return nil }
func (m *mockStore) GetAuthorPayout(context.Context, string, string) (*model.AuthorPayout, error) {
	return nil, errNotFound
}
func (m *mockStore) ListAuthorPayouts(context.Context, string) ([]*model.AuthorPayout, error) {
	return nil, nil
}
func (m *mockStore) UpsertRunCostBreakdown(context.Context, *model.RunCostBreakdown) error {
	return nil
}
func (m *mockStore) GetRunCostBreakdown(context.Context, string) (*model.RunCostBreakdown, error) {
	return nil, errNotFound
}
func (m *mockStore) UpsertStripeCustomer(context.Context, *model.StripeCustomer) error { return nil }
func (m *mockStore) GetStripeCustomer(context.Context, string) (*model.StripeCustomer, error) {
	return nil, errNotFound
}
func (m *mockStore) GetFreeQuotaUsage(context.Context, string, string, string) (*model.FreeQuotaUsage, error) {
	return nil, errNotFound
}
func (m *mockStore) IncrementFreeQuotaUsage(context.Context, string, string, string, int64) error {
	return nil
}
func (m *mockStore) CreateIMBinding(context.Context, *model.IMBinding) error {
	return nil
}
func (m *mockStore) GetIMBindingByPlatform(context.Context, string, string) (*model.IMBinding, error) {
	return nil, errNotFound
}
func (m *mockStore) ListIMBindingsByUser(context.Context, string) ([]*model.IMBinding, error) {
	return nil, nil
}
func (m *mockStore) DeleteIMBinding(context.Context, string) error { return nil }
func (m *mockStore) CreateBindingCode(context.Context, *model.BindingCode) error {
	return nil
}
func (m *mockStore) GetBindingCode(context.Context, string) (*model.BindingCode, error) {
	return nil, errNotFound
}
func (m *mockStore) DeleteBindingCode(context.Context, string) error { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("with explicit secret", func(t *testing.T) {
		a := New(newMockStore(), "my-secret")
		if a == nil {
			t.Fatal("expected non-nil Auth")
		}
		if string(a.jwtSecret) != "my-secret" {
			t.Fatalf("jwtSecret = %q, want %q", string(a.jwtSecret), "my-secret")
		}
	})

	t.Run("empty secret generates 32 random bytes", func(t *testing.T) {
		a := New(newMockStore(), "")
		if a == nil {
			t.Fatal("expected non-nil Auth")
		}
		if len(a.jwtSecret) != 32 {
			t.Fatalf("jwtSecret length = %d, want 32", len(a.jwtSecret))
		}
	})

	t.Run("two empty-secret instances get different secrets", func(t *testing.T) {
		a1 := New(newMockStore(), "")
		a2 := New(newMockStore(), "")
		if string(a1.jwtSecret) == string(a2.jwtSecret) {
			t.Fatal("expected different random secrets")
		}
	})
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("valid registration", func(t *testing.T) {
		a := New(newMockStore(), "s")
		u, err := a.Register(ctx, "alice@example.com", "strongpass", "Alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Email != "alice@example.com" {
			t.Errorf("email = %q, want alice@example.com", u.Email)
		}
		if u.Name != "Alice" {
			t.Errorf("name = %q, want Alice", u.Name)
		}
		if u.Plan != "free" {
			t.Errorf("plan = %q, want free", u.Plan)
		}
		if u.ID == "" {
			t.Error("expected non-empty ID")
		}
		if u.Password == "strongpass" {
			t.Error("password stored in plaintext")
		}
		if u.CreatedAt.IsZero() {
			t.Error("created_at not set")
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		a := New(newMockStore(), "s")
		if _, err := a.Register(ctx, "bob@example.com", "password1", "Bob"); err != nil {
			t.Fatal(err)
		}
		_, err := a.Register(ctx, "bob@example.com", "password2", "Bob2")
		if !errors.Is(err, ErrUserExists) {
			t.Fatalf("got %v, want ErrUserExists", err)
		}
	})

	t.Run("weak password", func(t *testing.T) {
		a := New(newMockStore(), "s")
		_, err := a.Register(ctx, "c@example.com", "short", "C")
		if !errors.Is(err, ErrWeakPassword) {
			t.Fatalf("got %v, want ErrWeakPassword", err)
		}
	})

	t.Run("password exactly 8 chars is accepted", func(t *testing.T) {
		a := New(newMockStore(), "s")
		_, err := a.Register(ctx, "d@example.com", "12345678", "D")
		if err != nil {
			t.Fatalf("8-char password should succeed: %v", err)
		}
	})

	t.Run("password 7 chars is rejected", func(t *testing.T) {
		a := New(newMockStore(), "s")
		_, err := a.Register(ctx, "e@example.com", "1234567", "E")
		if !errors.Is(err, ErrWeakPassword) {
			t.Fatalf("got %v, want ErrWeakPassword", err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		a := New(newMockStore(), "s")
		_, err := a.Register(ctx, "not-an-email", "password1", "Nobody")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("got %v, want ErrInvalidEmail", err)
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()

	setup := func() *Auth {
		a := New(newMockStore(), "secret")
		if _, err := a.Register(ctx, "user@example.com", "mypassword", "User"); err != nil {
			t.Fatal(err)
		}
		return a
	}

	t.Run("valid login returns token and user", func(t *testing.T) {
		a := setup()
		token, user, err := a.Login(ctx, "user@example.com", "mypassword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
		if user == nil || user.Email != "user@example.com" {
			t.Error("expected user with matching email")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		a := setup()
		_, _, err := a.Login(ctx, "user@example.com", "wrongpassword")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		a := setup()
		_, _, err := a.Login(ctx, "nobody@example.com", "password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestValidateToken(t *testing.T) {
	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		a := New(newMockStore(), "secret")
		reg, _ := a.Register(ctx, "u@example.com", "password1", "U")
		token, _, _ := a.Login(ctx, "u@example.com", "password1")

		user, err := a.ValidateToken(ctx, token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != reg.ID {
			t.Errorf("user.ID = %s, want %s", user.ID, reg.ID)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		a := New(newMockStore(), "secret")
		reg, _ := a.Register(ctx, "u@example.com", "password1", "U")

		claims := jwt.MapClaims{
			"sub": reg.ID,
			"exp": time.Now().Add(-1 * time.Hour).Unix(),
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := tok.SignedString(a.jwtSecret)

		_, err := a.ValidateToken(ctx, tokenStr)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("garbage token string", func(t *testing.T) {
		a := New(newMockStore(), "secret")
		_, err := a.ValidateToken(ctx, "this.is.garbage")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		a := New(newMockStore(), "secret")
		_, err := a.ValidateToken(ctx, "")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("wrong signing method (none)", func(t *testing.T) {
		a := New(newMockStore(), "secret")
		reg, _ := a.Register(ctx, "u@example.com", "password1", "U")

		claims := jwt.MapClaims{
			"sub": reg.ID,
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		tokenStr, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)

		_, err := a.ValidateToken(ctx, tokenStr)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("token signed with different secret", func(t *testing.T) {
		a := New(newMockStore(), "secret-A")
		reg, _ := a.Register(ctx, "u@example.com", "password1", "U")

		claims := jwt.MapClaims{
			"sub": reg.ID,
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := tok.SignedString([]byte("secret-B"))

		_, err := a.ValidateToken(ctx, tokenStr)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})
}

func TestGenerateAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("generates ak_ prefixed key and persists hash", func(t *testing.T) {
		a := New(newMockStore(), "s")
		u, _ := a.Register(ctx, "u@example.com", "password1", "U")

		rawKey, err := a.GenerateAPIKey(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(rawKey, "ak_") {
			t.Errorf("key %q does not start with ak_", rawKey)
		}
		// Raw key = "ak_" + 64 hex chars (32 bytes)
		if len(rawKey) != 3+64 {
			t.Errorf("key length = %d, want %d", len(rawKey), 3+64)
		}

		// Verify stored hash.
		expectedHash := hashAPIKey(rawKey)
		stored, _ := a.store.(*mockStore).GetUser(ctx, u.ID)
		if stored.APIKey != expectedHash {
			t.Errorf("stored hash mismatch")
		}
	})

	t.Run("user not found", func(t *testing.T) {
		a := New(newMockStore(), "s")
		_, err := a.GenerateAPIKey(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for missing user")
		}
	})
}

func TestValidateAPIKey(t *testing.T) {
	ctx := context.Background()
	a := New(newMockStore(), "s")
	u, _ := a.Register(ctx, "u@example.com", "password1", "U")
	rawKey, _ := a.GenerateAPIKey(ctx, u.ID)

	t.Run("valid key returns user", func(t *testing.T) {
		found, err := a.ValidateAPIKey(ctx, rawKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.ID != u.ID {
			t.Errorf("user.ID = %s, want %s", found.ID, u.ID)
		}
	})

	t.Run("invalid key returns error", func(t *testing.T) {
		_, err := a.ValidateAPIKey(ctx, "ak_bogus")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})
}

func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"a@b.c", true},
		{"user@sub.domain.com", true},
		{"plus+tag@example.com", true},
		{"", false},
		{"noatsign", false},
		{"@example.com", false},   // nothing before @
		{"user@", false},          // empty domain
		{"user@nodot", false},     // no dot in domain
		{"@", false},              // just @
		{"user@.com", true},       // technically has dot in domain part
	}
	for _, tc := range cases {
		t.Run(tc.email, func(t *testing.T) {
			got := isValidEmail(tc.email)
			if got != tc.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}
}
