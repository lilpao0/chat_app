# Scope, Decisions, and Assumptions

## Current decisions - 2026-09-21

- **U09 (confirmed):** the user asked to continue through eligible items until a question needs their answer. This supersedes U02's one-item-per-turn stopping pace. Keep separate Vietnamese explanations and verification for each item. No commit, push, or deployment is implied.
- **I05 (implementation choice):** implement the existing local-demo auth contracts D02, D04-D06: /api routes, registration without automatic login, HS256 tokens with default 24h TTL, explicit public DTOs, no refresh/revocation. Use github.com/golang-jwt/jwt/v5 v5.3.1, checked against https://golang-jwt.github.io/jwt/usage/parse/ . Require valid signature, expiration, canonical positive int64 subject, issuer, audience and time claims. Secret is environment-only, at least 32 bytes. JWT_TTL must be at least 1s because token dates have second precision.
- Registration trims names, normalizes email by trimming/lowercasing, accepts plain addresses, rejects invalid UTF-8/NUL in name/email, and relies on DB uniqueness for concurrent registrations. Internal errors are not echoed to clients. Missing users and wrong passwords receive identical status/body; no claim of end-to-end timing equivalence is made.
- **U10 (confirmed, 2026-09-21):** the user selected option 1 for B16: last_read_message_id alongside last_read_at, monotonic read position, and locking each conversation before allocating message IDs (D08-D09). Implemented by migration 000002 and the message/read repositories.
- **I06 (implementation):** composite foreign keys enforce member-only sending and same-conversation read markers. Both read fields are null together until the first read. Chat timestamps use clock_timestamp; updated_at never regresses. Messages use GENERATED ALWAYS identity with CACHE 1 NO CYCLE. The adapter transaction acquires the conversation lock before inserting. No schema trigger is used for ordering; all application write paths follow the repository contract.
- **I07 (implementation):** cmd/seed reads SEED_A_NAME/EMAIL/PASSWORD and SEED_B_NAME/EMAIL/PASSWORD. It adds missing users and one exact two-member conversation under a transaction/advisory lock, reuses existing accounts unchanged, and never seeds automatically at startup. Supplied passwords do not reset existing users. Verification used isolated fixtures, not the development database.
- **Pending before B30:** Flutter mobile only or also Flutter Web. The answer determines WebSocket authentication transport; no WebSocket code is implemented yet.
- **U11 (confirmed, 2026-09-21):** the user explicitly requested that one user find another and chat directly. This supersedes the original demo-only restriction. Authenticated users may search registered accounts, open/reuse an exact 1-1 conversation, then use the existing send/history/read endpoints. Group chat remains outside scope.
- **I08 (implementation):** GET /api/users performs literal case-insensitive substring matching on name or exact case-insensitive email matching. Results exclude the actor and expose id/name/avatar_url only. Default limit 20, maximum 50; ascending ID pagination via after_id. Search requires at least two Unicode code points and at most 254; empty q is invalid. This keeps the response useful without returning other users' email/password hashes.
- **I09 (implementation):** POST /api/conversations/direct accepts only user_id, rejects self/missing users and returns the exact 1-1 conversation. 201 means created; 200 means reused. Both request orders use a transaction-scoped advisory lock keyed by the sorted user pair, so simultaneous requests and seed use the same path. No database uniqueness constraint exists for unordered user pairs: direct SQL writers bypassing this application path could still create duplicates. No conversation is created on simple search or user registration.

Created: 2026-09-18. This document separates actual requirements from design choices so agents do not infer additional features or permission to execute work.

## 1. Confirmed user decisions

| ID | Decision | Source |
|---|---|---|
| U01 | Initial preparation was documentation only; later implementation requires an assigned item | Original direct request; B01 subsequently assigned under U07 |
| U02 | Implement one small piece per turn, explain the code in Vietnamese, then stop so the user can read | Answer about the working pace |
| U03 | The MVP uses two pre-created demo accounts and one sample conversation | Answer about starting conversations |
| U04 | The documentation must allow another agent to read it and continue the Go backend | Direct request |
| U05 | Keep the guidance together in `backend/docs/`; do not keep an `AGENTS.md` at the repository root | Follow-up request about document organization |
| U06 | Write the backend documentation in English; keep user-facing explanations and code walkthroughs in Vietnamese | Follow-up request about language |
| U07 | Initialize the Go module as B01 using the existing project | Direct implementation request |
| U08 | After every small step, explicitly report what was completed in addition to explaining the code | Direct follow-up request; report in Vietnamese under U06 |

