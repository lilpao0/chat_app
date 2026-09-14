# Go Chat App Backend

Backend API sử dụng Go (Gin) với Clean Architecture.

## Tech Stack

- Go
- Framework: Gin
- Database: PostgreSQL
- Auth: JWT
- Password: bcrypt
- Realtime: WebSocket

## Cấu trúc

```
cmd/
  api/
    main.go

internal/
  domain/
    user.go
    conversation.go
    message.go

  usecase/
    auth/
      login.go
      register.go
    conversation/
      get_conversations.go
      mark_as_read.go
    message/
      get_messages.go
      send_message.go

  repository/
    user_repository.go
    conversation_repository.go
    message_repository.go

  delivery/
    http/
      handler/
      middleware/
      router.go
    websocket/
      handler.go
      hub.go

  infrastructure/
    database/
      postgres.go
    repository/
      postgres_user_repository.go
      postgres_conversation_repository.go
      postgres_message_repository.go
    auth/
      jwt.go
      password.go

  config/

migrations/
```

## Chạy server

```bash
go mod download
go run cmd/api/main.go
```

## API Endpoints

- POST /api/auth/register
- POST /api/auth/login
- GET /api/conversations
- GET /api/conversations/:id/messages
- POST /api/conversations/:id/messages
- POST /api/conversations/:id/read
- WebSocket: WS /ws
