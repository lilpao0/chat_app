# Chat Images and Videos Plan

Updated: 2026-10-12. Status: MED-01 through MED-07 backend implemented; assigned cloud release has real API/GCS evidence. Actual browser/device acceptance remains under MED-08. See MEDIA_RELEASE_2026-10-12.md, MEDIA_SETUP.md and BACKLOG.md.

## 1. Scope and confirmed defaults

The user requested chat images/videos with optional ordinary message text, selected Google Cloud Storage (GCS), then assigned MED-01 and confirmed the limits below. MED-02 began as guided work; the user subsequently assigned completion of its migration portion, the entire backend upgrade, and then cloud steps 1–5. Frontend edits and Git commit/push remain separate. Follow [AGENTS.md](AGENTS.md); decisions live in [DECISIONS.md](DECISIONS.md), status in [BACKLOG.md](BACKLOG.md).

Support text-only, media-only and text + media. Text and attachments belong to ONE message ID, with one unread increment. Uploading alone creates no message or recipient notification.

The user confirmed these limits on 2026-10-09 (U19). Exact DTOs, errors and behavior are defined in [CONTRACTS section 9](CONTRACTS.md#9-media-contract):

| Item | Confirmed limit / contract choice |
|---|---|
| Attachments | Up to 5 images/videos per message, mixed types allowed, client order preserved |
| Images | JPEG/PNG, maximum 10 MiB/file and 25 million pixels |
| Videos | MP4, H.264 video with optional AAC audio, maximum 50 MiB/file |
| Text | Existing valid UTF-8, no NUL, maximum 2,000 Unicode code points |
| Empty content | Allowed with at least one ready attachment; missing content becomes empty string; reject explicit null |
| Access | Private bucket; membership check before issuing a 15-minute signed GET URL |
| Unsent files | Expire after 24 hours; maximum 10 pending attachments/user |
| Deferred | Thumbnails, transcoding, HEIC/HEVC/MOV/WebP/GIF, documents/voice, editing/deletion, CDN |

Unsupported camera formats require client conversion or later format expansion; do not claim all phone recordings work without device acceptance. Original bytes retain embedded image metadata; stripping metadata is a separate product choice.

## 2. Current code

Implemented in this checkout:

- Domain entities carry ordered attachment metadata; shared sending accepts optional text and ordered IDs, validates input and publishes only after commit.
- Migration 000004 stores attachment states and relaxes nonblank text CHECK. Migration 000005 stores encrypted session recovery and cleanup leases. Down migrations refuse destructive rollback.
- PostgreSQL binds all ready files in one message transaction; identical retry compares exact original payload before readiness/expiry checks. History pages messages then hydrates attachments in one query. Preview/unread remains per message.
- REST issues uploads, completes verification and grants private access. Existing WS framing carries expanded metadata; byte transfer remains direct to GCS.
- Official GCS SDK v1.69.0 handles object operations/V4 signing; ADC-authenticated JSON API creates resumable sessions and IAM signBlob uses request context. JPEG/PNG use stdlib, MP4 uses bounded native ffprobe. Docker includes ffprobe.
- API JSON/WS limits remain 16 KiB by default. Completion budget is30s with2 inspection slots/process; HTTP WriteTimeout45s. Cleanup CLI claims expired unbound files, cancels sessions, deletes pinned generations and reconciles tombstones.
- Real PostgreSQL/REST/WS checks and fake provider checks are separate from actual GCS/device acceptance. Configuration and operations: [MEDIA_SETUP.md](MEDIA_SETUP.md).

## 3. Upload, send and view

Use a server-created GCS resumable session; client uploads bytes directly to GCS over HTTPS. This avoids proxying videos through Go request deadlines and supports interruption recovery.

1. Client requests an attachment for a conversation with access token, declared type and byte size.
2. Backend checks membership, format/size and pending cap. Create an uploading row before granting storage access; generate object key server-side.
3. Backend creates a resumable session for that exact private object, with create-only generation precondition and expected length/type. Return attachment ID and session URI. Reconcile/cancel partially created grants if storage/DB fails.
4. Client uploads directly, displaying local progress and resuming the same session after interruption. Session URI is a bearer capability; never log it or include it in message data/analytics.
5. Client calls complete. Backend rechecks owner, membership and expiry; validates actual object bytes/type/size and pins generation. Only verified files become ready.
6. Client sends existing REST request or WS send_message with content, ordered attachment IDs and stable request_id. Transaction attaches ALL files to ONE message, updates conversation and commits.
7. Existing message_sent/new_message data includes attachments after commit. Failed upload/completion creates no partial chat message.
8. Recipient receives metadata through WS/history, asks backend for an authorized access URL and reads media directly from GCS. Refresh expired URL without changing message ID.

Keep draft until all selected uploads are ready. If one fails, retry it or remove it before sending. After lost send acknowledgement, reuse attachment IDs and the SAME request_id. Replacing an attachment requires a new request_id. Upload completion is not sending.

## 4. Defined contracts (MED-01)

All backend endpoints below require Bearer auth and existing REST envelopes. Active OpenAPI paths/components, AsyncAPI schemas and Postman media group describe implemented routes. x-media-contract retains limits/design notes; duplicate draft schemas were removed after activation.

| Route | Input | Success |
|---|---|---|
| POST /api/conversations/:id/attachments | content_type, size_bytes | 201: id, upload_url, expires_at, content_type, size_bytes, upload_method: PUT, upload_headers |
| POST /api/attachments/:id/complete | Empty JSON object | 200: verified metadata, state: ready/attached, expires_at (null when attached) |
| GET /api/attachments/:id/access | None | 200: id, signed url and expires_at |
| Existing POST /api/conversations/:id/messages | content, optional request_id, optional attachment_ids | Existing 201 with expanded Message in data |

Resumable PUT requires GCS Content-Range/chunk and incomplete-upload handling; a generic one-shot PUT example is insufficient. expires_at describes application eligibility, not exact GCS session invalidation. Cleanup must explicitly cancel expired sessions; verify provider cancellation semantics before release.

Existing WS command extended with attachments:

```json
{
  "type": "send_message",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "data": {
    "conversation_id": 10,
    "content": "Pictures from today",
    "attachment_ids": [501, 502]
  }
}
```

REST uses content/attachment_ids/request_id without WS type/data wrapping. Omitted attachment_ids means []; reject null, nonpositive IDs, duplicates and over-limit arrays. Existing text requests remain valid.

Same expanded Message under REST data, WS acknowledgement/event data and history items:

```json
{
  "id": 101,
  "conversation_id": 10,
  "sender_id": 1,
  "content": "Pictures from today",
  "created_at": "2026-10-09T03:00:00Z",
  "attachments": [
    {"id": 501, "kind": "image", "content_type": "image/jpeg", "size_bytes": 125000},
    {"id": 502, "kind": "video", "content_type": "video/mp4", "size_bytes": 4200000}
  ]
}
```

Text-only messages return attachments: []. Signed URLs are fetched separately: do not persist them or include them in WS/history payloads. Conversation members may view attached media; only owner may preview ready unsent media. Uploading, failed, deleting, deleted or expired unsent files cannot be viewed. Responses containing access/session URLs use Cache-Control: no-store.

Retain REST status/data/meta/error and WS type/request_id/data/error framing. Attachments intentionally expand WS schemas. Existing clients need media rendering and empty-content handling; additive fields alone do not prove UI compatibility.

Exact HTTP/WS codes, retry behavior and validation precedence are defined in CONTRACTS section 9. Inaccessible conversations/attachments return generic not_found before object-specific state errors. Oversized declaration returns 413; actual oversized/malformed bytes fail completion even if GCS accepted them. Storage/inspection timeout is retryable, never fake success.

## 5. Schema and atomic persistence

Migrations 000004/000005, history hydration, send transaction and media routes are implemented (see BACKLOG). Existing user entity/reservation work was retained.

Prefer ONE new attachments table, not separate image/video or upload/link tables:

- id, conversation_id, owner_id, nullable message_id/position;
- state: uploading, ready, attached, failed, deleting, deleted;
- object_key, nullable verified object_generation, declared type/size and verified content_type/size_bytes/kind;
- created_at, expires_at; retain session capability securely only as needed for cancellation/recovery, never in ordinary DTOs.

FKs bind owner to conversation member and message to the same conversation. Unique object_key and (message_id, position); indexes for page loading and pending expiry/owner counts. CHECKs enforce positive sizes and consistent state/message_id/position. Pin generation for verified reads/deletes; no overwrite grants.

Migration 000004 relaxes the nonempty-only content CHECK while retaining length validation. The cross-table rule "nonblank content OR attachment" belongs in the shared use case AND atomic repository write: PostgreSQL row CHECK cannot inspect children. DB alone does not enforce that invariant. Every writer/seed must follow the repository contract; add a deferred constraint trigger only if direct DB writers need support. Avoid duplicated attachment_count.

Extend existing send transaction:

1. Lock conversation and enforce current membership before allocating message ID.
2. Preserve sender/request_id uniqueness. On replay compare conversation, exact content and ORDERED attachment IDs with original message. Identical retry returns original without publication, even with already-attached files. Different payload conflicts. Check replay before ready-only validation and preserve concurrent unique-key handling across conversations.
3. For new send, lock attachment rows in ascending ID order; verify owner, conversation, ready state, expiry and unbound message_id. Recheck text-or-attachment invariant in this path.
4. Insert one message, bind all files in client order, update conversation and commit together. Any error rolls everything back. No GCS calls inside SQL transaction.
5. Publish populated Message only when newly created and committed. Publication failure after commit still means successful saved message; history recovers it.

Concurrent sends cannot reuse a file. Completion/cleanup use locked state transitions so cleanup cannot delete newly attached media. Failed send leaves ready files until expiry.

History: page messages FIRST, fetch attachments for all page IDs in one query, group by message_id/position. Avoid joins that change pagination and avoid one query per message. Preserve before_id/after_id.

Conversation preview uses nonblank content; otherwise Photo, Video or Media for mixed/multiple files. Retain last_message string/null and select newest message by ID. Read/unread stays per message, not file.

Down migration refuses if media data or empty-content messages exist; never delete user messages/files to restore old schema. Prefer forward correction after rollout.

## 6. Storage validation and operations

- Extension, declared MIME, GCS Content-Type and byte length are untrusted. Compare actual bytes/type/size against limits/declaration before ready, on exact object generation.
- JPEG/PNG: stdlib image decoders and bounded readers; enforce pixel/dimension budget before allocating image buffers; reject malformed/truncated files within bounded resources. No new image library for these formats.
- MP4: native ffprobe with fixed arguments, bounded temporary file, context timeout and concurrency cap; validate container/video track and H.264/optional AAC. No transcoding. Dockerfile installs ffmpeg/ffprobe. No shell execution or arbitrary client URL/path; clean temporary files on every exit.
- Completion needs finite inspection budget. Reconcile server/context deadlines with implemented 45-second WriteTimeout; no hidden work after request cancellation. WS sends do no storage reads/probing.
- Pending issuance cap must be atomic across requests. Bound issuance rate/completion concurrency and reserve declared sizes. Verify GCS expected-length enforcement: rejecting excess bytes AFTER upload is not a hard upload-cost cap.
- Official GCS Go client v1.69.0 and OAuth2 ADC are used; SDK canonicalizes V4 signed URLs. Request-bound IAM signBlob uses authenticated HTTP, without local private-key signing. Cloud Run service identity/ADC and local ADC; no committed service-account keys.
- Private bucket, uniform access, public access prevention. Grant selected service identity required object create/read/delete and signing permissions only; document signBlob/IAM for signed reads.
- Bucket CORS is separate from API CORS: approved browser origins, upload/read methods, Content-Range/status headers for resume. Verify in actual browser. CORS is not authorization.
- Generation-bound signed GET supports Range for video seeking. Serve verified Content-Type. Never log session/signed URLs.
- GCS is shared but Hub/tickets remain process-local. Media does not authorize raising Cloud Run instance count.

Cleanup: one scheduled authenticated maintenance operation or CLI job, no queue framework. Atomically claim expired unattached rows as deleting, cancel sessions, delete exact generations and mark deleted after success. Retry failures; missing object is success. Keep tombstones long enough to reconcile late uploads/cancellation races. No age-only lifecycle policy across attached objects: old messages must retain media. Test cleanup vs send/complete races.

## 7. Implementation sequence

Execution statuses live in BACKLOG. Proposed packages below each require focused tests/docs; split further if review scope grows. No empty scaffolding or unrelated refactor.

| Item | Outcome | Main files / proof |
|---|---|---|
| MED-01 | Finalize defaults, schemas and errors | CONTRACTS, OpenAPI, AsyncAPI, DECISIONS; client media-only and URL refresh behavior |
| MED-02 | Schema/entity/read foundation | migration 000004, schema.dbml, entity/chat.go, repositories; isolated constraints and unchanged text flow |
| MED-03 | Upload-session issuance | attachment use case, small repository/storage ports/adapters, HTTP handler/router, config/main, go.mod/go.sum; membership, concurrent cap and partial-failure tests |
| MED-04 | Completion/access | bounded image/video validation, Dockerfile tool, generation-bound signing; malformed, oversize, wrong owner/type, expiry and repeated completion |
| MED-05 | Atomic media sending | send.go, repository/interfaces, messages.go, WS command/event DTOs; replay, concurrent bind, rollback, after-commit publication |
| MED-06 | History/preview/artifacts | history/conversation repos and DTOs, OpenAPI/AsyncAPI/Postman/docs; ordered arrays, pagination and unread behavior |
| MED-07 | Cleanup/resource bounds | maintenance command/endpoint/instructions; cancel/delete recovery, late upload and cleanup races |
| MED-08 | Release/client acceptance | private test bucket, ADC/IAM/signing, bucket CORS, Android/iOS/Web playback, Range seek, interrupted uploads, reconnect and URL refresh |

Update runtime contracts, tests and docs with EACH implementation item. MED-08 requires cloud/frontend setup and actual device/provider evidence. Local tests alone do not complete deployment or actual Flutter acceptance.

## 8. Acceptance gates

- A sends text-only, image-only, video-only and text with mixed media. B sees same message ID/content/order through send acknowledgement, event and history. Outsider C cannot issue/complete/bind/view another conversation's files; C is an isolated test fixture.
- Upload alone changes no conversation preview/unread and emits no chat event. One media message increments unread once; existing mark_read works.
- Wrong owner/conversation, unknown/duplicate IDs, reused/unfinished/expired files, malformed bytes and boundary sizes fail without partial messages. Repeated complete before/after lost reply is safe, including an already-attached object without resetting state.
- Identical REST/WS send retry returns original ID without insert/publication; reordered/replaced attachments or changed text conflict. Concurrent same-key sends and cleanup races run against real PostgreSQL.
- Restart/reconnect loads durable metadata; URL refresh rechecks membership; attached media survives cleanup. Actual GCS/browser tests cover chunk resume, expiry/cancellation and video Range/seek, not only mocks.
- Exercise object uploaded but DB completion failed, ready but send failed, commit but reply lost, cancellation/delete failure and generation mismatch. GCS and PostgreSQL have no shared atomic transaction.
- Migration preserves existing text data; rollback refuses destructive cases. DB tests use isolated schemas in chat_app_test, never development DB.

Implementation checks: cwd backend/, module github.com/lilpao0/chat_app/backend. Run relevant checks per item, then go test ./..., go vet ./..., go build ./... and race checks for concurrency. Report PostgreSQL, GCS and actual client acceptance separately. MED-01 verified 61 schema/example/boundary cases, matching limits/shared schemas and local file links; go test -count=1 ./internal/presentation/http/swagger passed. Full Go/DB/GCS/client suites were not run for this contract-only step.

Provider references: [resumable uploads](https://docs.cloud.google.com/storage/docs/resumable-uploads), [upload protocol](https://docs.cloud.google.com/storage/docs/performing-resumable-uploads), [JSON headers](https://docs.cloud.google.com/storage/docs/json_api/v1/parameters), [signed URLs](https://cloud.google.com/storage/docs/access-control/signed-urls), [bucket CORS](https://cloud.google.com/storage/docs/cross-origin), [generation preconditions](https://cloud.google.com/storage/docs/request-preconditions). Resumable protocol/header documentation was read during MED-01; live bucket/configuration/API behavior remains unverified.
