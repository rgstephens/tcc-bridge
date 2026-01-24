package tcc

import (
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// Session manages the TCC authentication session
type Session struct {
	mu           sync.RWMutex
	client       *http.Client
	jar          *cookiejar.Jar
	username     string
	password     string
	authenticated bool
	lastLogin    time.Time
	loginExpiry  time.Duration
}

// NewSession creates a new TCC session
func NewSession() (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Session{
		client:      client,
		jar:         jar,
		loginExpiry: 30 * time.Minute, // Sessions expire after 30 minutes of inactivity
	}, nil
}

// SetCredentials sets the login credentials
func (s *Session) SetCredentials(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.username = username
	s.password = password
	s.authenticated = false
}

// GetCredentials returns the current credentials
func (s *Session) GetCredentials() (username, password string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username, s.password
}

// HasCredentials returns true if credentials are set
func (s *Session) HasCredentials() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username != "" && s.password != ""
}

// IsAuthenticated returns true if the session is authenticated
func (s *Session) IsAuthenticated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.authenticated {
		return false
	}
	// Check if session has expired
	if time.Since(s.lastLogin) > s.loginExpiry {
		return false
	}
	return true
}

// MarkAuthenticated marks the session as authenticated
func (s *Session) MarkAuthenticated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticated = true
	s.lastLogin = time.Now()
}

// MarkUnauthenticated marks the session as unauthenticated
func (s *Session) MarkUnauthenticated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticated = false
}

// RefreshSession updates the last login time
func (s *Session) RefreshSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastLogin = time.Now()
}

// GetClient returns the HTTP client
func (s *Session) GetClient() *http.Client {
	return s.client
}

// ClearSession clears all session data and cookies
func (s *Session) ClearSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create new cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	s.jar = jar
	s.client.Jar = jar
	s.authenticated = false
	s.lastLogin = time.Time{}

	return nil
}

// LastLogin returns the last login time
func (s *Session) LastLogin() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastLogin
}
