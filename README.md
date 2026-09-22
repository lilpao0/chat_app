# Chat App - Flutter + Go

Ung dung chat MVP gom 3 man hinh: Auth, Chat List, Chat Detail.
Thuc hien trong 6 tuan theo ke hoach `ke-hoach-6-tuan-flutter-golang-chat-app-1.md`.

## Kien truc

```
Flutter App
    | REST API + WebSocket
    v
Go Backend
    | PostgreSQL
    v
```

Ca frontend lan backend deu dung **Clean Architecture**:
- `Presentation` -> `Domain` <- `Data`

## Cau truc thu muc

```
chat_app/
|-- frontend/          # Flutter app
|   `-- lib/
|       |-- core/     # error, network, router, storage, utils
|       |-- features/
|       |   |-- auth/
|       |   |-- chat_list/
|       |   `-- chat_detail/
|       `-- main.dart
|
|-- backend/          # Go API
|   |-- cmd/api/     # entry point
|   |-- internal/
|   |   |-- presentation/ # HTTP / WebSocket
|   |   |-- domain/       # entities, repository interfaces, use cases
|   |   `-- data/         # database, SQL repositories, auth adapters
|   `-- migrations/
|
`-- ke-hoach-6-tuan-flutter-golang-chat-app-1.md
```

## Tech Stack

| Thanh phan | Cong nghe |
|------------|-----------|
| Mobile App | Flutter |
| Mobile Language | Dart |
| Navigation | go_router |
| State Management | Block |
| HTTP Client | Dio |
| Secure Token Storage | flutter_secure_storage |
| Backend | Go |
| HTTP Framework | Gin |
| Database | PostgreSQL |
| Authentication | JWT |
| Password Hash | bcrypt |
| Realtime | WebSocket |

## MVP Features

### Auth
- Register / Login / Logout
- JWT + token persistence
- Auto login

### Chat List
- Avatar, name, last message, time
- Unread badge + unread count
- Pull to refresh
- Tìm người dùng và mở lại hoặc tạo cuộc trò chuyện 1–1 với họ

### Chat Detail
- Load message history
- Send text message
- Receive realtime message via WebSocket
- Mark conversation as read
- Auto scroll to latest message

## API Endpoints

```
POST /api/auth/register
POST /api/auth/login
GET  /api/users?q=...
POST /api/conversations/direct
GET  /api/conversations
GET  /api/conversations/:id/messages
POST /api/conversations/:id/messages
POST /api/conversations/:id/read
WS   /ws
```

## 6-Week Roadmap

```
Tuan 1  -> Dart + Flutter UI (3 man hinh mock data)
Tuan 2  -> Go + Gin + PostgreSQL (REST API)
Tuan 3  -> Auth E2E (JWT, bcrypt, Flutter <-> Go)
Tuan 4  -> Chat List hoan chinh
Tuan 5  -> Chat Detail + WebSocket realtime
Tuan 6  -> Refactor Clean Architecture + Testing + Build APK
```

## Database Schema

```
users (id, name, email, password_hash, avatar_url, created_at)
conversations (id, created_at, updated_at)
conversation_members (conversation_id, user_id, last_read_at)
messages (id, conversation_id, sender_id, content, created_at)
```

## Setup

### Backend

Backend đã hoàn thành B01–B29: Auth và luồng REST chat (danh sách, gửi tin, lịch sử, đọc/chưa đọc).
Mã nguồn chia thành `presentation`, `domain`, `data`. Read marker dùng `last_read_message_id` cùng `last_read_at`.
B30 đang chờ chọn Flutter mobile hay cả Web để triển khai WebSocket.
Đọc [hướng dẫn backend](backend/README.md), [quy tắc cho agent](backend/docs/AGENTS.md)
và [backlog từng bước](backend/docs/BACKLOG.md) trước khi triển khai.
Theo yêu cầu mới: tiếp tục từng phần nhỏ, giải thích bằng tiếng Việt và dừng khi có câu hỏi cần người dùng quyết định.
Backend cần `DATABASE_URL` và `JWT_SECRET`; xem hướng dẫn backend về chạy ứng dụng và kiểm thử.

### Frontend

```bash
cd frontend
flutter pub get
flutter run
flutter build apk   # build release
```

## Design

UI tham khao Figma:
https://www.figma.com/design/g0ru7q7CMOvhAN4KWzzhM8/WhatsApp-UI--Community-?node-id=1-8820&p=f&m=dev
