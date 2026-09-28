# Kế hoạch 6 tuần học và xây dựng ứng dụng Chat bằng Flutter + Golang

## 1. Mục tiêu cuối cùng

Sau 6 tuần, mục tiêu là hoàn thành một ứng dụng chat MVP gồm 3 màn hình chính:

1. **Auth**
   - Đăng ký
   - Đăng nhập
   - Đăng xuất
   - Lưu phiên đăng nhập bằng JWT

2. **Chat List**
   - Avatar
   - Tên người chat
   - Tin nhắn cuối
   - Thời gian tin nhắn cuối
   - Trạng thái đã đọc / chưa đọc
   - Số lượng tin nhắn chưa đọc

3. **Chat Detail**
   - Load lịch sử tin nhắn
   - Gửi tin nhắn text
   - Nhận tin nhắn realtime
   - Scroll xuống tin nhắn mới nhất
   - Đánh dấu cuộc hội thoại là đã đọc

---

Design: https://www.figma.com/design/g0ru7q7CMOvhAN4KWzzhM8/WhatsApp-UI--Community-?node-id=1-8820&p=f&m=dev

# 2. Kiến trúc tổng thể

Project sử dụng **Clean Architecture** cho cả Flutter và Golang.

Mục tiêu của Clean Architecture:

- Tách UI, business logic và data access
- Giảm phụ thuộc giữa các layer
- Dễ test
- Dễ thay API, database hoặc framework
- Code dễ mở rộng khi project lớn hơn

Kiến trúc tổng thể:

```text
Flutter App
   |
   | REST API + WebSocket
   v
Golang Backend
   |
   v
PostgreSQL
```

Mỗi phía được chia thành các layer:

```text
Presentation
    |
    v
Domain
    |
    v
Data
```

Nguyên tắc dependency:

```text
Presentation
    |
    v
Domain
    ^
    |
Data
```

**Domain là layer trung tâm và không phụ thuộc vào UI, framework hoặc database.**

Luồng tổng quát:

```text
Flutter Presentation
        |
        v
Flutter Domain
        |
        v
Flutter Data
        |
        | HTTP / WebSocket
        v
Go Delivery / Handler
        |
        v
Go Use Case
        |
        v
Go Repository Interface
        |
        v
Go Infrastructure
        |
        v
PostgreSQL
```

---

# 3. Stack công nghệ đề xuất

| Thành phần | Công nghệ |
|---|---|
| Mobile App | Flutter |
| Mobile Language | Dart |
| Navigation | go_router |
| Architecture | Clean Architecture |
| State Management | Block |
| HTTP Client | Dio hoặc http |
| Secure Token Storage | flutter_secure_storage |
| Backend | Golang |
| HTTP Framework | Gin |
| Database | PostgreSQL |
| Database Access | database/sql |
| Authentication | JWT |
| Password Hash | bcrypt |
| Realtime | WebSocket |
| API Testing | Postman |
| Database GUI | DBeaver hoặc pgAdmin |
| Local DB | Docker |
| Version Control | Git + GitHub |

---

# 4. Những kiến thức nền tảng cần học

## Programming cơ bản

Cần hiểu:

- Variable
- Data type
- if / else
- for / while
- Function
- Class
- Object
- List
- Map
- Null
- Error handling
- async / await

Cần hình dung được luồng:

```text
Input
  |
  v
Processing
  |
  v
Output
```

Và:

```text
Client
  |
  | HTTP Request
  v
Server
  |
  v
Database
  |
  v
Server Response
  |
  v
Client
```

---

# 5. Kiến thức Flutter cần học

Ngoài Flutter cơ bản, project sẽ áp dụng **Clean Architecture**.

Các layer chính:

```text
Presentation
Domain
Data
```

Trong đó:

### Presentation

Chứa:

- Screen / Page
- Widget
- ViewModel / Provider
- State

Không gọi API hoặc database trực tiếp.

### Domain

Chứa:

- Entity
- Use Case
- Repository Interface

Đây là phần business logic quan trọng nhất.

Ví dụ:

```text
LoginUseCase
GetConversationsUseCase
GetMessagesUseCase
SendMessageUseCase
MarkConversationAsReadUseCase
```

### Data

Chứa:

- API Service
- WebSocket Service
- DTO / Model
- Repository Implementation
- Secure Storage

Data layer implement các interface được khai báo ở Domain.

Dependency đúng:

```text
Presentation
     |
     v
Domain
     ^
     |
Data
```

