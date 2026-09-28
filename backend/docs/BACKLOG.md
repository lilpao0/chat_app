# Backend Backlog: Small Steps

### 2026-09-21 - Auth test directory organization

- **Completed:** moved JWT/password tests to `internal/data/auth/test` and register/login tests to `internal/domain/usecase/auth/test`, as requested by the user. Each directory uses package auth_test and imports the parent package's exported API.
- **Adjustment:** JWT expiry coverage now signs an already expired token instead of changing the adapter's private clock. Production code and public APIs are unchanged.
- **Verified:** `go test -count=1 ./...` and `go vet ./...` passed. PostgreSQL integration tests skipped because TEST_DATABASE_URL was not set; no DB logic changed. Updated the targeted test command to include subdirectories.
- **Explained in Vietnamese:** co-located tests are conventional in Go; separate directories are separate packages and can access only exported parent APIs.
- **Next:** B16 still awaits the read-position choice; this directory change does not resolve D08-D09.

**Latest handoff (2026-09-28):** B01-B29, X01-X08, and P01-P14 are DONE. See the latest handoff entries below and [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md). U15 confirms Android/iOS-only realtime clients, so B30 is unblocked and remains the next runtime item. The user authorizes continuation until a question needs their answer (U09).

### X04 — DONE — Simple refresh-token flow

- **Scope:** login issues typed access and refresh JWTs; public `POST /api/auth/refresh` exchanges a valid refresh token for a new access token. `JWT_REFRESH_TTL` defaults to 720h.
- **Security boundary:** access and refresh tokens cannot substitute for each other. This simple user-requested version has no persistence, rotation, reuse detection, logout endpoint, or server-side revocation.
- **Verification:** adapter, use-case, handler, configuration, and production-router tests cover successful refresh, invalid/empty tokens, wrong token type, expiry/error mapping, and protected-route rejection.

### X05 — DONE — Mandatory real-database test audit

- **Scope:** review registration, login, refresh, JWT middleware, and PostgreSQL integration without allowing database tests to skip.
- **Changed:** created the isolated local `chat_app_test` database; centralized DB tests on `testutil.Database`; load the current consolidated migration in a random private schema; derive the local test URL from `.env` when `TEST_DATABASE_URL` is absent; fail unless the actual database name is `chat_app_test`.
- **Fixes found by mandatory tests:** updated the auth integration request from obsolete `name` to `first_name`/`last_name`; updated conversation listing from removed `users.name` to the current split-name schema.
- **Verification:** no Go `t.Skip`/`Skipf` remains. Full tests execute real PostgreSQL coverage and pass. Auth coverage includes duplicate normalized registration, bcrypt persistence, shared credential errors, complete login token response, token-type separation, refresh expiry/failure, refreshed access, and secret exclusion. Swagger UI was visually checked through Computer Use.

### X06 — DONE — Enforce OpenAPI/code parity

- **Scope:** make the Swagger contract exact for the currently implemented REST system and fail tests when production routes or registration validation errors drift.
- **Changed:** centralized authenticated route registration in `RegisterProtectedRoutes`; production wiring and contract tests now use the same Gin route table. Added exact registration error-code enums and precise trim, Unicode-code-point, UTF-8-byte, NUL, and preserved-content rules to OpenAPI.
- **Verification:** contract tests compare all application method/path pairs, public versus Bearer-protected operations, required registration fields, unknown-field rejection, and error codes derived from the actual exported domain validation errors. All OpenAPI references and presentation tests pass.

### X07 — DONE — Separate HTTP 401 error codes

- **Scope:** let clients distinguish failed login, failed access-token authentication, and failed refresh-token authentication without revealing whether a login email exists.
- **Contract:** login returns `invalid_credentials`; protected middleware returns `unauthenticated`; refresh returns `invalid_refresh_token`. Each uses HTTP 401, and unknown-email/wrong-password login responses remain identical.
- **Verification:** handler and real PostgreSQL integration tests assert `invalid_credentials` and identical credential-failure bodies. OpenAPI contract tests lock all three 401 mappings.

### X08 — DONE — Synchronize the Postman REST collection

- **Scope:** update the runnable Postman flow to match the current registration, login, refresh, chat, and error contracts.
- **Changed:** registration uses `first_name`/`last_name`; login captures access and refresh tokens plus expirations; added access-token refresh and explicit checks for `invalid_credentials`, `unauthenticated`, and `invalid_refresh_token`. The runner now contains 17 ordered requests and covers every OpenAPI operation.
- **Follow-up:** removed the email generator, all pre-request scripts, and every script that mutates Postman variables. IDs and tokens are copied manually using descriptions on the collection variables; remaining test scripts only assert responses and never change data.
- **Verification:** the collection parses as Postman Collection v2.1 JSON. A Go contract test checks that every OpenAPI method/path is represented and that registration bodies use the current fields. Newman was not installed, so no Newman execution was claimed.

## User-requested extension (U11)

### X01 — DONE — Search existing users

- **Scope:** authenticated case-insensitive literal name substring or exact email search, bounded ID pagination, actor exclusion and public summaries only.
- **Verification:** domain validation tests and real PostgreSQL/HTTP integration cover multiple pages, exact email, literal wildcard characters, invalid/duplicate query fields, no token, actor exclusion and absence of email/hash in responses.

### X02 — DONE — Open or reuse a direct 1-1 conversation

- **Scope:** authenticated `user_id` target, self/missing-user rejection, exact two-member conversation reuse, transaction-scoped unordered-pair lock shared with seed.
- **Verification:** real PostgreSQL/HTTP integration covers 12 simultaneous A↔B requests, one creation and 11 reuses with the same ID, seed reuse, a distinct A↔C conversation, missing/self/anonymous/forged input, and sending a message in the newly opened chat.

### X03 — DONE — Document and verify the extension

- **Scope:** update contract, decisions, run instructions and milestone scope. Existing B01-B34 status remains separately tracked.
- **Verification:** `go test -race -count=1 ./...` with TEST_DATABASE_URL targeting chat_app_test, `go vet ./...`, and `git diff --check` passed. Integration uses isolated schemas and does not alter existing development data. No Flutter search UI or WebSocket delivery is claimed.

## User profile extension

The user approved the private/public profile API design on 2026-09-28. The implemented contract is recorded in [CONTRACTS.md](CONTRACTS.md).

### P01 - DONE - Record the approved profile contract

- **Outcome:** update decisions and API contracts with the confirmed routes, privacy boundary, PATCH semantics, validation rules, fixed avatar, and explicitly deferred features.
- **Dependency:** none. Keep OpenAPI unchanged until routes are implemented because route/spec parity is enforced by tests.

### P02 - DONE - Establish the fixed default avatar

- **Outcome:** add a forward migration that sets the confirmed `avatar_url` default and backfills empty existing values; verify existing and newly registered users.
- **Dependency:** P01.

### P03 - DONE - Add private and public profile domain models

- **Outcome:** add separate safe representations and a stable phone-number-conflict error without Gin or SQL dependencies.
- **Dependency:** P01.

### P04 - DONE - Define profile read repository operations

- **Outcome:** define context-aware private/public lookup operations with active-user and not-found behavior.
- **Dependency:** P03.

### P05 - DONE - Implement PostgreSQL profile reads

- **Outcome:** read nullable private fields and minimal public fields with parameterized SQL and real PostgreSQL integration coverage.
- **Dependency:** P02, P04.

### P06 - DONE - Implement profile read use cases

- **Outcome:** add and unit-test separate private/public read actions through repository interfaces.
- **Dependency:** P04.

### P07 - DONE - Expose private and public GET profile APIs

- **Outcome:** add authenticated `GET /api/users/me` and `GET /api/users/:id`, production wiring, exact DTO privacy, OpenAPI, and HTTP/contract tests.
- **Dependency:** P05, P06.

### P08 - DONE - Preserve PATCH field presence

- **Outcome:** model omitted, explicit-null, and concrete-value states without allowing unknown or non-editable fields.
- **Dependency:** P03.

### P09 - DONE - Validate profile updates

- **Outcome:** validate normalized split names, nullable non-future ISO dates, nullable unique E.164 phone intent, and nonempty patches with unit tests.
- **Dependency:** P08.

### P10 - DONE - Implement PostgreSQL profile updates

- **Outcome:** update only supplied editable fields, support clearing nullable fields, map phone uniqueness safely, and return the private profile with integration tests.
- **Dependency:** P05, P08.

### P11 - DONE - Implement the profile update use case

- **Outcome:** normalize, validate, persist, and return an updated private profile with focused fake-repository tests.
- **Dependency:** P09, P10.

