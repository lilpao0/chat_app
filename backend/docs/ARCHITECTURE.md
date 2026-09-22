# Clean Architecture for the Go Backend

This document describes the **intended** chat MVP server architecture, following the [six-week roadmap](../../ke-hoach-6-tuan-flutter-golang-chat-app-1.md) and the existing directories. It guides future implementation; a feature appearing in a diagram does not mean it has been implemented or verified.

When these documents were prepared on 2026-09-18, the backend had `cmd/api/main.go` with an empty `main`, skeleton directories, and a README; there was no `go.mod`. Do not create the entire tree below in one turn. **Implement only one small, verifiable piece per turn, explain it in Vietnamese, then stop for the user to read.** Read [AGENTS.md](AGENTS.md) and [WORKFLOW.md](WORKFLOW.md) before editing code; select work from [BACKLOG.md](BACKLOG.md).

The user selected three top-level application layers on 2026-09-21: `presentation`, `domain`, and `data`. Configuration belongs to startup under `cmd/api/config`; active SQL migrations stay at `backend/migrations`. This supersedes the previous flat layer layout.

## 1. Core idea

The server receives HTTP requests or WebSocket connections, applies application rules, and reads/writes PostgreSQL. Clean Architecture separates decisions about **what the application does** from decisions about **which technology performs the work**.

For example, "only conversation members may send messages" belongs in a use case. PostgreSQL queries belong in the repository implementation. Parsing JSON and returning HTTP statuses belong in presentation. These responsibilities can change and be tested separately.

This architecture is a project convention, not a mandatory Go directory layout. One server, one database, and dependencies passed directly through constructors are sufficient for the MVP.

## 2. Components and responsibilities

| Component | Question it answers | Intended contents | Keep out |
|---|---|---|---|
| `internal/domain/entity` | What are the core concepts? | User, Conversation, ConversationMember, Message; shared business errors; pure data rules when needed | Gin, SQL, JWT, bcrypt, HTTP statuses |
| `internal/domain/repository` | What data operations do use cases need? | Small user/conversation/message interfaces; plain input/result types when needed | DB connections, SQL statements, `sql.Tx`, `sql.Rows` |
| `internal/domain/usecase` | What rules govern a user action? | Registration, login, conversation listing, message reading/sending, marking read | JSON parsing, HTTP responses, SQL, opening DB connections |
| `internal/presentation/http` | How does HTTP become a use case call? | Router, middleware, handlers, request/response DTOs | Conversation authorization decisions, password hashing, DB queries |
| `internal/presentation/websocket` | How are connections managed and events sent over WebSocket? | Handshake, connection management/hub, JSON event delivery | Persisting messages directly or bypassing use cases based on client-provided IDs |
| `internal/data/repository` | How are persistence contracts implemented in PostgreSQL? | SQL, Scan, transactions, DB-to-application error mapping | HTTP responses, handler dependencies |
| `internal/data/database` | How are DB connections opened and managed? | Pool initialization, connectivity checks, resource cleanup | Message-sending or login rules |
| `internal/data/auth` | How are passwords hashed/checked and tokens signed/verified? | bcrypt and JWT adapters | Deciding conversation membership |
| `cmd/api/config` | Which configuration values does the server need? | Reading and validating environment variables | Business logic, global services or DB connections |
| `cmd/api` | How are components assembled and the server started? | `main.go`, dependency construction, lifecycle, shutdown | Endpoint logic, SQL statements |
| `migrations` | How does storage structure change in controlled steps? | Incremental schema and index changes | Request handling code |

An adapter connects the application to a particular technology. A PostgreSQL repository is a persistence adapter; bcrypt/JWT are security adapters; the WebSocket hub is an event delivery adapter.

## 3. Package dependencies and runtime flow are different

### Compile-time dependencies

Here, `A -> B` means **package A may import package B**:

```text
presentation -> domain/usecase, domain/entity
domain/usecase -> domain/repository, domain/entity
domain/repository -> domain/entity
data/repository -> domain/repository, domain/entity
data/auth -> domain/usecase (when implementing its ports)
cmd/api -> config, presentation, domain, data
```

The domain layer must not import presentation or data. Entities must not import use cases or repositories; repository interfaces must not import use cases. Use cases must not import Gin, SQL, JWT libraries, or WebSocket libraries. Presentation receives use cases through constructors; it must not construct data adapters. `cmd/api` wires concrete implementations. Standard-library types such as `context.Context` are allowed across boundaries.

