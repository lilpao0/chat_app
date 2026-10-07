# Flutter Web: REST CORS and WebSocket tickets

Prepared and implemented: 2026-10-07. WEB-01 through WEB-04 are implemented; deployment and actual Flutter Web acceptance under WEB-05 remain pending. Runtime verification belongs in [BACKLOG.md](BACKLOG.md); copy-ready FE setup is in [FLUTTER_WEB_TESTING.md](FLUTTER_WEB_TESTING.md). The sequence below retains the original plan as an implementation record.

## 1. Goal and verified starting point

Enable the frontend team to test login, conversation loading, bidirectional chat, and reconnect recovery on Flutter Web alongside the existing Android/iOS clients.

Starting checkout before implementation (historical findings, not current runtime):

- `internal/presentation/http/router.go` uses `cors.Default()`. Installed `gin-contrib/cors v1.7.9` allows all origins but its default allowed headers omit `Authorization`.
- CORS already runs before protected-route authentication and handles browser preflight without a Bearer token. Preserve that order.
- `internal/presentation/websocket/handler.go` rejects every nonempty Origin and requires an Authorization Bearer header. Browser WebSocket cannot supply that custom header.
- No WebSocket ticket endpoint or ticket store exists.
- Existing `Identity` includes `UserID` and access-token `ExpiresAt`; session supervision already closes sockets on access-token expiration.
- The earlier local probe of port 8080 returned an Apache OPTIONS response. This does not verify the running Go API or the frontend's actual backend URL. Resolve the backend/proxy address before live acceptance tests.

This change adds browser connection authentication. Preserve the current REST response envelope, socket commands/events, membership checks, send retry keys, Android/iOS Bearer handshake, and REST history recovery. Topic routing and database schema changes are separate work. Frontend implementation remains owned by the frontend team.

## 2. Proposed contract

These choices are implementation proposals for this plan; the APIs below are not yet available.

### Shared browser origin configuration

Add `WEB_ALLOWED_ORIGINS` containing comma-separated exact HTTP/HTTPS origins. Example only:

```dotenv
WEB_ALLOWED_ORIGINS=http://localhost:5173
```

Replace 5173 with the frontend team's actual port. An origin consists of scheme, host, and optional port, with no path, query, fragment, userinfo, or wildcard. `localhost` and `127.0.0.1` are distinct origins. Reject malformed configuration at startup. Empty configuration disables cross-origin browser access and browser WS tickets; mobile clients without Origin keep working.

Use the same configured origins for REST CORS, ticket issuance, and browser WebSocket admission. Validate a single Origin value; duplicate, `null`, or unapproved values must not bypass admission. Origin checks supplement authentication and do not prove user identity.

REST CORS:

- Permit `Authorization` and `Content-Type`, along with any retained ordinary headers actually needed by existing clients.
- Permit GET, POST, PATCH, and OPTIONS, plus HEAD if used. All current application operations must remain covered.
- Run before authentication so OPTIONS returns 204 without Bearer verification.
- Include applicable CORS headers on REST success and application 4xx/5xx responses for approved origins, including auth refresh and ticket issuance.
- Bearer authentication needs no cookies; keep `AllowCredentials: false`. Use a short preflight cache, proposed five minutes, while testing.
- Requests without Origin remain usable by mobile clients and command-line tools. Same-origin REST requests remain governed by normal authentication.

### Issue a browser ticket

```http
POST /api/ws/tickets
Authorization: Bearer <access_token>
Origin: http://localhost:<actual_port>
```

No request payload is needed. The endpoint derives identity from the protected middleware and origin from the request. Do not accept a user ID, access token, or origin override in a JSON body. If a nonempty body is sent, reject it as `invalid_input` rather than silently accepting supplied identity fields.

Success: HTTP 201, `Cache-Control: no-store`:

```json
{
  "status": "success",
  "data": {
    "ticket": "<opaque_random_ticket>",
    "expires_at": "<UTC RFC3339 timestamp>"
  }
}
```

Ticket rules:

