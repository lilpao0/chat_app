package authentication

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

var (
	ErrInvalidTicket  = errors.New("invalid websocket ticket")
	ErrTicketCapacity = errors.New("websocket ticket capacity reached")
)

type ticketEntry struct {
	identity auth.Identity
	origin   string
	expires  time.Time
}

// ponytail: one API process, 4096 pending tickets; use a shared atomic store before scaling replicas.
type TicketStore struct {
	mu      sync.Mutex
	entries map[[32]byte]ticketEntry
	now     func() time.Time
}

func NewTicketStore() *TicketStore {
	return &TicketStore{entries: make(map[[32]byte]ticketEntry), now: time.Now}
}

func (s *TicketStore) Issue(identity auth.Identity, origin string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if identity.UserID <= 0 || !now.Before(identity.ExpiresAt) || origin == "" {
		return "", time.Time{}, ErrInvalidTicket
	}
	count := 0
	for key, entry := range s.entries {
		if !now.Before(entry.expires) {
			delete(s.entries, key)
		} else if entry.identity.UserID == identity.UserID {
			count++
		}
	}
	if count >= 8 || len(s.entries) >= 4096 {
		return "", time.Time{}, ErrTicketCapacity
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", time.Time{}, err
	}
	raw := base64.RawURLEncoding.EncodeToString(random[:])
	expires := now.Add(time.Minute)
	if identity.ExpiresAt.Before(expires) {
		expires = identity.ExpiresAt
	}
	s.entries[sha256.Sum256([]byte(raw))] = ticketEntry{identity: identity, origin: origin, expires: expires}
	return raw, expires, nil
}

func (s *TicketStore) Consume(raw, origin string) (auth.Identity, error) {
	if len(raw) != 43 {
		return auth.Identity{}, ErrInvalidTicket
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sha256.Sum256([]byte(raw))
	entry, ok := s.entries[key]
	if !ok {
		return auth.Identity{}, ErrInvalidTicket
	}
	if !s.now().Before(entry.expires) {
		delete(s.entries, key)
		return auth.Identity{}, ErrInvalidTicket
	}
	if origin != entry.origin {
		return auth.Identity{}, ErrInvalidTicket
	}
	delete(s.entries, key)
	return entry.identity, nil
}