### P12 - DONE - Expose PATCH `/api/users/me`

- **Outcome:** add authenticated strict JSON handling, safe error mapping, route/wiring, OpenAPI, and HTTP/contract tests.
- **Dependency:** P07, P11.

### P13 - DONE - Run profile privacy and avatar regressions

- **Outcome:** verify avatar consistency across auth/search/chat/profile and prove public responses expose no private fields.
- **Dependency:** P12.

### P14 - DONE - Complete profile documentation and verification

- **Outcome:** synchronize contracts, decisions, Swagger, Postman/run guidance, full PostgreSQL-backed tests, vet, formatting, diff checks, and the handoff record.
- **Dependency:** P13.

See [MVP_PLAN](MVP_PLAN.md) for the eight milestones grouping B01–B34, their exit criteria, and the final acceptance scenario. Update implementation status here only.

This is the **single source of work status**. Read each item's status and the latest handoff entry for current progress; the backlog is not an instruction to implement everything automatically. For an assigned implementation task, complete **one small item per turn, report what was done and explain it in Vietnamese, then stop** according to [AGENTS.md](AGENTS.md) and [WORKFLOW.md](WORKFLOW.md). Split an item into sub-items first if it is still too large.

Statuses: `TODO`, `IN_PROGRESS`, `DONE`, `BLOCKED`. Mark DONE only after meeting the criteria and explaining the work; record actual verification results in the log at the end. Read assumptions/questions in [DECISIONS.md](DECISIONS.md) and the relevant [CONTRACTS.md](CONTRACTS.md) sections before each item. Design proposals do not automatically become user decisions.

Each item specifies dependencies, verification, and concepts to explain. **Every explanation listed below must be delivered to the user in Vietnamese; keep the stored documentation and handoff entries in English.** DB items use separate development/test databases; do not delete user data. Necessary tests accompany each item instead of waiting until B33. Update run instructions and contracts as behavior changes; B34 is only the final overall review.

## B01 — DONE — Initialize the Go module

- **Dependencies:** an implementation request from the user. Determine a module path and Go version appropriate for the machine; create the backend module without adding other application libraries yet.
- **Verify / explain:** Go recognizes the module and the existing entrypoint builds; record actual commands. Explain modules, packages, `go.mod`, and why other dependencies are not needed yet.

## B02 — DONE — Configure the HTTP port

- **Dependencies:** B01. Read the port from the environment with a default and validation according to CONTRACTS; do not open a server yet.
- **Verify / explain:** missing configuration uses the default, valid ports are accepted, and invalid values are rejected. Explain environment variables, type conversion, returning errors, and the config package's role.

### B02 handoff — 2026-09-18

- **Changed:** created `backend/internal/config/config.go` with `LoadHTTPPort()`.
- **What it does:** đọc `HTTP_PORT` từ environment variable; nếu không có → default `8080`; nếu có → parse sang integer, reject nếu không phải số hoặc nằm ngoài 1–65535.
- **Verified:** `go build ./...` pass; chưa mở server.
- **Giải thích (Vietnamese):**
  - `os.Getenv("HTTP_PORT")` đọc biến môi trường — ứng dụng container/server không cần sửa code để đổi port.
  - `strconv.Atoi` parse string → int; lỗi → trả về `fmt.Errorf` với message rõ.
  - `1–65535` là range hợp lệ cho TCP port; port 0 không bind được.
  - `config` package giữ logic đọc config tách biệt với `main.go` — dễ test và tái sử dụng.
- **Next:** B03 — thêm Gin và `GET /health`.

## B03 — DONE — Gin and the health endpoint

- **Dependencies:** B02. Add Gin, create a router with `GET /health`, and wire the configured port into the entrypoint; do not connect DB/auth yet.
- **Verify / explain:** the endpoint returns the contract's status/body; other routes are not treated as health checks. Explain handlers, routers, requests/responses, and the composition root.

### B03 handoff — 2026-09-18

- **Changed:** added Gin dependency `v1.9.1`; created `backend/internal/handler/health.go`; updated `backend/cmd/api/main.go` to wire router.
- **What it does:** `GET /health` trả về `{"status":"ok"}`; route khác trả 404.
- **Verified:** server start thành công trên port 8080; `curl localhost:8080/health` → `{"status":"ok"}`; `curl localhost:8080/not-health` → `404 page not found`.
- **Giải thích (Vietnamese):**
  - **Handler:** function nhận `*gin.Context`, viết response qua `c.JSON()`. Tách handler ra `internal/handler/` để `main.go` không chứa logic HTTP.
  - **Router:** `r.GET("/health", handler.Health)` — Gin match path, gọi handler.
  - **Composition root:** `main.go` lắp ráp mọi thứ — đọc config, tạo router, đăng ký route, chạy server. Đây là nơi duy nhất biết toàn bộ app.
  - **Request/Response:** client gửi HTTP request → Gin routing → handler trả JSON response.
- **Next:** B04 — xử lý server lifecycle (shutdown, startup failure, timeouts).

## B04 — DONE — Minimal server lifecycle

- **Dependencies:** B03. Handle startup, port binding failures, and shutdown with a time limit; do not add WebSocket lifecycle management yet.
- **Verify / explain:** start/stop the server, try an occupied port, and record exit behavior. Explain the server's lifetime, signals/context, and why resources need cleanup.

### B04 handoff — 2026-09-18

- **Changed:** updated `backend/cmd/api/main.go` — server chạy trong goroutine, dùng `http.Server` thay vì `r.Run()`, thêm graceful shutdown.
- **What it does:**
  - Server start trong goroutine → main goroutine đợi signal.
  - Nhận `SIGINT`/`SIGTERM` → gọi `srv.Shutdown(ctx)` với timeout 10s.
  - `ReadTimeout 10s`, `WriteTimeout 10s`, `IdleTimeout 120s`.
  - Port bị chiếm → exit code 1 ngay.
- **Verified:**
  - Server start OK → `curl localhost:8080/health` → `{"status":"ok"}`.
  - Port 8080 đang bị chiếm → `listen tcp :8080: bind: Only one usage...` → exit status 1.
- **Giải thích (Vietnamese):**
  - **Server lifetime:** server chạy vô hạn trong goroutine cho đến khi nhận signal hoặc lỗi fatal.
  - **Signal/context:** `signal.Notify` bắt `SIGINT` (Ctrl+C) và `SIGTERM` (docker stop); `context.WithTimeout` giới hạn thời gian shutdown.
  - **Tại sao cần cleanup:** nếu không gọi `Shutdown`, connection đang mở sẽ bị drop đột ngột, client không nhận được response cuối; goroutine leak; port có thể bị chiếm lâu hơn.
  - **`http.ErrServerClosed`** là behavior bình thường khi shutdown thành công — không phải lỗi.
- **Next:** M1 hoàn thành (B01–B04 DONE). B05 — PostgreSQL và migration tooling. B05 cần quyết định về database trước.

## B05 — DONE — Development PostgreSQL and migration tooling

- **Dependencies:** B04. Select and document a migration tool; prepare separate development/test databases and placeholder-only example configuration. Do not create application tables yet.
- **Verify / explain:** connect to the intended development DB, identify the separate test DB, and verify the migration tool works. Explain databases, connection strings, migrations, and separation of test data.

### B05 handoff — 2026-09-18: Docker PostgreSQL host-port correction

- **Completed work:** diagnosed an authentication failure and moved the `chat-postgres` Docker host binding from `5432:5432` to `5433:5432`.
- **Why:** Windows `postgres.exe` owned host port 5432, so the backend was connecting to the Windows PostgreSQL instance rather than the Docker container. The Docker container itself was healthy but its credentials did not apply to the Windows service.
- **Data preservation:** before recreation, the PostgreSQL data mount was verified as volume `2f09a5de17ea68fbf55f65c04b28099c140224d1ad0d59b751a9305f91d61b21`. The container was recreated with that same volume mounted at `/var/lib/postgresql/data`; no database volume was removed.
- **Changed infrastructure:** `chat-postgres` now publishes `127.0.0.1:5433` / host port 5433 to container port 5432. No application source files or migrations changed in this step.
- **Verified:** container is running; `pg_isready` reports readiness; host TCP port 5433 is open. With `DATABASE_URL` targeting `postgres://postgres:<password>@127.0.0.1:5433/chat_app?sslmode=disable`, the existing backend logged `db: connected`, started on temporary port 18080, and `GET /health` returned HTTP 200 with `{"status":"ok"}`. The temporary backend process was stopped after verification.
- **Explained in Vietnamese:** the difference between a Docker container port and a host port; why port 5432 selected Windows PostgreSQL; why recreating a container is necessary to change port bindings; and how retaining the named volume preserves data.
- **Remaining scope at that time:** migration tool selection, separate test database, placeholder-only configuration, and migration-tool verification. These were completed in the later B05 completion entry below.
- **Next at that time:** continue the remaining B05 work in one small item.