- Generate 32 random bytes with `crypto/rand`, encoded using `encoding/base64.RawURLEncoding`. This is an opaque credential, not a JWT.
- Ticket deadline is the earlier of issuance plus 60 seconds and the original access-token expiration.
- Bind it to the authenticated identity and exact approved origin. Do not retain the raw access JWT in the store.
- Index tickets by SHA-256 digest. Return the raw ticket only in the issuance response.
- Use a bounded store for the current single API process. Proposed limits: 8 outstanding tickets per user and 4,096 total. Purge expired entries before checking capacity and return 429 rather than evicting live tickets.
- Use atomic validation and removal under a mutex when consuming a ticket. Two concurrent handshakes must not both succeed.
- Restart discards outstanding tickets; clients obtain a new ticket. Add a shared atomic store only when multiple API processes are introduced.
- Mark this ceiling in implementation with a `ponytail:` comment describing the single-process limit and shared-store upgrade path.

Proposed public failures:

| HTTP | `error.code` | Meaning |
|---|---|---|
| 400 | `invalid_input` | Unsupported ticket-endpoint body or malformed handshake credential/query shape |
| 401 | `unauthenticated` | Missing, invalid, expired, or wrong-type access token on ticket issuance or mobile handshake |
| 401 | `invalid_ws_ticket` | Ticket missing/invalid, consumed, expired, or bound to another origin; use one generic message |
| 403 | `origin_not_allowed` | Ticket request or browser handshake has an unapproved/invalid Origin |
| 429 | `rate_limited` | Outstanding-ticket capacity reached; bounded issuance fails without allocating more entries |
| 500 | `internal_error` | Ticket generation or unexpected server failure; no internal details |

These failures use the existing `status: fail` or `status: error` REST envelope. CORS rejection itself can still be a middleware-level response; the browser cannot read errors from disallowed origins.

### Open the socket

Browser uses:

```javascript
const socket = new WebSocket(
  `${wsBaseUrl}/ws?ticket=${encodeURIComponent(ticket)}`
);
```

Android/iOS retain `GET /ws` with `Authorization: Bearer <access_token>` and no Origin/ticket.

Handshake ordering:

1. Validate method, query parsing, credential shape, and Origin before allocating a session.
2. A browser request with approved Origin must contain exactly one nonempty ticket and no Authorization header. A mobile request without Origin uses exactly one valid Bearer header and no ticket. Reject mixed credentials, duplicate ticket parameters, unsupported query fields, and raw JWT query transport.
3. Validate and consume the browser ticket before accepting the upgrade. Origin mismatch must not consume a ticket belonging to an approved origin.
4. Configure `coder/websocket.Accept` for the allowed origins as well. The library's default same-origin check would otherwise reject a valid cross-origin browser after the application's check. Keep exact scheme/host/port checking in the application before allowing the library upgrade; do not disable Origin validation blindly.
5. Pass the ticket's original access identity to the existing session runner. Use access-token expiry, not ticket expiry, as socket lifetime.

A ticket consumed by a handshake is not restored if the subsequent upgrade/network attempt fails. Client obtains a new ticket for every connection attempt. 101 has no REST JSON envelope.

### Expiration and reconnect

- Expired/used tickets produce HTTP 401 before upgrade. Browser WebSocket exposes a connection error and often close code 1006; frontend must not rely on reading handshake JSON or its HTTP status through the WebSocket API.
- Once open, ticket expiration does not close the connection. Original access-token expiration closes it with the existing 1008 access-expiry reason.
- Other existing 1008 reasons include queue/rate violations; do not treat every 1008 as token expiration.
- After a network failure, request a new ticket with the current access token. If issuance returns 401 `unauthenticated`, refresh through `POST /api/auth/refresh`, then obtain another ticket. Invalid refresh means login again.
- Reconnect with bounded exponential backoff and jitter; avoid endless immediate retries or refresh on every opaque network error.
- Open the socket and buffer events before REST catch-up. Fetch every `after_id` page from the last completed REST checkpoint; merge by message ID. Read arrays from `data` and cursors from `meta.pagination`.
- Retry an unresolved logical send using its original UUID `request_id` so reconnect does not create duplicate messages.

## 3. Implementation sequence and file ownership

Paths below are relative to `backend/`. New filenames are proposals; reuse existing files where this keeps a smaller, clear diff.