### Runtime request flow

```text
Flutter / Postman
       |
       v
HTTP middleware + handler
       |
       v
Use case
       |
       v
Repository interface
       |
       v
PostgreSQL repository implementation
       |
       v
PostgreSQL
```

At runtime, the use case calls a PostgreSQL-backed object through an injected interface. This does not mean it imports the implementation. It knows only the contract; `main.go` selects the concrete objects and wires them together.

In Go, a type satisfies an interface when it has the required methods with matching signatures; there is no `implements` declaration. Both the real repository and a test fake can therefore satisfy a use case's requirements. [Source: Go — Interfaces and other types](https://go.dev/doc/effective_go#interfaces_and_types).

## 4. Intended directory tree

This is a map of where code belongs when needed, not a list of files to create immediately. File names may be refined within the relevant small item as long as responsibilities remain clear.

```text
backend/
  cmd/api/
    main.go
    config/config.go
  internal/
    presentation/
      http/
        handler/health.go
        middleware/                 # future
        router.go                   # future
      websocket/                    # future
    domain/
      entity/                       # future: User, Conversation, Message
      repository/                   # future: repository interfaces
      usecase/                      # future: auth, conversation, message
    data/
      database/
        postgres.go
        legacy_migrations/          # preserved historical SQL; do not apply
      repository/                   # future: PostgreSQL implementations
      auth/                         # future: bcrypt and JWT adapters
  migrations/                       # active SQL migrations
  docs/
```


Not every struct needs an interface. Add `ports.go` only when an actual use case needs an external capability. Do not create empty files merely to fill out the tree.

## 5. Small interfaces and dependency wiring

Repository interfaces live in `internal/domain/repository`, following the selected project structure. An operation describes a real need, such as finding a user by email or saving a message together with its conversation update, instead of offering a generic CRUD repository for every table.

Describe other external capabilities with small interfaces close to their consumers:

- `domain/usecase/auth` may define password hashing/comparison and token issuance contracts. Implementations live in `data/auth`.
- HTTP middleware needs token verification and may receive a small verification interface defined by its consumer package. Middleware must not construct the JWT implementation itself.
- `domain/usecase/message` may define a contract for publishing notifications about persisted messages. The hub in `presentation/websocket` satisfies it and converts events to JSON for authorized connections.

Contracts across boundaries use ordinary Go values, domain entities, plain input/result types, `context.Context`, and `error`. Do not pass `*gin.Context`, `*sql.DB`, `*sql.Tx`, `*sql.Rows`, or WebSocket connections into use cases.

`main.go` is the **composition root**: the place that knows how to assemble the application. It reads config, opens the DB, creates repositories and auth adapters, constructs use cases with those dependencies, builds handlers/the hub, registers routes, and runs the server. Manage server/hub lifecycles here or in nearby startup helpers. Use constructors directly; do not add a dependency injection framework or service locator.

Before the realtime milestone, implement only message persistence and responses. Add the publisher contract and wire the hub when working on WebSocket; do not build a general event system in advance.

## 6. Entities, DTOs, and database mapping

| Data representation | Purpose | Location | Example distinction |
|---|---|---|---|
| Domain entity | Represent application concepts | `domain` | Message has an ID, conversation, sender, content, and creation time |
| Use case input/output | Describe data required for an action | Relevant use case package, or shared domain/repository types when truly needed | Send-message input includes the authenticated user, conversation, and content |
| HTTP/WebSocket DTO | Define public request, response, and event shapes | Relevant presentation package | JSON names such as `sender_id`, timestamp parsing, API error structure |
| DB mapping | Convert SQL rows/columns into application data | `data/repository` | Handle nullable columns with SQL types, then convert to application Go types |

Handlers convert request DTOs into use case inputs and map results to response DTOs. Repositories read columns with `Scan` and return domain entities or plain query results. SQL objects must not escape the adapter.

Do not expose internal entities directly as responses just because some fields currently match. Sensitive data such as password hashes must never appear in JSON or logs. Follow [CONTRACTS.md](CONTRACTS.md) for JSON names, exposed fields, and pagination conventions.

A conversation list often needs the other participant, the last message, and unread count. A small query-result type suited to that view is fine; there is no need to force every result into an entity or build a CQRS system.

