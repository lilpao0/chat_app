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
	ID             int64            `json:"id"`
	ConversationID int64            `json:"conversation_id"`
	SenderID       int64            `json:"sender_id"`
	Content        string           `json:"content"`
	CreatedAt      time.Time        `json:"created_at"`
	Attachments    []attachmentData `json:"attachments"`
}

type attachmentData struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

func newMessageEvent(message entity.Message) event {
	attachments := make([]attachmentData, 0, len(message.Attachments))
	for _, a := range message.Attachments {
		attachments = append(attachments, attachmentData{ID: a.ID, Kind: a.Kind, ContentType: a.ContentType, SizeBytes: a.SizeBytes})
	}
	return event{Type: "new_message", Data: messageData{ID: message.ID, ConversationID: message.ConversationID, SenderID: message.SenderID, Content: message.Content, CreatedAt: message.CreatedAt.UTC(), Attachments: attachments}}
}
