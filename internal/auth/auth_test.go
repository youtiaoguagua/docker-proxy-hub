package auth

import (
	"context"
	"testing"
)

type mockStore struct {
	admins         map[string]Admin
	adminByID      map[int64]Admin
	nextID         int64
	settings       map[string]string
	hasAdminResult bool
	hasAdminErr    error
}

func newMockStore() *mockStore {
	return &mockStore{
		admins:    make(map[string]Admin),
		adminByID: make(map[int64]Admin),
		nextID:    1,
		settings:  make(map[string]string),
	}
}

func (m *mockStore) HasAdmin(_ context.Context) (bool, error) {
	if m.hasAdminErr != nil {
		return false, m.hasAdminErr
	}
	return m.hasAdminResult, nil
}

func (m *mockStore) CreateAdmin(_ context.Context, username, passwordHash string) (Admin, error) {
	id := m.nextID
	m.nextID++
	admin := Admin{ID: id, Username: username, PasswordHash: passwordHash, TokenVersion: 1}
	m.admins[username] = admin
	m.adminByID[id] = admin
	m.hasAdminResult = true
	return admin, nil
}

func (m *mockStore) GetAdminByUsername(_ context.Context, username string) (Admin, error) {
	admin, ok := m.admins[username]
	if !ok {
		return Admin{}, errNotFound
	}
	return admin, nil
}

func (m *mockStore) GetAdminByID(_ context.Context, id int64) (Admin, error) {
	admin, ok := m.adminByID[id]
	if !ok {
		return Admin{}, errNotFound
	}
	return admin, nil
}

func (m *mockStore) UpdateAdmin(_ context.Context, id int64, username, passwordHash string) (Admin, error) {
	admin, ok := m.adminByID[id]
	if !ok {
		return Admin{}, errNotFound
	}
	oldUsername := admin.Username
	admin.Username = username
	admin.PasswordHash = passwordHash
	admin.TokenVersion++
	m.adminByID[id] = admin
	delete(m.admins, oldUsername)
	m.admins[username] = admin
	return admin, nil
}

func (m *mockStore) GetOrCreateSetting(_ context.Context, key string, generator func() (string, error)) (string, error) {
	if val, ok := m.settings[key]; ok {
		return val, nil
	}
	val, err := generator()
	if err != nil {
		return "", err
	}
	m.settings[key] = val
	return val, nil
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

func TestSetupRequiredTrue(t *testing.T) {
	store := newMockStore()
	svc, err := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	required, err := svc.SetupRequired(context.Background())
	if err != nil {
		t.Fatalf("SetupRequired: %v", err)
	}
	if !required {
		t.Error("expected setup required when no admin exists")
	}
}

func TestSetupRequiredFalse(t *testing.T) {
	store := newMockStore()
	store.hasAdminResult = true
	svc, err := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	required, err := svc.SetupRequired(context.Background())
	if err != nil {
		t.Fatalf("SetupRequired: %v", err)
	}
	if required {
		t.Error("expected setup not required when admin exists")
	}
}

func TestSetupCreatesAdmin(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	admin, token, err := svc.Setup(context.Background(), "admin", "password123")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if admin.Username != "admin" {
		t.Errorf("expected username admin, got %s", admin.Username)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestSetupFailsWhenAdminExists(t *testing.T) {
	store := newMockStore()
	store.hasAdminResult = true
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	_, _, err := svc.Setup(context.Background(), "admin", "password123")
	if !IsSetupExists(err) {
		t.Errorf("expected ErrSetupExists, got %v", err)
	}
}

func TestSetupValidation(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	cases := []struct {
		name     string
		username string
		password string
	}{
		{"short username", "ab", "password123"},
		{"short password", "admin", "pass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.Setup(context.Background(), tc.username, tc.password)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestLoginSuccess(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	svc.Setup(context.Background(), "admin", "password123")
	admin, token, err := svc.Login(context.Background(), "admin", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if admin.Username != "admin" {
		t.Errorf("expected admin, got %s", admin.Username)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	svc.Setup(context.Background(), "admin", "password123")
	_, _, err := svc.Login(context.Background(), "admin", "wrongpassword")
	if !IsInvalidCredentials(err) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginWrongUsername(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	svc.Setup(context.Background(), "admin", "password123")
	_, _, err := svc.Login(context.Background(), "nonexistent", "password123")
	if !IsInvalidCredentials(err) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateWithValidToken(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	_, token, _ := svc.Setup(context.Background(), "admin", "password123")
	admin, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if admin.Username != "admin" {
		t.Errorf("expected admin, got %s", admin.Username)
	}
}

func TestAuthenticateWithInvalidToken(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	_, err := svc.Authenticate(context.Background(), "invalid.token.here")
	if !IsUnauthorized(err) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticateWithDifferentSecret(t *testing.T) {
	store := newMockStore()
	svc1, _ := NewService(context.Background(), store, "secret-one-long-enough", 0)
	svc2, _ := NewService(context.Background(), store, "secret-two-long-enough", 0)
	_, token, _ := svc1.Setup(context.Background(), "admin", "password123")
	_, err := svc2.Authenticate(context.Background(), token)
	if !IsUnauthorized(err) {
		t.Errorf("expected ErrUnauthorized for token signed with different secret, got %v", err)
	}
}

func TestChangeCredentialsInvalidatesOldToken(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	_, token, _ := svc.Setup(context.Background(), "admin", "password123")
	_, err := svc.ChangeCredentials(context.Background(), 1, "password123", "newadmin", "newpassword123")
	if err != nil {
		t.Fatalf("ChangeCredentials: %v", err)
	}
	_, err = svc.Authenticate(context.Background(), token)
	if !IsUnauthorized(err) {
		t.Error("expected old token to be invalidated after credential change")
	}
}

func TestChangeCredentialsWrongCurrentPassword(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	svc.Setup(context.Background(), "admin", "password123")
	_, err := svc.ChangeCredentials(context.Background(), 1, "wrongpassword", "newadmin", "newpassword123")
	if !IsInvalidCredentials(err) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestDefaultTTLOverride(t *testing.T) {
	store := newMockStore()
	svc, err := NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc.ttl != 24*60*60*1e9 {
		t.Errorf("expected default TTL 24h, got %v", svc.ttl)
	}
}

func IsSetupExists(err error) bool    { return err == ErrSetupExists }
func IsInvalidCredentials(err error) bool { return err == ErrInvalidCredentials }
func IsUnauthorized(err error) bool   { return err == ErrUnauthorized }