## 7. Example: sending a message

This describes responsibilities, not implementation code. Explain this flow to the user in Vietnamese when implementing it:

1. Middleware verifies the token and identifies the user. `sender_id` must come from that verified identity, not from a client-supplied body value.
2. The handler reads the conversation ID and JSON, handles parsing errors, and calls the use case with the request context.
3. The use case validates content against the contract and checks membership through a repository. Token authentication proves who the caller is; membership determines where they may send messages.
4. The use case requests one atomic persistence operation: saving the message and updating related conversation data must succeed or fail together.
5. The PostgreSQL adapter opens a transaction, performs the required operations, and commits. Write conditions and DB constraints protect consistency. If membership changes concurrently in the future, protect/check authorization at write time rather than relying only on an earlier read. Failures before commit must roll back and must not publish events.
6. Only after a successful commit does the use case ask the publisher to notify authorized members about the persisted message. The hub manages connections and converts the event to WebSocket format.
7. The handler returns the saved message. If persistence succeeded but realtime publication failed, saving still succeeded; record the delivery error and let clients recover through REST. A closed recipient connection must not turn a persisted message into a failed send response.

```text
Authenticate -> check membership -> atomic persistence -> COMMIT
                                                           |
                                                           +-> best-effort realtime event
                                                           |
                                                           +-> return persisted message
```

Best effort means the MVP attempts realtime delivery but does not guarantee every event arrives. The server can stop after commit and before publication. The database is authoritative; clients reload history when reconnecting. Do not automatically add Kafka, Redis, or an outbox in this small item.

REST sends messages in the MVP; WebSocket delivers events. Connection authentication, event format, and reconnect behavior are defined in [CONTRACTS.md](CONTRACTS.md) and [DECISIONS.md](DECISIONS.md).

## 8. Transactions, context, and errors

**Transactions:** use cases express atomicity requirements through repository operations. `sql.Tx`, begin/commit/rollback, and SQL statements stay in the adapter. Do not make independent insert-message and update-conversation calls when they must succeed together. Do not hold a transaction open while sending over WebSocket. [Source: Go — Executing transactions](https://go.dev/doc/database/execute-transactions).

**Context:** handlers pass the HTTP request context through use cases and repositories so deadlines/cancellation reach DB operations. Request-processing functions take context as a parameter; do not store a request context in a service struct or replace it with `context.Background()` partway through the flow. Context carries lifecycle signals, not every business input. [Source: Go — package context](https://pkg.go.dev/context).

**Errors:** the domain may define stable errors such as not found, conflict, or forbidden. Adapters translate meaningful DB failures to these errors; use cases add context as needed; presentation maps them to contract-defined HTTP statuses and error codes. Do not branch on error-message strings. Preserve the cause when wrapping errors so `errors.Is`/`errors.As` work, including cancellation/deadline errors. [Source: Go — package errors](https://pkg.go.dev/errors).

Recognize `context.Canceled` and `context.DeadlineExceeded` where relevant. Never return SQL statements, driver errors, stack traces, or secrets in responses. Log technical errors at the responsible boundary instead of repeatedly logging one error at every layer.

## 9. Verify architecture one small piece at a time

Auth tests live in `internal/data/auth/test` and `internal/domain/usecase/auth/test`, following the user's directory preference (2026-09-21). These are separate packages that import exported parent APIs. Do not export implementation details just for tests; use public behavior. Include `/...` when running tests from the parent auth directory.

- When adding a business rule, explain in Vietnamese why it belongs in the use case or domain; check valid inputs and a rejected case.
- When adding a repository, check real SQL and transactions with PostgreSQL once the DB is available. A fake repository tests use cases, not whether real SQL works.
- When adding a handler, check parsing, authentication state, response statuses, and DTOs against the contract; do not repeat the entire business test suite in handler tests.
- When adding realtime behavior, verify that only members receive events, publication never precedes commit, closed connections do not break persistence, and clients can recover history.
- After each piece, explain the completed data flow, related files, verification, and unfinished work in Vietnamese using [WORKFLOW.md](WORKFLOW.md).

There is no current need for a mock generator, generic repository, DI framework, general event bus, microservices, or extra service layers wrapping use cases. Add components only when a concrete item in [BACKLOG.md](BACKLOG.md) reveals an unclear responsibility.
