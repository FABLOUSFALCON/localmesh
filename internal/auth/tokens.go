// Package auth provides authentication and authorization for LocalMesh.
// Uses PASETO tokens (safer than JWT) and zone-based access control.
package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	paseto "github.com/o1egl/paseto/v2"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidZone        = errors.New("zone access denied")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrMaxSessionsReached = errors.New("maximum sessions reached")
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	TokenID   string    `json:"jti"`
	Subject   string    `json:"sub"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
	NotBefore time.Time `json:"nbf"`
	TokenType TokenType `json:"type"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Zone      string    `json:"zone"`
	Zones     []string  `json:"zones"`
	SessionID string    `json:"session_id"`
	NodeID    string    `json:"node_id,omitempty"`
	ServiceID string    `json:"service_id,omitempty"`
}

type TokenEngine struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	paseto     *paseto.V2
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	mu         sync.RWMutex
}

type TokenEngineConfig struct {
	KeyPath    string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

func DefaultTokenEngineConfig() TokenEngineConfig {
	return TokenEngineConfig{
		KeyPath:    "./data/keys",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		Issuer:     "localmesh",
	}
}

func NewTokenEngine(cfg TokenEngineConfig) (*TokenEngine, error) {
	engine := &TokenEngine{
		paseto:     paseto.NewV2(),
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		issuer:     cfg.Issuer,
	}
	if err := engine.loadOrGenerateKeys(cfg.KeyPath); err != nil {
		return nil, fmt.Errorf("loading keys: %w", err)
	}
	return engine, nil
}

func (e *TokenEngine) loadOrGenerateKeys(keyPath string) error {
	if err := os.MkdirAll(keyPath, 0700); err != nil {
		return fmt.Errorf("creating key directory: %w", err)
	}

	privateKeyPath := filepath.Join(keyPath, "private.key")
	publicKeyPath := filepath.Join(keyPath, "public.key")

	if privData, err := os.ReadFile(privateKeyPath); err == nil {
		privBytes, err := hex.DecodeString(string(privData))
		if err != nil {
			return fmt.Errorf("decoding private key: %w", err)
		}
		e.privateKey = ed25519.PrivateKey(privBytes)
		e.publicKey = e.privateKey.Public().(ed25519.PublicKey)
		return nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating keys: %w", err)
	}

	e.privateKey = priv
	e.publicKey = pub

	if err := os.WriteFile(privateKeyPath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		return fmt.Errorf("saving private key: %w", err)
	}
	if err := os.WriteFile(publicKeyPath, []byte(hex.EncodeToString(pub)), 0644); err != nil {
		return fmt.Errorf("saving public key: %w", err)
	}

	return nil
}

func (e *TokenEngine) GenerateAccessToken(userID, username, role, zone string, zones []string, sessionID string) (string, *Claims, error) {
	now := time.Now()
	claims := &Claims{
		TokenID:   uuid.New().String(),
		Subject:   userID,
		IssuedAt:  now,
		ExpiresAt: now.Add(e.accessTTL),
		NotBefore: now,
		TokenType: TokenTypeAccess,
		Username:  username,
		Role:      role,
		Zone:      zone,
		Zones:     zones,
		SessionID: sessionID,
	}
	token, err := e.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, claims, nil
}

func (e *TokenEngine) GenerateRefreshToken(userID, sessionID string) (string, *Claims, error) {
	now := time.Now()
	claims := &Claims{
		TokenID:   uuid.New().String(),
		Subject:   userID,
		IssuedAt:  now,
		ExpiresAt: now.Add(e.refreshTTL),
		NotBefore: now,
		TokenType: TokenTypeRefresh,
		SessionID: sessionID,
	}
	token, err := e.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, claims, nil
}

func (e *TokenEngine) sign(claims *Claims) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	footer := map[string]string{
		"iss": e.issuer,
		"kid": hex.EncodeToString(e.publicKey[:8]),
	}
	token, err := e.paseto.Sign(e.privateKey, claims, footer)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return token, nil
}

func (e *TokenEngine) Verify(token string) (*Claims, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var claims Claims
	var footer map[string]string

	if err := e.paseto.Verify(token, e.publicKey, &claims, &footer); err != nil {
		return nil, ErrInvalidToken
	}
	if time.Now().After(claims.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if time.Now().Before(claims.NotBefore) {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func (e *TokenEngine) VerifyAccessToken(token string) (*Claims, error) {
	claims, err := e.Verify(token)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (e *TokenEngine) VerifyRefreshToken(token string) (*Claims, error) {
	claims, err := e.Verify(token)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (e *TokenEngine) PublicKey() ed25519.PublicKey {
	return e.publicKey
}

func (e *TokenEngine) PublicKeyHex() string {
	return hex.EncodeToString(e.publicKey)
}