Theo thứ tự học:

```text
Dart
 ↓
Widget
 ↓
Layout
 ↓
State
 ↓
Navigation
 ↓
Form
 ↓
Async / Await
 ↓
HTTP
 ↓
JSON -> Model
 ↓
Repository
 ↓
Authentication
 ↓
WebSocket
```

## Dart cần nắm

- String
- int
- bool
- List
- Map
- class
- constructor
- Future
- async
- await
- try / catch

Ví dụ:

```dart
Future<User> login() async {
  final response = await api.login();
  return User.fromJson(response);
}
```

---

# 6. Flutter UI cần biết

Các Widget quan trọng:

- Scaffold
- AppBar
- Column
- Row
- Container
- Padding
- Expanded
- Center
- Text
- Image
- CircleAvatar
- Icon
- TextField
- TextFormField
- ElevatedButton
- ListView
- ListTile
- FutureBuilder

Cần hiểu:

```text
StatelessWidget
vs
StatefulWidget
```

Và:

```text
State thay đổi
      |
      v
Widget rebuild
      |
      v
UI thay đổi
```

---

# 7. Golang cần học

Backend cũng sử dụng **Clean Architecture**.

Các layer đề xuất:

```text
Delivery
Use Case
Domain
Repository
Infrastructure
```

Vai trò:

### Delivery

- HTTP Handler
- Gin Router
- Middleware
- WebSocket Handler

### Use Case

Chứa business logic:

- Login
- Register
- Get conversations
- Get messages
- Send message
- Mark as read

### Domain

Chứa các entity cốt lõi:

- User
- Conversation
- Message

### Repository

Khai báo interface:

```go
type UserRepository interface {
    FindByEmail(ctx context.Context, email string) (*User, error)
}
```

### Infrastructure

Chứa implementation cụ thể:

- PostgreSQL
- JWT
- bcrypt
- WebSocket Hub
- Config

Không cần học toàn bộ Go.

Tập trung:

- Variables
- Functions
- Struct
- Methods
- Pointer cơ bản
- Slice
- Map
- Interface cơ bản
- Package
- Module
- Error handling
- JSON
- HTTP request / response
- Gin handler
- Middleware
- database/sql
- Context
- Goroutine cơ bản
- Channel cơ bản
- WebSocket

Ví dụ:

```go
type User struct {
    ID     int64  `json:"id"`
    Name   string `json:"name"`
    Email  string `json:"email"`
    Avatar string `json:"avatar"`
}
```

Luồng backend theo Clean Architecture:

```text
HTTP Request
     |
     v
Handler
     |
     v
Use Case
     |
     v
Repository Interface
     |
     v
Repository Implementation
     |
     v
PostgreSQL
```

Nguyên tắc:

```text
Use Case
không biết
PostgreSQL đang được dùng

Use Case
chỉ biết
Repository Interface
```

---

# 8. SQL cần học

Các câu lệnh chính:

```sql
SELECT
INSERT
UPDATE
DELETE

WHERE
ORDER BY
LIMIT

JOIN
COUNT

CREATE TABLE
CREATE INDEX
```

Ví dụ:

```sql
SELECT *
FROM messages
WHERE conversation_id = 10
ORDER BY created_at DESC
LIMIT 20;
```

---

# 9. Thiết kế Database

## users

```text
users
-------------------
id
name
email
password_hash
avatar_url
created_at
```

## conversations

```text
conversations
-------------------
id
created_at
updated_at
```

## conversation_members

```text
conversation_members
-------------------
conversation_id
user_id
last_read_at
```

## messages

```text
messages
-------------------
id
conversation_id
sender_id
content
created_at
```

Quan hệ:

```text
User
 |
 +---- Conversation Member
              |
              v
        Conversation
              |
              v
           Messages
```

`last_read_at` dùng để tính số tin nhắn chưa đọc.

Ví dụ:

```text
User đọc gần nhất lúc 10:00

Có tin nhắn mới lúc:
10:01
10:03
10:05

unread_count = 3
```

Khi user mở Chat Detail:

```text
last_read_at = 10:06
```

Sau đó:

```text
unread_count = 0
```

---

# 10. API cần xây dựng

## Authentication

```http
POST /api/auth/register
POST /api/auth/login
```

Login response:

```json
{
  "access_token": "...",
  "user": {
    "id": 1,
    "name": "Andy",
    "avatar": "..."
  }
}
```

---

## Chat List

```http
GET /api/conversations
```

Response:

```json
[
  {
    "id": 10,
    "user": {
      "id": 2,
      "name": "Anna",
      "avatar": "..."
    },
    "last_message": "Hello Andy",
    "last_message_at": "2026-09-14T10:30:00Z",
    "unread_count": 2
  }
]
```

---

## Messages

```http
GET /api/conversations/:id/messages
```

```http
POST /api/conversations/:id/messages
```

Body:

```json
{
  "content": "Hello!"
}
```

---

## Read status

```http
POST /api/conversations/:id/read
```

---

## WebSocket

```text
WS /ws
```

Ví dụ event:

```json
{
  "type": "new_message",
  "data": {
    "id": 100,
    "conversation_id": 10,
    "sender_id": 2,
    "content": "Hello",
    "created_at": "..."
  }
}
```

---

# 11. Kế hoạch chi tiết 6 tuần

# Tuần 1 — Dart + Flutter + UI

## Mục tiêu

- Tạo project Flutter
- Hiểu Dart cơ bản
- Xây 3 màn hình bằng mock data
- Điều hướng được giữa các màn hình

## Ngày 1 — Setup môi trường

Cài:

- VS Code
- Flutter SDK
- Android Studio
- Android Emulator
- Git
- Go
- Docker Desktop
- Postman
- DBeaver

Kiểm tra:

```bash
flutter doctor
```

Tạo project:

```bash
flutter create chat_app
```

Kết quả:

```text
Flutter app chạy được trên emulator
```

---

## Ngày 2 — Dart cơ bản

Học:

- String
- int
- bool
- List
- Map
- if
- for
- function
- class
- constructor

Tạo model:

- User
- Message
- Conversation

Ví dụ:

```dart
class User {
  final int id;
  final String name;

  User({
    required this.id,
    required this.name,
  });
}
```

---

## Ngày 3 — Async / Await

Học:

- Future
- async
- await
- try / catch
- JSON

Ví dụ:

```dart
Future<String> getMessage() async {
  await Future.delayed(
    const Duration(seconds: 1),
  );

  return 'Hello';
}
```

---

## Ngày 4 — Flutter UI

Học:

- Widget tree
- Scaffold
- Column
- Row
- Container
- Padding
- Text
- TextField
- Button
- CircleAvatar

Xây Auth Screen:

```text
Email
Password
Login button
```

---

## Ngày 5 — Chat List UI

Học:

- ListView.builder
- ListTile
- CircleAvatar
- Expanded
- Row
- Column

Mock data:

```text
Anna
Hello Andy              2
10:30

John
See you tomorrow
09:00
```

---

## Ngày 6 — Chat Detail + Navigation

Học:

- Navigator hoặc go_router

Routes:

```text
/chat
/chat/:conversationId
```

Chat Detail gồm:

- List message
- TextField
- Send button

### Kết quả tuần 1

- Auth UI
- Chat List UI
- Chat Detail UI
- Navigation
- Mock User
- Mock Conversation
- Mock Message

---

# Tuần 2 — Go + REST API + PostgreSQL

## Mục tiêu

```text
Postman
   |
   v
Go API
   |
   v
PostgreSQL
```

---

## Ngày 1 — Go cơ bản

Học:

- variable
- function
- struct
- slice
- map
- pointer
- error
- package

Tạo project:

```bash
mkdir chat-api
cd chat-api

go mod init chat-api
```

---

## Ngày 2 — Gin + HTTP

Học:

- GET
- POST
- Status code
- Header
- Body
- JSON

Tạo:

```http
GET /health
```

Response:

```json
{
  "status": "ok"
}
```

---

## Ngày 3 — PostgreSQL

Học:

```sql
CREATE TABLE
INSERT
SELECT
UPDATE
DELETE
```

Tạo:

- users
- conversations
- conversation_members
- messages

---

## Ngày 4 — Go kết nối PostgreSQL

Học:

- database/sql
- db.Query
- db.QueryRow
- db.Exec
- Scan

Ví dụ:

```go
func FindUserByID(id int64) (*User, error)
```

---

## Ngày 5 — Conversation API

Viết:

```http
GET /api/conversations
```

Trả về:

- conversation
- other user
- last message

---

## Ngày 6 — Message API

Viết:

```http
GET /api/conversations/:id/messages
POST /api/conversations/:id/messages
```

Test toàn bộ bằng Postman.

### Kết quả tuần 2

- Go server
- PostgreSQL
- REST API
- User table
- Conversation table
- Message table
- Postman gọi được API