### Database schema review — 2026-09-18

- **Completed work:** added `docs/schema.dbml`, a DBML diagram source for dbdiagram.io, to support schema review before migrations.
- **Changed:** the diagram models users, conversations, conversation members, and messages; it includes primary keys, foreign keys, uniqueness, timestamps, pagination indexes, member-only sending, and a same-conversation read marker reference.
- **Verified:** reviewed against the target schema and invariants in CONTRACTS. The file is documentation only; no migration, database schema, or production data changed.
- **Explained in Vietnamese:** the User → ConversationMember → Conversation → Message relationship and why `conversation_members` stores each user's read position.
- **Remaining scope at that time:** user approval/review of the diagram, selection of a migration tool, and separate test DB setup. The technical B05 prerequisites were completed later; the diagram was approved by the user before B06.

### Database diagram simplification — 2026-09-18

- **Completed work:** simplified `docs/schema.dbml` so dbdiagram.io draws only four direct 1-N relationships.
- **Changed:** removed composite relationship lines for member-only sending and same-conversation read markers; added comments explaining that `conversation_members` is the junction table and that those two invariants remain enforced outside the visual diagram.
- **Verified:** the DBML still contains all four MVP tables and the four simple 1-N relationships: users → members, conversations → members, users → messages, and conversations → messages.
- **Explained in Vietnamese:** a junction table does not create a physical N-N foreign key; it represents the User–Conversation concept as two normal 1-N relationships.
- **Remaining scope:** this is only a diagram simplification. The migration design must still decide how to enforce the documented membership and read-marker invariants.

### B05 completion — 2026-09-18

- **Completed work:** selected the locally installed `migrate` CLI, created the isolated `chat_app_test` database in the Docker PostgreSQL instance, and added `.env.example` with placeholders only.
- **Changed:** documented the migration command and the Windows absolute-path requirement in `README.md`; recorded the choice in `DECISIONS.md`; added `.gitignore` to exclude real `.env` files and the local executable.
- **Verified:** `migrate` applied and reported version `1` against `chat_app_test`; `chat_app` remained without application tables. The test database is separate from development data.
- **Next:** B06 — users table migration.

## B06 — DONE — Users table migration

- **Dependencies:** B05. Create users with a primary key, email uniqueness, and fields from CONTRACTS; add only the schema needed for auth.
- **Verify / explain:** apply the migration to a fresh test DB; constraints reject invalid data/duplicate email. Explain primary keys, uniqueness, nullability, and why application validation does not replace constraints.

### B06 handoff — 2026-09-18

- **Completed work:** added migration version `000001` for the `users` table only.
- **Changed:** `migrations/000001_create_users.up.sql` creates an auto-incrementing bigint primary key; required name, email, password-hash, avatar URL, and timestamp fields; unique email; and checks rejecting blank name, email, and password-hash values. `migrations/000001_create_users.down.sql` drops only this table.
- **Verified:** applied version `1` to the fresh `chat_app_test` database. A valid insert succeeded with default avatar and timestamp. A duplicate email failed with `users_email_key`; a blank name failed with `users_name_check`. Development database `chat_app` was checked and has no application table.
- **Unverified / incomplete:** email normalization and request validation belong to the later registration use case; no users are seeded yet.
- **Next:** B07 — connect the database pool through `database/sql` with context and lifecycle cleanup.

## B07 — DONE — Connect through database/sql

- **Dependencies:** B06. Select a driver, configure the connection, open the pool, and check connectivity using context; connect pool cleanup to the existing lifecycle.
- **Verify / explain:** valid settings connect successfully; invalid settings/timeouts produce clear errors without leaking secrets. Explain drivers, connection pools, context, and resource ownership.

### B07 handoff — 2026-09-18

- **Completed work:** moved PostgreSQL pool creation and connectivity checking into `internal/infrastructure/database/postgres.go`; the startup path now uses a bounded context and the server lifecycle owns the returned pool.
- **Changed:** `database.Open` opens the `database/sql` pool, pings PostgreSQL through the supplied context, and closes the pool on failed startup. `cmd/api/main.go` now uses `run() error`, reports errors only after defers run, forwards server-goroutine errors through a channel, shuts down HTTP before returning, and closes the pool with a deferred cleanup function.
- **Verified:** valid Docker PostgreSQL settings connected successfully. Incorrect credentials previously returned a PostgreSQL authentication error without logging the connection string. `go test ./...`, `go vet ./...`, and `git diff --check` passed. A temporary server on port 18083 logged `db: connected` and `listening`; sending Ctrl+C logged `shutting down...` and `server gracefully stopped`.
- **Observed limitation:** port 8080 was occupied during one verification attempt; the server returned a clear bind error and exited rather than leaking the pool. The temporary port was used for the successful lifecycle check.
- **Next:** B08 — minimal user repository.

## B08 — TODO — Minimal user repository

- **Dependencies:** B07. Add required user entities/types, find-by-email/create-user contracts, and a PostgreSQL adapter; do not implement auth use cases yet.
- **Verify / explain:** integration tests cover create/find, not found, and duplicate email; hashes are not exposed as public data. Explain structs, interfaces, parameterized SQL, Scan, and DB error mapping.

## B09 — DONE — bcrypt password adapter

- **Dependencies:** B08. Add a small hashing/comparison contract and bcrypt adapter; follow CONTRACTS input rules.
- **Verify / explain:** the correct password matches, incorrect passwords do not, and out-of-range input is handled; never log passwords/hashes. Explain hashing, salts, and implicit interface implementation.

## B10 — DONE — Registration use case

- **Dependencies:** B08, B09. Implement registration through the repository and password adapter, with contract-defined validation/normalization; no endpoint yet.
- **Verify / explain:** fake-based tests cover success, invalid input, duplicate email, and persistence errors; plaintext passwords are never stored. Explain use case input/output, direct dependency injection, and error branches.

## B11 — DONE — Registration HTTP endpoint

- **Dependencies:** B03, B10. Connect `POST /api/auth/register` to the use case; follow CONTRACTS for DTOs and status/error mapping.
- **Verify / explain:** try valid JSON, invalid JSON, and duplicate registration; responses contain no hash. Explain JSON binding, DTOs versus entities, and the handler → use case → repository flow.

## B12 — DONE — JWT adapter

- **Dependencies:** B02, B10. Add token signing/verification using the algorithm, secret, and demo expiration from CONTRACTS; no HTTP middleware yet.
- **Verify / explain:** valid tokens verify; expired, modified, and wrong-algorithm tokens are rejected. Explain claims, signatures, expiration, and why tokens must not be logged.

## B13 — DONE — Login use case

- **Dependencies:** B08, B09, B12. Find the user, compare the password, and issue a token through small contracts; no endpoint yet.
- **Verify / explain:** test successful login, nonexistent email, incorrect password, and repository/token errors; credential failures do not reveal account existence. Explain dependency coordination and authentication.

## B14 — DONE — Login HTTP endpoint

- **Dependencies:** B11, B13. Connect `POST /api/auth/login`, token/user DTOs, and status/error mapping according to CONTRACTS.
- **Verify / explain:** call the endpoint with an account in the test DB; try incorrect credentials and invalid JSON. Explain how clients use access tokens, local logout, and the demo limitations in DECISIONS.

## B15 — DONE — HTTP authentication middleware

- **Dependencies:** B12, B14. Read and verify Bearer tokens and provide authenticated user IDs to handlers; preserve contract-defined public routes.
- **Verify / explain:** test missing, invalid, expired, and valid tokens with a test route rather than adding an MVP API solely for testing. Explain middleware, caller identity, and authentication versus authorization.

## B16 — DONE — Chat schema and read position

- **Dependencies:** B06, B15. Migrate conversations, conversation_members, and messages with the constraints/indexes in CONTRACTS; read DECISIONS about the added `last_read_message_id` and existing `last_read_at` before implementation.
- **Verify / explain:** apply migrations on the test DB; check foreign keys, unique membership, and rejection of invalid data. Explain table relationships, indexes, the read cursor, and why operation time alone cannot identify read messages.

## B17 — DONE — Seed exactly two users and one conversation

