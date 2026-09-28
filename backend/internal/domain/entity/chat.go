package entity

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput         = errors.New("invalid chat input")
	ErrConversationNotFound = errors.New("conversation not found")
)

type Participant struct {
	ID              int64
	Name, AvatarURL string
}
type ConversationSummary struct {
	ID            int64
	Participant   Participant
	LastMessage   *string
	LastMessageAt *time.Time
	UnreadCount   int64
}

type Message struct {
	ID, ConversationID, SenderID int64
	Content                      string
	CreatedAt                    time.Time
}

type MessageQuery struct {
	BeforeID, AfterID int64
	Limit             int
}
type MessagePage struct {
	Items                     []Message
	HasMore                   bool
	NextBeforeID, NextAfterID *int64
}
