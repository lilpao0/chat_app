# Backend backlog and current handoff

Updated: 2026-09-29. This is the single source of implementation status. Detailed historical handoffs remain available in Git history; they are not duplicated in active documentation.

## Current status

| Area | Status | Implemented outcome |
|---|---|---|
| B01-B34 | DONE | Server, PostgreSQL, auth, REST chat, read state, realtime delivery and acceptance |
| X01-X08 | DONE | User discovery, direct 1-1 chat, refresh tokens and OpenAPI/Swagger |
| P01-P14 | DONE | Private/public profile APIs and profile updates |
| WS2-01-WS2-08 | DONE | Bidirectional socket commands, acknowledgements, durable send retries, lifecycle and documentation |

The supported clients are Flutter Android and iOS. The backend uses one Go API process, PostgreSQL, Gin, JWT, bcrypt and `coder/websocket`. REST compatibility and REST history catch-up remain available.

## Verified state

On 2026-09-29:

- `go test -race -count=1 -timeout=120s ./...` passed, including mandatory integration tests against isolated schemas in `chat_app_test`.
- `go vet ./...`, `go build ./...`, `go mod tidy -diff`, changed-file formatting and `git diff --check` passed.
- OpenAPI and Postman documents parse as JSON; Swagger contract tests verify routes, authentication, retry input, socket message schemas and internal references.
- Development database `chat_app` reports migration version 3 with `dirty=false`. The default avatar, nullable message request UUID and sender-scoped unique constraint are present.
- The production Docker image builds from the repository-root `backend/` context. Cloud Build and Terraform trigger definitions use that same path and cap Cloud Run at one instance to match the in-memory hub.

## 2026-09-29 audit and cleanup

- Removed duplicated roadmap/progress/verification documents and an obsolete domain README. Git history retains their completed handoff details.
- Replaced the 700-line historical backlog with this current status, limitations and next-work summary.
- Corrected stale documentation about WebSocket completion, retry behavior, migration ownership and working pace; local Markdown links and JSON documents were checked.
- Formatted three legacy auth files that were the only Go files reported by `gofmt -l`; this was mechanical and did not change behavior.
- Fixed deployment paths, escaped Cloud Build substitutions in Terraform and restricted Cloud Run to one instance until a shared realtime broadcaster exists.
- No application migration, stored data, public REST compatibility route or deployment file was removed.

## Implemented contracts

- Authentication: register, login, refresh and protected Bearer authentication.
- Users: search; private self profile; limited profile update; public profile.
- Conversations: open/reuse a direct chat and list authorized conversations.
- Messages: transactional sends, history pagination and monotonic read positions.
- Realtime: authenticated `/ws`, `send_message`, `mark_read`, correlated acknowledgements/errors and member-authorized `new_message` fan-out.
- Retry safety: a canonical UUID is unique per sender. An identical retry returns the original message without republishing; conflicting reuse returns `request_conflict`. REST accepts the same optional key for fallback.

See [CONTRACTS.md](CONTRACTS.md) for the API contract, [WEBSOCKET_PLAN.md](WEBSOCKET_PLAN.md) for socket examples/reconnect behavior, [ARCHITECTURE.md](ARCHITECTURE.md) for ownership, and [DECISIONS.md](DECISIONS.md) for confirmed choices.

## Known limitations

- The in-memory hub supports one API process. Multiple API replicas need an external broadcast layer before realtime fan-out is complete across replicas.
- Live events are best effort; there is no event replay or exactly-once network delivery. Clients recover from REST history and merge by message ID.
- There are no recipient read-receipt broadcasts, browser support, media, groups, typing, presence, push notifications or calls.
- Refresh tokens are reusable until expiry; there is no server-side session revocation or rotation.
- Direct-conversation reuse is protected by the application transaction path, not a schema-level unordered-pair constraint for arbitrary manual SQL.

## Operational work not performed

- No Android/iOS device integration or load test has been run.
- No production deployment has been performed by this implementation session.
- Development migration is current; future migration files must still be applied explicitly to each environment.

## Next eligible work

1. Integrate the Flutter mobile client with the documented socket retry and reconnect flow.
2. Run an end-to-end test on real Android and iOS devices.
3. Before horizontal API scaling, design a shared realtime broadcast adapter and production load limits.

New work should be added here as small, verifiable items. Do not restore per-feature progress files; use Git history for completed handoff detail.
