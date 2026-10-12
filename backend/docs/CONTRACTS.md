# Backend Contracts

Implemented through B34, extensions X01-X08, and profile P01-P14: **health, authentication and refresh tokens, user search, private/public profile reads and updates, opening direct 1-1 chats, conversation lists, messaging, history, read/unread state, and authenticated realtime `new_message` delivery**. [DECISIONS](DECISIONS.md) records U10/U11/U14/U15. Explain these contracts in Vietnamese.

## 1. General conventions

- REST uses `/api`; health uses `/health`; WebSocket uses `/ws`.
- Request/response JSON uses `snake_case`. IDs are positive integers, stored as bigint and handled as int64. Timestamps use UTC RFC 3339.
- Protected endpoints accept `Authorization: Bearer <access_token>`. Obtain identity from the verified token, not the body/query.
- Handlers validate JSON/path/query formats; use cases enforce business rules and authorization. Reject unsupported fields, including a forged `sender_id` in send-message requests.
- Return empty arrays as `[]`, not `null`; document nullable fields explicitly. Do not expose entities containing sensitive data directly as JSON responses.
- Every REST success uses `{"status":"success","data":...}`. Paginated responses add `meta.pagination`; lists live directly in `data`.
- REST client failures use `{"status":"fail","error":...}`. Server failures use `{"status":"error","error":...}`. `data` and `error` never coexist.
- `meta` and `error.details` are omitted when absent. WebSocket frames keep their separate `type`/`request_id`/`data|error` protocol.
- REST JSON bodies are limited to 16 KiB and return 413 when exceeded. WebSocket queue, frame, write, heartbeat, and shutdown limits are configured separately below.
- Invalid input returns 400. JSON endpoints require a JSON content type; unsupported content types return 415.

### Consistent errors

```json
{
  "status": "fail",
  "error": {
    "code": "invalid_input",
    "message": "The submitted information is invalid."
  }
}
```

| HTTP | Code | Situation |
|---|---|---|
| 400 | `invalid_input` | Invalid JSON/path/query/body, or a message marker outside the conversation the caller may access |
| 401 | `invalid_credentials` | Login email does not exist or password is incorrect; both cases use one shared response |
| 401 | `unauthenticated` | Missing, invalid, or expired access token |
| 401 | `invalid_refresh_token` | Missing, invalid, expired, or wrong-type refresh token |
| 404 | `not_found` | Conversation does not exist or the caller is not a member |
| 409 | `email_taken` | Registration email is already in use |
| 409 | `phone_number_taken` | A profile update supplies a phone number owned by another user |
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
  "created_at": "2026-09-18T03:00:00Z",
  "attachments": []
}
```

Proposed demo limits:

| Data | Rule |
|---|---|
| Name | Trim surrounding whitespace; 1–100 Unicode code points |
| Email | Trim and lowercase according to app policy; a plain address without a display name; at most 254 bytes; unique after normalization |
| Password | Do not trim/modify; at least 8 Unicode code points and at most 72 UTF-8 bytes; never truncate silently |
| Content | Nonblank content or at least one attachment required; at most 2,000 Unicode code points; preserve original text; see section 9 |
| Avatar | Profile increment uses the fixed URL `https://clipart-library.com/img/1816203.png`; clients cannot upload or edit it |