- **Dependencies:** B09, B16. Add development seed data for A, B, one 1–1 conversation, and two memberships; accounts use bcrypt hashes. Do not add conversation creation/search APIs.
- **Verify / explain:** rerunning the seed does not duplicate data; A/B can log in and belong to the intended conversation. Explain seeds versus migrations, demo data, and test fixtures; do not overwrite existing accounts through seeding.

## B18 — DONE — Conversation list repository

- **Dependencies:** B08, B16, B17. Query a user's conversations with the other participant, last message, and unread count according to CONTRACTS; do not add a new use case/HTTP endpoint yet.
- **Verify / explain:** integration tests cover empty/populated conversations, exclusion of self-sent messages from unread counts, and no exposure of other users' conversations. Explain JOINs, aggregate queries, result ordering, and plain Go result types.

## B19 — DONE — Conversation list use case

- **Dependencies:** B18. Accept an authenticated user ID and request only that user's conversations from the repository.
- **Verify / explain:** test correct identity propagation, an empty list, and repository errors; no arbitrary input may request another user's list. Explain use case boundaries and identity-based access scope.

## B20 — DONE — Conversation list HTTP endpoint

- **Dependencies:** B15, B19. Connect `GET /api/conversations` with middleware and the defined response DTO.
- **Verify / explain:** A/B receive the intended conversation, missing tokens are rejected, and list data matches the contract. Explain query-result-to-JSON mapping and conversations without a last message.

## B21 — DONE — Transactional message persistence repository

- **Dependencies:** B16, B18. Save a message and update its conversation atomically: lock the conversation row before allocating `messages.id`; all insert paths must follow the same CONTRACTS rule.
- **Verify / explain:** integration tests cover rollback when a step fails, write authorization constraints, and concurrent sends without reversing ID commit order within a conversation. Explain transactions, row locks, identity/sequences, and why `sql.Tx` stays out of use cases.

## B22 — DONE — Send-message use case

- **Dependencies:** B21. Validate content and membership, then call the atomic persistence operation; take the sender from the authenticated identity. Do not publish WebSocket events yet.
- **Verify / explain:** test valid, empty/oversized, non-member, and persistence-error cases; unauthorized callers cannot write. Explain why business rules belong in use cases and why the body cannot choose the sender.

## B23 — DONE — Send-message HTTP endpoint

- **Dependencies:** B15, B22. Connect `POST /api/conversations/:id/messages` with a `content` body; return the committed message according to CONTRACTS.
- **Verify / explain:** A sends and the DB records the correct sender; invalid IDs/paths/bodies and non-members are rejected. Explain non-idempotent POST behavior, retry duplicates, and why the MVP does not promise exactly-once sending.

## B24 — DONE — Message pagination repository

- **Dependencies:** B21, B23. Query the initial page, descending IDs for `before_id`, and ascending IDs for `after_id`; always filter by the requested conversation. A cursor is a numeric boundary and does not require a corresponding existing message or proof of membership in that conversation.
- **Verify / explain:** integration tests cover multiple pages, no cursor-boundary duplicates/omissions, no data leakage when a boundary comes from another conversation's ID, and concurrent inserts. Explain keyset pagination, its difference from a read marker, and its relationship to B21's ID allocation rule.

## B25 — DONE — History/reconnect use case

- **Dependencies:** B24. Check membership and parameters: `before_id`/`after_id` are mutually exclusive, the default limit is 20, and the maximum is 100 according to CONTRACTS.
- **Verify / explain:** test initial/older pages, newer messages after a cursor, invalid limits, and non-members. Explain both retrieval directions and why reconnect recovery reads from the DB.

## B26 — DONE — Message history HTTP endpoint

- **Dependencies:** B15, B25. Connect `GET /api/conversations/:id/messages`, parse query parameters, and return contract-defined ordering/cursors.
- **Verify / explain:** request each page mode with enough test data, reject invalid queries, and deny the user C fixture. Explain parsing, limits/cursors, and the JSON ordering clients receive.

## B27 — DONE — Read-position update repository

- **Dependencies:** B16, B24. Validate that the target message belongs to the conversation and update `last_read_message_id` using an atomic maximum; preserve the meaning of `last_read_at` from CONTRACTS.
- **Verify / explain:** integration tests reject markers from another conversation, prevent regression under concurrent/older updates, and keep new unread messages counted. Explain atomic updates and operation time versus message position.

## B28 — DONE — Mark-read use case

- **Dependencies:** B27. Check membership, accept the last message actually displayed by the client, and update through the repository; do not automatically mark all messages present when the server handles the request.
- **Verify / explain:** test members/non-members, invalid messages, repeated requests, and repository errors. Explain races when new messages arrive during a read update and why the marker never moves backward.

## B29 — DONE — Mark-read HTTP endpoint

- **Dependencies:** B15, B20, B28. Connect `POST /api/conversations/:id/read` with `last_read_message_id` according to CONTRACTS.
- **Verify / explain:** B reads through a message and B's unread count decreases correctly; A's marker is unchanged; messages B has not seen remain unread. Explain the read response's relationship to refreshing the conversation list.

## S01 — DONE — Interactive OpenAPI documentation

- **Assigned request:** add Swagger documentation for the whole currently implemented system.
- **Changed:** added an embedded OpenAPI 3.0 JSON contract for all current paths and operations, including JWT bearer authentication, request/response schemas, pagination, validation limits, status codes, and reusable errors. Added public `/swagger`, `/swagger/index.html`, and `/swagger/openapi.json` routes without changing application business logic. X04 subsequently added the refresh operation to the same contract.
- **Verified:** Swagger package and all presentation HTTP tests pass; the contract test parses the embedded JSON and checks every implemented application path. Full repository checks and a live-browser smoke test are recorded in the handoff entry below.
- **Boundary:** Swagger UI assets are pinned to Swagger UI 5.17.14 on jsDelivr, so the interactive page needs internet access; the OpenAPI contract itself is embedded and always served locally. This historical S01 boundary excluded WebSocket; B30-B34 later added the `/ws` handshake operation and realtime contract.

## B30 — DONE — Authenticated WebSocket handshake

- **Dependencies:** B12, B15, B29; U15 confirms Android/iOS-only Bearer-header transport and I10 selects `coder/websocket` v1.8.15. Follow [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md) and create `/ws` accepting only authorized connections.
- **Verify / explain:** valid tokens can upgrade, missing/invalid/expired tokens cannot, and tokens never appear in URLs/logs. Explain the handshake versus a normal HTTP request, origin policy, and the connection authentication lifetime.

## B31 — DONE — Hub and connection lifecycle

- **Dependencies:** B04, B30. Manage connections by authenticated user, registration/removal, one writer per connection, bounded queues, and shutdown/expiration closure according to CONTRACTS.
- **Verify / explain:** test connect/disconnect, multiple connections, slow clients, and non-hanging shutdown; run the race detector if supported. Explain goroutines, channels, synchronization, and cleanup ownership; split into sub-items if still too large.

## B32 — DONE — Publish events after commit

- **Dependencies:** B22, B23, B31. Add a small publisher port to the send use case; wire the hub to publish `new_message` only to members after a successful commit.
- **Verify / explain:** rollback publishes nothing, outsiders receive nothing, and publication failures still return the persisted message while recording the error at the right boundary. Explain dependency inversion, best effort, and why the DB is authoritative.

## B33 — DONE — Verify the complete chat flow

- **Dependencies:** B20, B23, B26, B29, B32. Run A/B send/receive/read scenarios, deny C, disconnect, and catch up through REST; do not bundle every discovered bug fix into this turn.
- **Verify / explain:** record each step's result using test data; compare message IDs, ordering, unread counts, and reconnect behavior. Explain the flow across layers; create a small fix item and stop according to the workflow if an issue is found.

## B34 — DONE — Review run instructions and hand off the MVP

- **Dependencies:** B33 and all necessary fix items completed. Compare the README, sample configuration, contracts, migrations/seeds, and verification instructions against actual code.
- **Verify / explain:** follow the documentation in an appropriate test environment and record verified/unverified parts; do not declare the MVP complete while criteria are missing. Explain how the next agent starts the system, verifies it, locates layers, and chooses the next task.

## Handoff log

### 2026-09-28 - Full verification, documentation and release preparation

