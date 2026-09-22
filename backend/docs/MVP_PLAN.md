# Backend MVP Implementation Plan

Created: 2026-09-18. This plan turns the existing design into an ordered path to a working backend. It groups the 34 top-level items in [BACKLOG](BACKLOG.md) into eight milestones without replacing their detailed criteria.

**This plan was created during a planning-only request; it does not itself authorize implementation.** Once the user assigns implementation, complete one small backlog item, report what was done and explain the code in Vietnamese, and stop so the user can read. A milestone is not a single coding turn. Read BACKLOG for current implementation status.

## 1. Target outcome

Build one Go server that can demonstrate this flow without waiting for a Flutter UI:

1. Register a user and log in using REST.
2. Log in as each of the two seeded demo users, A and B.
3. List their shared, pre-created 1–1 conversation.
4. Send text messages and retrieve paginated history.
5. Show unread counts and mark messages as read through a displayed message.
6. Receive new-message events through authenticated WebSocket connections.
7. Reconnect and retrieve missed messages through REST.
8. Reject access from a separate non-member test user C.

Use API requests and two WebSocket clients for the server demonstration. Flutter implementation is outside this plan. The initial completion target is a reproducible local MVP; public hosting and production operations are separate work.

Keep the agreed stack: Go, Gin, PostgreSQL, `database/sql`, bcrypt, JWT, and WebSocket. Use the minimal Clean Architecture described in [ARCHITECTURE](ARCHITECTURE.md), introducing components only as needed.

## 2. Scope boundaries

| Included | Outside this MVP |
|---|---|
| Registration, login, expiring access tokens | Refresh tokens and server-side token revocation |
| Two seeded demo users plus user search and direct 1-1 chat opening (U11) | Group chat |
| Conversation list, last message, unread count | Recipient read-receipt ticks, presence, and typing indicators |
| Text sending, history pagination, marking read | Media, uploads, message editing/deletion, and reactions |
| WebSocket notifications and REST catch-up | Guaranteed event delivery, a distributed hub, and exactly-once sends |
| Local setup, migrations, tests, and run instructions | Flutter UI, public deployment, microservices, Redis, and Kafka |

Registration remains available, but a newly registered user has an empty conversation list. Client logout removes its stored token and closes its socket; the current proposal does not revoke the old token on the server. Detailed behavior and proposed additions remain in [CONTRACTS](CONTRACTS.md) and [DECISIONS](DECISIONS.md).

## 3. Milestone map

Follow M1 through M8 in order. Within each milestone, follow the listed backlog order and verify the item's explicit dependencies. The ranges below cover B01–B34 exactly once.

| Milestone | Backlog items | Observable result |
|---|---|---|
| M1 — Server foundation | B01–B04 | A configurable HTTP server responds to `/health` and shuts down cleanly |
| M2 — Database foundation | B05–B08 | The server connects to PostgreSQL; the user repository persists and retrieves data |
| M3 — Authentication | B09–B15 | Registration/login work and protected requests require a valid token |
| M4 — Demo conversations | B16–B20 | A and B can list their seeded conversation with the agreed response fields |
| M5 — REST messaging | B21–B26 | Members send persisted messages and retrieve older/newer pages |
| M6 — Read and unread state | B27–B29 | Read markers advance correctly and unread counts match them |
| M7 — Realtime delivery | B30–B32 | Authenticated members receive events for committed messages |
| M8 — Acceptance and handoff | B33–B34 | The complete demo passes and another developer can reproduce it from the docs |

M1 unlocks a basic server demo. M3 unlocks an auth demo. M6 delivers the REST chat flow. M7 adds realtime. Only M8 establishes completion of the backend MVP.

## 4. Work within each milestone

### M1 — Server foundation: B01–B04

**Goal:** establish the smallest working Go HTTP application.

- B01: choose the module path/Go version and initialize the backend module.
- B02: implement and validate HTTP port configuration.
- B03: add Gin and `GET /health`; wire only the components needed now.
- B04: implement startup failures, bounded shutdown, and server timeouts.

**Exit evidence:** the documented startup command works; `/health` returns the specified response; invalid configuration and an occupied port fail clearly; stopping the server releases resources. Explain modules, handlers, config, and the composition root in Vietnamese. Do not add DB/auth scaffolding ahead of its milestone.

### M2 — Database foundation: B05–B08

**Goal:** establish real persistence through the selected architecture.

- B05: prepare isolated development/test databases and select migration tooling.
- B06: create the users migration and its constraints.
- B07: connect using `database/sql`, with configuration, context, and pool cleanup.
- B08: implement the smallest useful user interface and PostgreSQL repository.