---

# Tuần 3 — Authentication End-to-End

## Ngày 1

Học:

- Authentication
- Authorization
- Password hashing
- JWT

Luồng:

```text
password
   |
   v
bcrypt
   |
   v
password_hash
   |
   v
database
```

---

## Ngày 2 — Register API

```http
POST /auth/register
```

Luồng:

```text
email/password
      |
      v
validate
      |
      v
bcrypt
      |
      v
INSERT user
```

---

## Ngày 3 — Login API

```http
POST /auth/login
```

Luồng:

```text
email/password
      |
      v
Find user
      |
      v
Compare bcrypt
      |
      v
Generate JWT
      |
      v
Return token
```

Viết thêm:

```text
AuthMiddleware
```

---

## Ngày 4 — Flutter gọi API + Clean Architecture

Học:

- HTTP request
- JSON decode
- DTO / Model
- Entity
- Use Case
- Repository Interface
- Repository Implementation
- Service

Luồng:

```text
AuthScreen
     |
     v
AuthViewModel
     |
     v
LoginUseCase
     |
     v
AuthRepository
     |
     v
AuthRepositoryImpl
     |
     v
AuthApiService
     |
     v
Go API
```

Mục tiêu là hiểu rõ:

```text
Presentation
     |
     v
Domain
     ^
     |
Data
```

---

## Ngày 5 — Login thật

Flutter gọi:

```http
POST /auth/login
```

Lưu JWT bằng:

```text
flutter_secure_storage
```

Login thành công:

```text
Auth
 |
 v
Chat List
```

---

## Ngày 6 — Error Handling

Xử lý:

- Loading
- Invalid password
- Invalid email
- Server error
- Token persistence
- Auto login
- Logout

### Kết quả tuần 3

- Register
- Login
- bcrypt
- JWT
- JWT middleware
- Flutter gọi API
- Save token
- Auto login
- Logout

---

# Tuần 4 — Chat List hoàn chỉnh

## Ngày 1 — Conversation API nâng cao

```http
GET /conversations
```

Trả:

- conversation_id
- user
- avatar
- name
- last_message
- last_message_at
- unread_count

---

## Ngày 2 — SQL nâng cao

Học:

- JOIN
- COUNT
- GROUP BY
- ORDER BY

Dùng để tính:

- Last message
- Unread messages

---

## Ngày 3 — Flutter Clean Architecture

Tạo:

```text
ChatListScreen
ChatListViewModel

GetConversationsUseCase

ConversationRepository
ConversationRepositoryImpl

ConversationApiService
```

Luồng:

```text
ChatListScreen
     |
     v
ChatListViewModel
     |
     v
GetConversationsUseCase
     |
     v
ConversationRepository
     |
     v
ConversationRepositoryImpl
     |
     v
ConversationApiService
     |
     v
API
```

---

## Ngày 4 — UI Chat List

Hiển thị:

- CircleAvatar
- Name
- Last message
- Last message time
- Unread badge

Nếu:

```text
unread_count > 0
```

thì:

- Name bold
- Message bold
- Hiện badge

---

## Ngày 5 — UI States

Xử lý:

- Loading
- Empty
- Success
- Error

---

## Ngày 6 — Refresh + Navigation

Thêm:

- Pull to refresh
- Click conversation
- Mở Chat Detail

### Kết quả tuần 4

```text
Login
  |
  v
Chat List
  |
  v
Chat Detail
```

---

# Tuần 5 — Chat Detail + Realtime

## Ngày 1 — Load message

```http
GET /conversations/:id/messages
```

Hiển thị:

```text
sender == me
           Hello

sender != me
Hi
```

---

## Ngày 2 — Send message

```http
POST /conversations/:id/messages
```

Luồng:

```text
TextField
   |
   v
Send
   |
   v
POST /messages
   |
   v
DB
   |
   v
Update UI
```

---

## Ngày 3 — UX Chat

Làm:

- Keyboard handling
- Scroll bottom
- Clear TextField
- Disable empty message
- Loading khi send
- Timestamp

Có thể thêm optimistic UI nếu còn thời gian.

---

## Ngày 4 — Read status

Khi mở Chat Detail:

```http
POST /conversations/:id/read
```

Backend:

```text
conversation_members.last_read_at = now()
```

Quay về Chat List:

```text
unread_count = 0
```

---

## Ngày 5 — WebSocket

Hiểu sự khác nhau:

### REST

```text
Flutter -> Server
```

