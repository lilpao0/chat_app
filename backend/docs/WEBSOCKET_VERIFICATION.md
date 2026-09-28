# Bidirectional WebSocket verification and file map

Date: 2026-09-28. Scope: the complete backend test suite, documentation/Swagger synchronization, and the user-authorized commit/push of the WebSocket work.

## Executed verification

- PASS: `go test -race -count=1 -timeout=120s ./...`, including real PostgreSQL integration tests. No database-test skips.
- PASS: `go vet ./...` and `go build ./...`.
- PASS: OpenAPI and Postman JSON parsing; Swagger tests check registered routes, authentication, optional REST retry keys, HTTP 409, socket message schemas and all internal references.
- PASS: formatting of changed/new Go files and `git diff --check`.
- Repository-wide formatting audit also found three pre-existing, unchanged auth files that gofmt would reformat: `internal/domain/usecase/auth/password.go`, `validation.go`, and `test/validation_test.go`. They were left untouched to keep this change scoped; this does not prevent compilation or tests.

Tests use disposable schemas in `chat_app_test`. No development/production migration, Flutter device test, load test or deployment was performed. Tests do not establish production capacity or multi-process support.

## Runtime flow and ownership

1. `internal/presentation/authentication/bearer.go` verifies the Bearer header for HTTP and WS. `websocket/handler.go` authenticates before upgrade and rejects browser Origin values.
2. `websocket/connection.go` owns reader, bounded command queue, sequential worker, writer, heartbeat, expiry and cancellation. `hub.go` tracks all authenticated devices and handles fan-out/shutdown.
3. `websocket/command.go` decodes send/read envelopes, derives the actor from the session, invokes injected use cases, and produces correlated acknowledgements or sanitized errors.
4. `internal/domain/usecase/message/send.go` validates content and request IDs, invokes persistence, then publishes only new committed messages. Replay returns the same saved result without another event.
5. `internal/data/repository/postgres_message_repository.go` locks the conversation and checks membership before inserting. `ON CONFLICT DO NOTHING` and a subsequent lookup resolve concurrent sender/key retries without using an aborted transaction.
6. `migrations/000003_message_request_id.up.sql` adds nullable UUID keys and sender-scoped uniqueness. Its down migration removes retry metadata but preserves messages.
7. `websocket/publisher.go`, `event.go`, and `postgres_conversation_members.go` publish public messages only to current member devices. Delivery is best effort.
8. `internal/domain/usecase/conversation/read.go` retains the shared monotonic read-marker rules. Socket `read_updated` acknowledges the submitting device; it is not a recipient read receipt.
9. `internal/presentation/http/handler/messages.go` keeps REST compatibility and accepts the same optional request ID for transport fallback.
10. `cmd/api/main.go` wires shared use cases, hub and routes; `cmd/api/config/websocket.go` validates limits documented in `.env.example`.

The channels bound queued work; goroutines separate I/O and database latency; contexts cancel pending operations. Domain interfaces keep SQL and WebSocket library types outside application rules.

## Relevant tests

| Area | Files | Main coverage |
|---|---|---|
| Persistence | `internal/data/repository/test/message_retry_test.go`, `messages_test.go` | Concurrent retries, cross-conversation conflict, sender scope, authorization, migration round trip, rollback and ordering |
| Use case | `internal/domain/usecase/message/test/retry_test.go`, `send_test.go` | Publish only after successful new persistence; no republish on retry; publication failure does not lose success |
| End-to-end | `internal/presentation/websocket/bidirectional_test.go`, `acceptance_test.go` | Real JWT/PostgreSQL/socket flow, member-device fan-out, outsider denial, reconnect and REST fallback, read markers |
| Protocol/lifecycle | `command_test.go`, `lifecycle_test.go`, `realtime_test.go`, `hub_internal_test.go` in the WS package | Invalid JSON/data, bounds, timeout/rate/queue limits, blocked work/write/pong, expiry and shutdown |
| Handshake | `handler_test.go`, `real_auth_test.go` in the WS package | Bearer/Origin checks, real refresh and expired-token rejection |
| API documentation | `internal/presentation/http/swagger/websocket_contract_test.go`, `contract_test.go` | Routes and socket/REST contract references |

## Documentation and operation

README and WEBSOCKET_PLAN contain mobile commands/retry/catch-up examples. CONTRACTS defines the wire behavior; DECISIONS and ARCHITECTURE no longer describe notification-only sending. OpenAPI 1.1.0 documents the upgrade plus WS component schemas through an `x-websocket` extension. Swagger UI is an HTTP documentation/client surface, not an interactive socket client. Postman retains a manually controlled request-ID variable so retry does not accidentally generate a new key.

Before running the updated API, apply migration 000003 to the intended development database. Change an existing `WS_MAX_MESSAGE_BYTES=1024` override to `16384`. On reconnect, buffer socket events, fetch all REST catch-up pages from the last completed REST synchronization checkpoint, and merge by message ID.

Removed/replaced legacy pieces: the close-code alias `websocket/library.go` is absent; the reject-all reader and blanket-rejection test were replaced in place. Necessary REST APIs, stored data, migrations and active adapters remain.

Limitations: Android/iOS only, one API process, no guaranteed event replay/exactly-once delivery, no distributed hub or recipient read receipts. Commit/push confirmation and its commit ID are reported after Git succeeds; this report does not claim a deployment.