- **Request:** run all tests, synchronize documentation/Swagger, commit and push the WebSocket work.
- **Verified:** complete backend suite passed with `go test -race -count=1 -timeout=120s ./...`, including real PostgreSQL; `go vet ./...`, `go build ./...`, changed-file formatting, JSON parsing and diff checks passed.
- **Changed:** corrected stale notification-only/no-implementation statements in DECISIONS/ARCHITECTURE/MVP_PLAN, added WS command/event/error component schemas and examples to OpenAPI 1.1.0, added contract/reference regression tests, and linked [WEBSOCKET_VERIFICATION.md](WEBSOCKET_VERIFICATION.md) with the file map and verification boundaries.
- **Formatting caveat:** three untouched auth files have pre-existing gofmt differences; details are in the report. No unrelated application code was reformatted.
- **Git scope:** user authorized publishing the complete pending backend WebSocket increment to the current `backend/Bao` branch on `origin`; actual commit/push result is confirmed in the final response after execution.
- **Not performed:** device tests, development/production migration or deployment. Next operational step remains applying 000003 and integrating the mobile protocol.

### Bidirectional migration tracking (U16)

The earlier W01-W13 handoffs describe the notification-only baseline, not completion of bidirectional chat. The active plan is [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md).

| Item | Status | Outcome |
|---|---|---|
| WS2-01 | DONE | Strict command/response DTOs and protocol tests |
| WS2-02 | DONE | Durable PostgreSQL send idempotency and migration |
| WS2-03 | DONE | Shared send/read use cases and keyed REST compatibility |
| WS2-04 | DONE | Bounded command dispatcher and lifecycle cancellation fixes |
| WS2-05 | DONE | Socket send, acknowledgement and production wiring |
| WS2-06 | DONE | Socket mark-read and monotonic acknowledgement |
| WS2-07 | DONE | Real-token, PostgreSQL, retry, reconnect and lifecycle acceptance |
| WS2-08 | DONE | Implemented contracts, client examples and final cleanup |

### 2026-09-28 - WS2-01 through WS2-08: Bidirectional implementation

- **Authorization:** user explicitly requested implementation and obsolete-file cleanup.
- **Protocol/domain:** strict text command decoding, canonical UUID correlation, sanitized errors, socket send/read acknowledgements. Both transports use the same send/read use cases; only newly committed messages publish.
- **Storage:** forward migration 000003 adds nullable UUID retry keys and sender-scoped uniqueness. Concurrent identical retries return the original row; conflicting payloads/conversations fail safely. Membership and conversation write locks remain enforced. Up/down tested in disposable test schemas; down retains messages but removes retry metadata.
- **Lifecycle:** bounded command queue and sequential worker, command timeout, per-connection token bucket, independent reader/writer/heartbeat/supervision, cancellation on disconnect/expiry/shutdown, bounded graceful close with I/O cancellation fallback. Checked all Origin header values. API shutdown cleanup now also runs on error paths.
- **Cleanup:** replaced the obsolete reject-all reader and its blanket-rejection test in place; the former library.go aliases remain removed. Kept necessary hub, authentication, event/publisher adapters, REST APIs, migrations and stored data. No application-data deletion.
- **Documentation:** updated README, contracts, decisions, plan, schema diagram, environment example, OpenAPI and Postman keyed-send example.
- **Verified:** go test -race -count=1 -timeout=90s ./...; focused race tests for WebSocket/config; go vet ./...; git diff --check. Real PostgreSQL tests did not skip. Coverage includes concurrent same-key/cross-conversation retries, sender scoping, migration round trip, real refresh/expired JWT rejection, multi-device fanout, outsider exclusion, socket/REST retry, reconnect catch-up, monotonic read acknowledgements, malformed/binary/oversized input, command timeout/overflow/rate limiting, missing pong, expiry during blocked SQL/write/ping and forced shutdown.
- **Reported/explained in Vietnamese:** retry transaction and publish boundary, queue/worker/context roles, lifecycle bug found by tests, compatibility cleanup and verification results.
- **Not performed:** development/production migration, running Flutter on Android/iOS, deployment, commit or push. Apply migration 000003 before using the updated server; update any existing WS_MAX_MESSAGE_BYTES=1024 override to 16384.
- **Limits:** one API process; best-effort events with no replay/exactly-once delivery; no distributed hub, recipient read receipts or browser support. REST synchronization is still required.
- **Next:** apply the new migration to the intended development DB and integrate the documented mobile send/retry/catch-up flow.

### 2026-09-28 - U16: Bidirectional migration plan and cleanup

- **Completed:** replaced the obsolete active notification-only plan with WS2-01 through WS2-08, including command envelopes, acknowledgement/error correlation, durable retry keys, lifecycle fixes, compatibility strategy, tests, and safe reconnect cursor guidance. Updated decisions, contracts and entry-point documentation to distinguish current runtime from planned behavior.
- **Removed:** the temporary `presentation/websocket/library.go` constant-alias shim. Its remaining internal test references now use `coder/websocket` constants directly. The removed aliases contain no application data and can be recreated from the dependency constants.
- **Preserved:** REST endpoints, stored messages, authentication, hub, membership checks, history and current rejection guard until the command dispatcher replaces it safely.
- **Verification:** non-database WebSocket tests passed under the race detector and `git diff --check` passed. The complete WebSocket package failed at `TestRealtimeRESTAcceptance` with `requires chat_app_test`, including on retry outside the sandbox; database-backed acceptance could not be verified in this turn. No migration was executed and bidirectional commands remain unimplemented.
- **Next:** WS2-01. Commit/push are not part of this planning cleanup.

### 2026-09-28 - W06-W13 / B30-B34: Complete realtime WebSocket delivery

- **Status:** DONE. The user explicitly requested completion of the remaining WebSocket scope and a final report.
- **Hub/lifecycle:** added a thread-safe in-memory hub keyed by user ID, multiple devices per user, idempotent registration cleanup, bounded per-connection queues, one sequential writer, one reader, configured read/write/ping/pong limits, policy closure for unsupported client frames and token expiry, slow-client isolation, and graceful server shutdown.
- **Commit/publish boundary:** added a domain publisher port and best-effort failure reporter. `Send` publishes only the message returned after the PostgreSQL transaction commits; validation, rollback, or repository failure never publishes. A publication failure is reported but the persisted message remains a successful REST result.
- **Authorization/event:** added deterministic PostgreSQL member-ID lookup and a presentation publisher that sends the exact `new_message` DTO only to persisted active conversation members, including every online device of the sender and recipient. Outsiders receive no event; clients recover missed messages through REST `after_id`.
- **Production:** loads validated WebSocket configuration, registers authenticated `GET /ws`, wires the hub publisher into REST send, documents `/ws` in OpenAPI while keeping Postman REST-only, and closes upgraded connections during graceful shutdown before database teardown.
- **Acceptance:** real WebSocket/HTTP/PostgreSQL coverage connects A, B, and outsider C; verifies member delivery and outsider isolation; disconnects B, persists a message while offline, reconnects B, and recovers exactly that message through history. Focused tests also cover handshake failures, multiple devices, unsupported input, token expiry, shutdown, recipient failures, canceled DB context, and commit-before-publish behavior. Existing REST coverage verifies monotonic read/unread behavior.
- **Verification:** `gofmt` completed; OpenAPI and Postman JSON parse; `go test -race -count=1 ./...`, `go vet ./...`, and `git diff --check` passed. PostgreSQL tests ran against `chat_app_test` without skips.
- **Limitations:** one process/in-memory hub, best-effort live delivery, no replay/exactly-once guarantee, REST-only sends/read markers, no browser/Flutter Web, and no typing/presence/read-receipt/media/group-chat events.
- **Next:** connect the Flutter Android/iOS client using the documented Bearer-header example and use REST `after_id` after every reconnect.

### 2026-09-28 - W05: Authenticated WebSocket handshake handler