U01 does not prohibit future implementation turns when the user explicitly assigns an item. U02 remains in effect until the user changes the pace.

## 2. Inherited product scope

Source: [Six-week roadmap](../../ke-hoach-6-tuan-flutter-golang-chat-app-1.md), especially sections 7, 9, 10, and 15–17.

- Go + Gin, PostgreSQL through `database/sql`, bcrypt, JWT, and WebSocket.
- Clean Architecture; existing directories provide a skeleton for gradual development.
- Authentication, conversation lists, history, text messages, realtime delivery, and read/unread state.
- The users, conversations, conversation_members, and messages tables.
- Outside the MVP: group chat, files/images/voice, message editing/deletion, reactions, typing indicators, presence, push notifications, and calls. Do not add Redis, Kafka, microservices, or Kubernetes.
- Avatars are existing URLs/values or empty; there is no image upload feature.

The roadmap contains inconsistent endpoint prefixes and a simplified read-state schema. The choices below address these gaps; they are not quotations of requirements separately confirmed by the user.

## 3. Proposed implementation design

Every D item below is a **technical proposal in this documentation, not a choice separately confirmed by the user**. Explain relevant proposals in Vietnamese before implementing them. Small reversible choices can be resolved within an assigned task; ask according to AGENTS when a choice changes features/contracts or has difficult-to-reverse consequences. Do not ask again about confirmed U decisions.

| ID | Proposal | Reason / limitation |
|---|---|---|
| D01 | One Go module in backend, one server, one PostgreSQL database; use the existing layer structure | Fits the learning goal and avoids premature services and abstractions |
| D02 | Standardize REST under `/api`; keep `/health` and `/ws` separate | Resolves inconsistent prefixes in the roadmap |
| D03 | JSON IDs remain positive integers; DB columns use bigint and Go uses int64 | Follows source examples and is easy to learn. Revisit representation before changing the contract if JavaScript clients and very large IDs are introduced |
| D04 | Expiring access tokens with a configurable demo TTL of 24 hours; no refresh tokens or server-side revocation | Reopening the app reuses an unexpired token; expiration requires login. Client logout deletes the token and closes the socket but does not immediately invalidate the old token on the server |
| D05 | JWT uses one explicitly configured algorithm; propose HS256 for one server, checking signature, exp, sub, issuer, and audience | Clients must not select the algorithm; reject tokens missing expiration or identity |
| D06 | DTOs use `avatar_url`; login includes `expires_at`; registration returns a user without automatic login | Standardizes DB/API naming; `avatar` in the source is illustrative |
| D07 | Message pagination uses `before_id` or `after_id`, default limit 20, maximum 100; conversations return an array for the small demo | Supports history and reconnect catch-up without a general cursor framework |
| D08 | Add `last_read_message_id` alongside `last_read_at`; the client sends the last displayed message ID | Avoids marking unseen messages as read based on the server's current time; adds schema/request fields beyond the source |
| D09 | Every message insert locks the same conversation row in a transaction before allocating its message ID; read markers only advance | Establishes commit order within a conversation so ID cursors and read markers remain correct under concurrent sends. A sequence alone does not guarantee commit order |
| D10 | Send through REST; WS provides best-effort notifications about persisted messages within one process | The DB is authoritative; event failures do not undo persistence; reconnect uses REST catch-up |
| D11 | Sending a message has no idempotency key yet | Retrying after a lost response may create duplicates. Do not promise exactly-once behavior; client double-click prevention only reduces the risk |
| D12 | Both a nonexistent conversation and a non-member caller receive 404 from chat endpoints | Avoids distinguishing resources the caller cannot access; invalid/expired tokens still receive 401 |
| D13 | Start with minimal Clean Architecture instead of postponing it to the final week's refactor | Teaches the intended flow and avoids rewriting large sections; add layers only as the current item needs them |

Explain D08–D09 clearly in Vietnamese before the chat migration item in BACKLOG because they change the source schema and read contract. If the user wants to keep the roadmap's original `last_read_at` approach, update CONTRACTS and the checks before implementation; do not silently mix two unread calculations.

