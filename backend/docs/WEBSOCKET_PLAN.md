# Bidirectional WebSocket migration plan

Updated: 2026-09-28. This replaces the completed W01-W13 implementation plan for notification-only sockets. Historical verification remains in [BACKLOG.md](BACKLOG.md). The new implementation items are WS2-01 through WS2-08; status is tracked only in the backlog.

## Scope and current state

The user requested bidirectional chat. The runtime now accepts socket commands and retains REST compatibility. The protocol below is implemented; verification status is tracked in BACKLOG.

Reuse Android/iOS Bearer authentication, `/ws`, the user/device hub, event DTO, membership checks, PostgreSQL message transactions, history, and read-position use cases. Keep the existing REST routes during migration; deleting a public endpoint is not necessary to enable socket commands. REST fallback and catch-up responses use the shared `status`/`data`/`meta` HTTP envelope; socket frames below do not.

Commands: `send_message` and `mark_read`. Responses: `message_sent`, `read_updated`, and `error`. Existing `new_message` remains the fan-out event. `read_updated` acknowledges the requesting device's marker; it does not introduce recipient read receipts.

The single-process hub remains. Browser support was added separately under [FLUTTER_WEB_SUPPORT_PLAN.md](FLUTTER_WEB_SUPPORT_PLAN.md); browser admission uses one-use tickets, without changing the protocol below. See [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md). Media, groups, typing, presence, distributed broadcasting, guaranteed event replay, and exactly-once network delivery remain outside scope.

## Cleanup boundaries

- Remove the temporary `library.go` close-code aliases; use the library constants directly.
- Replace the old active plan and REST-only future requirement with this migration plan.
- Replace the reject-all reader and its specific test only when the validated command dispatcher is ready. Removing the guard early would silently discard client messages.
- Keep message validation, membership filtering, bounded queues, expiry, shutdown, public DTOs, and tests protecting those behaviors.
- Keep REST history, conversation discovery/listing, authentication, profile APIs, and mark-read/send routes for compatibility. Removing REST mutations is a separate later API decision.
- Do not delete stored messages or rewrite applied migrations. Add a forward migration for idempotency.

## Target wire protocol

Each client command is exactly one JSON object in a text WebSocket message. Reject unknown fields at both envelope and data levels, invalid types, trailing JSON, and unsupported command types. Never accept actor identity from the payload.

```json
{"type":"send_message","request_id":"550e8400-e29b-41d4-a716-446655440000","data":{"conversation_id":10,"content":"Hello"}}
```

After successful commit, reply only to the originating connection:

```json
{"type":"message_sent","request_id":"550e8400-e29b-41d4-a716-446655440000","data":{"id":101,"conversation_id":10,"sender_id":1,"content":"Hello","created_at":"2026-09-28T03:00:00Z"}}
```

Fan out `new_message` to all online member devices, including the sender. Acknowledgement and broadcast may arrive in either order; merge both by message ID. `message_sent` confirms persistence, not delivery or reading by another device.

```json
{"type":"mark_read","request_id":"550e8400-e29b-41d4-a716-446655440001","data":{"conversation_id":10,"last_read_message_id":101}}
{"type":"read_updated","request_id":"550e8400-e29b-41d4-a716-446655440001","data":{"conversation_id":10,"last_read_message_id":101}}
```

Return the effective monotonic read marker, which may exceed the requested one.

```json
{"type":"error","request_id":"550e8400-e29b-41d4-a716-446655440000","error":{"code":"not_found","message":"Conversation not found."}}
```

Error codes: `invalid_input`, `unsupported_command`, `not_found`, `request_conflict`, `rate_limited`, and `internal_error`. Invalid/missing request IDs produce an error with `request_id: null`; never echo an unvalidated unbounded value. These are socket events, not HTTP status responses after upgrade.

Use a canonical UUID request ID generated once per logical send. Retry the same command with the same ID until resolved. A new logical message must use a new ID.

## Durable duplicate-send prevention

Storage: nullable UUID `client_request_id` on messages and a unique constraint on `(sender_id, client_request_id)`. Existing rows and legacy REST requests may retain null IDs. The REST send endpoint accepts the same optional request ID for safe transport fallback; requests without a key retain existing duplicate-on-retry behavior.

Within the repository transaction, check current membership, serialize conversation writes, and resolve concurrent requests through database uniqueness:

- Same sender/key and identical conversation/content: return the original message with `created=false`.
- Same sender/key but different conversation/content: return `request_conflict`, without inserting.
- First accepted key: insert, commit, return `created=true`.
- Rollback: no acknowledgement of success and no publish.

Do not implement this with a process-local map or a check-then-insert without a unique constraint. Resolve cross-conversation races safely; do not continue SQL work in a transaction already aborted by a constraint error. Use a conflict-aware insert or rollback and a fresh lookup as appropriate.

Publish only for newly created messages. Replays return an acknowledgement even if the original acknowledgement was lost. If initial publish failed, replay does not guarantee a replacement event; REST catch-up remains necessary. Retain keys for the lifetime of the message in this MVP. This prevents duplicate persistence for keyed sends, not exactly-once delivery.

## Connection execution model

Reader -> bounded command queue -> sequential command worker -> shared use case -> outbound queue -> writer.