The 72-byte limit matches [Go's bcrypt package](https://pkg.go.dev/golang.org/x/crypto/bcrypt). It is a byte limit, not a 72-character limit; Vietnamese characters, for example, may use multiple bytes.

## 3. Endpoints and behavior

| Method/path | Auth | Input | Success |
|---|---|---|---|
| `GET /health` | No | None | 200 `{"status":"success","data":{"status":"ok"}}`; confirms only that the HTTP process responds, not DB health |
| `POST /api/auth/register` | No | `first_name`, optional `last_name`, `email`, `password` | 201 with the public user in `data`; no token issued |
| `POST /api/auth/login` | No | `email`, `password` | 200 as illustrated below |
| `POST /api/auth/refresh` | No | `refresh_token` | 200 with a new access token |
| `GET /api/users` | Yes | `q`, optional `limit`/`after_id` | 200 paginated public user summaries excluding the caller |
| `GET /api/users/me` | Yes | None | 200 authenticated user's private profile |
| `PATCH /api/users/me` | Yes | Any nonempty subset of editable profile fields | 200 updated private profile |
| `GET /api/users/{id}` | Yes | Positive user ID | 200 public profile, including when `{id}` is the caller |
| `POST /api/conversations/direct` | Yes | `user_id` | 201 new or 200 existing direct conversation |
| `GET /api/conversations` | Yes | None | 200 array of the caller's conversations |
| `GET /api/conversations/:id/messages` | Yes | Pagination query | 200 according to the history contract below |
| `POST /api/conversations/:id/messages` | Yes | `content`, optional `request_id` | 201 with the Message in `data` after commit |
| `POST /api/conversations/:id/read` | Yes | `last_read_message_id` | 200 with the effective updated marker |
| Upgrade `GET /ws` | Yes | Handshake described below | 101 after successful authentication |

User discovery and opening direct chats were added under U11. There is still no group chat or server-side logout/revocation. Newly registered users initially receive `[]` until opening a chat.

User discovery accepts a trimmed `q` of 2-254 Unicode code points. It searches literal name substrings without wildcard interpretation or exact email (case-insensitive). It never returns the caller or another user's email/hash. Optional `limit` defaults to 20 and is at most 50; `after_id` is a positive integer. Results use ascending user IDs with one extra row to determine `has_more`:

```json
{"status":"success","data":[{"id":2,"name":"Binh","avatar_url":"https://clipart-library.com/img/1816203.png"}],"meta":{"pagination":{"has_more":false,"next_after_id":null}}}
```

`POST /api/conversations/direct` accepts `{"user_id":2}`. It derives the caller from the Bearer token, rejects self-chat with 400 and missing users with 404. It returns `{"status":"success","data":{"id":10,"user":{"id":2,"name":"Binh","avatar_url":"https://clipart-library.com/img/1816203.png"}}}` with 201 on first creation and 200 on reuse. A and B receive the same conversation ID regardless of which opens first. No message is sent by opening it; subsequent message/history/read calls use the existing routes and membership checks. Application creation paths serialize by the unordered pair, including demo seeding. Manual database writes that bypass those paths are not covered by this deduplication rule.

### Profile contract

All three profile routes require `Authorization: Bearer <access_token>`. Handlers derive the caller from the verified token. Clients cannot supply an actor ID to private-profile operations.

`GET /api/users/me` returns the authenticated user's private profile:

```json
{
  "status": "success",
  "data": {
    "id": 1,
    "first_name": "Pao",
    "last_name": "Nguyen",
    "name": "Pao Nguyen",
    "date_of_birth": "2002-05-21",
    "phone_number": "+84901234567",
    "avatar_url": "https://clipart-library.com/img/1816203.png"
  }
}
```

`date_of_birth` and `phone_number` may be null. The API returns both split names and the derived combined name so an edit form never has to infer the name boundary.

`GET /api/users/{id}` always returns the public representation, even when `{id}` is the authenticated user's ID:

```json
{
  "status": "success",
  "data": {
    "id": 2,
    "name": "An Nguyen",
    "avatar_url": "https://clipart-library.com/img/1816203.png"
  }
}
```

It must not expose email, phone number, date of birth, password hash, or split name fields. A non-positive/malformed ID returns 400; an absent or inactive user returns 404.

`PATCH /api/users/me` accepts a nonempty subset of these fields only:

```json
{
  "first_name": "Pao",
  "last_name": "Nguyen",
  "date_of_birth": null,
  "phone_number": "+84901234567"
}
```

- Omitted fields remain unchanged.
- JSON null clears `date_of_birth` or `phone_number`.
- An empty string clears `last_name`; supplied `first_name` cannot be null or empty after trimming.
- `name` is derived and cannot be submitted directly.
- `email` and `avatar_url` are not editable; as unknown fields they make the request invalid.
- `{}` is invalid because it requests no change.
- A date of birth uses `YYYY-MM-DD`, must be a real calendar date, and cannot be later than the current calendar date. No minimum age rule applies.
- A phone number is optional but, when supplied, must already be `+` followed by 7-15 digits and must be unique. The server does not rewrite local numbers and does not perform OTP verification.
- Name UTF-8, NUL, trimming, and 100-code-point limits follow the registration name policy.

A successful PATCH returns the same private representation as `GET /api/users/me`. A phone conflict returns 409 `phone_number_taken`. Invalid fields, formats, dates, or an empty patch return 400 `invalid_input`. Existing JSON media-type and 16 KiB body rules apply.

The fixed avatar URL is returned consistently by profile and existing user-summary response surfaces. Uploading or editing avatars, changing email, OTP verification, and deleting/deactivating profiles are explicitly deferred.

Successful login:

```json
{
  "status": "success",
  "data": {
    "access_token": "<token>",
    "expires_at": "2026-09-19T03:00:00Z",
    "refresh_token": "<refresh-token>",
    "refresh_expires_at": "2026-10-19T03:00:00Z",
    "user": {"id": 1, "name": "An", "email": "an@example.test", "avatar_url": ""}
  }
}
```

The auth adapter signs/verifies tokens; middleware extracts the actor ID only from a token carrying `type=access`; use cases still enforce resource authorization. JWTs require a valid signature, an allowed algorithm, valid `exp` and `sub`, and the configured issuer/audience; fixed algorithm selection and audience verification follow [RFC 8725](https://www.rfc-editor.org/rfc/rfc8725.html). Access tokens default to 24 hours. Refresh tokens carry `type=refresh`, default to 30 days, cannot access protected resources, and may obtain a new access token through `POST /api/auth/refresh`. Access tokens cannot be used as refresh tokens.

This intentionally simple MVP does not persist, rotate, or revoke refresh tokens. A valid refresh token can be reused until expiration, and client logout only deletes its local tokens. Production hardening should use server-side sessions, hashed opaque refresh tokens, rotation/reuse detection, and revocation.

Conversation list (`data` is `[]` when empty):

```json
{
  "status": "success",
  "data": [{
    "id": 10,
    "user": {"id": 2, "name": "Binh", "avatar_url": ""},
    "last_message": "Hello!",
    "last_message_at": "2026-09-18T03:00:00Z",
    "unread_count": 1
  }]
}
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
  "status": "success",
  "data": [
    {"id": 100, "conversation_id": 10, "sender_id": 1, "content": "Hello!", "created_at": "2026-09-18T03:00:00Z", "attachments": []}
  ],
  "meta": {
    "pagination": {
      "has_more": false,
      "next_before_id": null,
      "next_after_id": null
    }
  }
}
```

When another page exists, `next_before_id` is the smallest returned ID in default/before mode; `next_after_id` is the largest returned ID in after mode. The inapplicable field is always null. Determine `has_more` by querying one extra record; both next fields are null when no next page remains. Clients may reverse descending pages to display older-to-newer messages.

Clients retain a synchronization checkpoint from successfully received REST pages, including the highest ID of the final page even when next is null. On reconnect, open WS, buffer incoming events, fetch all pages using `after_id` from that checkpoint, then merge buffered events by message ID. Without a checkpoint, load the newest page first and use before for older history. Do not advance the REST checkpoint solely because a WS event has a higher ID: events may arrive out of order or an earlier event may have been missed. An HTTP response and a WS event with the same ID represent one message.

## 5. Data model and invariants

Migrations 000001-000003 implement the text MVP; 000004 adds the attachment schema foundation below. Apply migrations explicitly; test verification does not migrate the development database:

| Table | Main fields | Required constraints |
|---|---|---|
| users | id, name, email, password_hash, avatar_url, created_at | PK id; unique normalized email; required data NOT NULL |
| conversations | id, created_at, updated_at | PK id; timestamps with time zone |
| conversation_members | conversation_id, user_id, last_read_at, **last_read_message_id** | Composite PK conversation_id/user_id; conversation and user FKs; both read fields nullable before reading |
| messages | id, conversation_id, sender_id, content, created_at | PK id; conversation/user FKs; sender must be a member; DB text length <=2000; current use case still requires nonblank text |
| attachments | id, conversation_id, owner_id, message_id, position, state, object_key/generation, declared and verified metadata, created_at, expires_at, session_cipher, cleanup_token/claimed_at/checked_at | Migrations 000004/000005; owner-member/same-conversation-message FKs; distinct positions 0-4; size/state/metadata/expiry CHECKs; encrypted session recovery and cleanup leases |

`last_read_message_id` alongside `last_read_at` was **confirmed by the user** as option 1 (U10, D08-D09). A composite foreign key and repository validation ensure that the target belongs to the same conversation. Both fields are null until the first read. The marker only advances; the timestamp changes only when it advances.

Initial indexes: unique email; membership `(user_id, conversation_id)` for conversation lookup; messages `(conversation_id, id)` for pagination/last-message lookup. Add other indexes only when real queries need them. Use `timestamptz` for time values; the server/DB assigns timestamps, and clients do not determine message ordering.

Development seed: exactly two sample users and one conversation with two members. Hash passwords with bcrypt and read demo values from development configuration; never use real credentials. Rerunning the seed must not duplicate users/conversations. Do not automatically seed on every production startup or reset the DB. Create a separate user C fixture when testing authorization.

### Sending messages and concurrent ordering

The use case validates content and membership. The adapter performs one atomic write: begin transaction → lock the conversation row with `FOR UPDATE` → enforce write authorization → insert message → update conversation.updated_at → commit. PostgreSQL holds this lock until transaction completion, preventing another transaction from taking a conflicting lock on the same row, as described in its [row-locking documentation](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS). The DB must allocate the message ID **after acquiring the lock**; every message-writing path, including any seeded messages, follows this rule. Use an increasing identity/sequence with `CACHE 1`, no cycling, no reset/reseed, and no manually assigned IDs in active data. Do not allocate IDs early or bypass the lock.

Serializing writes within a conversation prevents a higher ID from committing before a lower pending ID in that conversation. IDs may have gaps; do not depend on consecutiveness. An increasing sequence without the lock is insufficient for correct cursors/read markers. This is a simple choice for a single-server demo load, not an optimized large-scale design.

Assign insertion time from the server/DB clock after acquiring the lock, not from the client; `updated_at` must not move backward. Rollback leaves no partial update. Use the `database/sql` transaction API and execute all transaction work through that same transaction, following the [Go guide](https://go.dev/doc/database/execute-transactions). Do not pass `sql.Tx` into domain/use case code.

Publish `new_message` only after commit. Publication failure still produces a successful send response when the HTTP response can be delivered, because the message has been saved. If the client loses the response, the send outcome may be uncertain; use the same request_id and exact conversation/content/ordered attachment IDs for durable retry; unkeyed requests can duplicate.

### Marking messages as read

Request:

```json
{"last_read_message_id": 100}
```

Response:

```json
{"status":"success","data":{"conversation_id":10,"last_read_message_id":100}}
```

The client sends a marker only up to the newest message displayed in that conversation. It means "read through this point," including older messages, not proof that each historical message was individually viewed. The server verifies membership and that the target exists in this conversation; do not use `now()` as the boundary of read content.

Update the marker to the greater of its old value and the target in **one atomic operation**; a delayed older request must not move it backward. Return the effective marker, which may exceed the requested target if another device has advanced it. `last_read_at` records the server update time when the marker advances; it is informational and is not used to calculate unread counts.

Unread count equals messages in the conversation sent by someone other than the actor whose IDs exceed the marker; null means nothing has been read. For example, B has read through 100 and A sends 101 before read(100) completes: 101 remains unread. An uncommitted lower-ID message from A would break cursor semantics; the write-lock invariant above prevents this. Empty conversations need no read request; reject marker 0 or a marker from another conversation.

## 6. Realtime WebSocket

Bidirectional commands are implemented; [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md) contains full command/response examples and retry semantics.

Supported clients are Flutter Android, iOS and Web. Browser setup and refresh/reconnect examples are in [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md).

- Mobile supplies the Bearer token through a handshake header; [Dart WebSocket.connect](https://api.dart.dev/dart-io/WebSocket/connect.html) supports additional headers. Authenticate before upgrade; invalid tokens return HTTP 401. Do not put long-lived access tokens in URL query parameters.
- Browser calls `POST /api/ws/tickets` with Bearer and an approved Origin, with no request body. HTTP 201 returns `{"status":"success","data":{"ticket":"<43-character base64url value>","expires_at":"<RFC3339 timestamp>"}}` and `Cache-Control: no-store`. Expiry is the earlier of 60 seconds or access-token expiry. Tickets bind the verified identity and Origin, are consumed atomically before upgrade, and cannot be reused even after a failed upgrade. Wrong Origin does not consume another origin's ticket.
- Browser opens `GET /ws?ticket=<ticket>` without Authorization. Mobile keeps Bearer with no Origin/ticket. Reject mixed credentials, duplicate/empty ticket parameters, malformed queries and unknown query fields with 400 `invalid_input`. Missing/expired/used browser tickets return 401 `invalid_ws_ticket`; invalid mobile authentication returns 401 `unauthenticated`. Invalid/unapproved Origin returns 403 `origin_not_allowed`. Non-GET handling returns 405 `method_not_allowed` before upgrade when routed to the handler; the registered endpoint is GET only.
- Ticket issuance returns 401 `unauthenticated`, 403 `origin_not_allowed`, 400 `invalid_input`, 429 `rate_limited` at capacity, or 500 `internal_error`. Store holds at most 8 pending tickets per user and 4096 globally, purging expired entries during issuance. It is process-local; restart/revision changes lose outstanding tickets. Use a fresh ticket on every attempt. Redact ticket query URLs from infrastructure logs and use HTTPS/WSS outside local development.
- Pre-upgrade application errors use the HTTP fail/error envelope. Browser WebSocket cannot inspect their JSON/status; handle error/close and diagnose through REST issuance or server diagnostics. Ticket expiry does not close an established socket; original access-token expiry does.
- Derive connection identity from the token; clients cannot freely subscribe to other users/conversations. Determine recipients from stored membership.
- `new_message` contains the committed Message DTO. Deliver it to connections belonging to both members so multiple devices can synchronize; deduplicate by message ID.
- Send `send_message` or `mark_read` as one text JSON object with `type`, canonical lowercase UUID `request_id`, and `data`. Receive `message_sent`, `read_updated`, or a correlated `error`. Invalid/unparseable request IDs yield `request_id: null`. Unknown fields, missing/null required data and invalid types are rejected. Unsupported commands produce `unsupported_command`; binary closes with 1003; oversized input closes with 1009.
- Send retries share a durable `(sender_id, client_request_id)` key. Identical conversation/content/ordered attachment IDs returns the original message without republishing; different payload returns `request_conflict`. REST send accepts the same optional `request_id` (omitted/null means unkeyed), returning HTTP 409 for conflicts. Unkeyed REST retains its original behavior. Keys live as long as the message.
- Acknowledgement and fan-out ordering is unspecified. Merge by message ID. Read markers are monotonic and acknowledgements are private to the submitting socket.
- Each connection has a bounded buffer and one sequential writer; slow clients must not block the entire hub. Add ping/pong, deadlines, and goroutine cleanup on disconnect/shutdown. Select concrete limits during lifecycle work and record them in configuration.
- If the token expires while a socket is open, close it (close code 1008, a policy violation under [RFC 6455](https://www.rfc-editor.org/rfc/rfc6455.html#section-7.4.1)); the client must refresh or obtain a valid access token before reconnecting. Do not authenticate once and allow the socket to remain open indefinitely.
- `WEB_ALLOWED_ORIGINS` is a comma-separated exact http/https origin allowlist shared by REST CORS, ticket issuance and WS admission. Reject paths, wildcards, userinfo, query, fragment and invalid ports at startup; empty configuration disables cross-origin browser support. Duplicate/empty/null Origin headers are rejected. Origin is not identity authentication.
- REST CORS runs before Bearer middleware, allowing unauthenticated OPTIONS preflight and Authorization/Content-Type headers for approved origins, including application error responses and refresh. Cookies are not enabled. CORS rejection may be a bodyless 403; disallowed origins cannot read responses through browsers. Same-origin REST follows normal authentication, but browser WS still requires an explicitly approved Origin.

```json
{
  "type": "new_message",
  "data": {
    "id": 100,
    "conversation_id": 10,
    "sender_id": 1,
    "content": "Hello!",
    "created_at": "2026-09-18T03:00:00Z",
    "attachments": []
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
| `JWT_REFRESH_TTL` | Default `720h` (30 days); duration of at least 1s; refresh JWT lifetime for the simple MVP flow | U12 |
| `WEB_ALLOWED_ORIGINS` | Exact comma-separated http/https origins; empty disables cross-origin browser access/tickets; example `http://localhost:5173` | WEB-01 |
| A/B seed configuration | Demo emails/names and local passwords supplied by the operator; required only by the seed, not normal server execution | B17 |
| `WS_COMMAND_QUEUE_CAPACITY` | Default 16; integer 1-10,000 | WS2-04 |
| `WS_COMMAND_TIMEOUT` | Default 5s; duration of at least 1s | WS2-04 |
| `WS_COMMAND_RATE` | Default 10 commands/second; integer 1-10,000 per connection | WS2-04 |
| `WS_COMMAND_BURST` | Default 20; integer 1-10,000; exhausted bucket closes with 1008 (best-effort rate_limited error) | WS2-04 |
| `WS_SEND_QUEUE_CAPACITY` | Default 64; integer 1-10,000; bounded outbound events per connection | WS2-04 |
| `WS_MAX_MESSAGE_BYTES` | Default 16,384; integer 1-1,048,576; entire inbound WebSocket message limit | WS2-04 |
| `WS_WRITE_TIMEOUT` | Default 10s; duration of at least 1s | WS2-04 |
| `WS_PONG_TIMEOUT` | Default 60s; duration of at least 1s | WS2-04 |
| `WS_PING_INTERVAL` | Default 25s; duration of at least 1s and shorter than `WS_PONG_TIMEOUT` | WS2-04 |
| `WS_SHUTDOWN_TIMEOUT` | Default 10s; duration of at least 1s | WS2-04 |

Example files contain placeholders for secrets/passwords only. HTTP and WS use separate configured deadlines so short-request timeouts are not mistakenly applied to long-lived connections.

## 8. Required scenarios to verify incrementally

- Auth: duplicate email, including concurrent registration; incorrect passwords; expired/invalid-signature tokens; no unintended hash/token exposure.
- Authorization: non-member C cannot read/send/mark read in A–B's conversation or receive its events.
- Sending: whitespace-only content; transaction failures leave no partial message/conversation update; a successful commit with failed publication still means the message was saved.
- History: equal timestamps; multiple pages without cursor-boundary duplicates/omissions; concurrent sends and catch-up with after_id.
- Reading: older requests arriving later; multiple devices; reading through 100 does not mark 101; self-sent messages do not increase unread counts.
- WS: correct delivery to both users; disconnect/reconnect catch-up; full buffers; closure on token expiration; resource cleanup at shutdown.

Attach these scenarios to their related implementation items in [BACKLOG](BACKLOG.md); do not postpone all verification until the end.

## 9. Media contract

U19 confirms limits below; D15 defines transport/lifecycle choices. The user's later assignment authorizes the whole backend upgrade. MED-02 through MED-07 is implemented and wired; active OpenAPI paths/components, AsyncAPI schemas and Postman media group describe runtime. `x-media-contract` retains the design reference. Section 9 extends earlier text examples with optional text, attachments and ordered retry comparison. See [MEDIA_SETUP](MEDIA_SETUP.md) for enabling storage and pending MED-08 cloud/device acceptance.

### 9.1 Limits and their effects

| Limit | Exact value | Reason and tradeoff |
|---|---|---|
| Attachments/message | 0-5; mixed image/video allowed; ordered, distinct positive IDs | Bounds draft upload work, metadata and rendered media per message; albums beyond 5 require multiple messages |
| JPEG/PNG bytes | 1-10,485,760 inclusive | Bounds transfer/storage and image validation I/O; some original photos need client compression |
| Image pixels | width x height <=25,000,000; positive dimensions | Compressed bytes do not bound decoded RAM: 25 million RGBA pixels alone need about 100 million bytes, excluding decoder/buffer overhead |
| MP4 bytes | 1-52,428,800 inclusive | Bounds upload/inspection/storage work; duration depends on bitrate, not this byte limit |
| Video codecs | Exactly one H.264 video track, zero or one AAC audio track; no other tracks | Avoids server transcoding; container extension alone does not prove playback; device/profile compatibility still needs acceptance |
| Content | <=2,000 Unicode code points, valid UTF-8, no NUL; original text preserved | Preserves existing text limit; bytes, code points and visible graphemes differ |
| Signed GET lifetime | 900 seconds from issuance | Balances player use against bearer-link exposure; refresh as needed; long playback/new Range requests may need a new URL |
| Unsent eligibility | created_at +86,400 seconds | Gives interrupted drafts time to recover while bounding abandoned-file retention |
| Pending/user | At most 10 uploading/ready, unexpired rows across all conversations | Allows two five-file drafts; concurrent issuance must not exceed cap; expires does not imply immediate physical deletion |

No combined message-byte cap or video-duration cap is added: a five-video draft can total 250 MiB. Pending count alone is not a daily quota or hard cost bound. Pixel/size limits need bounded validation concurrency too. Factors affecting send/view: available bandwidth and retries, codecs/profiles, device memory/playback support, GCS/API IAM and browser CORS, verification deadlines, private-link expiry and orphan cleanup. Direct upload removes API transfer bottleneck but does not remove these limits.

### 9.2 Upload grant, completion and private access

All backend operations use authenticated identity and positive integer IDs. Clients cannot supply sender_id, owner_id, object_key, generation or bucket URLs. Authorization/membership checks precede object-specific lifecycle errors; unknown/inaccessible files/conversations return generic 404 not_found.

| Route | JSON input | Response data / activation |
|---|---|---|
| POST /api/conversations/:id/attachments | Exactly content_type and integer size_bytes | 201: id, upload_url, upload_method: PUT, upload_headers, content_type, size_bytes, expires_at; MED-03 |
| POST /api/attachments/:id/complete | Exactly `{}` with application/json | 200: id, kind, content_type, size_bytes, state: ready or attached, expires_at (date-time when ready, null when attached); MED-04 |
| GET /api/attachments/:id/access | No body | 200: id, url, expires_at; MED-04 |

All successes retain status: success/data; session/access-bearing responses have Cache-Control: no-store. Upload content_type is exactly image/jpeg, image/png or video/mp4, not extension-only detection. size_bytes must be positive integer within the type's limit. No idempotency key for grant issuance: retry after a lost response may create another pending slot; old grant is cleaned later. Do not promise an automatic grant lookup endpoint.

upload_url is a GCS resumable session URI, not a signed read URL. upload_headers contains fixed Content-Type equal to declared type. Client supplies dynamic Content-Range for chunks/status queries; Content-Length is body length, set by HTTP stack (browser JS cannot set it manually). Never send application JWT to GCS. Browser Origin must be forwarded by backend when creating its session and supplied by browser on subsequent requests.

Protocol choices: use GCS JSON resumable API, create-only object precondition and known total size. Client uses PUT chunks (8 MiB suggested, nonfinal chunks multiples of 256 KiB). 308 Resume Incomplete is progress, not failure or a redirect to follow; use returned Range to resume. Status query uses empty PUT with Content-Range: bytes */TOTAL. No Range on incomplete status means no bytes persisted. Final 200/201 only means object uploaded; call backend complete before sending. GCS completion/object JSON is not an application REST envelope. Read [GCS upload protocol](https://docs.cloud.google.com/storage/docs/performing-resumable-uploads) for exact wire behavior.

Application expires_at is not GCS session TTL. GCS sessions may remain active up to a week; expiry cleanup cancels with DELETE (JSON API cancellation success is 499), then reconciles objects. For lost/invalid session client requests a new grant and uploads from start. Do not claim a 24-hour application cutoff directly revokes GCS capability. Provider length-precondition enforcement, cancellation races and browser headers still require real GCS acceptance in MED-08.

Complete validates pinned object bytes and actual length/type/codec, not just stored Content-Type. Missing/incomplete object returns 409 attachment_not_ready. Actual size beyond limit returns 413 media_too_large; unsupported detected format/codec/track layout returns 415 unsupported_media_type; malformed bytes, invalid dimensions/pixel budget or declaration mismatch returns 422 invalid_media. Permanent validation failure makes upload unusable; client obtains a new grant for a corrected file. Inspection/storage timeout returns 503 storage_unavailable and leaves completion retryable. Internal configuration/tool failure returns 500 internal_error, not invalid_media.

Repeated complete for the same owner's ready file returns saved metadata; for an attached file returns state attached/expires_at null. Never reset state or rebind. Both require current conversation membership. Eligibility expiry rejects pending completion/send at now >= expires_at; attached files have no unsent expiry. Pending cap counts only unexpired uploading/ready rows; cleanup must recover failed/deleting/expired physical objects separately.

Access: any current conversation member may view attached media; only owner may preview ready/unexpired unsent media. Other states return 404 not_found. Sign read for exact validated generation; respect Range for video seeking. Return fresh URL and expiry on each access request. Membership is checked at issuance; an already-issued signed URL remains usable until its expiry, so immediate membership revocation is NOT promised. Client handles URL expiry by calling access again, not by resending the message. Player access may fail for removal/storage errors too: do not retry forever or assume every 403 is token expiry.

### 9.3 Send and returned message

REST send accepts only content, request_id and attachment_ids. content omitted means empty string; explicit null rejected. attachment_ids omitted means []; null rejected. Reject unknown fields, duplicate/nonpositive IDs and more than five files. At least one of nonblank content or nonempty attachments is required. All attachments must be owned by caller, belong to target conversation and be ready/unbound/unexpired. Preserve original text and attachment order.

REST request_id remains optional; null/omitted means existing unkeyed behavior, so network retries can duplicate text messages. Use a stable canonical lowercase UUID for every media send to get safe retry. WS send_message requires outer type/request_id/data; data accepts conversation_id, optional content and optional attachment_ids with same rules. No binary/base64 file upload over WS. Existing mark_read frames/auth transport remain unchanged.

One committed message contains id, conversation_id, sender_id, content, created_at and attachments (always array, [] for text-only). Each attachment contains exactly id, kind (image/video), content_type and size_bytes; no session URI/object key/signed URL. Same Message appears under REST data, history data items, WS message_sent.data and new_message.data. This extension does not add client_request_id to Message or request_id to broadcasts.

Keyed send comparison is exact conversation + original content + ORDERED attachment IDs. Identical retry returns original message without republishing, even if files are already attached or their former pending expiry has passed. Different payload returns request_conflict. Recheck membership before replay. Bind all attachments and update conversation within current transaction; publish only after commit. A file attached to another message cannot be reused. Media-only preview is Photo for one image, Video for one video, Media for multiple/mixed files; nonblank content takes precedence. One message increments unread once.

### 9.4 Error contract and client action

| HTTP | Code | Operation / client action |
|---|---|---|
| 400 | invalid_input | Malformed fields/JSON, duplicate IDs, empty text+attachments, bad UUID; correct request |
| 401 | unauthenticated | Backend auth failure; refresh access token before retry |
| 404 | not_found | Unknown/inaccessible conversation/attachment, including wrong owner/conversation; do not reveal existence |
| 409 | attachment_not_ready | Complete/send before validation-ready state or missing uploaded object; finish upload/complete before new send |
| 409 | attachment_unavailable | Complete references failed/deleting/deleted file, or send references unusable/already-bound file outside identical replay; remove/reupload it |
| 409 | request_conflict | Existing sender/request_id used for different conversation/content/order/IDs; new logical send needs new key |
| 410 | attachment_expired | Owner completes/sends expired pending file; new grant needed |
| 413 | media_too_large | Declared/actual file bytes beyond allowed size; reduce file |
| 413 | payload_too_large | Backend JSON body exceeds existing 16 KiB; never put file bytes in JSON |
| 415 | unsupported_media_type | Backend body is not application/json, unsupported declared/detected file format/codec/tracks; convert file or fix header |
| 422 | invalid_media | Malformed file, pixel budget/dimensions or actual type/length differs from allowed declaration; correct file and create new grant |
| 429 | upload_limit_exceeded | Pending cap reached; finish/send existing draft or wait for expiry |
| 429 | rate_limited | Operational upload issuance limit/user/process reached; Retry-After: 60 |
| 503 | storage_unavailable | Temporary GCS/validation I/O or inspection timeout; retry with bounded backoff |
| 500 | internal_error | Internal DB/config/probe setup error; no internals returned |

HTTP 4xx uses status: fail; 5xx uses status: error. Keep error.code/error.message and never expose SQL, probe output, signed/session URLs. Pending-cap 429 includes integer Retry-After seconds to earliest pending expiry (may become eligible sooner if files are sent). MED-07 operational issuance limit defaults to 60/min/user/process through MEDIA_UPLOADS_PER_MINUTE, separate from confirmed pending cap. It is not a global quota or billing ceiling.

API CORS exposes Retry-After for approved browser origins. Bucket CORS remains separately configured for direct upload/read.

WS send errors use existing type: error/request_id/error framing with codes invalid_input, not_found, attachment_not_ready, attachment_unavailable, attachment_expired, request_conflict or internal_error. Existing unsupported_command/rate_limited remain. Invalid/unparseable request ID returns request_id null. Upload-only errors (size/type/validation/storage/cap) occur over HTTP; WS send does no storage inspection. Reconnect catches up persisted metadata through REST, merging by message ID regardless of acknowledgement/event order.

### 9.5 Client integration boundary

Keep local draft content and upload IDs until send acknowledgement or keyed retry confirms persistence. If one upload fails, retry it or remove it; never silently send an incomplete selection. On URL expiry request access again. On grant loss/expiry create a new grant; expired draft does not get sent automatically. Display media-only messages even with content empty, and render the original ordered attachments in one message bubble. Use actual device/browser testing for codec profiles, resume, CORS, seek and URL refresh. Backend implements these behaviors; Flutter implementation and live storage acceptance remain MED-08.