**Exit evidence:** migrations work on a fresh test DB; create/find/not-found/duplicate-email checks exercise real PostgreSQL; invalid connectivity fails without leaking credentials. Record runnable DB and migration instructions. Never reset the user's existing database to make a test pass.

### M3 — Authentication: B09–B15

**Goal:** provide working registration, login, and request authentication before exposing chat data.

- B09–B11: password adapter → registration use case → registration HTTP endpoint.
- B12–B14: JWT adapter → login use case → login HTTP endpoint.
- B15: authenticate protected requests and pass the verified identity to handlers.

**Exit evidence:** registration stores a hash, login returns the contract-defined token/user response, invalid credentials are rejected, and missing/invalid/expired tokens cannot reach a protected test handler. Public auth/health routes remain accessible. Verify duplicate-email behavior, including concurrent registration. Explain token lifetime and local logout limitations in Vietnamese.

### M4 — Demo conversations: B16–B20

**Goal:** make the agreed two-user demo discoverable through a real API.

- B16: add the conversation, membership, and message schema after resolving the read-marker design for this implementation.
- B17: seed A, B, one conversation, and its two memberships without duplicating or overwriting existing accounts.
- B18–B20: conversation list repository → use case → protected HTTP endpoint.

**Exit evidence for the original milestone:** A and B log in and see the shared conversation; a new unrelated user initially receives an empty list; list fields and empty-conversation values match CONTRACTS. The user later added discovery and direct chat opening under U11; see X01-X03 in BACKLOG for that extension.

### M5 — REST messaging: B21–B26

**Goal:** persist messages safely and retrieve both historical and missed messages.

- B21–B23: atomic message persistence → send use case → send HTTP endpoint.
- B24–B26: pagination repository → history/catch-up use case → history HTTP endpoint.
- Test membership, transaction rollback, ID ordering, validation, and pagination with the related item, rather than deferring them to M8.

**Exit evidence:** A sends and B fetches the committed message with the same ID/content; history supports initial, older, and newer pages; invalid content and unauthorized access are rejected; a failed transaction leaves no partial update. Committed history survives a server restart. Explain that retrying a send after a lost response can duplicate a message under the current non-idempotent contract.

### M6 — Read and unread state: B27–B29

**Goal:** update the user's read position without accidentally reading newly arrived messages.

- B27: validate the target message and persist a read marker that never moves backward.
- B28: enforce membership and apply the mark-read use case.
- B29: expose the read endpoint and verify its effect on the conversation list.

**Exit evidence:** reading through message 100 leaves message 101 unread; older/repeated requests cannot move the marker backward; one user's read does not change another user's marker; self-sent messages do not increase unread counts. Use actual fixture IDs when running the scenario. Concurrent updates must preserve these rules.

### M7 — Realtime delivery: B30–B32

**Goal:** notify connected members after successful message persistence.

- B30: confirm the client platform and implement authenticated `/ws` upgrade.
- B31: implement connection registration/removal, bounded queues, a single writer per connection, ping/pong/deadlines, expiration, and shutdown cleanup.
- B32: connect a small publisher interface to the send use case and publish only after commit.

**Exit evidence:** A's REST send appears as an event on authorized connections; C receives no private events; rollback emits nothing; a failed publication does not undo a saved message. Check disconnects, slow clients, token expiration, and shutdown. Run the race detector when supported and record any environment limitation.

B31 contains several concurrency concepts. Split it into separately verifiable sub-items in BACKLOG before implementation if it cannot be explained comfortably in one small turn. Apply the same rule to any other oversized item; never treat a milestone as permission to batch its work.

### M8 — Acceptance and handoff: B33–B34

**Goal:** establish evidence that the whole backend works and can be reproduced.

- B33: execute the complete acceptance scenario below and record actual results.
- Create separately tracked small fix items for failures. Resolve required fixes before declaring acceptance successful; do not hide fixes inside a large final cleanup.
- B34: follow the setup/migration/seed/run instructions in a suitable test environment, verify the published API examples, and document known MVP limitations.

**Exit evidence:** the acceptance criteria pass, required fixes are complete, and the README describes verified commands. Another developer can run the demo without relying on the original conversation. A successful happy-path demo alone does not waive authorization or data-integrity failures.

## 5. Decisions to resolve at the point of use

Do not request every decision upfront. Read the relevant DECISIONS entry, explain the choice in Vietnamese, and ask only when an unresolved answer affects that item as described in AGENTS.