- **Status:** DONE as the isolated handshake step; B30 remains TODO until the production route is wired to a real lifecycle component.
- **Changed:** added a Gin WebSocket handler that rejects non-GET and browser-Origin requests, applies shared Bearer authentication before upgrade, accepts the socket with `coder/websocket`, passes the complete identity to a narrow `SessionRunner`, and force-closes the connection after the runner returns. No request context is incorrectly reused as the long-lived socket context.
- **Contract:** pre-upgrade failures use JSON HTTP responses: 405 `method_not_allowed`, 403 `origin_not_allowed`, and 401 `unauthenticated`. Once upgrade succeeds, the handler does not attempt to write HTTP errors.
- **Tests:** real `coder/websocket.Dial` coverage proves successful upgrade and identity/expiry propagation; missing, malformed, wrong-scheme, duplicate, invalid, refresh, expired, and internally failed authentication cannot upgrade; browser Origin and non-GET requests never reach verification/session execution.
- **Verification:** `gofmt` completed; focused WebSocket tests, all presentation tests, full PostgreSQL-backed `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed without skips.
- **Boundary:** no production route, hub, connection loops, queue, ping/pong, token-expiry close, publisher, or realtime event was added.
- **Next:** W06/B31a - implement the in-memory hub's registration, idempotent removal, user-targeted bounded fan-out, and shutdown behavior.

### 2026-09-28 - W04: Shared strict Bearer authentication

- **Status:** DONE. The user implemented the shared authentication helper and refactored REST middleware; review found no code correction necessary.
- **Changed:** added presentation-level `authentication.VerifyBearer`, which accepts exactly one Authorization header, parses a case-insensitive Bearer scheme, delegates token verification once, rejects non-positive identities, preserves token expiry, and sanitizes every failure to `ErrInvalidToken`. HTTP middleware aliases the shared verifier interface and retains its existing Gin identity storage and 401 response.
- **Tests:** direct helper coverage proves valid identity/expiry propagation, whitespace and scheme behavior, zero verifier calls for malformed headers, one call for structurally valid tokens, duplicate rejection, non-positive identity rejection, and sanitization of internal verifier details. Existing middleware and HTTP integration coverage confirms no REST behavior regression.
- **Verification:** `gofmt -d` produced no output; targeted authentication/middleware/presentation tests, full PostgreSQL-backed `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed without skips.
- **Boundary:** no `/ws` route, upgrade, connection, hub, goroutine, publisher, or realtime event was implemented.
- **Next:** W05/B30 - implement and test the authenticated WebSocket handshake without adding hub lifecycle or message publication.

### 2026-09-28 - W03: Validated WebSocket lifecycle configuration

