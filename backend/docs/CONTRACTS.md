# Proposed Backend Contracts

Implemented through B29 and extension X01-X03: **health, authentication, user search, opening direct 1-1 chats, conversation lists, messaging, history and read/unread state**. WebSocket remains planned. [DECISIONS](DECISIONS.md) records U10/U11. Explain these contracts in Vietnamese.

## 1. General conventions

- REST uses `/api`; health uses `/health`; WebSocket uses `/ws`.
- Request/response JSON uses `snake_case`. IDs are positive integers, stored as bigint and handled as int64. Timestamps use UTC RFC 3339.
- Protected endpoints accept `Authorization: Bearer <access_token>`. Obtain identity from the verified token, not the body/query.
- Handlers validate JSON/path/query formats; use cases enforce business rules and authorization. Reject unsupported fields, including a forged `sender_id` in send-message requests.
- Return empty arrays as `[]`, not `null`; document nullable fields explicitly. Do not expose entities containing sensitive data directly as JSON responses.
- Proposed body limit: 16 KiB for current JSON requests; return 413 when exceeded. Set HTTP/DB timeouts and introduce connection/frame-size limits when implementing WebSocket.
- Invalid input returns 400. JSON endpoints require a JSON content type; unsupported content types return 415.

### Consistent errors

```json
{
  "error": {
    "code": "invalid_input",
    "message": "The submitted information is invalid."
  }
}
```

| HTTP | Code | Situation |
|---|---|---|
| 400 | `invalid_input` | Invalid JSON/path/query/body, or a message marker outside the conversation the caller may access |
| 401 | `unauthenticated` | Missing, invalid, or expired token; incorrect login email/password uses one shared message |
| 404 | `not_found` | Conversation does not exist or the caller is not a member |
| 409 | `email_taken` | Registration email is already in use |
| 413 | `payload_too_large` | Body exceeds the limit |
| 415 | `unsupported_media_type` | Incorrect content type for a JSON endpoint |
| 500 | `internal_error` | Server error without exposed internal details |

Clients branch on `code`, not the wording of `message`. Login errors must not distinguish a nonexistent email from an incorrect password. DB errors/stack traces are for server diagnostics and must not appear in responses.

## 2. Shared DTOs and validation

Public user:

```json
{
  "id": 1,
  "name": "An",
  "email": "an@example.test",
  "avatar_url": ""
}
```

A conversation's user summary contains only `id`, `name`, and `avatar_url`; do not return the other participant's email. Never return `password_hash`.

Message:

```json
{
  "id": 100,
  "conversation_id": 10,
  "sender_id": 1,
  "content": "Hello!",
  "created_at": "2026-09-18T03:00:00Z"
}
```

Proposed demo limits:

| Data | Rule |
|---|---|
| Name | Trim surrounding whitespace; 1–100 Unicode code points |
| Email | Trim and lowercase according to app policy; a plain address without a display name; at most 254 bytes; unique after normalization |
| Password | Do not trim/modify; at least 8 Unicode code points and at most 72 UTF-8 bytes; never truncate silently |
| Content | Must be nonempty after trimming for validation; at most 2,000 Unicode code points; preserve original text to retain intentional whitespace/newlines |
| Avatar | Return an existing URL or an empty string; registration does not accept avatars/uploads yet |

