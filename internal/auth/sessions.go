package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/FABLOUSFALCON/localmesh/internal/storage"
	"github.com/google/uuid"
)

type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Zone         string    `json:"zone"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastActivity time.Time `json:"last_activity"`
	RefreshToken string    `json:"-"`
}

type SessionStore struct {
	badger      *storage.BadgerStore
	maxSessions int
	sessionTTL  time.Duration
}

func NewSessionStore(badger *storage.BadgerStore, maxSessions int, sessionTTL time.Duration) *SessionStore {
	return &SessionStore{
		badger:      badger,
		maxSessions: maxSessions,
		sessionTTL:  sessionTTL,
	}
}

func (s *SessionStore) Create(userID, username, zone, ipAddress, userAgent string) (*Session, error) {
	existing, err := s.GetByUserID(userID)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}

	if len(existing) >= s.maxSessions {
		oldest := existing[0]
		for _, sess := range existing[1:] {
			if sess.CreatedAt.Before(oldest.CreatedAt) {
				oldest = sess
			}
		}
		s.Delete(oldest.ID)
	}

	now := time.Now()
	session := &Session{
		ID:           uuid.New().String(),
		UserID:       userID,
		Username:     username,
		Zone:         zone,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.sessionTTL),
		LastActivity: now,
	}

	if err := s.save(session); err != nil {
		return nil, err
	}

	if err := s.addToUserIndex(userID, session.ID); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *SessionStore) Get(sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := s.badger.Get(key)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		s.Delete(sessionID)
		return nil, ErrSessionNotFound
	}

	return &session, nil
}

func (s *SessionStore) GetByUserID(userID string) ([]*Session, error) {
	indexKey := fmt.Sprintf("user_sessions:%s", userID)
	data, err := s.badger.Get(indexKey)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	if err := json.Unmarshal(data, &sessionIDs); err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, id := range sessionIDs {
		session, err := s.Get(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *SessionStore) Extend(sessionID string) error {
	session, err := s.Get(sessionID)
	if err != nil {
		return err
	}
	session.ExpiresAt = time.Now().Add(s.sessionTTL)
	session.LastActivity = time.Now()
	return s.save(session)
}

func (s *SessionStore) Delete(sessionID string) error {
	session, err := s.Get(sessionID)
	if err == nil {
		s.removeFromUserIndex(session.UserID, sessionID)
	}
	key := fmt.Sprintf("session:%s", sessionID)
	return s.badger.Delete(key)
}

func (s *SessionStore) DeleteAllForUser(userID string) error {
	sessions, err := s.GetByUserID(userID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		s.badger.Delete(fmt.Sprintf("session:%s", session.ID))
	}
	indexKey := fmt.Sprintf("user_sessions:%s", userID)
	return s.badger.Delete(indexKey)
}

func (s *SessionStore) save(session *Session) error {
	key := fmt.Sprintf("session:%s", session.ID)
	ttl := time.Until(session.ExpiresAt)
	if ttl < 0 {
		ttl = 0
	}
	return s.badger.SetJSON(key, session, ttl)
}

func (s *SessionStore) addToUserIndex(userID, sessionID string) error {
	indexKey := fmt.Sprintf("user_sessions:%s", userID)

	var sessionIDs []string
	data, err := s.badger.Get(indexKey)
	if err == nil {
		json.Unmarshal(data, &sessionIDs)
	}

	sessionIDs = append(sessionIDs, sessionID)
	return s.badger.SetJSON(indexKey, sessionIDs, s.sessionTTL)
}

func (s *SessionStore) removeFromUserIndex(userID, sessionID string) error {
	indexKey := fmt.Sprintf("user_sessions:%s", userID)

	var sessionIDs []string
	data, err := s.badger.Get(indexKey)
	if err != nil {
		return nil
	}
	json.Unmarshal(data, &sessionIDs)

	var filtered []string
	for _, id := range sessionIDs {
		if id != sessionID {
			filtered = append(filtered, id)
		}
	}

	if len(filtered) == 0 {
		return s.badger.Delete(indexKey)
	}
	return s.badger.SetJSON(indexKey, filtered, s.sessionTTL)
}