- **Status:** DONE. The user implemented the configuration and tests; review found no code correction necessary.
- **Changed:** added `WebSocketConfig` and `LoadWebSocket` with bounded queue/frame sizes, write/pong/ping/shutdown durations, safe defaults, per-variable errors, and the invariant that ping interval is shorter than pong timeout. Added matching optional values to `.env.example` and recorded the concrete contract values.
- **Tests:** cover all defaults, a complete valid override, malformed/zero/negative/too-large values, sub-second durations, and equal ping/pong intervals. Environment state is isolated with `t.Setenv`; no environment-mutating test runs in parallel.
- **Verification:** `gofmt -d` produced no output; `go test -count=1 ./cmd/api/config`, full PostgreSQL-backed `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed without skips.
- **Boundary:** configuration is intentionally not loaded by `main.go` yet. No `/ws` route, authentication refactor, handshake, hub, goroutine, publisher, or realtime event exists.
- **Next:** W04 - share strict Bearer authentication between REST middleware and the future WebSocket handshake.

### 2026-09-28 - W02: Select and pin the WebSocket library

- **Status:** DONE. The user added the selected dependency and compile-smoke package; the assistant completed dependency normalization, documentation, and verification.
- **Changed:** pinned `github.com/coder/websocket` v1.8.15 as a direct dependency and added `internal/presentation/websocket/library.go`, which imports the library and exposes the normal, going-away, and policy-violation close statuses needed by later lifecycle work. Recorded implementation decision I10.
- **Reason:** the library provides context-aware server/client APIs, close codes, ping/pong, read limits, and no transitive dependencies. The application will retain one sequential writer loop per connection for ordering and simple ownership.
- **Verification:** `go mod tidy`, `go list -m github.com/coder/websocket`, `go mod verify`, `go test -count=1 ./...`, and `go vet ./...` passed. PostgreSQL integration tests executed against the isolated test database and did not skip.
- **Boundary:** no `/ws` route, configuration, authentication refactor, handshake, hub, connection lifecycle, publisher, or realtime event was implemented.
- **Next:** W03 - add validated WebSocket lifecycle configuration.

### 2026-09-28 - W01: Android/iOS WebSocket implementation plan

- **Status:** DONE. The user confirmed Flutter Android/iOS-only support and requested a detailed stored implementation plan.
- **Changed:** added `WEBSOCKET_PLAN.md` with W01-W13 covering library selection, configuration, shared Bearer authentication, `/ws` handshake, hub and connection lifecycle, commit-before-publish integration, authoritative recipients, production wiring, A/B/C acceptance testing, and final documentation. Linked it from the README and B30.
- **Decision/contract:** recorded U15. Mobile uses the access token in the Authorization handshake header; query-string tokens, Flutter Web, and browser clients are out of scope. Browser Origin requests are rejected before upgrade.
- **Verification:** documentation links, statuses, terminology, and whitespace were checked with `rg` and `git diff --check`. No Go code, dependency, route, runtime behavior, commit, or push was added by this planning item.
- **Next:** W02/B30 - select and pin the WebSocket library, then implement in the small ordered steps in `WEBSOCKET_PLAN.md`.

### 2026-09-28 - P09-P14: Complete profile updates and final verification

- **Status:** DONE. The user explicitly requested completing the remaining profile backlog through P14 and removing the temporary profile plan.
- **P09/P11:** added the update use case with field-presence-aware validation and normalization. Patches must be nonempty; names are trimmed and follow the existing UTF-8/NUL/100-code-point policy; first name cannot be empty; date of birth is strict `YYYY-MM-DD` and non-future; phone numbers are either null or an unchanged E.164 value matching `+` plus 7-15 digits.
- **P10:** added a focused profile update repository interface and PostgreSQL implementation. One parameterized `UPDATE ... RETURNING` changes only supplied fields, clears nullable fields on explicit null, ignores inactive users, preserves email/avatar, and maps the phone unique constraint to `ErrPhoneNumberTaken`.
- **P12:** exposed authenticated `PATCH /api/users/me` with strict JSON, field-presence mapping, private-profile output, safe 400/401/404/409/413/415/500 behavior, production wiring, OpenAPI, and Postman coverage. Email, avatar, derived name, and unknown fields cannot be submitted.
- **P13:** regression coverage verifies that public profiles expose only ID/name/avatar, private fields stay private, immutable fields survive updates, and the fixed avatar default remains consistent across repository-backed user surfaces. The complete existing auth, discovery, conversation, message, profile, and migration suites passed together.
- **P14:** synchronized contracts, decisions, README state, Swagger, Postman, task statuses, and this handoff. Removed the completed temporary `PROFILE_PLAN.md`; `CONTRACTS.md` and this backlog remain the durable sources.
- **Verification:** `gofmt` completed; JSON parsing passed for OpenAPI and Postman; `go test -race -count=1 ./...`, `go vet ./...`, and `git diff --check` passed. PostgreSQL integration tests ran against the isolated `chat_app_test` database and did not skip.
- **Deferred:** avatar upload/edit, email changes, phone OTP, and profile/account deletion remain outside this increment. U15 later resolved the platform decision; B30 remains unimplemented.

### 2026-09-28 - P08: Preserve PATCH field presence

- **Status:** DONE. The user assigned the complete P08 implementation to the assistant.
- **Changed:** added generic domain `UpdateField` and `UpdateProfileInput` types that preserve omitted versus explicit-null update intent without JSON dependencies. Added an internal presentation `optionalJSONField` decoder and the four-field update request DTO.
- **Behavior:** omitted fields remain unset; explicit JSON null is distinct from an empty/concrete string; concrete strings are retained; wrong JSON types fail decoding. Name nullability is preserved for P09 to reject, while date/phone nullability remains available for clearing.
- **Verification:** focused handler tests cover all three presence states and wrong types. Targeted presentation/domain tests and full `go test -count=1 ./...` passed; `go vet ./...` and `git diff --check` passed.
- **Boundary:** no PATCH route, request-to-domain mapping, business validation, date parsing, E.164 validation, SQL update, or phone conflict mapping was added.
- **Next:** P09 - validate and normalize profile update intent, including non-null names, nullable non-future dates, E.164 phone values, and empty-patch rejection.

### 2026-09-28 - P07: Authenticated private/public profile GET APIs

- **Status:** DONE. The user created the initial handler/router/wiring; the assistant completed the requested corrections, tests, OpenAPI, Postman coverage, and verification.
- **Changed:** added authenticated `GET /api/users/me` and `GET /api/users/:id`, private/public HTTP DTOs, date-only DOB formatting, safe error mapping, production dependency wiring, and exact route registration. Public self lookup always uses the public representation.
- **Fixes:** aligned handler interface names, made the public DTO internal, corrected the Go date layout to `2006-01-02`, and removed duplicate `/users` registration.
- **Contracts/tests:** OpenAPI documents both Bearer-protected GET operations and profile schemas; Postman covers both operations. Handler tests cover JWT identity use, date/phone JSON, public privacy, self lookup, invalid IDs, missing/internal errors, and missing authentication. Router/OpenAPI/Postman parity tests pass.
- **Verification:** targeted handler, Swagger, router, and PostgreSQL HTTP tests passed; full `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` passed.
- **Boundary:** `PATCH /api/users/me`, profile update validation/persistence, avatar editing, email editing, OTP, and deletion remain unimplemented.
- **Next:** P08 - model omitted, explicit-null, and concrete-value PATCH field states without exposing non-editable fields.

### 2026-09-28 - P06: Private/public profile read use cases

- **Status:** DONE. The user implemented both read use cases and their fake-repository tests; the assistant corrected the requested method-name and test-block syntax issues.
- **Changed:** added separate `GetPrivateProfile` and `GetPublicProfile` use cases. Each rejects non-positive IDs before repository access, forwards the original context and ID, depends only on its narrow reader interface, and preserves repository results/errors.
- **Tests:** external-package unit tests cover successful private/public results, context and ID forwarding, zero/negative validation without repository calls, and repository error propagation.
- **Verification:** targeted `go test -count=1 -v ./internal/domain/usecase/user/...` and the full PostgreSQL-backed `go test -count=1 ./...` suite passed; `go vet ./...` and `git diff --check` passed.
- **Boundary:** no HTTP handler, DTO, route, production wiring, OpenAPI path, or profile endpoint exists yet.
- **Next:** P07 - expose authenticated private/public GET profile APIs with exact privacy DTOs, wiring, OpenAPI, and HTTP/contract tests.

### 2026-09-28 - P05: PostgreSQL private/public profile reads

- **Status:** DONE. The user implemented both PostgreSQL read methods; the assistant completed the requested external-package integration tests and fixed the two production issues those tests exposed.
- **Changed:** `PostgresUserRepository` now implements `PrivateProfileReader` and `PublicProfileReader`. Private reads convert nullable date/phone columns to domain pointers; public reads select only ID, combined name, and avatar. Both parameterize the ID, filter `deleted_at IS NULL`, map missing rows to `ErrUserNotFound`, preserve wrapped causes, and propagate request context.
- **Fixes from tests:** the public-name `concat_ws` delimiter now contains one space, producing `Binh Tran` instead of `BinhTran`; the public lookup error wrapper now includes the conventional separator before `%w`.
- **Verification:** real PostgreSQL tests in `internal/data/repository/test` cover complete and nullable private profiles, public output, missing users, soft-deleted users, context cancellation, and the fixed avatar. Targeted tests and the full `go test -count=1 ./...` suite passed; `go vet ./...` and `git diff --check` passed.
- **Boundary:** no profile use case, handler, route, response DTO, or OpenAPI path was added.
- **Next:** P06 - implement and unit-test separate private/public profile read use cases.

### 2026-09-28 - P04: Profile read repository interfaces

- **Status:** DONE. The user implemented and corrected the P04 interface file after review.
- **Changed:** added `internal/domain/repository/profile_repository.go` with separate `PrivateProfileReader` and `PublicProfileReader` interfaces. Each accepts request context and a user ID and returns only its matching domain profile type.
- **Design boundary:** the existing auth-focused `UserRepository` was not expanded. P04 contains no SQL, Gin, PostgreSQL driver, validation, handler, route, or OpenAPI behavior.
- **Verified:** the file name and formatting are correct; domain tests and the full PostgreSQL-backed `go test -count=1 ./...` suite passed; `go vet ./...` and `git diff --check` passed.
- **Next:** P05 - implement both read interfaces on `PostgresUserRepository` with active-user filtering and integration coverage.

### 2026-09-28 - P03: Private and public profile domain models

- **Status:** DONE. The user implemented the P03 domain types after the assistant reverted its earlier implementation.
- **Changed:** `internal/domain/entity/user.go` now defines separate `PrivateProfile` and `PublicProfile` values plus stable `ErrPhoneNumberTaken`. Nullable date of birth and phone number use pointers. The public type contains only ID, combined name, and avatar URL.
- **Boundary:** no profile repository operation, use case, handler, route, DTO, OpenAPI path, or JSON serialization rule was added. The domain types import neither Gin nor SQL.
- **Verified:** `gofmt -d internal/domain/entity/user.go` produced no diff; domain tests and the full PostgreSQL-backed `go test -count=1 ./...` suite passed; `go vet ./...` and `git diff --check` passed.
- **Note:** the error text currently starts with an uppercase letter. This does not affect `errors.Is` identity or the future HTTP code mapping, but lowercase error text is the usual Go convention.
- **Next:** P04 - define context-aware repository interfaces for private and public profile reads.

### 2026-09-28 - P02: Fixed default user avatar

- **Status:** DONE. The user created the forward migration and updated existing repository fixtures; the missing migration-transition coverage and completion record were then added on request.
- **Changed:** migration 000002 sets the fixed avatar default and backfills only empty values; its down migration restores the original empty-string default without deleting backfilled data. The isolated test database helper applies migrations 000001 and 000002 in order, and user repository expectations use the new default.
- **Verified:** a real PostgreSQL migration test exercises down, old empty/custom fixtures, up backfill, preservation of the custom value, the new default, and restoration of the old default after down. Full tests, vet, formatting, and diff checks passed as recorded in the completion report.
- **Boundary:** no upload/edit API, profile route, repository profile operation, or OpenAPI path was added. The P03 implementation previously added by the assistant was reverted so the user can implement it while following guidance.
- **Next:** P03 - add separate private/public profile domain models and a stable phone-number-conflict error.

### 2026-09-28 - P01: Approved user profile contract

- **Status:** DONE. The user assigned documentation-only P01 after confirming the profile design.
- **Changed:** recorded U14 in `DECISIONS.md`; documented the three authenticated profile routes, private/public DTO boundary, PATCH presence/null behavior, validation, fixed avatar, safe errors, and deferred deletion/upload/email/OTP scope in `CONTRACTS.md`.
- **Implementation boundary:** no Go, router, OpenAPI, migration, database, Postman collection, or runtime behavior changed. The profile routes remain unimplemented, so OpenAPI remains aligned with the production route table.
- **Verified:** reviewed the recorded decisions against the approved profile contract; checked links/status/terminology and ran `git diff --check`. Application and database tests were not run because P01 changes documentation only.
- **Next:** P02 - establish the fixed default avatar with a forward migration and database-backed verification.

### 2026-09-25 — Registration input normalization cleanup

- **Status:** DONE. The user requested the small cleanup discussed while reviewing presentation versus domain validation.
- **Changed:** `Register.Execute` now normalizes first name, last name, and email once at the use-case boundary before validating them; password remains byte-for-byte unchanged. Registration then uses only the normalized input for persistence. The registration test now exercises `Execute` directly and proves invalid input cannot reach the password hasher or repository.
- **Contract:** no HTTP request/response, validation limit, or stored-data contract changed.
- **Verified:** `gofmt` completed; `go test -count=1 ./internal/domain/usecase/auth/...` and `go vet ./...` passed. `go test -count=1 ./...` passed all non-database packages but the full command failed because the required isolated `chat_app_test` database was unavailable. Diff checks passed for the files changed in this cleanup.
- **Next:** continue the user's architecture/router questions; B30 remains the next unimplemented product item and still requires the Flutter platform decision.

### 2026-09-23 — X04: Simple refresh-token flow

- **Status:** DONE. The user selected a simple refresh-token implementation for the current MVP.
- **Changed:** login issues typed access/refresh JWTs and returns both expirations; `POST /api/auth/refresh` validates a refresh JWT and issues a new access JWT; `JWT_REFRESH_TTL` defaults to 720h. Updated configuration, Swagger, contracts, decisions, README, and authentication handoff.
- **Security boundary:** middleware accepts only `type=access`; refresh accepts only `type=refresh`. The endpoint uses the existing JSON limits and returns `Cache-Control: no-store`.
- **Verified:** adapter, use-case, handler, configuration, router, and full repository tests pass; `go vet ./...` and `git diff --check` pass. A temporary live server returned 401 for an invalid refresh token, exposed the refresh operation in its nine-path Swagger contract, and shut down gracefully. PostgreSQL integration remains conditional on `TEST_DATABASE_URL`.
- **Known limitation:** refresh JWTs are stateless and reusable until expiry. There is no rotation, token reuse detection, database session, logout endpoint, or server-side revocation.
- **Next:** connect the Flutter client to login/refresh and secure storage, or resolve the platform choice before B30 WebSocket authentication.

### 2026-09-23 — S01: Interactive OpenAPI documentation

- **Status:** DONE. The user directly requested Swagger coverage for the complete currently implemented backend.
- **Changed:** added `internal/presentation/http/swagger` with an embedded OpenAPI 3.0.3 contract, a Swagger UI page, redirect/spec routes, and route/spec tests; registered it from the production router; documented the URL and authorization workflow in the backend README.
- **Coverage:** health, register, login, user search, open direct conversation, conversation list, send/history messages, and mark-read are documented with their actual authentication, media type, body-size, validation, pagination, response, and error contracts.
- **Verified:** `go test ./internal/presentation/http/swagger ./internal/presentation/http/...`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. A temporary server on port 18081 returned HTTP 200 for the UI and served OpenAPI 3.0.3 with 8 paths and 9 operations; it then shut down gracefully.
- **Unverified / incomplete:** WebSocket is intentionally absent until B30-B32 exist. The UI shell depends on pinned CDN assets; `/swagger/openapi.json` does not.
- **Next:** B30 remains the next product item and still requires the Flutter platform decision.

### 2026-09-18 — Documentation preparation

- **Scope:** architecture, rules, workflow, backlog, and contract design only. B01–B34 have not been implemented.
- **Source code:** the skeleton `main.go` has no behavior; there is no module, dependency setup, migration, or application test suite.
- **Confirmed pace:** one small piece per turn, explain it in Vietnamese, then stop. Demo data consists of two users and one conversation; there is no conversation creation API.
- **Verification:** documentation was compared with the roadmap and directory state; no build, server, or feature tests were run.
- **Next handoff:** B01 starts only after an implementation request from the user. Read DECISIONS before work depending on unresolved design choices; use the WORKFLOW log template for subsequent implementation turns.

### 2026-09-18 — Documentation language and location

- **Scope:** backend guidance is consolidated under `backend/docs/` and translated into English, including this backlog; the backend README links to it.
- **Language rule:** user-facing explanations and code walkthroughs must remain in Vietnamese before and after each small change.
- **Implementation status:** all B01–B34 items remain TODO; translation does not implement application behavior.

### 2026-09-18 — MVP implementation plan

- **Scope:** added MVP_PLAN.md to organize B01–B34 into eight milestones with observable outcomes, decision checkpoints, acceptance criteria, and progress tracking rules.
- **Tracking:** this backlog remains the single source of task status; milestone completion is derived from the related items and evidence.
- **Implementation status:** planning only; all B01–B34 items remain TODO. No application code or environment setup was added. B01 remains the first item to assign when the user is ready.

### 2026-09-18 — B01: Initialize the Go module

- **Status:** DONE. The user explicitly requested B01 and an additional rule requiring agents to report completed work after every small step.
- **Changed:** created `backend/go.mod` with module `github.com/lilpao0/chat_app/backend`; the current module target is Go 1.26.0. Preserved the existing empty `cmd/api/main.go` at that stage and added no application dependencies. Updated rules, workflow, project guidance, and decision records.
- **Selection basis:** the module path follows the existing origin `https://github.com/lilpao0/chat_app.git` and the backend subdirectory. The current installed toolchain reports `go version go1.27.1 windows/amd64` and builds the Go 1.26.0 module target. See implementation choice I01 in DECISIONS.
- **Verified:** `go mod init github.com/lilpao0/chat_app/backend` succeeded; `go list -m` returned the expected module path; `go env GOMOD GOWORK` identified `backend/go.mod` and no active workspace; `go build ./...` passed from `backend/`.
- **Verification environment:** `GOTOOLCHAIN=local`; build cache directed to the OS temporary directory with `GOCACHE`. The generated `backend/api.exe` was removed after verification so no build artifact remains in the project.
- **Completion report / explanation in Vietnamese:** module creation, selected path/version, successful build, updated agent reporting rules, the purpose of `go.mod`, and the distinction between a buildable empty entrypoint and a running HTTP server.
- **Unverified / not implemented:** no HTTP server, configuration loading, dependencies, database, or auth. No application tests were added or run because this step only initializes the module and the entrypoint is empty.
- **Next:** B02 — configure the HTTP port. B02–B34 remain TODO; do not begin B02 automatically.