| Before item | Required decision or check |
|---|---|
| B01 | Installed Go version and a real or explicitly proposed local module path |
| B05–B07 | Available DB environment, separate development/test DBs, migration tool, and compatible DB driver |
| B12 | JWT library/configuration and the proposed demo session behavior; no implicit promise of refresh or immediate revocation |
| B16 | D08–D09: added read marker and ordering invariant; these are proposed additions to the source schema, not already confirmed user requirements |
| B30 | Flutter mobile only or also Web, and a compatible authentication handshake |

Version selections are made and recorded when needed. This plan does not install tools, resolve these choices on the user's behalf, or turn a design proposal into an approved product change.

## 6. Final backend acceptance scenario

Run against isolated test/demo data with a REST client and two WebSocket clients. Record the test setup, actual IDs, requests, expected/actual results, and remaining issues in the B33 handoff entry or a linked report created during B33.

| Step | Action | Passing result |
|---|---|---|
| 1 | Follow documented setup, migrations, seed, and startup | Server starts; health responds; seed can be repeated without duplicate demo data |
| 2 | Register a separate test user and log in as A/B | Auth responses match the contract; the unrelated user has no conversations |
| 3 | Request A/B's conversation lists | Both see the same shared conversation with correct participant and last-message fields |
| 4 | Open A/B's authenticated sockets; A sends text through REST | A receives a committed-message response; B receives an event with the same message ID; the sender's event is not treated as a second message |
| 5 | Fetch history and inspect unread state | Message content/order match persistence; B's unread count increases, while A's own message is not unread for A |
| 6 | B marks read through a displayed message, then replies | B's marker/count update correctly; A can receive/fetch the reply |
| 7 | Exercise an older read request while a newer message arrives | Marker never regresses; the newer unseen message remains unread |
| 8 | Disconnect B, send more messages, reconnect, and paginate catch-up | Persisted missed messages can all be retrieved through REST and merged by ID without duplicates |
| 9 | Restart the server and fetch history again | Committed messages remain available; clients can reconnect with valid tokens |
| 10 | Try chat access and event receipt as non-member C | C cannot read, send, mark read, or receive events for A/B's conversation |
| 11 | Try expired/invalid tokens and invalid request data | HTTP/WS reject them according to CONTRACTS, without exposing internal errors or secrets |
| 12 | Review unit/integration/concurrency evidence and run instructions | Required checks and fixes are complete; limitations are documented; the setup is reproducible |

Some server guarantees require focused tests rather than manual clicking: rollback, commit-before-publish, concurrent sends, concurrent marker updates, and slow-client isolation. Keep that evidence with the related backlog items and reference it during acceptance.

## 7. Tracking progress without duplicate status lists

**BACKLOG remains the single editable source of task status.** This plan owns milestone grouping and exit criteria, not a second set of completion checkboxes.

At the creation of this plan, B01–B34 are all TODO and the next implementation candidate is B01. This sentence is a dated starting snapshot; read BACKLOG for current progress in later sessions.

- A milestone is complete only when its mapped items, necessary sub-items/fixes, and exit evidence are complete.
- Report progress as completed top-level items out of 34, with the active milestone and item. This is a count of completed items, not a percentage of total effort; items vary in size.
- If an item is split, retain the parent ID and mark the parent DONE only after all required sub-items are complete. Keep sub-item status in BACKLOG, not in this plan.
- At the end of each turn, update the item's status and handoff entry with changed files, checks/results, the Vietnamese explanation delivered, remaining issues, and one suggested next item.
- Also give the user a concise Vietnamese work-completed report covering actual actions, changed files, check results, unfinished scope, and the next item. A code explanation alone is not enough.
- Do not check off future work because its interface, diagram, or documentation exists. No server run, build, or feature test was performed during the original creation of this plan; later verification is recorded in BACKLOG.
- There is no fixed calendar deadline here. Schedule by these dependencies and the user's reading pace; do not assume the original full-stack six-week roadmap is a backend delivery commitment.

## 8. Starting and resuming work

First implementation request, when the user is ready:

> Read backend/docs/AGENTS.md, WORKFLOW.md, MVP_PLAN.md, and the relevant BACKLOG.md entry. Implement only B01. Explain the goal and code changes in Vietnamese, run appropriate checks, update the backlog handoff, and stop before B02.

For subsequent turns:

> Read the backend rules and the latest BACKLOG handoff. Identify the next eligible small item within MVP_PLAN.md. Complete only that item, explain it in Vietnamese, record verification and remaining work, then stop.

If a prior item is incomplete, finish or clarify that item before selecting new dependent work. This plan does not authorize automatic implementation, commits, deployment, or expanding MVP scope.
