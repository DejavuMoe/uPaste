package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// SessionCookieName is the host-only HttpOnly admin session cookie.
	SessionCookieName = "upaste_admin_session"
	// CSRFHeaderName is required on every state-changing admin request.
	CSRFHeaderName = "X-uPaste-CSRF"

	defaultSessionTTL = 8 * time.Hour
	sessionRandomSize = 32
	sessionRandomLen  = 43
	maxSessions       = 64

	loginBurst    = 5
	loginInterval = 12 * time.Second
	loginIdleTTL  = 10 * time.Minute
	maxLoginIPs   = 4096
)

var (
	ErrUnauthorized = errors.New("admin authentication failed")
	ErrRateLimited  = errors.New("admin authentication rate limited")
	ErrNoSession    = errors.New("admin session not found")
)

// Options controls session lifetime and cookie transport behavior.
type Options struct {
	SessionTTL   time.Duration
	SecureCookie bool
	Now          func() time.Time
}

// Session is the server-side representation of an authenticated admin session.
// The raw session cookie value is never retained.
type Session struct {
	CSRFToken string
	ExpiresAt time.Time
}

type sessionRecord struct {
	csrf      string
	expiresAt time.Time
}

type loginEntry struct {
	limiter *rate.Limiter
	last    time.Time
}

// Manager verifies the single admin token and owns in-memory sessions.
type Manager struct {
	verifier     Verifier
	sessionTTL   time.Duration
	secureCookie bool
	now          func() time.Time

	mu          sync.Mutex
	sessions    map[[sha256.Size]byte]sessionRecord
	loginLimits map[string]*loginEntry
	lastSweep   time.Time
}

func NewManager(verifier Verifier, options Options) *Manager {
	if options.SessionTTL <= 0 {
		options.SessionTTL = defaultSessionTTL
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Manager{
		verifier:     verifier,
		sessionTTL:   options.SessionTTL,
		secureCookie: options.SecureCookie,
		now:          options.Now,
		sessions:     make(map[[sha256.Size]byte]sessionRecord),
		loginLimits:  make(map[string]*loginEntry),
	}
}

func (manager *Manager) SecureCookie() bool { return manager.secureCookie }

// SessionTTL returns the configured admin session lifetime.
func (manager *Manager) SessionTTL() time.Duration { return manager.sessionTTL }

// Login verifies the admin token under a dedicated per-IP rate limit and
// creates a fresh opaque session. It returns the raw session cookie value and
// CSRF token exactly once.
func (manager *Manager) Login(candidate, clientIP string) (sessionToken string, csrfToken string, err error) {
	now := manager.now()
	if !manager.allowLogin(clientIP, now) {
		return "", "", ErrRateLimited
	}
	if !VerifyToken(candidate, manager.verifier) {
		return "", "", ErrUnauthorized
	}
	raw, err := randomBase64URL(sessionRandomSize)
	if err != nil {
		return "", "", err
	}
	csrf, err := randomBase64URL(sessionRandomSize)
	if err != nil {
		return "", "", err
	}
	record := sessionRecord{
		csrf:      csrf,
		expiresAt: now.Add(manager.sessionTTL),
	}
	key := sha256.Sum256([]byte(raw))

	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepLocked(now)
	if len(manager.sessions) >= maxSessions {
		// Fail closed rather than evicting an active session silently.
		return "", "", ErrRateLimited
	}
	manager.sessions[key] = record
	return raw, csrf, nil
}

// Lookup returns a session snapshot for a raw cookie value.
func (manager *Manager) Lookup(raw string) (Session, bool) {
	key := sha256.Sum256([]byte(raw))
	now := manager.now()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepLocked(now)
	record, ok := manager.sessions[key]
	if !ok {
		return Session{}, false
	}
	if !now.Before(record.expiresAt) {
		delete(manager.sessions, key)
		return Session{}, false
	}
	return Session{CSRFToken: record.csrf, ExpiresAt: record.expiresAt}, true
}

// CSRFMatches verifies a session-bound CSRF value in constant time.
func (manager *Manager) CSRFMatches(raw, csrf string) bool {
	key := sha256.Sum256([]byte(raw))
	now := manager.now()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepLocked(now)
	record, ok := manager.sessions[key]
	if !ok || !now.Before(record.expiresAt) {
		return false
	}
	if len(csrf) != len(record.csrf) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(csrf), []byte(record.csrf)) == 1
}

// Logout removes a session. It is idempotent.
func (manager *Manager) Logout(raw string) {
	key := sha256.Sum256([]byte(raw))
	manager.mu.Lock()
	defer manager.mu.Unlock()
	delete(manager.sessions, key)
}

// ActiveSessions reports the number of non-expired sessions, used by tests.
func (manager *Manager) ActiveSessions() int {
	now := manager.now()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepLocked(now)
	return len(manager.sessions)
}

func (manager *Manager) allowLogin(clientIP string, now time.Time) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if now.Sub(manager.lastSweep) >= loginIdleTTL {
		for key, entry := range manager.loginLimits {
			if now.Sub(entry.last) >= loginIdleTTL {
				delete(manager.loginLimits, key)
			}
		}
		manager.lastSweep = now
	}
	entry := manager.loginLimits[clientIP]
	if entry == nil {
		if len(manager.loginLimits) >= maxLoginIPs {
			return false
		}
		entry = &loginEntry{limiter: rate.NewLimiter(rate.Every(loginInterval), loginBurst)}
		manager.loginLimits[clientIP] = entry
	}
	entry.last = now
	return entry.limiter.AllowN(now, 1)
}

func (manager *Manager) sweepLocked(now time.Time) {
	for key, record := range manager.sessions {
		if !now.Before(record.expiresAt) {
			delete(manager.sessions, key)
		}
	}
}

func newRandomBase64URL(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
