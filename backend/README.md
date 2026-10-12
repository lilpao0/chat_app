# Go Chat App Backend

All backend guidance is collected in [docs/README.md](docs/README.md).

Follow the [MVP implementation plan](docs/MVP_PLAN.md) for the eight milestones and acceptance criteria; track individual task status in [BACKLOG](docs/BACKLOG.md).

Read [docs/AGENTS.md](docs/AGENTS.md) and current decisions first. The user authorizes continuation until a decision is needed; retain small steps, verification and **Vietnamese** explanations. Documentation is written in English.

The Go module is `github.com/lilpao0/chat_app/backend`. Auth, REST chat, discovery/direct chat, profiles, and Android/iOS/Web realtime delivery are implemented through B34 plus the documented extensions. Startup requires `DATABASE_URL` and `JWT_SECRET`.

For Flutter Web, set `WEB_ALLOWED_ORIGINS=http://localhost:5173` (replace with actual origin). REST accepts Authorization preflight; browser WS uses Bearer-authenticated `POST /api/ws/tickets`, then `/ws?ticket=...` without Authorization. Mobile keeps Bearer admission without Origin. See [browser setup and Cloud Run release checks](docs/FLUTTER_WEB_TESTING.md). Hosted deployment and actual Flutter Web acceptance are not performed by local tests.

## Local database migrations

The project uses the locally installed `migrate` CLI with SQL files in `migrations/`. Use `.env.example` as a reference; never commit real credentials.

From PowerShell in `backend/`, apply migrations with an absolute forward-slash path:

```powershell
$env:DATABASE_URL = 'postgres://postgres:YOUR_POSTGRES_PASSWORD@127.0.0.1:5433/chat_app?sslmode=disable'
migrate -path "C:/absolute/path/to/backend/migrations" -database $env:DATABASE_URL up
```

Tests always use the isolated `chat_app_test` database and never skip database coverage. Set `TEST_DATABASE_URL` explicitly when needed; for local development the test helper can derive the sibling `chat_app_test` URL from `DATABASE_URL` in `.env`. The database itself must already exist. Tests refuse any database whose actual name is not `chat_app_test`. Do not run destructive migration commands against `chat_app`.

Migration `000004_message_attachments` adds attachment metadata/constraints/indexes and permits empty text with media. `000005_media_sessions` adds encrypted upload-session recovery and cleanup leases. Apply all five migrations explicitly to the intended database; tests do not migrate development data. Down migrations refuse destructive rollback: 000004 requires no attachments/incompatible blank messages; 000005 requires no recovery credentials/claims. Prefer forward correction after rollout.

## Images and videos

Backend supports text-only, media-only and text with up to five ordered images/videos. Files upload directly to private Google Cloud Storage; API verifies bytes, binds files atomically to one message, and issues authorized generation-bound read URLs. REST/WS/history expose metadata only. See [MEDIA_SETUP.md](docs/MEDIA_SETUP.md) for configuration, IAM/CORS, Postman upload/resume and cleanup; [cloud release evidence](docs/MEDIA_RELEASE_2026-10-12.md) records hosted API/GCS checks and remaining browser/device acceptance. Empty `MEDIA_BUCKET` leaves text chat available and storage operations return 503.

`internal/domain/usecase/attachment/media.go` manages issuance/completion/access; `internal/data/storage` implements GCS and bounded inspection; `postgres_media_lifecycle.go` encrypts session recovery and claims cleanup; existing send transaction binds ready files before commit/publication. Docker includes ffprobe and `/app/media-cleanup`.

With `TEST_DATABASE_URL` configured, run `go test -count=1 -v ./internal/testutil -run TestMessageAttachmentsMigration` from `backend/` for preservation, constraints, up/down, rollback refusal and concurrent-writer protection.

## Application layers

- `internal/presentation`: HTTP handlers/middleware and WebSocket delivery.
- `internal/domain`: entities, repository interfaces, and use cases; no dependencies on presentation or data.
- `internal/data`: database connection, repository implementations, and auth adapters.

`cmd/api/main.go` wires the layers; startup configuration lives in `cmd/api/config`. Only `migrations/` is the active migration source.

## Repository checks

