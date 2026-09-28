package websocket

import (
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type event struct {
	Type string      `json:"type"`
	Data messageData `json:"data"`
}

type messageData struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func newMessageEvent(message entity.Message) event {
	return event{Type: "new_message", Data: messageData{ID: message.ID, ConversationID: message.ConversationID, SenderID: message.SenderID, Content: message.Content, CreatedAt: message.CreatedAt.UTC()}}
}
