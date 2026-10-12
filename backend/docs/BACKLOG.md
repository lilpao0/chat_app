# Backend backlog and current handoff

Updated: 2026-10-12. This is the single source of implementation status. Earlier dated entries below are historical and superseded by this handoff.

## 2026-10-12 commit and CI handoff

- User explicitly assigned review, commit and push on `backend/Bao` so they can create a PR to `main`. The branch was fast-forwarded to current `origin/main` while preserving media edits; only the existing workflow relocation changed.
- Verified freshly with a disposable loopback PostgreSQL18 `chat_app_test`: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...` all passed. Changed Go files are gofmt-clean; YAML/embedded Python, workflow gates and Terraform formatting checks pass.
- Existing GitHub Actions on main had successful past deploys, but its direct deploy omitted the media key/job. Workflow now checks PRs with isolated DB/race/vet/build and deploys only after a successful main push using the shared Cloud Build pipeline. Alternative Terraform/YAML trigger definitions use that same file and builder, with a SHA tag. No cloud trigger was provisioned or deployment rerun in this review.
- Source/Git exclude generated auth credentials, local environment files and livecheck artifacts. Fixture upload handles412 safely on same-commit rebuilds. Existing `GCP_SA_KEY` is reused; new submitter permissions and actual GitHub execution remain event-time verification, not a claimed CI pass.
- Media runtime/migrations/tests/contracts/release evidence are selected for commit. Local seed/debug tools and older live-check artifacts remain outside this change. Actual Flutter/device work remains MED-08; user owns PR creation and merge.

## 2026-10-12 cloud media release handoff

- Assigned cloud steps 1–5: DONE. Hosted DB backup and guarded baseline/migrations verify version5/clean and unchanged migration-time counts. Private GCS/IAM/stable shared key provisioned. API/job deployed with 2 GiB and single-instance WS topology. Scheduler every5 minutes enabled and invoked successfully. Frontend unchanged; no commit/push.
- Final full Cloud Build `a02564f5-a103-4fd8-b56a-50440e8008d8` SUCCESS; ready revision `chat-api-00006-mvn`, image `media-20261012-cancel200`. Native ffmpeg/ffprobe passed in built image.
- Final revision passed167 real API/GCS/WS assertions. Cleanup execution `chat-media-cleanup-xtd6f` succeeded with cleaned4. Final fixture orphan/failed objects return404 and rows are deleted; three attached generations/bindings remain. Natural09:45 schedule logged cleaned0 after queue drained. Provider/DB cleanup assertions pass.
- Live findings fixed: completed GCS session cancellation returns200; adapter now proceeds to object deletion. Signed lifetime rounds down to899–900 seconds; bounded assertions corrected. Focused storage/use-case tests and vet pass. Earlier failed builds/cleanup attempts and safe resolution are documented, not counted as pass.
- Automatic approval review rejected broad deployment grants to runtime. Dedicated CI uses service/job/resource-specific grants; runtime retains existing unrelated permissions, not certified minimal. Bucket soft delete stays7 days; application deletion is not immediate physical erasure.
- Evidence and handoff: [release report](MEDIA_RELEASE_2026-10-12.md), [API results](MEDIA_LIVE_RESULTS_2026-10-12.json), [cleanup results](MEDIA_CLEANUP_RESULTS_2026-10-12.json). Temporary task secrets/capabilities removed; dedicated fixture users/messages/attached media retained.
- MED-08 remains IN_PROGRESS: assigned cloud operations complete, actual Flutter/browser/device playback, player refresh after expiry, full media boundaries, active session/late-upload races and hosted restart/concurrency acceptance remain unverified. Existing full local SQL tests remain separate evidence; fixture suite was never run against hosted DB.

## 2026-10-12 media backend upgrade handoff

- Assigned: user's latest direct instruction authorizes finishing the whole backend upgrade and reporting results, overriding earlier coaching-only pace. MED-02 through MED-07 backend DONE; MED-08 operational/provider/Flutter acceptance remains TODO. Existing user entity, validation, reservation and history work was retained. No frontend edits, cloud provisioning, hosted migration, deployment, commit or push.
- MED-02: entity/migration000004 and paged history hydration are complete. Migration000005 adds encrypted session recovery/cleanup claims; tests cover rollback refusal without data loss.
- MED-03: authenticated upload route, shared PostgreSQL pending cap, official GCS SDK/ADC, create-only resumable session/type/length, encrypted recovery before URL response, failure compensation and operational issuance limit. Application24h expiry and provider session lifetime remain different.
- MED-04: owner/current-membership completion, immutable generation, actual length/type/byte validation, stdlib JPEG/PNG pixel/full decode, bounded ffprobe MP4 codec/track checks, idempotent completion, private15-minute generation-bound access. Missing file and permanent/temporary errors have distinct states/statuses.
- MED-05: shared REST/WS optional-text/media validation, ordered atomic attachment binding, one message/unread increment, exact ordered retry comparison before readiness/expiry checks, publish only for newly committed message. Existing WS framing/auth/mark_read preserved; message data intentionally gains attachments.
- MED-06: ordered metadata on REST/WS/history, always-array attachments for text, Photo/Video/Media previews. Active OpenAPI/AsyncAPI/Postman artifacts now include runtime media; duplicate draft schemas removed. All three registered media routes have auth and error/response contracts.
- MED-07: `/app/media-cleanup`/`go run ./cmd/media-cleanup`, encrypted cancellation recovery, SKIP LOCKED claims,5-minute leases, claim tokens, generation-pinned delete/retry and8-day tombstone reconciliation. Completion budget30s/two slots, HTTP WriteTimeout45s; Docker includes ffprobe/cleanup binary.
- Verified in backend/: `go test -count=1 ./...`, `go vet ./...`, `go build ./...` passed. `go test -race -count=1 ./internal/domain/usecase/attachment ./internal/data/repository/... ./internal/presentation/websocket` passed, including the added concurrent cleanup/complete/send case. Real PostgreSQL tests cover pending cap, rollback, atomic bind/order, concurrent reuse, replay after former expiry, encryption/tampering/restart/leases and before/after pagination. REST/WS acceptance uses real auth/handlers/SQL/socket transport with fake external storage: upload creates no message/event/unread, metadata agrees on ack/event/history, retries do not republish, outsider/owner/null/unknown fields reject correctly.
- Artifact validation:71 active schema/example/boundary cases passed with installed JSON Schema validator; OpenAPI/AsyncAPI internal references and local documentation file links resolved. Route/OpenAPI/Postman coverage tests passed; `git diff --check` passed. A first artifact run found outdated flat-collection/required-content test assumptions; corrected for Postman groups/media-only contract, then reran successfully.
- Final browser fix: API CORS exposes Retry-After so browser clients can read upload backoff. Focused TestCORSPreflightAndApplicationErrors, Swagger tests, vet and build passed after this change. A blanket HTTP-package rerun after stopping the disposable DB correctly refused `requires chat_app_test`; prior full-suite DB evidence above remains the integration result. Final CORS check uses fake auth and requires no DB.
- Environment: temporary password-authenticated PostgreSQL18 loopback cluster with isolated `chat_app_test` schemas; installed service and development DB untouched. Cluster stopped and task credential file removed after checks. Full runtime GCS, actual ffprobe execution, Docker image build and Android/iOS/Web playback were not run. GCS HTTP/signing and probe-format tests are offline, not provider/device proof.
- Next release work: apply all five migrations to explicitly selected hosted DB; configure private bucket, ADC/signBlob/IAM/CORS, stable shared session key, resource sizing and cleanup schedule; perform MED-08 checklist in [MEDIA_SETUP.md](MEDIA_SETUP.md). No populated credentials/capability URLs exist in committed examples. Frontend entrypoint currently contains no chat UI; rendering/playback requires separate work. In-memory WS Hub/tickets/rate limiter remain single-instance constraints.

## 2026-10-09 MED-02 migration handoff

- Assigned: after guided entity/test/migration editing, the user asked the agent to finish all migration work. Migration portion DONE; MED-02 remains IN_PROGRESS because history attachment hydration is not implemented. Resume user-guided work for that remaining portion.
- Finished 000004 up/down: metadata table, member/message composite FKs, size/state/verified-metadata/binding/expiry CHECKs, unique positions 0-4 and pending/cleanup partial indexes. Down holds exclusive locks and refuses attachment rows or blank messages before restoring the original text CHECK; no data deletion to satisfy rollback.
- Added one migration integration test file and appended 000004 to the isolated fixture list. Existing user entity/send-test changes were retained. Updated schema.dbml, README/CONTRACTS and D16 to describe the schema foundation and pending runtime work.
- Verified from backend/ (module github.com/lilpao0/chat_app/backend): go test -count=1 -timeout=180s ./... passed, including PostgreSQL integration and concurrent upload-vs-rollback protection; go vet ./... and git diff --check passed. Tests cover existing text/timestamp/request UUID preservation, type/byte/null/FK/position boundaries, rollback refusal, restored old text CHECK and reapplication. Native migrate CLI applied 1-4, down 1 to v3, then up to clean v4 in a private schema. Local Markdown file links checked.
- Environment: configured local Docker DB at 127.0.0.1:5433 was unavailable (requires chat_app_test); Docker daemon was stopped. Used installed PostgreSQL 18 in a temporary password-authenticated loopback cluster with a separate chat_app_test database. Windows sandbox blocked pg_ctl restricted-token creation; approved escalation started only that temporary cluster. After verification, the temporary cluster was stopped and its client credential file removed. Development/hosted databases and public schemas were not migrated.
- Remaining: history hydration, runtime upload/send/access, GCS and client acceptance; race suite was not run. Next user-guided step: fetch page attachments in one query while preserving existing message pagination. No frontend edits, commit, push or deployment.

## 2026-10-09 MED-01 contract handoff

- Status: DONE. User assigned MED-01 and explicitly confirmed all proposed size/count/format/expiry limits (U19); asked why these limits matter. Later runtime items remain unassigned.
- Defined CONTRACTS section 9: exact upload/complete/access/send DTOs, optional-text validation, ordered keyed retry, idempotent completion, lifecycle/error/status mapping, private URL refresh and client draft behavior. MEDIA_PLAN and DECISIONS now reflect confirmed limits and contract scope; README explains activation.
- Updated OpenAPI/AsyncAPI with future JSON Schema variants under x-media-contract; Postman extension holds draft requests/responses and schema_source. Active paths/components, channels/components and Postman item/variable values were compared with Git HEAD and remain identical. Swagger/importers may ignore extensions; inspect JSON directly. These variants are not implemented routes.
- Verified: 61 schema/example/boundary cases (including max bytes, distinct IDs, media-only, null/NUL/Unicode whitespace and 2,000 emoji code points), internal JSON references, matching limits/shared schemas, future Postman bodies/responses and local Markdown file links. From backend/ (module github.com/lilpao0/chat_app/backend), go test -count=1 ./internal/presentation/http/swagger passed; git diff --check passed.
- Initial Unicode boundary check failed because PowerShell piped a literal emoji as question marks; rerun used chr(0x1F600) and passed. No contract defect was inferred from that shell-encoding failure.
- Unrun: full Go/race/vet/build suites, PostgreSQL migrations/integration, actual GCS/session/signing/CORS, manual Postman or Flutter playback. Provider resumable/header documentation read; no cloud changes, runtime handler/entity/migration edits or frontend changes.
- Explained to user in Vietnamese: count/bytes/pixels/codecs affect draft transfer, decode RAM, playback compatibility and cleanup; view-link expiry differs from attached-file lifetime. Next item: MED-02 schema/entity/read foundation, when assigned.

## 2026-10-09 media planning handoff

- Planning request completed in [MEDIA_PLAN.md](MEDIA_PLAN.md); U18 records user-selected Google Cloud Storage and optional text with images/videos. Runtime remains text-only. No implementation, frontend/cloud changes, commit, push or deployment authorized by this planning request.
- Initial plan covered direct resumable upload, private access, validation, atomic binding, durable retries, history/preview/unread and orphan cleanup. File formats/limits, attachment count and lifetimes were proposals at that point; U19 above subsequently confirmed them.
- Verification: current send/entity/SQL/REST/WS/history code read; local links and documentation diff checked. No Go/DB/GCS/client tests run for this documentation-only change.
- Initial next item was MED-01; its subsequent completion is recorded above. Next: MED-02 when assigned. MED-08 requires separately assigned cloud/frontend work.

| Item | Status | Planned outcome |
|---|---|---|
| MED-01 | DONE | Confirmed limits and active REST/WS upload/access contracts |
| MED-02 | DONE | Schema/entity and batched attachment history hydration |
| MED-03 | DONE | GCS resumable-session issuance; real provider acceptance under MED-08 |
| MED-04 | DONE | Validated completion and private access; native/provider/device acceptance pending |
| MED-05 | DONE | Atomic REST/WS media sending and durable ordered retries |
| MED-06 | DONE | History, preview/unread and active contract artifacts |
| MED-07 | DONE | Cleanup CLI, recovery/resource bounds and verified scheduled cloud job |
| MED-08 | IN_PROGRESS | Assigned cloud steps 1–5 verified; actual client and remaining provider acceptance pending |

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
- There are no recipient read-receipt broadcasts, groups, typing, presence, push notifications or calls. Media backend is implemented in the current handoff above.
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

## 2026-10-07 deployed API verification

- User authorized exactly two hosted QA accounts and broad functional edge-case testing. Created user IDs 17/18; conversation 3 retains 38 test messages. Final login succeeds for both; temporary phone/DOB cleared and both read through message 64. Passwords were delivered privately, not committed or stored in reports.
- Cloud Run revision `chat-api-00004-dgk` and `WEB_ALLOWED_ORIGINS=http://localhost:5173` verified; both service URL aliases were exercised. Deployment/configuration had completed before this test session; no cloud writes or application/FE changes were performed during testing.
- Executed 570 HTTP curl requests plus WS handshakes/frames; 1348 assertions/observations across five runs. Normal auth/profile/discovery/chat/read/pagination, concurrency/idempotency, browser/mobile admission, real ticket TTL/capacity and offline reconnect/catch-up passed. Five harness expectation errors were corrected/rechecked; raw artifacts retain original results.
- One remaining deployed transport anomaly: 11/20 oversized WS probes ended with EOF/reset without expected close 1009. Both aliases affected; local production-session boundary reproducer passed 12 subtests. Root cause is not established and no fix was assigned. Details/evidence: [LIVE_API_TEST_REPORT_2026-10-07.md](LIVE_API_TEST_REPORT_2026-10-07.md).
- WEB-05 remains IN_PROGRESS: hosted API/account verification is now performed, but actual Flutter Web acceptance and oversize-close investigation remain. No third user, stress test, shortened access TTL, forced outage or restart was used.
- Next eligible work: diagnose/fix hosted oversize close propagation if assigned, then actual FE A/B integration. No commit/push was requested for these test/report artifacts.