### WEB-01: Shared origin configuration and REST CORS

| File | Planned change |
|---|---|
| `cmd/api/config/web.go` and `web_test.go` (new) | Load and validate `WEB_ALLOWED_ORIGINS`; cover empty config, exact ports, invalid URLs, duplicates and wildcard rejection |
| `internal/presentation/authentication/origin.go` and `origin_test.go` (new) | Small shared Origin admission helper, including duplicate-header checks; used by ticket handler and WS handshake |
| `internal/presentation/http/router.go` | Replace `cors.Default()` with explicit config; receive startup origins and keep CORS before auth |
| `internal/presentation/http/cors_test.go` (new) | Test real router preflight with requested `authorization,content-type`, no verifier calls, and CORS on success/401/400/500 and refresh paths |
| `cmd/api/main.go`, `.env.example` | Load/pass origin configuration and document placeholders |
| Existing `NewRouter` callers in tests | Pass explicit test origins or an empty list; preserve protected-route registration |

Exit: approved origin receives 204 preflight allowing both headers; protected GET still requires Bearer; disallowed origin fails; malformed origin config fails startup.

### WEB-02: One-use ticket store and issuance endpoint

| File | Planned change |
|---|---|
| `internal/presentation/authentication/ticket.go` and `ticket_test.go` (new) | Concrete `TicketStore` shared by HTTP and WS adapters; issue/consume with randomness, digest keys, expiry, capacity, and mutex |
| `internal/presentation/http/handler/ws_ticket.go` (new), exercised by `websocket/browser_test.go` | Read authenticated middleware identity, enforce approved single Origin and empty request body, issue ticket, serialize 201/no-store and public errors |
| `internal/presentation/http/router.go` | Register protected `POST /api/ws/tickets`; extend protected handler wiring |
| `cmd/api/main.go` | Create one store instance and inject it into ticket handler and WS handler |

Keep tickets under presentation authentication: they adapt an already verified identity to a browser transport. They add no chat business rule, SQL repository, domain use case, or new dependency. Inject a clock into the concrete store only where needed for deterministic expiry tests; avoid background cleanup goroutines for a bounded store.

Exit: untrusted callers cannot issue tickets; TTL cannot outlive access identity; same ticket has one successful consumer under concurrent requests; invalid Origin does not consume another origin's ticket; expired entries and capacity are handled predictably.

### WEB-03: Browser handshake with mobile regression checks

| File | Planned change |
|---|---|
| `internal/presentation/websocket/handler.go` | Allow approved browser origins, choose ticket versus Bearer path, validate credentials before upgrade, configure library Origin handling |
| `internal/presentation/websocket/handler_test.go` | Replace reject-all-browser expectations with allowed/denied origins; retain mobile authentication rejection cases |
| `internal/presentation/websocket/real_auth_test.go` | Verify ticket identity and access-token expiry using real JWT issuance |
| `internal/presentation/websocket/browser_test.go` (new if needed) | Ticket endpoint plus browser-shaped handshake, reuse/concurrent reuse, expiry, malformed queries, wrong origin, failed upgrade, and access-expiry closure |
| Existing WS handler constructor callers | Pass origins/store explicitly; review `acceptance_test.go`, `bidirectional_test.go`, `realtime_test.go`, and lifecycle helpers |

Keep the existing hub/session queues and command execution model. Add only the admission logic required for browser clients.

Exit: approved-origin ticket opens a connection without Authorization; unauthorized origin/credential cannot start a session; Android/iOS Bearer continues working; access expiry still closes the socket.

### WEB-04: Contract and frontend handoff

