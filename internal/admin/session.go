package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	sessionCookie = "belfast_admin"
	sessionTTL    = 12 * time.Hour
	// how long a wrong password waits, so guesses come slowly
	loginFailDelay = time.Second
)

// sessions are the logged in browsers, by cookie token. They don't survive a
// restart.
type sessions struct {
	mu      sync.Mutex
	expires map[string]time.Time
}

func (sessions *sessions) create() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.expires == nil {
		sessions.expires = make(map[string]time.Time)
	}
	now := time.Now()
	for token, expires := range sessions.expires {
		if now.After(expires) {
			delete(sessions.expires, token)
		}
	}
	sessions.expires[token] = now.Add(sessionTTL)
	return token, nil
}

func (sessions *sessions) valid(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	expires, ok := sessions.expires[cookie.Value]
	return ok && time.Now().Before(expires)
}

func (sessions *sessions) remove(r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		sessions.mu.Lock()
		delete(sessions.expires, cookie.Value)
		sessions.mu.Unlock()
	}
}

// handleLogin opens a session for the right password, compared in constant
// time (hashed first so the length doesn't leak either).
func (server *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	given, want := sha256.Sum256([]byte(body.Password)), sha256.Sum256([]byte(server.Password))
	if subtle.ConstantTimeCompare(given[:], want[:]) != 1 {
		time.Sleep(loginFailDelay)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong password"})
		return
	}
	token, err := server.sessions.create()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil,
	})
	writeJSON(w, http.StatusOK, struct{}{})
}

func (server *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	server.sessions.remove(r)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, struct{}{})
}
