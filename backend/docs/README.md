# Go Chat App Backend

A 1–1 chat backend for Flutter: registration/login, conversation lists, message history, text messages, realtime delivery, and unread state.

**Current state (2026-09-21):** B01-B29 of the original 34-item plan and X01-X03 (user discovery/direct chat extension) are DONE. Auth and REST chat, including search/open direct 1-1 chats, are implemented and tested with PostgreSQL. Startup requires DATABASE_URL and JWT_SECRET; chat requires migrations 000001 and 000002. Code is grouped under presentation/domain/data. B30 awaits the Flutter platform choice. See [discovery handoff](DISCOVERY_PROGRESS.md), [REST chat handoff](CHAT_PROGRESS.md) and [authentication handoff](AUTH_PROGRESS.md).

## Where to start

| Document | Purpose |
|---|---|
| [AGENTS.md](AGENTS.md) | Mandatory rules: small steps, Vietnamese code explanations, and scope control |
| [WORKFLOW](WORKFLOW.md) | Workflow for one turn and the handoff template |
| [MVP_PLAN](MVP_PLAN.md) | Eight implementation milestones, observable outcomes, acceptance criteria, and progress tracking |
| [ARCHITECTURE](ARCHITECTURE.md) | Clean Architecture in Go, structure, and dependency direction |
| [DECISIONS](DECISIONS.md) | Source requirements, proposed additions, and open questions |
| [CONTRACTS](CONTRACTS.md) | Intended API, data, authentication, and WebSocket behavior |
| [schema.dbml](schema.dbml) | Database diagram source for dbdiagram.io |
| [BACKLOG](BACKLOG.md) | Small ordered tasks, dependencies, and completion criteria |

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

From `backend/`, run `go test ./...` and `go vet ./...`. Repository integration tests require TEST_DATABASE_URL pointing to chat_app_test; without it they skip. Password adapter and JSON hash-exclusion tests run without a database. See [backend README](../README.md) for database configuration and migration instructions. The server requires `DATABASE_URL`; `HTTP_PORT` defaults to 8080.

Continue with B30 after confirming mobile-only or also Flutter Web. D08-D09 were confirmed as option 1 and implemented. The user authorizes continuation until a question needs their decision.

Product source: [Six-week Flutter + Golang roadmap](../../ke-hoach-6-tuan-flutter-golang-chat-app-1.md). Implementation additions are listed separately in DECISIONS; do not rewrite the source document to make proposals appear to be confirmed requirements.
