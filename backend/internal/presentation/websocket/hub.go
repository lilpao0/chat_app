package websocket

import (
	"context"
	"errors"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

var (
	ErrHubClosed  = errors.New("websocket hub is closed")
	ErrSlowClient = errors.New("websocket client queue is full")
)

type LifecycleConfig struct {
	CommandQueueCapacity int
	CommandTimeout       time.Duration
	CommandRate          int
	CommandBurst         int
	SendQueueCapacity    int
	MaxMessageBytes      int64
	WriteTimeout         time.Duration
	PongTimeout          time.Duration
	PingInterval         time.Duration
}

type Hub struct {
	commands *Commands
	mu       sync.RWMutex
	users    map[int64]map[*session]struct{}
	closed   bool
	config   LifecycleConfig
	sessions sync.WaitGroup
}

func NewHub(config LifecycleConfig, commands ...*Commands) *Hub {
	if config.CommandQueueCapacity <= 0 {
		config.CommandQueueCapacity = 16
	}
	if config.CommandTimeout <= 0 {
		config.CommandTimeout = 5 * time.Second
	}
	if config.CommandRate <= 0 {
		config.CommandRate = 10
	}
	if config.CommandBurst <= 0 {
		config.CommandBurst = 20
	}
	h := &Hub{users: make(map[int64]map[*session]struct{}), config: config}
	if len(commands) > 0 {
		h.commands = commands[0]
	}
	return h
}

func (h *Hub) Run(connection *coderws.Conn, identity domainauth.Identity) {
	s := newSession(connection, identity, h.config)
	s.commands = h.commands
	if !h.register(s) {
		_ = connection.Close(coderws.StatusGoingAway, "server shutting down")
		return
	}
	defer h.unregister(s)
	s.run()
}

func (h *Hub) register(s *session) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	connections := h.users[s.identity.UserID]
	if connections == nil {
		connections = make(map[*session]struct{})
		h.users[s.identity.UserID] = connections
	}
	connections[s] = struct{}{}
	h.sessions.Add(1)
	return true
}

func (h *Hub) unregister(s *session) {
	h.mu.Lock()
	connections := h.users[s.identity.UserID]
	if _, ok := connections[s]; ok {
		delete(connections, s)
		if len(connections) == 0 {
			delete(h.users, s.identity.UserID)
		}
		h.sessions.Done()
	}
	h.mu.Unlock()
}

func (h *Hub) Publish(userIDs []int64, payload []byte) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return ErrHubClosed
	}
	targets := make([]*session, 0)
	seen := make(map[*session]struct{})
	for _, userID := range userIDs {
		for s := range h.users[userID] {
			if _, exists := seen[s]; !exists {
				seen[s] = struct{}{}
				targets = append(targets, s)
			}
		}
	}
	h.mu.RUnlock()
	var slow bool
	for _, s := range targets {
		if !s.enqueue(payload) {
			slow = true
			s.requestClose(coderws.StatusPolicyViolation, "client is too slow")
		}
	}
	if slow {
		return ErrSlowClient
	}
	return nil
}

func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		for _, connections := range h.users {
			for s := range connections {
				s.requestClose(coderws.StatusGoingAway, "server shutting down")
			}
		}
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.sessions.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		h.mu.RLock()
		for _, connections := range h.users {
			for s := range connections {
				s.forceClose()
			}
		}
		h.mu.RUnlock()
		return ctx.Err()
	}
}

func (h *Hub) ConnectionCount(userID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users[userID])
}
