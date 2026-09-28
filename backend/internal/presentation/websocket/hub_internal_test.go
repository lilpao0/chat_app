package websocket

import (
	coderws "github.com/coder/websocket"
	"testing"
)

func TestSessionOutboundQueueIsBoundedAndCopiesPayload(t *testing.T) {
	s := &session{outbound: make(chan []byte, 1), close: make(chan closeRequest, 1)}
	payload := []byte("first")
	if !s.enqueue(payload) {
		t.Fatal("first event was rejected")
	}
	payload[0] = 'X'
	if s.enqueue([]byte("second")) {
		t.Fatal("event was accepted after queue became full")
	}
	if got := string(<-s.outbound); got != "first" {
		t.Fatalf("queued payload=%q, want copied first payload", got)
	}
}

func TestSessionCloseRequestIsNonBlocking(t *testing.T) {
	s := &session{close: make(chan closeRequest, 1)}
	s.requestClose(coderws.StatusPolicyViolation, "first")
	s.requestClose(coderws.StatusGoingAway, "second")
	request := <-s.close
	if request.status != coderws.StatusPolicyViolation || request.reason != "first" {
		t.Fatalf("close request=%+v", request)
	}
}