From `backend/`, run `go test -count=1 -v ./...`, then `go vet ./...`. Database tests are mandatory and fail clearly if `chat_app_test` is unavailable. They use unique private schemas removed at cleanup; existing data is preserved. Use `go test -race -count=1 ./...` for the full race-enabled suite.

## Password adapter checks

Run `go test -v ./internal/data/auth/... ./internal/domain/usecase/auth/...` from backend to verify auth adapters and use cases without PostgreSQL. Tests live in each auth package's `test/` subfolder and exercise exported APIs. The `...` includes these subfolders. Domain owns the interfaces; data/auth implements bcrypt and JWT. Registration/login are wired through these interfaces.

## Running the REST API

From backend, set DATABASE_URL to a database with migrations applied and JWT_SECRET to a random secret of at least 32 bytes. Optional defaults: JWT_ISSUER=chat-app, JWT_AUDIENCE=chat-app-mobile, JWT_TTL=24h, JWT_REFRESH_TTL=720h, HTTP_PORT=8080. The API and seed commands optionally load `.env` from the current working directory; explicit process variables take precedence. Tests use `TEST_DATABASE_URL` from the process environment. Never commit actual credentials.

Start with `go run ./cmd/api`. Send Content-Type: application/json. REST success responses use `status: "success"` and `data`; paginated responses add `meta.pagination`. Client failures use `status: "fail"`; server failures use `status: "error"`. POST /api/auth/register accepts first name, optional last name, email and password; it returns a public user without automatically logging in. POST /api/auth/login accepts email/password and returns the user, access token, refresh token, and their expiration times inside `data`. POST /api/auth/refresh accepts `{"refresh_token":"..."}` and returns a new access token inside `data`. Protected routes use Authorization: Bearer <access_token>.

Interactive Swagger documentation is available at `http://localhost:8080/swagger` (replace `8080` when `HTTP_PORT` is different). Use **Authorize** with the `access_token` returned by login to call protected routes. The embedded OpenAPI 3 contract is available at `/swagger/openapi.json`; the UI loads its pinned Swagger UI assets from jsDelivr and therefore needs internet access for the page assets.

Contract tests compare the OpenAPI method/path set against Gin's production route registration and verify public/protected authentication declarations plus registration validation codes. Adding or removing an API route or auth validation error without updating OpenAPI causes the test suite to fail.

Tokens expire after their configured TTLs. This simple MVP does not rotate or revoke refresh tokens; logout removes tokens from the client only. The verified behavior and limitations are summarized in [BACKLOG](docs/BACKLOG.md).

## Demo seed

After applying all five migrations, set SEED_A_NAME, SEED_A_EMAIL, SEED_A_PASSWORD, SEED_B_NAME, SEED_B_EMAIL and SEED_B_PASSWORD, then run `go run ./cmd/seed`. DATABASE_URL selects the target. Passwords use the registration policy; the two normalized emails must differ.

Repeat runs reuse existing accounts without changing names/passwords and reuse the exact pair's conversation. User-driven direct chat opening uses the same pair lookup. If an email already exists, its existing password still applies, regardless of the supplied seed password. Seeding is never automatic. Verification created fixtures only in private chat_app_test schemas, not development accounts.

## Protected chat endpoints

| Endpoint | Input / result |
|---|---|
| GET /api/users | q: name substring or exact email; optional limit (1-50), after_id; returns public summaries without emails |
| POST /api/conversations/direct | JSON user_id; returns a new conversation with 201 or existing one with 200 |
| GET /api/conversations | Actor's conversations, counterpart, last message and unread count |
| POST /api/conversations/:id/messages | Optional content, attachment_ids and request_id; text or attachments required; returns committed/original retry result |
| POST /api/conversations/:id/attachments | Declared content_type/size_bytes; returns private resumable upload grant |
| POST /api/attachments/:id/complete | JSON {}; validates actual bytes and returns ready/attached metadata |
| GET /api/attachments/:id/access | Returns authorized 15-minute generation-bound read URL |
| GET /api/conversations/:id/messages | Optional limit (1-100), before_id or after_id; messages and cursors |
| POST /api/conversations/:id/read | JSON last_read_message_id; returns effective read position |

Initial/before history is descending by ID; after history is ascending. The client marks only through its last displayed message. Older requests cannot regress the marker; unseen newer messages stay unread. Non-member/missing conversations return 404. Keyed retries are durable; requests without `request_id` retain legacy duplicate-on-retry behavior. See [CONTRACTS](docs/CONTRACTS.md).

