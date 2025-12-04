package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// WSTokenStore manages WebSocket CSRF tokens
// Tokens are one-time use and expire after a short TTL
type WSTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*wsTokenEntry
}

type wsTokenEntry struct {
	username  string
	createdAt time.Time
}

const (
	// WSTokenTTL is how long a token is valid
	WSTokenTTL = 30 * time.Second
	// WSTokenLength is the byte length of the token (will be hex encoded to 2x)
	WSTokenLength = 32
)

// NewWSTokenStore creates a new WebSocket token store
func NewWSTokenStore() *WSTokenStore {
	store := &WSTokenStore{
		tokens: make(map[string]*wsTokenEntry),
	}
	// Start cleanup goroutine
	go store.cleanupLoop()
	return store
}

// Generate creates a new one-time token for a user
func (s *WSTokenStore) Generate(username string) (string, error) {
	bytes := make([]byte, WSTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	s.mu.Lock()
	s.tokens[token] = &wsTokenEntry{
		username:  username,
		createdAt: time.Now(),
	}
	s.mu.Unlock()

	return token, nil
}

// Validate checks if a token is valid and consumes it (one-time use)
// Returns the username associated with the token, or empty string if invalid
func (s *WSTokenStore) Validate(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tokens[token]
	if !exists {
		return "", false
	}

	// Delete token immediately (one-time use)
	delete(s.tokens, token)

	// Check if expired
	if time.Since(entry.createdAt) > WSTokenTTL {
		return "", false
	}

	return entry.username, true
}

// cleanupLoop periodically removes expired tokens
func (s *WSTokenStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes all expired tokens
func (s *WSTokenStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for token, entry := range s.tokens {
		if now.Sub(entry.createdAt) > WSTokenTTL {
			delete(s.tokens, token)
		}
	}
}
