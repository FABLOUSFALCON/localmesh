package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/FABLOUSFALCON/localmesh/internal/storage"
)

type Service struct {
	tokens   *TokenEngine
	users    *UserStore
	sessions *SessionStore
	zones    *ZoneManager
	logger   *slog.Logger
}

type ServiceConfig struct {
	KeyPath         string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	SessionTTL      time.Duration
	MaxSessions     int
	SQLite          *storage.SQLiteStore
	Badger          *storage.BadgerStore
	Logger          *slog.Logger
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		KeyPath:         "./data/keys",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		SessionTTL:      7 * 24 * time.Hour,
		MaxSessions:     5,
	}
}

func NewService(cfg ServiceConfig) (*Service, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	tokens, err := NewTokenEngine(TokenEngineConfig{
		KeyPath:    cfg.KeyPath,
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
		Issuer:     "localmesh",
	})
	if err != nil {
		return nil, fmt.Errorf("initializing token engine: %w", err)
	}

	return &Service{
		tokens:   tokens,
		users:    NewUserStore(cfg.SQLite),
		sessions: NewSessionStore(cfg.Badger, cfg.MaxSessions, cfg.SessionTTL),
		zones:    NewZoneManager(),
		logger:   logger,
	}, nil
}

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	IPAddress string `json:"-"`
	UserAgent string `json:"-"`
}

type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         *User     `json:"user"`
	SessionID    string    `json:"session_id"`
}

func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	user, err := s.users.GetByUsername(ctx, req.Username)
	if err != nil {
		s.logger.Warn("login failed: user not found", "username", req.Username, "ip", req.IPAddress)
		return nil, ErrInvalidCredentials
	}

	if !VerifyPassword(req.Password, user.PasswordHash) {
		s.logger.Warn("login failed: invalid password", "username", req.Username, "ip", req.IPAddress)
		return nil, ErrInvalidCredentials
	}

	session, err := s.sessions.Create(user.ID, user.Username, user.Zone, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	accessToken, claims, err := s.tokens.GenerateAccessToken(
		user.ID, user.Username, user.Role, user.Zone, user.Zones, session.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, _, err := s.tokens.GenerateRefreshToken(user.ID, session.ID)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	s.users.UpdateLastSeen(ctx, user.ID)

	s.logger.Info("user logged in",
		"user_id", user.ID, "username", user.Username,
		"session_id", session.ID, "ip", req.IPAddress,
	)

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    claims.ExpiresAt,
		User:         user,
		SessionID:    session.ID,
	}, nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (s *Service) Refresh(ctx context.Context, req *RefreshRequest) (*RefreshResponse, error) {
	claims, err := s.tokens.VerifyRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.Get(claims.SessionID)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	user, err := s.users.GetByID(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}

	accessToken, newClaims, err := s.tokens.GenerateAccessToken(
		user.ID, user.Username, user.Role, user.Zone, user.Zones, session.ID,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, _, err := s.tokens.GenerateRefreshToken(user.ID, session.ID)
	if err != nil {
		return nil, err
	}

	s.sessions.Extend(session.ID)

	return &RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    newClaims.ExpiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.sessions.Delete(sessionID)
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.sessions.DeleteAllForUser(userID)
}

func (s *Service) ValidateToken(token string) (*Claims, error) {
	return s.tokens.VerifyAccessToken(token)
}

func (s *Service) CheckZoneAccess(ctx context.Context, claims *Claims, zone, clientIP string) error {
	return s.zones.CheckAccess(ctx, claims, zone, clientIP)
}

func (s *Service) CreateUser(ctx context.Context, user *User, password string) error {
	return s.users.Create(ctx, user, password)
}

func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *Service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return s.users.GetByUsername(ctx, username)
}

func (s *Service) RegisterZone(zone *Zone) error {
	return s.zones.RegisterZone(zone)
}

func (s *Service) SetZonePolicy(policy *ZonePolicy) {
	s.zones.SetPolicy(policy)
}

func (s *Service) GetZones() []*Zone {
	return s.zones.ListZones()
}

func (s *Service) GetSession(sessionID string) (*Session, error) {
	return s.sessions.Get(sessionID)
}

func (s *Service) GetUserSessions(userID string) ([]*Session, error) {
	return s.sessions.GetByUserID(userID)
}

func (s *Service) Tokens() *TokenEngine {
	return s.tokens
}

func (s *Service) Users() *UserStore {
	return s.users
}

func (s *Service) Zones() *ZoneManager {
	return s.zones
}