### 2026-09-21 ? Three-layer directory reorganization

- **Completed work:** reorganized application packages into `internal/presentation`, `internal/domain`, and `internal/data` at the user's request. B01?B07 remain DONE; B08 remains TODO.
- **Moved:** `internal/handler/health.go` ? `internal/presentation/http/handler/health.go`; `internal/infrastructure/database/postgres.go` ? `internal/data/database/postgres.go`; `internal/config/config.go` ? `cmd/api/config/config.go`. Updated main imports without changing application logic.
- **Preserved:** old SQL from `internal/migrate` moved to `internal/data/database/legacy_migrations` with a warning explaining that only `backend/migrations` is active. No migration was executed.
- **Documentation:** updated architecture, agent rules, decisions, entry-point READMEs, and domain guidance. Earlier handoff paths are historical; use this entry for their current locations. Empty subdivisions indicate future responsibilities, not implemented features.
- **Verified:** `go test ./...` and `go vet ./...` passed with local toolchain and dependency downloads disabled. All Go packages report no test files. `git diff --check` passed; no Go imports reference the old locations.
- **Unverified:** did not start HTTP/PostgreSQL or rerun migration checks for this directory-only change.
- **Reported/explained in Vietnamese:** the three layer responsibilities, composition root, moved files, verification limits, and unchanged B08 continuation point.
- **Next:** B08 ? User entity in `domain/entity`, repository interface in `domain/repository`, PostgreSQL implementation in `data/repository`.

### 2026-09-21 ? B08: Minimal User repository

- **Completed:** added `domain/entity/user.go` (User and stable errors), `domain/repository/user_repository.go` (CreateUser and UserRepository interface), and `data/repository/postgres_user_repository.go` (PostgreSQL implementation).
- **Behavior:** Create uses parameterized INSERT RETURNING; FindByEmail uses parameterized SELECT. Missing rows map to ErrUserNotFound; only the users_email_key unique constraint maps to ErrEmailTaken. Other errors retain their cause. Request context reaches SQL. PasswordHash is excluded from JSON; future handlers must still use public DTOs.
- **Verification:** local toolchain with downloads disabled; go test -count=1 -v ./... passed including real PostgreSQL integration on chat_app_test. Tests cover create/find/defaults, missing users, duplicate emails, parameterized lookup, non-email constraint errors, canceled contexts, and JSON hash exclusion. The test uses a connection-local temporary table built from the active migration; persistent tables are not modified. go vet and diff checks also passed. Initial checks encountered sandbox cache restrictions, DB startup timing, and a test migration path error; these were resolved before the passing run.
- **Environment:** Docker Desktop was started with approval; chat-postgres remains available. No passwords were printed or committed.
- **Explanation delivered in Vietnamese:** entities versus interfaces versus SQL adapters, implicit interface implementation, placeholders and Scan, error mapping, and context propagation.
- **Remaining:** no registration/login endpoint, normalization, hashing adapter, or use case wiring was added. Repository construction will be wired when a use case consumes it.
- **Next:** B09 ? bcrypt password adapter. M2's B05?B08 are now DONE.

### 2026-09-21 - B09: bcrypt password adapter

- **Completed:** added `domain/usecase/auth/password.go` with PasswordHasher, ErrInvalidPassword, and reusable validation; added `data/auth/password.go` implementing Hash and Compare with bcrypt. No registration or login use case was added.
- **Policy:** valid UTF-8, at least 8 Unicode code points, at most 72 bytes, with no trimming or truncation. Both Hash and Compare reject invalid inputs. Compare returns false/nil for a mismatch; malformed stored hashes return a sanitized error.
- **Dependency:** reused the already pinned golang.org/x/crypto v0.9.0; marked it direct in go.mod. No dependency upgrade/download. Default bcrypt cost is 10; hashing generates its own salt.
- **Verified:** go test -count=1 ./... and go vet ./... passed. Tests cover correct/incorrect passwords, fresh salts, preserved whitespace, malformed hashes, Unicode code-point and byte boundaries, invalid UTF-8, and overlong input sharing a valid 72-byte prefix. PostgreSQL integration was skipped because TEST_DATABASE_URL was not set; B09 does not access a database. git diff --check passed.
- **Explained in Vietnamese:** domain owns the contract and policy; data implements the technology. Go checks interface implementation through matching methods. Hashing, salts, comparisons, boundaries, and verification commands were explained.
- **Next:** B10 - registration use case using UserRepository and PasswordHasher; no HTTP endpoint until B11.
