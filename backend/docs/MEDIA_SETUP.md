# Private chat media: setup and release checks

Backend MED-02 through MED-07 is implemented. The assigned cloud release has migrated the hosted DB, provisioned private GCS/IAM and deployed the API/job. Real API/GCS checks and scheduled cleanup verification pass. Actual browser/device acceptance remains under MED-08. See [cloud release evidence](MEDIA_RELEASE_2026-10-12.md). The Flutter entrypoint currently has no chat UI.

## Runtime configuration

Apply **all five** SQL migrations to the intended database before starting this API. The application does not migrate automatically. Existing message history now reads attachments even for text-only messages.

| Variable | Meaning |
|---|---|
| `MEDIA_BUCKET` | Private bucket name; empty disables new storage operations (503) |
| `MEDIA_SIGNER_EMAIL` | Service account used for V4 signed GET URLs |
| `MEDIA_SESSION_KEY` | Base64 encoding of exactly 32 random bytes; stable, shared by API and cleanup |
| `MEDIA_FFPROBE` | Executable path/name, default `ffprobe`; checked at startup when media enabled |
| `MEDIA_UPLOADS_PER_MINUTE` | Operational per-process issuance limit/user, default 60; 1-10000 |

Store the session key in Secret Manager and inject the same secret version into API and cleanup. Never commit/print it or rotate while encrypted recovery records remain. AES-GCM binds the ciphertext to attachment ID/owner. Back up this key with the DB; losing it prevents cancellation of outstanding sessions. Rotation needs an explicit decrypt/re-encrypt migration, currently absent. Session/access URLs are bearer credentials; keep them out of logs, exports and telemetry.

Use Cloud Run service identity via Application Default Credentials (ADC). For local development, use `gcloud auth application-default login` with approved identity/permissions. Backend requests the cloud-platform OAuth scope for both Storage and IAM Credentials. Do not distribute service-account key files. Docker image includes ffmpeg/ffprobe and both API/cleanup binaries.

Start with at least **2 GiB RAM** for media-enabled runtime, then measure actual peak memory/CPU. Two concurrent inspections may decode 25M-pixel PNGs (RGBA64 can need 200 MB per image), plus decoders, bounded temporary files and ffprobe. Cloud Run temporary files consume instance memory. Each completion has 30s total budget, including queue wait; HTTP write timeout is 45s. Slow/complex video can return retryable 503. `ffprobe -count_frames` validates a decodable H.264 stream; no transcoding, thumbnail generation or complete playback compatibility guarantee.

Keep the existing single-instance Cloud Run topology until Hub/tickets are shared or routing is redesigned. Media rows and pending cap are shared through PostgreSQL; WS Hub/tickets and issuance-rate limiter remain process-local. Default60/min is a protective operational choice, not a confirmed user quota or aggregate billing cap.

## GCS and IAM preparation

1. Create a private bucket in the chosen region near the service. Enable uniform bucket-level access and public access prevention. Do not enable age-only deletion for the shared attachment prefix; old attached media must remain readable. Select bucket soft-delete/retention/versioning policies intentionally: they can delay physical removal and incur costs beyond application expiry.
2. Give runtime identity bucket-scoped permissions `storage.objects.create`, `storage.objects.get`, `storage.objects.delete` (custom role, or review broader built-in `roles/storage.objectUser`). No object list/update permission is required by this adapter. Give signer identity `storage.objects.get` on this bucket for signed reads. If runtime and signer are the same identity, these grants combine.
3. Enable IAM Service Account Credentials API. Grant runtime identity `iam.serviceAccounts.signBlob` on the **signer service account**, using a narrowly scoped custom role or `roles/iam.serviceAccountTokenCreator` on that account. Local ADC principal also needs signing permission when testing. A separate signer does not need to be impersonated for ordinary object operations.
4. API and cleanup job must use the same bucket, DB and session key. Set exact `WEB_ALLOWED_ORIGINS` on API. Bucket CORS is separate and must use the same approved browser origins.

Example bucket CORS file (replace origin; apply only after cloud setup is assigned):

```json
[
  {
    "origin": ["https://YOUR_CHAT_WEB_ORIGIN"],
    "method": ["GET", "HEAD", "PUT", "DELETE"],
    "responseHeader": ["Content-Type", "Content-Range", "Range", "ETag", "Content-Length"],
    "maxAgeSeconds": 3600
  }
]
```

Use `gcloud storage buckets update gs://YOUR_PRIVATE_BUCKET --cors-file=media-cors.json` against the explicitly selected bucket. No public ACL is needed. Browser automatically supplies Origin; API forwards it when initializing resumable session. Verify resumable JSON API behavior from actual browser; CORS is not access control. Signed reads use `storage.googleapis.com`, support Range and pin validated generation.

## Client/API flow

