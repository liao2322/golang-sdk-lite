package webui

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/halalcloud/golang-sdk-lite/halalcloud/config"
)

const sessionCookieName = "halal_session"

type sessionData struct {
	ID        string
	Store     config.ConfigStore
	CreatedAt time.Time
	LastSeen  time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*sessionData
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[string]*sessionData),
	}
}

func (s *sessionStore) create() (*sessionData, error) {
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	session := &sessionData{
		ID:        id,
		Store:     config.NewMapConfigStore(),
		CreatedAt: now,
		LastSeen:  now,
	}
	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()
	return session, nil
}

func (s *sessionStore) get(id string) (*sessionData, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	s.mu.Lock()
	session.LastSeen = time.Now()
	s.mu.Unlock()
	return session, true
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *sessionStore) fromRequest(r *http.Request) (*sessionData, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	return s.get(cookie.Value)
}

func (s *sessionStore) writeCookie(w http.ResponseWriter, session *sessionData) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7,
	})
}

func (s *sessionStore) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