- Keep control-frame reading responsive while a command waits for PostgreSQL; do not run SQL directly inside the read loop.
- Serialize commands per connection; do not launch an unbounded goroutine per incoming frame.
- Only the writer sends application events, including acknowledgements and errors.
- Use a session-owned cancellation context and a bounded per-command timeout. Cancel work on disconnect, expiry, or shutdown. If commit succeeded just before cancellation, retry with the same key resolves ambiguity.
- A heartbeat waiting for pong must not block token-expiry or shutdown processing. Add independent cancellation/close supervision and wait for all workers to terminate.
- Stop accepting commands after expiry/shutdown. Graceful close must have a hard deadline and forced close fallback; HTTP shutdown alone does not close hijacked sockets.
- Handle all exit paths in `cmd/api`, including HTTP server failure and shutdown timeout, before database teardown.

Implemented configuration: command queue capacity 16, command timeout 5s, per-connection command rate 10/s with burst 20. Validate bounds and document them when implemented. Full queues or repeated abuse close the connection; rate errors are best effort and must not grow another unbounded queue.

The default inbound message limit is 16 KiB (previously 1 KiB). It limits the whole WebSocket message, not an individual frame. This accommodates the envelope and 2,000 Unicode code points in typical UTF-8 encoding. Byte and Unicode limits are separate: heavily escaped JSON may exceed the byte budget even for valid-length content.

## Implementation sequence

### WS2-01 - Strict command and response DTOs

Add `command.go`, decoder tests, response DTOs and error mapping in presentation/websocket. Reuse the full message DTO. Validate UUIDs, envelope/data unknown fields, nulls, IDs, malformed/trailing JSON, unsupported command types, and size limits. Keep domain code independent of JSON/WebSocket.

Exit: protocol tests pass; no command execution enabled yet.

### WS2-02 - Idempotent PostgreSQL message writes

Add a new up/down migration, domain input/result/error changes, and transactional repository handling. Preserve existing message IDs and data. Update isolated test schema migration loading and registration of new migrations as needed.

Exit: real PostgreSQL tests cover concurrent identical retries, conflicting content/conversation, two senders using the same key, missing membership, rollback, legacy null keys, and migration transitions. Document that down removes deduplication metadata but keeps messages.

### WS2-03 - Shared send/read use cases

Extend send to accept an optional client key and use the repository's `created` result. Return the committed result when publishing fails; do not publish again on replay. Reuse monotonic mark-read authorization and validation. Extend REST input with the optional key so fallback can share deduplication, preserving old requests.

Exit: tests prove publish-after-commit, no publish on replay/rollback, retry resolution, and unchanged unkeyed REST behavior. Update OpenAPI/Postman for the optional field only when implemented.

### WS2-04 - Lifecycle and bounded command dispatcher

Refactor `connection.go`, hub/session injection, and WebSocket config. Replace the one-shot reject-all reader with bounded parsing/dispatch. Add a command worker, timeout/rate controls, response queuing, and cancellation supervision. Current browser admission checks one approved Origin plus a valid ticket; duplicate headers must not bypass origin validation.

Exit: tests cover reader responsiveness during blocked SQL, ping/pong success and failure, expiry during blocked ping/write/command, slow outbound consumers, command overflow, oversized input, binary messages, disconnect cleanup, and forced shutdown. Run under the race detector.

### WS2-05 - Socket send and acknowledgement

Inject the shared send use case into sessions; use authenticated session identity. Queue `message_sent` after persistence and correlated safe errors on failure. Wire the actual production constructors through `cmd/api`; do not let the hub construct repositories.

Exit: A's socket command creates one message; A's submitting device receives acknowledgement; all A/B devices receive the public event; C receives neither; retry after losing the acknowledgement returns the same ID. Tests must permit acknowledgement/event reordering.

### WS2-06 - Socket mark-read

Dispatch `mark_read` to the existing use case and queue `read_updated` to the requesting connection. No recipient read-receipt broadcasts are introduced.

Exit: valid updates, repeated/older markers, concurrent devices, outsider denial, foreign-message rejection, and REST unread counts all behave consistently.

### WS2-07 - Reconnect and full acceptance

Run real JWT + PostgreSQL + WebSocket scenarios through production route registration. Cover expired/refresh-token rejection with real tokens, text/binary errors, concurrent retry uniqueness, lost acknowledgement, rollback, publication failure, socket/REST fallback using the same key, read/unread, and shutdown.

On reconnect, establish the socket and buffer events before REST catch-up. Use the last completed REST synchronization cursor per conversation, not merely the largest live-event ID: live events can arrive out of order and a higher ID does not prove lower messages were received. Follow every `has_more` page, merge by message ID, then apply buffered live events. Newly discovered conversations need their own history synchronization. This still provides best-effort live delivery; no server replay is claimed.

Exit: `go test -race -count=1 ./...`, `go vet ./...`, formatting and diff checks pass. Database tests do not skip. Tests exercise publication ordering, retry races, and lifecycle deadlines rather than only matching the happy-path implementation.

### WS2-08 - Documentation and final cleanup

Update implemented contracts, config examples, Flutter socket send/read/retry/catch-up examples, REST optional-idempotency input, OpenAPI and Postman. Remove the obsolete blanket-rejection test only after its replacement tests pass. Remove genuinely unused adapters found by reference search; keep compatibility routes and historical handoffs.

Record executed checks and remaining limitations honestly in BACKLOG. Never label the bidirectional migration DONE solely because the earlier notification-only test suite passes. No automatic commit, push, deployment, or Flutter UI work is implied.

## Acceptance outcome

Android/iOS and approved-origin Web clients send messages and read markers through the authenticated socket, receive correlated acknowledgements/errors, and receive member-authorized new-message broadcasts. Lost acknowledgements can be retried without duplicate persisted messages. History and reconnect recovery remain REST-based. All connection workers have bounded resources and deterministic cleanup.
