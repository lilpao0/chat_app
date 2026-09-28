package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type closeRequest struct {
	status coderws.StatusCode
	reason string
}

type session struct {
	force      chan struct{}
	commands   *Commands
	connection *coderws.Conn
	identity   domainauth.Identity
	config     LifecycleConfig
	outbound   chan []byte
	close      chan closeRequest
}

func newSession(connection *coderws.Conn, identity domainauth.Identity, config LifecycleConfig) *session {
	return &session{force: make(chan struct{}, 1), connection: connection, identity: identity, config: config, outbound: make(chan []byte, config.SendQueueCapacity), close: make(chan closeRequest, 1)}
}

func (s *session) forceClose() {
	select {
	case s.force <- struct{}{}:
	default:
	}
}

func (s *session) enqueue(payload []byte) bool {
	copyOfPayload := append([]byte(nil), payload...)
	select {
	case s.outbound <- copyOfPayload:
		return true
	default:
		return false
	}
}

func (s *session) requestClose(status coderws.StatusCode, reason string) {
	select {
	case s.close <- closeRequest{status: status, reason: reason}:
	default:
	}
}

func (s *session) respond(response reply) {
	payload, err := json.Marshal(response)
	if err != nil || !s.enqueue(payload) {
		s.requestClose(coderws.StatusPolicyViolation, "client is too slow")
	}
}

func (s *session) run() {
	s.connection.SetReadLimit(s.config.MaxMessageBytes)
	ctx, cancel := context.WithCancel(context.Background())
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()
	var workers sync.WaitGroup
	defer func() { cancel(); _ = s.connection.CloseNow(); workers.Wait() }()
	commands := make(chan []byte, s.config.CommandQueueCapacity)
	done := make(chan struct{}, 3)
	start := func(f func()) {
		workers.Add(1)
		go func() { defer workers.Done(); defer func() { done <- struct{}{} }(); f() }()
	}
	start(func() {
		tokens := float64(s.config.CommandBurst)
		last := time.Now()
		for {
			kind, raw, err := s.connection.Read(ctx)
			if err != nil {
				return
			}
			now := time.Now()
			tokens += now.Sub(last).Seconds() * float64(s.config.CommandRate)
			if tokens > float64(s.config.CommandBurst) {
				tokens = float64(s.config.CommandBurst)
			}
			last = now
			if tokens < 1 {
				s.respond(failure(nil, "rate_limited", "Too many commands."))
				s.requestClose(coderws.StatusPolicyViolation, "command rate exceeded")
				return
			}
			tokens--
			if kind != coderws.MessageText {
				s.requestClose(coderws.StatusUnsupportedData, "text JSON required")
				return
			}
			select {
			case commands <- raw:
			case <-ctx.Done():
				return
			default:
				s.requestClose(coderws.StatusPolicyViolation, "command queue full")
				return
			}
		}
	})
	start(func() {
		for {
			select {
			case <-workCtx.Done():
				return
			case raw := <-commands:
				if workCtx.Err() != nil || !time.Now().Before(s.identity.ExpiresAt) {
					return
				}
				commandCtx, stop := context.WithTimeout(workCtx, s.config.CommandTimeout)
				result := s.commands.execute(commandCtx, s.identity.UserID, raw)
				stop()
				if workCtx.Err() != nil {
					return
				}
				s.respond(result)
			}
		}
	})
	start(func() {
		ticker := time.NewTicker(s.config.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, stop := context.WithTimeout(ctx, s.config.PongTimeout)
				err := s.connection.Ping(pingCtx)
				stop()
				if err != nil {
					return
				}
			}
		}
	})
	// The application writer is separate so expiry/shutdown never waits for a blocked write.
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case payload := <-s.outbound:
				writeCtx, stop := context.WithTimeout(ctx, s.config.WriteTimeout)
				err := s.connection.Write(writeCtx, coderws.MessageText, payload)
				stop()
				if err != nil {
					s.requestClose(coderws.StatusGoingAway, "write failed")
					return
				}
			}
		}
	}()
	expiry := time.NewTimer(max(0, time.Until(s.identity.ExpiresAt)))
	defer expiry.Stop()
	var request closeRequest
	select {
	case <-s.force:
		return
	case request = <-s.close:
	case <-expiry.C:
		request = closeRequest{coderws.StatusPolicyViolation, "access token expired"}
	case <-done:
		select {
		case request = <-s.close:
		default:
			return
		}
	}
	// Cancel SQL immediately, but allow a short close handshake before forcing the transport shut.
	cancelWork()
	closed := make(chan struct{})
	go func() { _ = s.connection.Close(request.status, request.reason); close(closed) }()
	timer := time.NewTimer(min(s.config.WriteTimeout, time.Second))
	defer timer.Stop()
	select {
	case <-closed:
	case <-s.force:
		cancel()
		_ = s.connection.CloseNow()
		<-closed
	case <-timer.C:
		cancel()
		_ = s.connection.CloseNow()
		<-closed
	}
}
