# Media cloud release — 2026-10-12

The user authorized hosted migration, private storage/IAM, deployment, live backend checks and scheduled cleanup. Frontend changes and Git commit/push are outside this release.

## Resources and migration

| Resource | Actual value |
|---|---|
| Project / region | `vinai-lab` / `asia-southeast1` |
| API | `https://chat-api-bzhivzj3sq-as.a.run.app` |
| Cloud SQL | `chat-db`, database `chat` |
| Before-migration backup | `1791794067578`, `SUCCESSFUL` |
| Schema | Version `5`, `dirty=false` |
| Private bucket | `gs://vinai-lab-chat-media` |
| Browser origin | `http://localhost:5173` |
| Runtime and signer | `506751356289-compute@developer.gserviceaccount.com` |
| CI | `chat-media-builder@vinai-lab.iam.gserviceaccount.com` |
| Cleanup job / schedule | `chat-media-cleanup` / every 5 minutes, `Asia/Ho_Chi_Minh` |
| Scheduler identity | `chat-media-scheduler@vinai-lab.iam.gserviceaccount.com` |
| Shared session secret | `media-session-key:1` |

Hosted DB matched migrations 1–3 without migration history. After a successful native backup and guarded schema check, history was baselined at 3, then migrations 4–5 applied. Before/after migration counts stayed at 17 users, 49 messages, zero attachments. Live checks subsequently added dedicated test accounts/conversations/messages; these are separate from migration preservation evidence. The fixture integration suite was not pointed at this hosted database.

Bucket uses uniform access and enforced public access prevention. CORS allows the exact origin above for GET/HEAD/PUT/DELETE and exposes Range/Content-Range. Default GCS soft delete remains **7 days**: application deletion removes access to live objects, while recoverable bytes can remain and incur charges during retention.

Runtime received bucket-scoped create/get/delete and self-signBlob grants, plus access to the new session secret. Existing unrelated permissions on the pre-existing runtime account were not removed or certified as minimal.

Automatic approval review rejected project-wide Run Developer and permanent self-actAs grants to the API runtime identity. Rejected commands did not execute. Dedicated CI instead received deployment rights on the existing API service and cleanup job, actAs only on their runtime account, repository-scoped image writes, source-prefix reads and log writes. Fixture uploads use a prefix-scoped create/get role and direct GCS HTTP upload; `gcloud storage cp` would require bucket listing. Scheduler has Run Invoker on the cleanup job.

API and job use 2 GiB RAM and the same image, bucket, DB and pinned session-key version. API remains limited to one instance because WS Hub/tickets are process-local. Cleanup uses `/app/media-cleanup -limit 10`, one task, one retry and a 300-second task timeout.

## Live checks and release fixes

First API release: `chat-api-00005-nfm`, image tag `media-20261012-mv50fw5h`, digest `sha256:1fe04dbfb884832b52426e02208e3785eb54b8cd99c985bad8cb586c74111e0d`.

`scripts/media-livecheck.py` passed **167 assertions** against the hosted API and real GCS: resumable 308/status/Range, PNG/JPEG and H.264/AAC completion, ownership/membership errors, no message/unread from upload, mixed WS send, identical ack/event/recipient metadata, ordered files, REST/WS durable retry without republishing, private generation-pinned reads, exact bytes, video Range 206, history, media-only/text-only messages, preview/unread/read and malformed-file rejection. Sanitized results: [MEDIA_LIVE_RESULTS_2026-10-12.json](MEDIA_LIVE_RESULTS_2026-10-12.json).

Two live findings were fixed:

- GCS SDK floors signed lifetime to seconds; a requested 15-minute expiry produces 899–900 seconds. Assertions now accept this bounded rounding rather than requiring exactly 900.
- DELETE on a **completed** GCS resumable session returns HTTP 200 with object metadata. Cancellation adapter now accepts 200 and proceeds to generation-bound object deletion. Original code left failed/expired rows retryable in `deleting`; no attached object was deleted. A regression check accepts terminal statuses and rejects 400.

