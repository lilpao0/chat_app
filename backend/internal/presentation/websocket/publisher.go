package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	messageusecase "github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
)

type Publisher struct {
	members repository.ConversationMemberReader
	hub     *Hub
}

var _ messageusecase.Publisher = (*Publisher)(nil)

func NewPublisher(members repository.ConversationMemberReader, hub *Hub) *Publisher {
	return &Publisher{members: members, hub: hub}
}

func (p *Publisher) PublishNewMessage(ctx context.Context, message entity.Message) error {
	memberIDs, err := p.members.MemberIDs(ctx, message.ConversationID)
	if err != nil {
		return fmt.Errorf("load realtime recipients: %w", err)
	}
	payload, err := json.Marshal(newMessageEvent(message))
	if err != nil {
		return fmt.Errorf("encode new message event: %w", err)
	}
	if err := p.hub.Publish(memberIDs, payload); err != nil {
		return fmt.Errorf("publish new message event: %w", err)
	}
	return nil
}