The 72-byte limit matches [Go's bcrypt package](https://pkg.go.dev/golang.org/x/crypto/bcrypt). It is a byte limit, not a 72-character limit; Vietnamese characters, for example, may use multiple bytes.

## 3. Endpoints and behavior

| Method/path | Auth | Input | Success |
|---|---|---|---|
| `GET /health` | No | None | 200 `{"status":"ok"}`; confirms only that the HTTP process responds, not DB health |
| `POST /api/auth/register` | No | `name`, `email`, `password` | 201 `{"user": <Public user>}`; no token issued |
| `POST /api/auth/login` | No | `email`, `password` | 200 as illustrated below |
| `GET /api/users` | Yes | `q`, optional `limit`/`after_id` | 200 paginated public user summaries excluding the caller |
| `POST /api/conversations/direct` | Yes | `user_id` | 201 new or 200 existing direct conversation |
| `GET /api/conversations` | Yes | None | 200 array of the caller's conversations |
| `GET /api/conversations/:id/messages` | Yes | Pagination query | 200 according to the history contract below |
| `POST /api/conversations/:id/messages` | Yes | `content` | 201 `{"message": <Message>}` after commit |
| `POST /api/conversations/:id/read` | Yes | `last_read_message_id` | 200 with the effective updated marker |
| Upgrade `GET /ws` | Yes | Handshake described below | 101 after successful authentication |

User discovery and opening direct chats were added under U11. There is still no group chat, refresh-token, server-logout, or `/me` API. Newly registered users initially receive `[]` until opening a chat.

User discovery accepts a trimmed `q` of 2-254 Unicode code points. It searches literal name substrings without wildcard interpretation or exact email (case-insensitive). It never returns the caller or another user's email/hash. Optional `limit` defaults to 20 and is at most 50; `after_id` is a positive integer. Results use ascending user IDs with one extra row to determine `has_more`:

```json
{"items":[{"id":2,"name":"Binh","avatar_url":""}],"has_more":false,"next_after_id":null}
```

`POST /api/conversations/direct` accepts `{"user_id":2}`. It derives the caller from the Bearer token, rejects self-chat with 400 and missing users with 404. It returns `{"conversation":{"id":10,"user":{"id":2,"name":"Binh","avatar_url":""}}}` with 201 on first creation and 200 on reuse. A and B receive the same conversation ID regardless of which opens first. No message is sent by opening it; subsequent message/history/read calls use the existing routes and membership checks. Application creation paths serialize by the unordered pair, including demo seeding. Manual database writes that bypass those paths are not covered by this deduplication rule.

Successful login:

```json
{
  "access_token": "<token>",
  "expires_at": "2026-09-19T03:00:00Z",
  "user": {"id": 1, "name": "An", "email": "an@example.test", "avatar_url": ""}
}
```

The auth adapter signs/verifies tokens; middleware extracts the actor ID from a valid token; use cases still enforce resource authorization. JWTs require a valid signature, an allowed algorithm, valid `exp` and `sub`, and the configured issuer/audience; fixed algorithm selection and audience verification follow [RFC 8725](https://www.rfc-editor.org/rfc/rfc8725.html). The proposed demo TTL is 24 hours; expiration requires login again. Client logout deletes the stored token and closes the socket; the server does not revoke the old token yet. Do not promise permanent sessions.

Conversation list:

```json
[
  {
    "id": 10,
    "user": {"id": 2, "name": "Binh", "avatar_url": ""},
    "last_message": "Hello!",
    "last_message_at": "2026-09-18T03:00:00Z",
    "unread_count": 1
  }
]
```

`user` is the other participant in the 1–1 conversation. Conversations without messages return `last_message: null`, `last_message_at: null`, and `unread_count: 0`. Select the last message by the conversation's highest message ID, not timestamp alone. Sort conversations by descending `conversations.updated_at`, breaking ties by descending conversation ID. The small demo does not paginate this list; return only conversations where the caller is a member.

## 4. History and missed-message synchronization

By default, return the latest 20 messages; `limit` must be between 1 and 100. Allow at most one of `before_id` and `after_id`; specifying both or invalid values returns 400. A cursor is a positive numeric boundary within a conversation query; a message with that exact ID does not have to exist.

| Query | Condition and result order | Purpose |
|---|---|---|
| No cursor | Latest messages, descending ID | Open Chat Detail |
| `before_id=100` | IDs below 100, descending | Load older messages |
| `after_id=100` | IDs above 100, ascending | Catch up on messages since the last successful fetch |

```json
{
  "items": [
    {"id": 100, "conversation_id": 10, "sender_id": 1, "content": "Hello!", "created_at": "2026-09-18T03:00:00Z"}
  ],
  "has_more": false,
  "next_before_id": null,
  "next_after_id": null
}
```

When another page exists, `next_before_id` is the smallest returned ID in default/before mode; `next_after_id` is the largest returned ID in after mode. The inapplicable field is always null. Determine `has_more` by querying one extra record; both next fields are null when no next page remains. Clients may reverse descending pages to display older-to-newer messages.

Clients retain a synchronization checkpoint from successfully received REST pages, including the highest ID of the final page even when next is null. On reconnect, open WS, buffer incoming events, fetch all pages using `after_id` from that checkpoint, then merge buffered events by message ID. Without a checkpoint, load the newest page first and use before for older history. Do not advance the REST checkpoint solely because a WS event has a higher ID: events may arrive out of order or an earlier event may have been missed. An HTTP response and a WS event with the same ID represent one message.

## 5. Data model and invariants

This schema is implemented by migrations 000001 and 000002. Apply them explicitly; test verification does not migrate the development database:

| Table | Main fields | Required constraints |
|---|---|---|
| users | id, name, email, password_hash, avatar_url, created_at | PK id; unique normalized email; required data NOT NULL |
| conversations | id, created_at, updated_at | PK id; timestamps with time zone |
| conversation_members | conversation_id, user_id, last_read_at, **last_read_message_id** | Composite PK conversation_id/user_id; conversation and user FKs; both read fields nullable before reading |
| messages | id, conversation_id, sender_id, content, created_at | PK id; conversation/user FKs; sender must be a member; nonempty content according to application rules |

`last_read_message_id` alongside `last_read_at` was **confirmed by the user** as option 1 (U10, D08-D09). A composite foreign key and repository validation ensure that the target belongs to the same conversation. Both fields are null until the first read. The marker only advances; the timestamp changes only when it advances.

Initial indexes: unique email; membership `(user_id, conversation_id)` for conversation lookup; messages `(conversation_id, id)` for pagination/last-message lookup. Add other indexes only when real queries need them. Use `timestamptz` for time values; the server/DB assigns timestamps, and clients do not determine message ordering.

Development seed: exactly two sample users and one conversation with two members. Hash passwords with bcrypt and read demo values from development configuration; never use real credentials. Rerunning the seed must not duplicate users/conversations. Do not automatically seed on every production startup or reset the DB. Create a separate user C fixture when testing authorization.

### Sending messages and concurrent ordering

The use case validates content and membership. The adapter performs one atomic write: begin transaction → lock the conversation row with `FOR UPDATE` → enforce write authorization → insert message → update conversation.updated_at → commit. PostgreSQL holds this lock until transaction completion, preventing another transaction from taking a conflicting lock on the same row, as described in its [row-locking documentation](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS). The DB must allocate the message ID **after acquiring the lock**; every message-writing path, including any seeded messages, follows this rule. Use an increasing identity/sequence with `CACHE 1`, no cycling, no reset/reseed, and no manually assigned IDs in active data. Do not allocate IDs early or bypass the lock.

Serializing writes within a conversation prevents a higher ID from committing before a lower pending ID in that conversation. IDs may have gaps; do not depend on consecutiveness. An increasing sequence without the lock is insufficient for correct cursors/read markers. This is a simple choice for a single-server demo load, not an optimized large-scale design.

Assign insertion time from the server/DB clock after acquiring the lock, not from the client; `updated_at` must not move backward. Rollback leaves no partial update. Use the `database/sql` transaction API and execute all transaction work through that same transaction, following the [Go guide](https://go.dev/doc/database/execute-transactions). Do not pass `sql.Tx` into domain/use case code.

Publish `new_message` only after commit. Publication failure still produces a successful send response when the HTTP response can be delivered, because the message has been saved. If the client loses the response, the send outcome may be uncertain; POST is not idempotent yet, so do not retry unconditionally or promise duplicate-free sending.

### Marking messages as read

Request:

```json
{"last_read_message_id": 100}
```

Response:

```json
{"conversation_id": 10, "last_read_message_id": 100}
```

The client sends a marker only up to the newest message displayed in that conversation. It means "read through this point," including older messages, not proof that each historical message was individually viewed. The server verifies membership and that the target exists in this conversation; do not use `now()` as the boundary of read content.

Update the marker to the greater of its old value and the target in **one atomic operation**; a delayed older request must not move it backward. Return the effective marker, which may exceed the requested target if another device has advanced it. `last_read_at` records the server update time when the marker advances; it is informational and is not used to calculate unread counts.

Unread count equals messages in the conversation sent by someone other than the actor whose IDs exceed the marker; null means nothing has been read. For example, B has read through 100 and A sends 101 before read(100) completes: 101 remains unread. An uncommitted lower-ID message from A would break cursor semantics; the write-lock invariant above prevents this. Empty conversations need no read request; reject marker 0 or a marker from another conversation.

## 6. Realtime WebSocket

The current assumption is Flutter mobile. Verify the target platform before implementing the handshake; if Flutter Web is included, revise authentication transport for browser capabilities.

- Mobile supplies the Bearer token through a handshake header; [Dart WebSocket.connect](https://api.dart.dev/dart-io/WebSocket/connect.html) supports additional headers. Authenticate before upgrade; invalid tokens return HTTP 401. Do not put long-lived access tokens in URL query parameters.
- Derive connection identity from the token; clients cannot freely subscribe to other users/conversations. Determine recipients from stored membership.
- `new_message` contains the committed Message DTO. Deliver it to connections belonging to both members so multiple devices can synchronize; deduplicate by message ID.
- Sending remains REST-based. The MVP accepts no WS commands to create messages or change read markers; unsupported client data frames are rejected/the connection closed according to the protocol documented during implementation.
- Each connection has a bounded buffer and one sequential writer; slow clients must not block the entire hub. Add ping/pong, deadlines, and goroutine cleanup on disconnect/shutdown. Select concrete limits during lifecycle work and record them in configuration.
- If the token expires while a socket is open, close it (proposed close code 1008, a policy violation under [RFC 6455](https://www.rfc-editor.org/rfc/rfc6455.html#section-7.4.1)); the client must log in before reconnecting. Do not authenticate once and allow the socket to remain open indefinitely.
- If a browser Origin is present, apply a configured allowlist; HTTP CORS does not replace WS Origin validation. Do not default to a wildcard.

```json
{
  "type": "new_message",
  "data": {
    "id": 100,
    "conversation_id": 10,
    "sender_id": 1,
    "content": "Hello!",
    "created_at": "2026-09-18T03:00:00Z"
  }
}
```

The in-memory hub serves one server. It does not guarantee complete event delivery, absolute ordering, event replay, or exactly-once delivery. REST and PostgreSQL recover persisted messages; clients combine reconnects with the synchronization procedure in section 4.

## 7. Proposed development configuration

The API and seed commands load optional `.env` values from the current working directory; explicit process environment variables take precedence. `.env.example` documents placeholders. Tests use `TEST_DATABASE_URL` from the process environment. JWT variables are implemented through B12.

| Variable | Proposed convention | First needed |
|---|---|---|
| `HTTP_PORT` | Default 8080; integer 1–65535; invalid values cause a clear startup failure | B02 |
| `DATABASE_URL` | Required when connecting the DB; targets a separate DB; never log a complete string containing a password | B05–B07 |
| `JWT_SECRET` | Random secret of at least 32 bytes for HS256; no public default | B12 |
| `JWT_ISSUER` | Proposed default `chat-app` | B12 |
| `JWT_AUDIENCE` | Proposed default `chat-app-mobile`; adjust if additional platforms are confirmed | B12 |
| `JWT_TTL` | Default `24h`; duration of at least 1s for JWT second precision; a demo value rather than production policy | B12 |
| A/B seed configuration | Demo emails/names and local passwords supplied by the operator; required only by the seed, not normal server execution | B17 |
| WS origin, buffer, timeout | Select and document concrete values during lifecycle work; no default wildcard origin | B30–B31 |

Example files contain placeholders for secrets/passwords only. Select, explain in Vietnamese, and document HTTP read/write/idle timeouts and the shutdown timeout in B04. WS receives its own deadlines in B31 so short-request timeouts are not mistakenly applied to long-lived connections.

## 8. Required scenarios to verify incrementally

- Auth: duplicate email, including concurrent registration; incorrect passwords; expired/invalid-signature tokens; no unintended hash/token exposure.
- Authorization: non-member C cannot read/send/mark read in A–B's conversation or receive its events.
- Sending: whitespace-only content; transaction failures leave no partial message/conversation update; a successful commit with failed publication still means the message was saved.
- History: equal timestamps; multiple pages without cursor-boundary duplicates/omissions; concurrent sends and catch-up with after_id.
- Reading: older requests arriving later; multiple devices; reading through 100 does not mark 101; self-sent messages do not increase unread counts.
- WS: correct delivery to both users; disconnect/reconnect catch-up; full buffers; closure on token expiration; resource cleanup at shutdown.

Attach these scenarios to their related implementation items in [BACKLOG](BACKLOG.md); do not postpone all verification until the end.
