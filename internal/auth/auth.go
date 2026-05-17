package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSetupExists        = errors.New("admin already exists")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrValidation         = errors.New("validation error")
)

type Admin struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	TokenVersion int    `json:"-"`
}

type Store interface {
	HasAdmin(context.Context) (bool, error)
	CreateAdmin(context.Context, string, string) (Admin, error)
	GetAdminByUsername(context.Context, string) (Admin, error)
	GetAdminByID(context.Context, int64) (Admin, error)
	UpdateAdmin(context.Context, int64, string, string) (Admin, error)
	GetOrCreateSetting(context.Context, string, func() (string, error)) (string, error)
}

type Service struct {
	store  Store
	secret []byte
	ttl    time.Duration
}

type Claims struct {
	Subject      string `json:"sub"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
}

func NewService(ctx context.Context, store Store, configuredSecret string, ttl time.Duration) (*Service, error) {
	secret := configuredSecret
	if secret == "" {
		var err error
		secret, err = store.GetOrCreateSetting(ctx, "jwt_secret", randomSecret)
		if err != nil {
			return nil, err
		}
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{store: store, secret: []byte(secret), ttl: ttl}, nil
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	hasAdmin, err := s.store.HasAdmin(ctx)
	if err != nil {
		return false, err
	}
	return !hasAdmin, nil
}

func (s *Service) Setup(ctx context.Context, username, password string) (Admin, string, error) {
	required, err := s.SetupRequired(ctx)
	if err != nil {
		return Admin{}, "", err
	}
	if !required {
		return Admin{}, "", ErrSetupExists
	}
	username, password, err = validateCredentials(username, password)
	if err != nil {
		return Admin{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Admin{}, "", err
	}
	admin, err := s.store.CreateAdmin(ctx, username, string(hash))
	if err != nil {
		return Admin{}, "", err
	}
	token, err := s.sign(admin)
	return admin, token, err
}

func (s *Service) Login(ctx context.Context, username, password string) (Admin, string, error) {
	admin, err := s.store.GetAdminByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return Admin{}, "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return Admin{}, "", ErrInvalidCredentials
	}
	token, err := s.sign(admin)
	return admin, token, err
}

func (s *Service) Authenticate(ctx context.Context, token string) (Admin, error) {
	claims, err := s.verify(token)
	if err != nil {
		return Admin{}, ErrUnauthorized
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return Admin{}, ErrUnauthorized
	}
	admin, err := s.store.GetAdminByID(ctx, id)
	if err != nil || admin.TokenVersion != claims.TokenVersion {
		return Admin{}, ErrUnauthorized
	}
	return admin, nil
}

func (s *Service) ChangeCredentials(ctx context.Context, adminID int64, currentPassword, newUsername, newPassword string) (Admin, error) {
	admin, err := s.store.GetAdminByID(ctx, adminID)
	if err != nil {
		return Admin{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(currentPassword)) != nil {
		return Admin{}, ErrInvalidCredentials
	}
	// Allow partial updates: if a field is empty, keep the current value
	if newUsername == "" {
		newUsername = admin.Username
	}
	if len(newUsername) < 3 || len(newUsername) > 64 {
		return Admin{}, fmt.Errorf("%w: username must be 3-64 characters", ErrValidation)
	}
	var hashToStore string
	if newPassword == "" {
		hashToStore = admin.PasswordHash
	} else {
		if len(newPassword) < 8 || len(newPassword) > 128 {
			return Admin{}, fmt.Errorf("%w: password must be 8-128 characters", ErrValidation)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return Admin{}, err
		}
		hashToStore = string(hash)
	}
	return s.store.UpdateAdmin(ctx, adminID, newUsername, hashToStore)
}

func (s *Service) sign(admin Admin) (string, error) {
	now := time.Now()
	claims := Claims{Subject: strconv.FormatInt(admin.ID, 10), Username: admin.Username, TokenVersion: admin.TokenVersion, IssuedAt: now.Unix(), ExpiresAt: now.Add(s.ttl).Unix()}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := header + "." + body
	sig := hmac.New(sha256.New, s.secret)
	_, _ = sig.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig.Sum(nil)), nil
}

func (s *Service) verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrUnauthorized
	}
	unsigned := parts[0] + "." + parts[1]
	sig := hmac.New(sha256.New, s.secret)
	_, _ = sig.Write([]byte(unsigned))
	expected := sig.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(actual, expected) {
		return Claims{}, ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrUnauthorized
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrUnauthorized
	}
	if time.Now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrUnauthorized
	}
	return claims, nil
}

func validateCredentials(username, password string) (string, string, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return "", "", fmt.Errorf("%w: username must be 3-64 characters", ErrValidation)
	}
	if len(password) < 8 || len(password) > 128 {
		return "", "", fmt.Errorf("%w: password must be 8-128 characters", ErrValidation)
	}
	return username, password, nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