### WebSocket

```text
Flutter <-> Server
```

---

## Ngày 6 — Realtime test

Test bằng 2 user:

```text
User A gửi
   |
   v
Go server
   |
   v
WebSocket
   |
   v
User B nhận ngay
```

### Kết quả tuần 5

- Load messages
- Send message
- Receive message
- WebSocket
- Scroll
- Read conversation
- Unread count

---

# Tuần 6 — Refactor Clean Architecture + Testing + Build

## Ngày 1 — Refactor Flutter theo Clean Architecture

Cấu trúc gợi ý:

```text
lib/

  core/
    error/
    network/
    router/
    storage/
    utils/

  features/

    auth/
      presentation/
        pages/
        widgets/
        view_models/

      domain/
        entities/
        repositories/
        usecases/

      data/
        datasources/
        models/
        repositories/

    chat_list/
      presentation/
        pages/
        widgets/
        view_models/

      domain/
        entities/
        repositories/
        usecases/

      data/
        datasources/
        models/
        repositories/

    chat_detail/
      presentation/
        pages/
        widgets/
        view_models/

      domain/
        entities/
        repositories/
        usecases/

      data/
        datasources/
        models/
        repositories/

  main.dart
```

Ví dụ flow login:

```text
LoginPage
   |
   v
AuthViewModel
   |
   v
LoginUseCase
   |
   v
AuthRepository
   |
   v
AuthRepositoryImpl
   |
   v
AuthRemoteDataSource
   |
   v
Go API
```

Nguyên tắc dependency:

```text
Presentation
     |
     v
Domain
     ^
     |
Data
```

Không làm:

```text
Widget -> Dio
Widget -> SecureStorage
Widget -> API
```

Thay vào đó:

```text
Widget
  |
  v
ViewModel
  |
  v
Use Case
  |
  v
Repository Interface
  |
  v
Repository Implementation
  |
  v
Data Source
```

---

## Ngày 2 — Refactor Golang theo Clean Architecture

Cấu trúc gợi ý:

```text
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

Luồng:

```text
Gin Handler
     |
     v
Use Case
     |
     v
Repository Interface
     |
     v
PostgreSQL Repository
     |
     v
Database
```

Dependency:

```text
Delivery
   |
   v
Use Case
   |
   v
Domain / Repository Interface
   ^
   |
Infrastructure
```

Business logic không đặt trực tiếp trong:

- Gin handler
- SQL query
- WebSocket handler

Business logic nên nằm trong:

```text
Use Case
```

---

## Ngày 3 — Edge cases

Test:

- Không có internet
- Token expired
- Wrong password
- Message rỗng
- Double click Send
- Server down
- Conversation không tồn tại
- Unauthorized access

---

## Ngày 4 — Testing

Manual test:

```text
Account A login
Account B login
A -> B message
B receives
B opens chat
Unread -> 0
B replies
A receives
```

Có thể viết test cho:

- Auth service
- Message service

---

## Ngày 5 — Build APK

```bash
flutter build apk
```

Test trên điện thoại thật:

- Login
- Logout
- Restart app
- Chat list
- Chat detail
- Send
- Receive
- Read status

---

## Ngày 6 — Documentation

README nên có:

- Giới thiệu project
- Tech stack
- Architecture
- How to run backend
- How to run database
- How to run Flutter
- API list
- Database schema
- Screenshot

---

# 12. Roadmap tổng thể

```text
WEEK 1
Dart
Flutter
UI
Navigation
    |
    v
STATIC APP

WEEK 2
Go
Gin
SQL
PostgreSQL
REST
    |
    v
BACKEND

WEEK 3
JWT
bcrypt
Flutter <-> Go
    |
    v
AUTH

WEEK 4
Conversation API
Chat List
Unread
    |
    v
CHAT LIST

WEEK 5
Messages
Send
Read
WebSocket
    |
    v
CHAT APP

WEEK 6
Fix
Refactor
Test
Build APK
    |
    v
DONE
```

---

# 13. Phân chia thời gian mỗi ngày

Nếu có khoảng 3 giờ/ngày:

```text
30 phút
Học lý thuyết

45 phút
Tutorial nhỏ

1 giờ 30 phút
Code trực tiếp project

15 phút
Ghi note + Git commit
```

Khuyến nghị:

```text
20% học
80% code
```

Tránh:

```text
90% xem video
10% code
```

---

# 14. Git cần học ngay từ tuần 1

Ví dụ commit:

```bash
git commit -m "feat: create auth screen"