Resolved during B05: **I02 — migration tool and local databases (2026-09-18).** Use the already installed `migrate` CLI with PostgreSQL and ordered SQL files under `backend/migrations/`. On Windows, the CLI needs an absolute forward-slash path passed through `-path`; a relative backslash path produces an invalid source URL. `chat_app` is the development database and `chat_app_test` is the isolated test database, both exposed by Docker on host port `5433`. This is a local implementation choice; production connection values remain operator configuration.

## 4. Deliberately open questions

Resolved during B01: **I01 — module/toolchain selection (2026-09-18).** Selected `github.com/lilpao0/chat_app/backend` from the existing origin `https://github.com/lilpao0/chat_app.git` and the backend subdirectory. `go mod init` recorded `go 1.27.1`, matching the installed Windows/amd64 toolchain. This is an implementation choice made within the authorized B01 scope, not a new product requirement. Module recognition and the existing entrypoint build both passed. No third-party dependencies were added.

| Open point | When to resolve | Approach |
|---|---|---|
| Gin, DB driver, JWT/WS library, migration tool, and PostgreSQL versions | First item needing each tool | Check official documentation and Go compatibility; select and record specific versions. Keep `database/sql` as the DB API; libraries are not selected during documentation preparation |
| Flutter mobile only or also Flutter Web | Before the WS handshake item | Current assumption: mobile, based on the roadmap's APK. Mobile can use auth headers; if Web is included, design a browser-compatible handshake first without putting long-lived tokens in URLs |
| Production token TTL/refresh/revocation | Before expanding auth or deploying for real use | Demo proposal D04 does not promise permanent login or immediate server-side logout |
| Existing or new local DB address, port, and credentials | Resolved in B05 | Docker PostgreSQL uses host port 5433; keep development and test databases separate as recorded in I02 |

These questions do not authorize implementing unassigned items. Ask when an assigned small item needs the answer rather than requiring the user to decide the entire system in advance.

## 5. MVP limitations

- Newly registered users without a conversation initially receive an empty list. Under U11, they can search existing users and open a direct 1-1 chat; the A/B seed remains an optional demo fixture.
- Read/unread state refers to the current user's own marker and unread count. Recipient read ticks and read-receipt events are not included yet.
- One server holds the WebSocket hub in memory. Restarting it loses active connections; committed messages remain in PostgreSQL.
- Restarting the app does not guarantee that its JWT is valid; the client reuses the stored token until expiration, then logs in again.
- Persisted content is text, subject to CONTRACTS limits and formatting. Do not expand it into rich text or media automatically.

## 6. Recording a new decision

Record its ID, date, status, problem, choice, reason, API/schema impact, and confirmation source in English. If it replaces an earlier decision, identify the superseded decision. Use the label "confirmed by the user" only when a real instruction or answer supports it. Explain the decision to the user in Vietnamese.

## I03 ? Three application layers (2026-09-21)

Confirmed by the user's explicit request: group backend application code into `presentation`, `domain`, and `data` for readability. Domain contains entities, repository interfaces, and use cases. Data contains concrete persistence/auth adapters. Presentation contains HTTP and WebSocket adapters. `cmd/api` wires the layers and owns configuration under `cmd/api/config`. Active migrations remain in `backend/migrations`; old `internal/migrate` SQL is preserved under `data/database/legacy_migrations` and must not be applied. This replaces the earlier flat package layout without changing APIs or database behavior.

## I04 - Password adapter (2026-09-21)

For B09, reuse the existing pinned golang.org/x/crypto v0.9.0 bcrypt package and its default cost 10; no unrelated dependency upgrade. Domain owns PasswordHasher and ValidatePassword under domain/usecase/auth. Data owns BcryptPasswordHasher. Hash and Compare enforce the existing 8-code-point/72-byte contract, reject invalid UTF-8, and preserve whitespace. Compare distinguishes mismatches (false, nil) from invalid input or broken stored hashes (error). Future login code must map credential failures to the shared public response. Library-specific errors do not cross the interface. The adapter uses no I/O context because bcrypt has no cancellation API. Source: https://pkg.go.dev/golang.org/x/crypto/bcrypt
