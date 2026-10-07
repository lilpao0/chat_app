package authentication

import (
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

func TestTicketExpiryOriginAndSingleUse(t *testing.T) {
	now := time.Now()
	s := NewTicketStore()
	s.now = func() time.Time { return now }
	identity := auth.Identity{UserID: 8, ExpiresAt: now.Add(time.Hour)}
	raw, expiry, err := s.Issue(identity, "http://localhost:5173")
	if err != nil || len(raw) != 43 || !expiry.Equal(now.Add(time.Minute)) {
		t.Fatalf("expiry=%v err=%v", expiry, err)
	}
	if _, ok := s.entries[sha256.Sum256([]byte(raw))]; !ok {
		t.Fatal("raw ticket is not indexed by digest")
	}
	if _, err := s.Consume(raw, "http://localhost:5174"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatal("wrong origin accepted")
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.Consume(raw, "http://localhost:5173")
			if err == nil {
				if got != identity {
					t.Error("identity changed")
				}
				accepted.Add(1)
			} else if !errors.Is(err, ErrInvalidTicket) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("consumers=%d", accepted.Load())
	}
	identity.ExpiresAt = now.Add(10 * time.Second)
	raw, expiry, err = s.Issue(identity, "http://localhost:5173")
	if err != nil || !expiry.Equal(identity.ExpiresAt) {
		t.Fatal("ticket outlives access token")
	}
	now = expiry
	if _, err := s.Consume(raw, "http://localhost:5173"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatal("expiry boundary accepted")
	}
	if _, _, err := s.Issue(identity, "http://localhost:5173"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatal("expired identity issued ticket")
	}
}

func TestTicketCapacityAndCleanup(t *testing.T) {
	now := time.Now()
	s := NewTicketStore()
	s.now = func() time.Time { return now }
	identity := auth.Identity{UserID: 1, ExpiresAt: now.Add(time.Hour)}
	for range 8 {
		if _, _, err := s.Issue(identity, "http://localhost:5173"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.Issue(identity, "http://localhost:5173"); !errors.Is(err, ErrTicketCapacity) {
		t.Fatal("per-user cap missing")
	}
	now = now.Add(time.Minute)
	if _, _, err := s.Issue(identity, "http://localhost:5173"); err != nil || len(s.entries) != 1 {
		t.Fatal("expired entries not purged")
	}
	for id := int64(2); id <= 4096; id++ {
		identity.UserID = id
		if _, _, err := s.Issue(identity, "http://localhost:5173"); err != nil {
			t.Fatal(err)
		}
	}
	identity.UserID = 5000
	if _, _, err := s.Issue(identity, "http://localhost:5173"); !errors.Is(err, ErrTicketCapacity) {
		t.Fatal("global cap missing")
	}
}
