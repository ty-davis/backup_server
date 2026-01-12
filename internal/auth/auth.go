package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

type Session struct {
	UserID   int
	Username string
	GroupIDs []int
	Expires  time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
	}
	go store.cleanupExpired()
	return store
}

func (s *SessionStore) Create(userID int, username string, groupIDs []int) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	sessionID := base64.URLEncoding.EncodeToString(token)

	s.mu.Lock()
	s.sessions[sessionID] = &Session{
		UserID:   userID,
		Username: username,
		GroupIDs: groupIDs,
		Expires:  time.Now().Add(24 * time.Hour),
	}
	s.mu.Unlock()

	return sessionID, nil
}

func (s *SessionStore) Get(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists || session.Expires.Before(time.Now()) {
		return nil, false
	}

	return session, true
}

func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

func (s *SessionStore) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, session := range s.sessions {
			if session.Expires.Before(now) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

func GetSessionFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
		SameSite: http.SameSiteStrictMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
