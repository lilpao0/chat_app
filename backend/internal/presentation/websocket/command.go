package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"io"
	"unicode/utf8"
)

type MessageSender interface {
	ExecuteWithRequestID(context.Context, int64, int64, string, string) (entity.Message, error)
}
type ReadMarker interface {
	Execute(context.Context, int64, int64, int64) (int64, error)
}
type Commands struct {
	Send MessageSender
	Read ReadMarker
}
type command struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}
type reply struct {
	Type      string        `json:"type"`
	RequestID *string       `json:"request_id"`
	Data      any           `json:"data,omitempty"`
	Error     *commandError `json:"error,omitempty"`
}
type commandError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func failure(id *string, code, message string) reply {
	return reply{Type: "error", RequestID: id, Error: &commandError{Code: code, Message: message}}
}
func strictJSON(raw []byte, target any) error {
	if !utf8.Valid(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return entity.ErrInvalidInput
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return entity.ErrInvalidInput
	}
	return nil
}
func (c *Commands) execute(ctx context.Context, actor int64, raw []byte) reply {
	var cmd command
	if err := strictJSON(raw, &cmd); err != nil {
		return failure(nil, "invalid_input", "Invalid command.")
	}
	if !entity.ValidRequestID(cmd.RequestID) {
		return failure(nil, "invalid_input", "A canonical UUID request_id is required.")
	}
	id := &cmd.RequestID
	if cmd.Type == "" {
		return failure(id, "invalid_input", "Command type is required.")
	}
	var data any
	var err error
	switch cmd.Type {
	case "send_message":
		var body struct {
			ConversationID int64  `json:"conversation_id"`
			Content        string `json:"content"`
		}
		if strictJSON(cmd.Data, &body) != nil || body.ConversationID <= 0 || body.Content == "" {
			return failure(id, "invalid_input", "Invalid message.")
		}
		if c == nil || c.Send == nil {
			return failure(id, "internal_error", "Command unavailable.")
		}
		var m entity.Message
		m, err = c.Send.ExecuteWithRequestID(ctx, body.ConversationID, actor, body.Content, cmd.RequestID)
		data = newMessageEvent(m).Data
		cmd.Type = "message_sent"
	case "mark_read":
		var body struct {
			ConversationID int64 `json:"conversation_id"`
			MessageID      int64 `json:"last_read_message_id"`
		}
		if strictJSON(cmd.Data, &body) != nil || body.ConversationID <= 0 || body.MessageID <= 0 {
			return failure(id, "invalid_input", "Invalid read marker.")
		}
		if c == nil || c.Read == nil {
			return failure(id, "internal_error", "Command unavailable.")
		}
		body.MessageID, err = c.Read.Execute(ctx, body.ConversationID, actor, body.MessageID)
		data = body
		cmd.Type = "read_updated"
	default:
		return failure(id, "unsupported_command", "Unsupported command.")
	}
	if err != nil {
		switch {
		case errors.Is(err, entity.ErrInvalidInput):
			return failure(id, "invalid_input", "Invalid command data.")
		case errors.Is(err, entity.ErrConversationNotFound):
			return failure(id, "not_found", "Conversation not found.")
		case errors.Is(err, entity.ErrRequestConflict):
			return failure(id, "request_conflict", "Request ID was already used.")
		default:
			return failure(id, "internal_error", "Unable to complete command.")
		}
	}
	return reply{Type: cmd.Type, RequestID: id, Data: data}
}