git commit -m "feat: create conversation list"

git commit -m "feat: add login api"

git commit -m "feat: implement message sending"

git commit -m "feat: add websocket"
```

---

# 15. Những thứ không nên học trong 6 tuần

Không ưu tiên:

- Kubernetes
- Microservices
- Kafka
- Redis
- GraphQL
- Clean Architecture quá sâu
- BLoC nếu chưa hiểu state
- Riverpod nếu ChangeNotifier đã đủ
- Firebase
- Docker nâng cao
- CI/CD nâng cao
- Push Notification
- Upload image
- Voice message
- Group chat
- Video call

---

# 16. Scope MVP nên đóng băng

## Auth

- Register
- Login
- Logout
- JWT
- Remember login

## Chat List

- Avatar
- Name
- Last message
- Last message time
- Unread badge
- Read status

## Chat Detail

- Message history
- Send text
- Receive text realtime
- Timestamp
- Mark as read

## Không làm trong MVP

- Image
- File
- Voice
- Sticker
- Reaction
- Edit message
- Delete message
- Group
- Push notification
- Online/offline
- Typing indicator

---

# 17. Definition of Done

Project được coi là hoàn thành khi flow sau chạy ổn:

```text
1. User mở app

2. Login

3. Server trả JWT

4. Flutter lưu JWT

5. Vào Chat List

6. Hiển thị:
   Avatar
   Name
   Last message
   Unread count
   Last message time

7. Click conversation

8. Chat Detail load messages

9. Gửi:
   "Hello"

10. User khác nhận realtime

11. User khác reply

12. User hiện tại nhận realtime

13. Đọc message

14. Quay về Chat List

15. unread_count = 0

16. Đóng app

17. Mở lại

18. Vẫn đăng nhập
```

---

# 18. Thứ tự học cần bám sát

```text
Dart cơ bản
    |
    v
Flutter UI
    |
    v
Flutter Navigation
    |
    v
Go cơ bản
    |
    v
HTTP / REST
    |
    v
PostgreSQL / SQL
    |
    v
Auth / JWT
    |
    v
Flutter <-> Go
    |
    v
Chat List
    |
    v
Chat Detail
    |
    v
WebSocket
    |
    v
Testing
```

Điểm quan trọng nhất là bạn phải hiểu được luồng:

```text
Flutter
   |
   | POST /login
   v
Go
   |
   | SELECT
   v
PostgreSQL
   |
   v
Go
   |
   | JSON
   v
Flutter
```

Nếu hiểu được luồng này, bạn đã nắm được phần cốt lõi của ứng dụng full-stack Flutter + Golang.

---

# 19. Clean Architecture cần hiểu sau project

Sau project này, bạn cần tự giải thích được:

```text
Tại sao UI không gọi API trực tiếp?
Tại sao Domain không phụ thuộc Dio?
Tại sao Use Case không phụ thuộc PostgreSQL?
Repository Interface dùng để làm gì?
Repository Implementation khác Repository Interface thế nào?
Entity khác DTO / Model thế nào?
```

Ví dụ Flutter:

```text
ChatDetailPage
      |
      v
ChatDetailViewModel
      |
      v
SendMessageUseCase
      |
      v
MessageRepository
      |
      v
MessageRepositoryImpl
      |
      v
MessageRemoteDataSource
```

Ví dụ Go:

```text
HTTP Handler
     |
     v
SendMessageUseCase
     |
     v
MessageRepository Interface
     |
     v
PostgresMessageRepository
     |
     v
PostgreSQL
```

Nếu đổi PostgreSQL thành một database khác, Use Case gần như không cần thay đổi.

Nếu đổi Dio sang một HTTP client khác, Domain gần như không cần thay đổi.

Đây là giá trị chính của Clean Architecture.

---

# 20. Mục tiêu sau 6 tuần

Sau khi hoàn thành roadmap này, bạn sẽ có kiến thức thực hành về:

- Dart
- Flutter UI
- State Management
- Navigation
- REST API
- Golang
- Gin
- PostgreSQL
- SQL
- JWT
- bcrypt
- Authentication
- WebSocket
- Git
- API Testing
- Mobile App Architecture
- Backend Architecture
- Clean Architecture
- Entity
- Use Case
- Repository Interface
- Repository Implementation
- Data Source
- Dependency Rule

Và quan trọng nhất: bạn có một sản phẩm hoàn chỉnh có thể đưa lên GitHub làm portfolio.