Import [Postman collection](chat-app-rest.postman_collection.json), group **Media uploads and messages**. Set `mediaContentType`, `mediaSizeBytes` to actual local file length, `mediaLastByte` to length minus one. Obtain grant, then copy `data.id` into `attachmentId` and `data.upload_url` into `mediaUploadUrl` locally. Select matching file for direct PUT; no backend JWT goes to GCS. Do not export populated capability variables.

1. `POST /api/conversations/:id/attachments` with `{"content_type":"image/jpeg","size_bytes":ACTUAL_BYTES}` returns201. Grant alone creates no message/unread/event.
2. Upload directly to session URL. One final chunk uses `Content-Range: bytes 0-LAST/TOTAL`; HTTP stack sets Content-Length. For larger/interrupted transfer, nonfinal chunks are multiples of256 KiB (8 MiB suggested). Handle308 as **Resume Incomplete**, not redirect; read Range to find confirmed offset. Empty PUT with `Content-Range: bytes */TOTAL` queries progress. If Range absent, begin at0. Final200/201 is provider upload success only.
3. Call `POST /api/attachments/:id/complete` with `{}`. Only verified files become ready. Missing object409; unsupported codec415; malformed/mismatched file422; temporary storage/probe timeout503 stays retryable. Permanent validation failure requires fresh grant.
4. Send content and/or ordered `attachment_ids` through REST or WS. Example: `{"content":"Trip","attachment_ids":[501,502],"request_id":"550e8400-e29b-41d4-a716-446655440000"}`. Omit content for media-only. Wait until every selected file is ready. Use stable request_id and exact original payload on uncertain send outcome; changed text/order/selection requires a new key.
5. Message/WS/history carries metadata only. Call `GET /api/attachments/:id/access` as current member for fresh 15-minute GET URL. Owner alone can preview ready unsent file. Refresh URL after expiry without resending message; test player resume/seek after URL replacement.

Confirmed limits: 5 files/message, JPEG/PNG ≤10 MiB and25M pixels, MP4 exactly one H.264 video and optional single AAC audio ≤50 MiB, text ≤2000 Unicode code points,10 unexpired pending uploads/user,24h pending expiry. No combined byte/duration quota. HEIC/HEVC/MOV/WebP/GIF require client conversion or future support. Original image metadata is retained.

## Cleanup operations

Run `go run ./cmd/media-cleanup -limit 10` locally, or `/app/media-cleanup -limit 10` as a Cloud Run Job with the same configuration/secrets/identity and DB connectivity. Command loads optional `.env`, outputs only `{"cleaned":N}` and sanitized errors. The `vinai-lab` release provisions Scheduler `chat-media-cleanup` every5 minutes with OAuth invocation of the identically named Run Job. Tune batch size1-100/frequency using backlog and storage latency, avoiding a permanently growing orphan queue. One run stops after4 minutes; DB claims have5-minute leases. Job retries are safe. A success count includes tombstone reconciliation, not only newly removed files.

Cleanup claims expired unattached uploads/ready files and failed/deleting rows with `FOR UPDATE SKIP LOCKED`. It cancels stored encrypted resumable session first, deletes pinned object generation (or discovers current generation for unverified upload), then marks deleted only on success. Failures stay retryable; abandoned leases recover after5 minutes. Attached media is excluded. Deleted tombstones are reconciled at most hourly for8 days to catch late completion/cancellation races; recovery session ciphertext is cleared after final pass. Small tombstone rows remain; bulk metadata retention pruning is deferred until volume requires it.

Application24h expiry does not immediately revoke GCS capability: sessions can live up to a week. A lost response during provider initiation can leave a session unknown to API until provider expiry; it was never handed to client. API/GCS have no shared transaction. Session cancellation semantics and expected-length enforcement need live validation. Pending/rate limits alone do not guarantee a hard upload-cost ceiling.

Migration000004 down refuses while any attachments or incompatible blank messages remain. Migration000005 down refuses while encrypted sessions or cleanup claims remain. Never clear recovery columns to force rollback: prefer forward correction after rollout.

## Required MED-08 evidence

- Private test bucket: ADC/IAM create/read/delete/signBlob, actual type/length enforcement, create-only overwrite rejection, generation-pinned signed read and cleanup deletion.
- Actual browser: approved/unapproved origins, PUT status308/Range, interrupted chunk resume, session cancellation499 and late-upload reconciliation.
- Real media: JPEG/PNG boundaries/truncation, H.264/AAC MP4 and rejected HEVC/extra tracks, playback on Android/iOS/Web; Range seeking and access URL refresh after15min.
- Hosted DB: migrate intended environment, replay after revision restart, pending cap and cleanup races, backups/key recovery. Never aim test suite at production DB; fixtures require `chat_app_test`.
- Flutter: ordered media bubbles, empty text, failed upload removal/retry, progress, reconnect/history merge. Current frontend contains no such implementation.

Offline checks are real SQL/REST/WS plus fake external storage, stdlib image decoding, provider HTTP/signing request checks and probe-output checks. Cloud release evidence separately covers actual Docker build/native ffprobe and real API/GCS behavior; neither proves device playback.