Focused `go test ./internal/data/storage ./internal/domain/usecase/attachment -count=1` and `go vet ./internal/data/storage/... ./cmd/media-cleanup/...` passed after the cancellation fix. Vet first encountered sandbox cache access denial, then passed with approved cache access.

## Final deployed evidence — all assigned cloud steps complete

- Full seven-step build **`a02564f5-a103-4fd8-b56a-50440e8008d8`**: `SUCCESS`, including Docker build, native H.264/AAC fixture generation/probe, image push, API/job deploy and fixture upload.
- Final image: `asia-southeast1-docker.pkg.dev/vinai-lab/chat-repo/chat-api:media-20261012-cancel200`; digest `sha256:0a0ef865ea40f33fc5812618827e30d29b376d728473c937eca7600e5e54a708`.
- Final ready API revision: **`chat-api-00006-mvn`**. API/GCS/WS acceptance reran on this revision: **167/167 assertions passed**. API and cleanup use the same final image.
- Scheduler-triggered execution **`chat-media-cleanup-xtd6f`**: `Completed=True`, one succeeded task, completed `2026-10-12T09:44:04.691855Z`; runtime log `{"cleaned":4}`. This includes retrying the two earlier failed cleanup fixtures and the two final fixtures.
- Final test attachment IDs **11/12** are `deleted`, have no message binding/lease, and their live GCS objects return **404**. Attached IDs **8/9/10** remain `attached` with original message bindings and exact generations, GCS metadata returns **200**. Six provider assertions and DB-state checks passed; see [cleanup results](MEDIA_CLEANUP_RESULTS_2026-10-12.json).
- Completed upload session still replays HTTP 200 metadata on status query after object deletion; this does not mean the object exists or prove cancellation of an active incomplete session. Live object absence is checked independently. Active-session cancellation/late-write races remain separate acceptance work.
- Scheduler is `ENABLED`, `*/5 * * * *`; its verified manual attempt was `2026-10-12T09:43:50.597359Z`, RPC status success. Natural scheduled execution `chat-media-cleanup-cx4x7` logged `{"cleaned":0}` at `09:45:09Z` after the queue drained.

Earlier failed builds were resolved, not presented as successful releases: `d2b39ab5-fa7a-4560-aff8-aa5f8a9df22d` failed image push; `08fffa31-7891-4495-9bbb-dee15a3e0055` deployed the first API but hit cleanup `--args` parsing; recovery `c89fc10b-abb3-4de9-a7d7-c391aa675836` deployed cleanup then failed artifact bucket access; fixture build `058a7755-83c0-47af-a954-ec0e3a9538e2` hit CLI list permission. Direct HTTP fixture build `4f758077-98b1-4038-ab26-bb2bed02f559` succeeded. The final full pipeline above succeeds without broader runtime/fixture-list grants.

## Evidence limits and next client work

These are real network/API/GCS/native ffprobe checks with browser Origin headers; they are not manual Postman, actual browser rendering or Android/iOS playback. Refresh endpoint and generation pinning passed; waiting 15 minutes and resuming a device player did not run. Full format/size boundaries, provider late-upload races, hosted revision-restart retry and concurrent production DB acceptance remain separate MED-08 gates.

Flutter entrypoint has no chat UI. Client still needs file selection, direct resumable upload/progress, completion, ordered `attachment_ids`, media rendering and access-link refresh. Frontend was untouched. Exact localhost origin must be replaced/extended when a real hosted client origin is chosen.

Deployment used current workspace source. No commit/push was performed; future Git-trigger releases require committing and pushing these changes separately. Task-only DB credentials/session key/capability files are removed after verification; reports contain no passwords, JWTs, signed URLs or resumable session URLs. Dedicated live fixture accounts/messages and attached media are retained as evidence; failed/orphan objects were cleaned.
