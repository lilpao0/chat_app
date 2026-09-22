# Go Chat App Backend

All backend guidance is collected in [docs/README.md](docs/README.md).

Follow the [MVP implementation plan](docs/MVP_PLAN.md) for the eight milestones and acceptance criteria; track individual task status in [BACKLOG](docs/BACKLOG.md).

Read [docs/AGENTS.md](docs/AGENTS.md) and current decisions first. The user authorizes continuation until a decision is needed; retain small steps, verification and **Vietnamese** explanations. Documentation is written in English.

The Go module is `github.com/lilpao0/chat_app/backend`. Auth and REST chat endpoints are implemented through B29, with user discovery/direct chat added under X01-X03. Startup requires `DATABASE_URL` and `JWT_SECRET`. B30 awaits the mobile/Web platform choice for WebSocket.

## Local database migrations

The project uses the locally installed `migrate` CLI with SQL files in `migrations/`. Use `.env.example` as a reference; never commit real credentials.

From PowerShell in `backend/`, apply migrations with an absolute forward-slash path:

```powershell
$env:DATABASE_URL = 'postgres://postgres:YOUR_POSTGRES_PASSWORD@127.0.0.1:5433/chat_app?sslmode=disable'
migrate -path "C:/absolute/path/to/backend/migrations" -database $env:DATABASE_URL up
```

Use `TEST_DATABASE_URL` and `chat_app_test` for migration and repository tests. Do not run destructive migration commands against `chat_app`.

## Application layers

- `internal/presentation`: HTTP handlers/middleware and future WebSocket delivery.
- `internal/domain`: entities, repository interfaces, and use cases; no dependencies on presentation or data.
- `internal/data`: database connection, repository implementations, and auth adapters.

`cmd/api/main.go` wires the layers; startup configuration lives in `cmd/api/config`. Only `migrations/` is the active migration source. Historical SQL is preserved under `internal/data/database/legacy_migrations` and must not be applied.

## Repository checks

From `backend/`, set `TEST_DATABASE_URL` to `chat_app_test` and run `go test -count=1 -v ./...`, then `go vet ./...`. Database tests skip without the variable and refuse other database names. They use temporary tables or unique private schemas removed at cleanup; existing data is preserved. `go test -race -count=1 ./...` also passed with PostgreSQL configured.

## Password adapter checks

Run `go test -v ./internal/data/auth/... ./internal/domain/usecase/auth/...` from backend to verify auth adapters and use cases without PostgreSQL. Tests live in each auth package's `test/` subfolder and exercise exported APIs. The `...` includes these subfolders. Domain owns the interfaces; data/auth implements bcrypt and JWT. Registration/login are wired through these interfaces.

## Running the REST API

From backend, set DATABASE_URL to a database with migrations 000001 and 000002 applied and JWT_SECRET to a random secret of at least 32 bytes. Optional defaults: JWT_ISSUER=chat-app, JWT_AUDIENCE=chat-app-mobile, JWT_TTL=24h, HTTP_PORT=8080. The API and seed commands optionally load `.env` from the current working directory; explicit process variables take precedence. Tests use `TEST_DATABASE_URL` from the process environment. Never commit actual credentials.

Start with `go run ./cmd/api`. Send Content-Type: application/json. POST /api/auth/register accepts name, email and password; it returns a public user without automatically logging in. POST /api/auth/login accepts email/password and returns access_token, expires_at and user. Protected routes use Authorization: Bearer <access_token>.

Tokens expire after the configured TTL. Logout removes the client token; server revocation and refresh tokens are outside this demo. See [the auth handoff](docs/AUTH_PROGRESS.md) for verification. This session exercised the production router through httptest, not a separately launched network server.

## Demo seed

After applying both migrations, set SEED_A_NAME, SEED_A_EMAIL, SEED_A_PASSWORD, SEED_B_NAME, SEED_B_EMAIL and SEED_B_PASSWORD, then run `go run ./cmd/seed`. DATABASE_URL selects the target. Passwords use the registration policy; the two normalized emails must differ.

Repeat runs reuse existing accounts without changing names/passwords and reuse the exact pair's conversation. User-driven direct chat opening uses the same pair lookup. If an email already exists, its existing password still applies, regardless of the supplied seed password. Seeding is never automatic. Verification created fixtures only in private chat_app_test schemas, not development accounts.

## Protected chat endpoints

| Endpoint | Input / result |
|---|---|
| GET /api/users | q: name substring or exact email; optional limit (1-50), after_id; returns public summaries without emails |
| POST /api/conversations/direct | JSON user_id; returns a new conversation with 201 or existing one with 200 |
| GET /api/conversations | Actor's conversations, counterpart, last message and unread count |
| POST /api/conversations/:id/messages | JSON content; returns committed message |
| GET /api/conversations/:id/messages | Optional limit (1-100), before_id or after_id; messages and cursors |
| POST /api/conversations/:id/read | JSON last_read_message_id; returns effective read position |

Initial/before history is descending by ID; after history is ascending. The client marks only through its last displayed message. Older requests cannot regress the marker; unseen newer messages stay unread. Non-member/missing conversations return 404. Send retries may duplicate messages. See [REST chat verification](docs/CHAT_PROGRESS.md). WebSocket remains unfinished.

To start a chat, authenticate, search `GET /api/users?q=binh`, then send `POST /api/conversations/direct` with `{"user_id":2}`. The response contains the conversation ID for the existing message/history/read endpoints. Repeated or simultaneous requests for the same unordered pair reuse one conversation through an application transaction lock. Manual SQL that bypasses this path can still create duplicates because the schema has no unordered-pair uniqueness constraint. See [discovery verification](docs/DISCOVERY_PROGRESS.md).
