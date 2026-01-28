package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/FABLOUSFALCON/localmesh/internal/storage"
	"github.com/google/uuid"
)

const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

type User struct {
	ID           string            `json:"id"`
	Username     string            `json:"username"`
	DisplayName  string            `json:"display_name"`
	Email        string            `json:"email,omitempty"`
	Role         string            `json:"role"`
	Zone         string            `json:"zone"`
	Zones        []string          `json:"zones"`
	PasswordHash string            `json:"-"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	LastSeenAt   *time.Time        `json:"last_seen_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type UserStore struct {
	sqlite *storage.SQLiteStore
}

func NewUserStore(sqlite *storage.SQLiteStore) *UserStore {
	return &UserStore{sqlite: sqlite}
}

func (s *UserStore) Create(ctx context.Context, user *User, password string) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	user.PasswordHash = hash

	_, err = s.sqlite.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, role, zone, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.DisplayName, user.Email, user.Role, user.Zone, user.PasswordHash, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}
	return nil
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	row := s.sqlite.QueryRow(ctx, `
		SELECT id, username, display_name, email, role, zone, password_hash, created_at, updated_at, last_seen_at
		FROM users WHERE id = ?
	`, id)
	return s.scanUser(row)
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	row := s.sqlite.QueryRow(ctx, `
		SELECT id, username, display_name, email, role, zone, password_hash, created_at, updated_at, last_seen_at
		FROM users WHERE username = ?
	`, username)
	return s.scanUser(row)
}

func (s *UserStore) scanUser(row *sql.Row) (*User, error) {
	var user User
	var email sql.NullString
	var lastSeenAt sql.NullTime

	err := row.Scan(
		&user.ID, &user.Username, &user.DisplayName, &email, &user.Role,
		&user.Zone, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt, &lastSeenAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNoRows
		}
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if lastSeenAt.Valid {
		user.LastSeenAt = &lastSeenAt.Time
	}
	user.Zones = []string{user.Zone}
	return &user, nil
}

func (s *UserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	_, err := s.sqlite.Exec(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, time.Now(), userID)
	return err
}

func (s *UserStore) List(ctx context.Context, limit, offset int) ([]*User, error) {
	rows, err := s.sqlite.Query(ctx, `
		SELECT id, username, display_name, email, role, zone, password_hash, created_at, updated_at, last_seen_at
		FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var email sql.NullString
		var lastSeenAt sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Username, &user.DisplayName, &email, &user.Role,
			&user.Zone, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt, &lastSeenAt,
		)
		if err != nil {
			continue
		}

		if email.Valid {
			user.Email = email.String
		}
		if lastSeenAt.Valid {
			user.LastSeenAt = &lastSeenAt.Time
		}
		user.Zones = []string{user.Zone}
		users = append(users, &user)
	}
	return users, nil
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads, b64Salt, b64Hash), nil
}

func VerifyPassword(password, encodedHash string) bool {
	var memory, t uint32
	var threads uint8
	var salt string

	_, err := fmt.Sscanf(encodedHash, "$argon2id$v=19$m=%d,t=%d,p=%d$%s", &memory, &t, &threads, &salt)
	if err != nil {
		return false
	}

	parts := splitLast(encodedHash, "$")
	if len(parts) != 2 {
		return false
	}
	hashPart := parts[1]

	saltParts := splitLast(parts[0], "$")
	if len(saltParts) != 2 {
		return false
	}
	salt = saltParts[1]

	saltBytes, err := base64.RawStdEncoding.DecodeString(salt)
	if err != nil {
		return false
	}

	hashBytes, err := base64.RawStdEncoding.DecodeString(hashPart)
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), saltBytes, t, memory, threads, uint32(len(hashBytes)))
	return subtle.ConstantTimeCompare(computed, hashBytes) == 1
}

func splitLast(s, sep string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if string(s[i]) == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