| File | Planned change |
|---|---|
| `internal/presentation/http/swagger/openapi.json` | Document ticket POST and browser `/ws` query/auth alternatives, shared errors, 201/no-store, lifetime and browser error visibility; retain WS frame schemas |
| `internal/presentation/http/swagger/contract_test.go`, `handler_test.go`, `websocket_contract_test.go` | Update route coverage, protected ticket operation, and WS alternatives without weakening existing REST/WS contract assertions |
| `docs/chat-app-rest.postman_collection.json` | Add ticket request and response checks using `data.ticket`; preserve current manual token/ID variable behavior |
| `docs/chat-app-websocket.asyncapi.json` | Update connection descriptions/examples for tickets; preserve command/event payloads and the user's existing edits |
| `docs/CONTRACTS.md`, `docs/WEBSOCKET_PLAN.md`, `docs/DECISIONS.md` | Replace mobile-only implementation wording when browser support is implemented; document one-use ticket and reconnect rules |
| `README.md`, `docs/README.md`, `docs/ARCHITECTURE.md`, `docs/AGENTS.md`, `docs/BACKLOG.md` | Update supported clients, ownership, setup and actual verification state; check repository-root README for stale mobile-only statements |

Provide a copy-ready FE example for login, list conversations, issue ticket, connect, send/receive, refresh and reconnect. Use placeholders for secrets and deployment URLs. Explain that curl/Postman can inspect handshake HTTP errors while browser WebSocket cannot.

Exit: route/auth schema tests pass; OpenAPI/Postman/AsyncAPI parse; ticket example agrees with runtime; no raw tokens/tickets appear in saved artifacts or logs.

### WEB-05: Operational and browser acceptance

Inputs needed from the frontend team/operator:

- Actual frontend origin, including scheme and fixed port. Recommend a fixed Flutter Web port for repeatable tests.
- Actual backend HTTP base URL and WS URL used by FE, including any proxy path.
- Whether HTTPS frontend requires a `wss://` endpoint. Production connections use HTTPS/WSS.

Before acceptance, verify the Go process/listener and proxy route. Probe OPTIONS against both direct API and FE-facing URL to locate proxy interception. Confirm the proxy forwards WebSocket upgrades and query strings, preserves relevant CORS headers, and does not log ticket query values. Proxy edits/deployment require separate target-specific authorization; this plan does not invent an Apache config path.

For test users, inspect the existing seed configuration and verify the accounts by login in the intended test environment. `run-seed.ps1` names Alice/Bob, but its presence does not prove those accounts exist. Supply two dedicated accounts and their shared conversation ID through a private handoff; do not put passwords in committed documentation. Seed reuse does not reset an existing password. Keep fixtures isolated and preserve existing data.

Acceptance scenarios:

1. Approved-origin login and refresh succeed; authenticated conversation GET passes browser preflight.
2. A and B obtain distinct tickets and connect from Flutter Web without Authorization handshake headers.
3. A sends a keyed message; A receives acknowledgement and both receive the committed message. B can reply and mark read.
4. Non-member C cannot access A/B's conversation or receive its events.
5. Disconnect B, send while offline, reconnect with a fresh ticket, and recover every missed message through paginated REST history without duplicate sends.
6. Used/expired/wrong-origin ticket fails; two simultaneous attempts with one ticket produce at most one accepted connection.
7. Access expiry triggers socket close; valid refresh/new ticket reconnects; invalid refresh returns to login.
8. Android/iOS Bearer handshakes and multiple devices still work.

Backend HTTP/socket tests can simulate Origin and prove server behavior, but they do not prove browser CORS enforcement. Record actual Flutter Web browser results separately from Go results.

## 4. Verification gates

From `backend/`, format only modified Go files, then run relevant groups as each item finishes:

```powershell
go test ./cmd/api/config ./internal/presentation/authentication ./internal/presentation/http/handler ./internal/presentation/http/middleware ./internal/presentation/http/response ./internal/presentation/http/swagger
go test ./internal/presentation/http/... ./internal/presentation/websocket/...
go test -race ./internal/presentation/authentication ./internal/presentation/websocket/...
go test ./...
go vet ./...
git diff --check
```

Use a permitted temporary Go cache if Windows denies the default cache. Race tests require a working supported toolchain; state any limitation. Full DB-backed tests require the isolated `chat_app_test` database. The earlier response migration suite was blocked at `requires chat_app_test`; resolve valid test configuration without resetting development data.

Mark items DONE only after their exit criteria pass. Do not claim Flutter Web end-to-end acceptance from JSON parsing, Postman, or Go tests alone. Final handoff must state exact tested origin/URLs, client environment, passed checks, failed/unrun checks, and remaining operational setup.