## Realtime WebSocket

Flutter Android/iOS/Web supports bidirectional `send_message` and `mark_read` commands, correlated acknowledgements/errors and committed `new_message` events. Apply migration `000003_message_request_id` before starting the updated API. REST mutations remain compatible; see [protocol and retry rules](docs/WEBSOCKET_PLAN.md).

To exercise the complete API in Postman, import [the REST collection](docs/chat-app-rest.postman_collection.json) and [the WebSocket AsyncAPI document](docs/chat-app-websocket.asyncapi.json) separately. Open the generated `/ws` request at `ws://localhost:8080/ws`, add `Authorization: Bearer <access_token>` as a handshake header, then send one of the provided `send_message` or `mark_read` examples. Postman keeps HTTP and WebSocket requests in different collection types.

```dart
import 'dart:convert';
import 'dart:io';

final socket = await WebSocket.connect(
  'ws://10.0.2.2:8080/ws',
  headers: {'Authorization': 'Bearer $accessToken'},
);

socket.listen((raw) {
  final event = jsonDecode(raw as String);
  if (event['type'] == 'new_message' || event['type'] == 'message_sent') {
    // Deduplicate and store by event['data']['id'].
  }
});

// Generate and persist one canonical lowercase UUID per logical send.
// Reuse it after timeout/reconnect; never generate a new ID for a retry.
final requestId = pendingMessageRequestId;
socket.add(jsonEncode({
  'type': 'send_message', 'request_id': requestId,
  'data': {'conversation_id': conversationId, 'content': 'Hello'},
}));
socket.add(jsonEncode({
  'type': 'mark_read', 'request_id': readRequestId,
  'data': {'conversation_id': conversationId, 'last_read_message_id': messageId},
}));
```

Use the machine's reachable LAN address instead of `10.0.2.2` on a physical Android/iOS device. For a command-line smoke test with `wscat` already installed:

```powershell
wscat -c ws://localhost:8080/ws -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

The hub is in-memory and serves one API process. Live delivery is best effort; it has no replay or exactly-once guarantee. Access-token expiry closes the socket, binary input closes with 1003, oversized messages with 1009, queue/rate abuse with 1008, and shutdown uses code 1001 where practical.

The checked-in Cloud Build/Cloud Run configuration builds from the repository's `backend/` context and caps the service at one instance to match this hub limitation. Do not raise the instance count until realtime fan-out uses a shared broadcast adapter.

### GitHub checks and deployment

`.github/workflows/deploy.yml` runs the full race test suite against its isolated PostgreSQL18 `chat_app_test` service, then vet/build, on PRs to `main` and pushes to `main`. PRs only run checks. A successful push to `main` submits `backend/cloudbuild.yaml` with the commit SHA; Cloud Build deploys both API and cleanup using `chat-media-builder`. The existing `GCP_SA_KEY` secret authenticates the submitter, which needs Cloud Build submission/source-upload permissions and `iam.serviceAccounts.actAs` on that builder account. The workflow itself has not run until a PR/push event occurs.

Generated `gha-creds-*.json` files are excluded from Git and Cloud Build source. A rerun preserves an existing create-only video fixture instead of failing on HTTP412. `trigger.tf`/`trigger.yaml` are alternative Cloud Build trigger definitions, aligned to the same YAML; no live trigger was found in the inspected regions. Do not enable a second deployment trigger alongside GitHub Actions for the same push.

On reconnect, establish the socket and buffer events first. Fetch every REST history page using `after_id` from the last completed REST synchronization checkpoint, merge by message ID, then merge buffered events. Never advance this checkpoint solely from live-event IDs. Retry unresolved sends with the original request ID; REST fallback accepts the same optional `request_id`. Acknowledgement confirms persistence, not recipient delivery. `read_updated` is only an acknowledgement, not a broadcast read receipt.

To start a chat, authenticate, search `GET /api/users?q=binh`, then send `POST /api/conversations/direct` with `{"user_id":2}`. The response contains the conversation ID for the existing message/history/read endpoints. Repeated or simultaneous requests for the same unordered pair reuse one conversation through an application transaction lock. Manual SQL that bypasses this path can still create duplicates because the schema has no unordered-pair uniqueness constraint.
