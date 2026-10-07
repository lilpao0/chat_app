# Backend backlog and current handoff

Updated: 2026-10-07. This is the single source of implementation status. Detailed historical handoffs remain available in Git history; they are not duplicated in active documentation.

## Current status

| Area | Status | Implemented outcome |
|---|---|---|
| B01-B34 | DONE | Server, PostgreSQL, auth, REST chat, read state, realtime delivery and acceptance |
| X01-X08 | DONE | User discovery, direct 1-1 chat, refresh tokens and OpenAPI/Swagger |
| P01-P14 | DONE | Private/public profile APIs and profile updates |
| WS2-01-WS2-08 | DONE | Bidirectional socket commands, acknowledgements, durable send retries, lifecycle and documentation |
| WEB-01-WEB-04 | DONE | Exact-origin CORS, one-use browser tickets, mobile compatibility and FE contracts |
| WEB-05 | IN_PROGRESS | Local backend acceptance and hosted URL/preflight diagnosis complete; release and actual Flutter Web acceptance pending |

The supported clients are Flutter Android, iOS and Web. The backend uses one Go API process, PostgreSQL, Gin, JWT, bcrypt and `coder/websocket`. REST compatibility and REST history catch-up remain available.

## Verified state

On 2026-09-29:

- `go test -race -count=1 -timeout=120s ./...` passed, including mandatory integration tests against isolated schemas in `chat_app_test`.
- `go vet ./...`, `go build ./...`, `go mod tidy -diff`, changed-file formatting and `git diff --check` passed.
- OpenAPI, the REST Postman collection and the WebSocket AsyncAPI import document parse as JSON; Swagger contract tests verify routes, authentication, retry input, socket message schemas and internal references.
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
- There are no recipient read-receipt broadcasts, media, groups, typing, presence, push notifications or calls.
- Browser tickets are process-local: restart/rolling revisions lose pending tickets. Multiple replicas need both shared atomic ticket storage and realtime broadcasting. Ticket query URLs must be redacted/excluded from infrastructure logs.
- Refresh tokens are reusable until expiry; there is no server-side session revocation or rotation.
- Direct-conversation reuse is protected by the application transaction path, not a schema-level unordered-pair constraint for arbitrary manual SQL.

## Operational work not performed

- No actual Flutter Web or Android/iOS device integration or load test has been run.
- No production deployment has been performed by this implementation session.
- Development migration is current; future migration files must still be applied explicitly to each environment.

## Next eligible work

1. Obtain explicit release authorization, deploy the changed backend, configure the actual origin and logging, and verify the hosted revision; see [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md).
2. Confirm dedicated hosted A/B test accounts privately and run actual Flutter Web acceptance; mobile device acceptance remains separate.
3. Before horizontal API scaling, design shared atomic ticket storage, a shared realtime broadcast adapter and production load limits.

New work should be added here as small, verifiable items. Do not restore per-feature progress files; use Git history for completed handoff detail.

## 2026-10-06 response migration planning

- Planning: DONE. [RESPONSE_MIGRATION_PLAN.md](RESPONSE_MIGRATION_PLAN.md) records the file-by-file migration to the selected status/data/meta/error envelope.
- Implementation: DONE for backend REST runtime, REST handshake failures before WebSocket upgrade, tests, OpenAPI, Postman and documentation. Frontend, domain/data behavior, database schema and post-upgrade WebSocket frames were not changed.
- Contract: success uses `status: success` plus `data`; lists live directly in `data`; pagination uses `meta.pagination`; 4xx uses `status: fail`; 5xx uses `status: error`; absent `meta`/`details` are omitted.
- Verification: focused HTTP handler/middleware/response/swagger tests pass; OpenAPI, Postman and AsyncAPI JSON parse; `go vet ./...` passes; `git diff --check` passes.
- Earlier blocked verification (`requires chat_app_test`) was resolved on 2026-10-07 using the existing isolated test database and private schemas, without resetting development data. Full normal/race suites and vet now pass. Fixed an empty-list test to decode JSON instead of depending on object-key ordering; added missing profile envelope assertions to Postman. Manual Postman execution remains unrun.

## 2026-10-07 Flutter Web implementation handoff

- Planning: DONE. [FLUTTER_WEB_SUPPORT_PLAN.md](FLUTTER_WEB_SUPPORT_PLAN.md) retains the original sequence and exit criteria; [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md) supplies the FE/operator contract.
- Backend implementation: DONE for WEB-01 through WEB-04. No frontend, domain/data behavior, schema, dependency or deployment file changes were required. Existing uncommitted response migration and user seed/debug files were preserved.

| Item | Status | Outcome / dependency |
|---|---|---|
| WEB-01 | DONE | Shared validated origin allowlist; Authorization/Content-Type preflight before auth; application error CORS |
| WEB-02 | DONE | Bounded atomic TicketStore, access-capped expiry, Origin binding, protected empty-body issuance and no-store |
| WEB-03 | DONE | Browser ticket admission, failed-upgrade/reuse/concurrent-reuse rejection, mobile regression and access-expiry closure |
| WEB-04 | DONE | OpenAPI/security/schema assertions, Postman issuance/profile checks, AsyncAPI alternatives and FE examples |
| WEB-05 | IN_PROGRESS | Local browser-shaped backend acceptance passes and hosted URL/preflight inspected; deployment, hosted A/B credentials and actual Flutter Web run pending |

- Origin assumption: `http://localhost:5173`, based on user's suggested port; confirm actual `location.origin`. Read-only Cloud Run lookup confirmed `https://chat-api-bzhivzj3sq-as.a.run.app` (WS uses `wss` with `/ws`). Hosted OPTIONS `/api/conversations` still returns wildcard Origin and omits Authorization, proving the deployed revision has not received this fix. The earlier Apache localhost probe was not this service.
- Files/flow: config/web.go validates origins; router.go applies CORS before auth; authentication/ticket.go holds only hashed ticket keys plus verified identity/Origin; handler/ws_ticket.go returns 201; websocket/handler.go consumes before Accept and runs the existing identity-bound session. main.go shares one store. Domain use cases still authorize membership and persist before fan-out.
- Verified from `backend/`: `go test -count=1 -timeout=120s ./...`, `go test -race -count=1 -timeout=120s ./...` and `go vet ./...` passed, including mandatory PostgreSQL integration. Browser/mobile acceptance covers WS send/read, multiple devices, durable retry/reconnect, REST fallback/catch-up and outsider isolation. Atomic tests prove exactly one consumer, including concurrent real handshakes. All three JSON artifacts parse; formatting and `git diff --check` pass.
- Unrun: actual Flutter Web browser CORS/FE integration, manual Postman execution, hosted login/accounts, upgraded hosted ticket/socket flow and network interruption through Cloud Run. No cloud writes, commit, push or frontend edits were performed.
- Final checks: `go build ./...` passed; local Markdown links resolved; all 21 Postman scripts parsed as JavaScript. Scripts were not executed against a server. Removed only the session-created reproducible Go cache after verification.
- Next: separately authorized release plus origin/logging setup, private A/B credential confirmation and real browser acceptance. Do not mark WEB-05 DONE from Go tests alone.
