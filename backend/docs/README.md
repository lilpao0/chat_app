# Go Chat App Backend

A 1–1 chat backend for Flutter: registration/login, conversation lists, message history, text messages, realtime delivery, and unread state.

**Current state (2026-10-07):** B01-B34, X01-X08, P01-P14, WS2-01-WS2-08 and WEB-01-WEB-04 are DONE. REST envelopes, exact-origin CORS and browser one-use WS tickets join existing Android/iOS bidirectional chat. Local PostgreSQL acceptance passes; Cloud Run release and actual Flutter Web acceptance remain pending under WEB-05. Startup requires DATABASE_URL and JWT_SECRET; apply all three current migrations. Set WEB_ALLOWED_ORIGINS for browser clients. See [BACKLOG](BACKLOG.md) for the latest handoff.

## Where to start

| Document | Purpose |
|---|---|
| [AGENTS.md](AGENTS.md) | Mandatory rules: small steps, Vietnamese code explanations, and scope control |
| [WORKFLOW](WORKFLOW.md) | Workflow for one turn and the handoff template |
| [MVP_PLAN](MVP_PLAN.md) | Eight implementation milestones, observable outcomes, acceptance criteria, and progress tracking |
| [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md) | Implemented bidirectional protocol, retry rules and WS2 implementation sequence |
| [RESPONSE_MIGRATION_PLAN.md](RESPONSE_MIGRATION_PLAN.md) | File-by-file implementation record for the REST status/data/meta/error migration |
| [FLUTTER_WEB_SUPPORT_PLAN.md](FLUTTER_WEB_SUPPORT_PLAN.md) | Browser CORS/ticket implementation sequence and acceptance checks |
| [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md) | FE browser examples, Cloud Run setup and refresh/reconnect procedure |
| [ARCHITECTURE](ARCHITECTURE.md) | Clean Architecture in Go, structure, and dependency direction |
| [DECISIONS](DECISIONS.md) | Source requirements, proposed additions, and open questions |
| [CONTRACTS](CONTRACTS.md) | Intended API, data, authentication, and WebSocket behavior |
| [schema.dbml](schema.dbml) | Database diagram source for dbdiagram.io |
| [BACKLOG](BACKLOG.md) | Small ordered tasks, dependencies, and completion criteria |
| [REST Postman collection](chat-app-rest.postman_collection.json) | Importable HTTP requests for the complete REST flow |
| [WebSocket AsyncAPI](chat-app-websocket.asyncapi.json) | Importable Postman definition for bidirectional socket commands and events |

A new agent should read AGENTS and WORKFLOW first, inspect BACKLOG, then read the relevant design sections. The entire backlog is not an instruction to implement everything. **Documentation is in English; explanations to the user and code walkthroughs must be in Vietnamese.**

After each small item, agents must also provide a **Vietnamese report of completed work**: actions, changed files, actual verification results, unfinished scope, and the next suggested item. Explaining the code alone does not satisfy this requirement.

For the overall implementation sequence, start with [MVP_PLAN](MVP_PLAN.md). It groups B01–B34 into milestones; continue updating task status only in BACKLOG.

## Architecture at a glance

```text
Request → Handler → Use case → Repository interface
                                     ↑ implementation
                              PostgreSQL adapter → Database
```

Handlers understand HTTP; use cases understand the user's action; repository interfaces describe required data operations; adapters execute SQL. The domain contains User, Conversation, and Message. Use cases do not import PostgreSQL or Gin. `cmd/api` wires the components at startup.

The stack follows the roadmap: Go, Gin, PostgreSQL, `database/sql`, bcrypt, JWT, and WebSocket. This is one simple server; multiple services are not needed yet.

## Build and continuation

From `backend/`, run `go test ./...` and `go vet ./...`. PostgreSQL integration tests are mandatory: they use `TEST_DATABASE_URL`, or derive `chat_app_test` from the local `.env` development URL, and fail rather than skip when the isolated database is unavailable. See [backend README](../README.md) for database configuration and migration instructions. The server requires `DATABASE_URL`; `HTTP_PORT` defaults to 8080.

Bidirectional socket commands, correlated acknowledgements and durable retries are implemented. See [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md) for the protocol and [BACKLOG](BACKLOG.md) for verification. REST compatibility and history remain available.

For Postman, import the REST collection and WebSocket AsyncAPI JSON separately. In the generated WebSocket request, connect to `ws://localhost:8080/ws`, add `Authorization: Bearer <access_token>` to the handshake headers, and use the examples generated from the AsyncAPI document.

For browser clients, obtain a fresh ticket through `POST /api/ws/tickets`, then open `/ws?ticket=...` from an approved Origin without Authorization. See [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md).

Product source: [Six-week Flutter + Golang roadmap](../../ke-hoach-6-tuan-flutter-golang-chat-app-1.md). Implementation additions are listed separately in DECISIONS; current verification and remaining work live in BACKLOG.
