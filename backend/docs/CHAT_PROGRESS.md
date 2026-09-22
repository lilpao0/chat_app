# REST chat handoff - 2026-09-21

The user selected option 1 for B16 (U10) and previously authorized continuation until a decision is needed (U09). B16-B29 are implemented, in presentation/domain/data, with new tests under test/ subdirectories. No frontend changes, commits or deployments were made.

## B16 - Schema

Added migration 000002 up/down: conversations, conversation_members and messages. Composite foreign keys enforce member-only sending and same-conversation read positions. Read ID/time are both null until reading. Identity sequences use CACHE 1 NO CYCLE. Indexes support membership lookup and message pagination.

Verified actual up/down/up SQL in a private transactional schema on chat_app_test: foreign keys, duplicate membership, non-member send, blank/oversized content, foreign/missing read targets, paired read fields, valid marker and sequence configuration. The test transaction rolls back all its fixtures. Public development/test tables were not migrated or reset.

Vietnamese explanation: last_read_message_id identifies read content; last_read_at records operation time. ID ordering also requires the write transaction implemented in B21.

## B17 - Demo seed

Added cmd/seed and data/seed.Demo. The CLI validates environment inputs and hashes passwords. An atomic, advisory-locked operation adds missing users, reuses existing users unchanged, and finds or creates exactly one conversation with that exact pair. No messages are seeded.

Integration verifies repeated seeding returns the same IDs/counts, preserves existing names/hashes, rejects identical account emails and allows both seeded accounts to log in. This was tested on private schemas; no operator-owned demo accounts were created or overwritten. Vietnamese explanation: seed data differs from schema migrations; repeat runs preserve existing accounts and credentials.

## B18-B20 - Conversation list

- **B18:** data repository returns the other participant, latest message by ID, nullable last-message fields and unread count. Filters by actor membership; excludes self-sent messages from unread count. Integration covers empty/populated chats, outsider exclusion, read marker effects and public DTO privacy.
- **B19:** domain List use case propagates authenticated identity/context through the repository contract, rejects invalid actor IDs, normalizes nil to an empty list and propagates repository errors. Fake tests verify these branches.
- **B20:** GET /api/conversations is registered in the protected group. Presentation maps a DTO without the other user's email/hash. HTTP integration verifies A/B counterparts, outsider empty list and 401 without token.

Vietnamese explanation: SQL composes list data; domain scopes the operation to the actor; presentation produces JSON and obtains actor identity only from middleware.

## B21-B23 - Sending

- **B21:** PostgreSQL Send locks an authorized conversation before INSERT allocates its identity, saves a message and updates conversation.updated_at in one transaction. Updated time uses GREATEST to prevent regression. Foreign keys enforce membership as well.
- **B22:** domain Send validates IDs, UTF-8, no NUL, non-whitespace content and a 2000-code-point limit while preserving original whitespace. The atomic writer contract enforces membership during persistence. Tests cover valid/error/non-member inputs and forwarding.
- **B23:** POST /api/conversations/:id/messages accepts only content and takes sender identity from middleware. Returns a committed Message DTO. HTTP integration checks content preservation, sender, invalid paths/content/types/forged sender_id and anonymous/non-member rejection.

PostgreSQL tests deliberately fail the conversation update after INSERT and verify rollback leaves no partial message. A held writer transaction plus a second writer verifies the second blocks before consuming a sequence ID, then commits with a higher ID. Vietnamese explanation: transactions preserve both writes; per-conversation locking makes ID boundaries reliable. POST retries can still duplicate messages; there is no idempotency key.

## B24-B26 - History

- **B24:** data repository filters by conversation and membership, returns descending initial/before pages or ascending after pages, fetches limit+1 for has_more and emits only the applicable next cursor. IDs are numeric boundaries; no lookup of the cursor message is required.
- **B25:** domain History checks IDs, default limit 20/max 100, exclusive before/after modes and membership before fetching. Fake tests cover invalid parameters, default limit, non-member and repository failures.
- **B26:** GET /api/conversations/:id/messages strictly parses query parameters, maps Message DTOs and nullable cursors, and rejects malformed/duplicate/unknown query fields.

HTTP/PostgreSQL tests verify empty history, initial/older/newer pages, boundary exclusion, new messages inserted between page requests, foreign-conversation numeric cursors without data leakage, invalid limits/cursors and outsider rejection. Vietnamese explanation: before retrieves history; after retrieves missed persisted messages for future reconnect recovery. No WebSocket recovery has been claimed yet.

## B27-B29 - Read state

- **B27:** MarkRead locks the actor's membership row, validates the target in the same conversation, and updates with GREATEST. Timestamp changes only when advancing; repeated/older updates return the effective current ID. The SQL transaction protects concurrent device updates.
- **B28:** domain MarkRead validates positive IDs and membership, then invokes the data interface. Fake tests cover invalid targets, outsiders, effective marker and repository errors.
- **B29:** POST /api/conversations/:id/read accepts last_read_message_id only and returns the effective marker. HTTP integration verifies reduced unread count, another user's marker unchanged, foreign read target rejection and outsider rejection.

PostgreSQL tests show reading through message 2 leaves message 3 unread, older requests do not regress ID/time, concurrent updates finish at the maximum marker, and self-sent messages do not increase unread count. Vietnamese explanation: the client reports the last displayed message, rather than marking whatever happens to exist when the request reaches the server.

## Verification and next step

`go test -count=1 ./...` and `go test -race -count=1 ./...` passed with TEST_DATABASE_URL configured, including migration, seed, transaction, pagination, authorization and read-state tests. `go vet ./...` and `git diff --check` passed. Private schemas are removed at cleanup; existing data is preserved. Without TEST_DATABASE_URL, database tests skip.

No standalone server/Flutter demo or production DB migration was performed in this session. Before running chat locally, apply both migrations and run the seed explicitly with operator-provided environment values. Source-level checks and integration tests are not a claim of B33/B34 acceptance completion.

Next: B30, after asking whether the Flutter target is mobile only or also Web. WebSocket handshake, hub, events and final acceptance remain unfinished.
