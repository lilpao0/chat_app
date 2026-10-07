# REST response migration: implementation record

Prepared: 2026-10-06 and implemented in the backend. This file retains the chosen contract, changed surfaces, and verification procedure; current verification status belongs in [BACKLOG.md](BACKLOG.md).

## 1. Scope and contract

Migrate application REST JSON responses to a custom envelope inspired by JSend. This is not strict JSend: both `fail` and `error` use an `error` object.

| HTTP result | Required body | Optional fields |
|---|---|---|
| 200/201 | `{"status":"success","data":...}` | `meta` |
| 4xx | `{"status":"fail","error":{"code":"...","message":"..."}}` | `error.details`, `meta` when real metadata exists |
| 5xx | `{"status":"error","error":{"code":"internal_error","message":"Unable to complete the request."}}` | `meta` when real metadata exists |

Rules:

- Success always includes `data`. Objects remain objects; lists are arrays directly under `data`, without `data.items`.
- Empty lists are `[]`, not `null`. An intentionally empty success payload can be `data: null`.
- `data` and `error` never coexist. Omit unused `meta` and `details` instead of returning null or empty placeholders.
- Pagination lives at `meta.pagination`. Preserve cursor values, ordering and existing null semantics.
- Keep current HTTP codes, request bodies, DTO fields, authorization, timestamps, and `Cache-Control: no-store`.
- Do not invent total counts, page numbers, tracing IDs, or a new pagination algorithm.
- No current route needs a new 204 response. Existing bodyless CORS responses, redirects, Swagger HTML and the raw OpenAPI document stay outside the envelope.
- WebSocket frames retain `type`, `request_id`, and `data`/`error`. HTTP errors emitted by the application's handshake handler before upgrade adopt the REST error envelope. `101` has no JSON envelope.
- Do not modify domain/data, migrations, JWT behavior, WebSocket command dispatch, or `frontend/`.

This is a breaking response change. Existing consumers must update their response paths before using the changed backend. Implement locally as small steps, but release the completed migration together. This guide does not authorize deployment.

## 2. Target examples

Login:

```json
{
  "status": "success",
  "data": {
    "access_token": "<access_token>",
    "expires_at": "2026-10-07T04:40:02Z",
    "refresh_token": "<refresh_token>",
    "refresh_expires_at": "2026-11-05T04:40:02Z",
    "user": {"id": 8, "name": "Bao", "email": "bao@example.test", "avatar_url": "..."}
  }
}
```

History:

```json
{
  "status": "success",
  "data": [{"id": 101, "conversation_id": 10, "sender_id": 8, "content": "Hello", "created_at": "2026-10-06T04:40:02Z"}],
  "meta": {
    "pagination": {"has_more": true, "next_before_id": 101, "next_after_id": null}
  }
}
```

Validation failure (proposed field mapping in step 3):

```json
{
  "status": "fail",
  "error": {
    "code": "invalid_input",
    "message": "The submitted information is invalid.",
    "details": [{"field": "email", "code": "invalid_email", "message": "Email is invalid."}]
  }
}
```

Keep `invalid_credentials`, `unauthenticated`, `invalid_refresh_token`, `not_found`, `request_conflict`, `email_taken`, and `phone_number_taken` for their existing conditions. Never expose SQL errors, credentials, or stack traces in error payloads.

## 3. Add the shared response helpers first

Paths in the tables below are relative to `backend/`. Function names are better search anchors than line numbers, which shift during editing.

| File | Change |
|---|---|
| `internal/presentation/http/response/success.go` (new) | Add `Success` and `SuccessWithMeta`. Keep JSON construction in presentation. |
| `internal/presentation/http/response/error.go` | Preserve the existing `Error(c, status, code, message)` call signature. Derive body status from HTTP status: 4xx is `fail`, 5xx is `error`. Add optional typed field details. |
| `internal/presentation/http/response/response_test.go` (new) | Test success object/list/null, absent metadata, nested pagination, 400/401/409 versus 500, optional details and mutually exclusive data/error. |

Suggested minimal success helper:

```go
package response

import "github.com/gin-gonic/gin"

func Success(c *gin.Context, status int, data any) {
    c.JSON(status, gin.H{"status": "success", "data": data})
}

func SuccessWithMeta(c *gin.Context, status int, data, meta any) {
    c.JSON(status, gin.H{"status": "success", "data": data, "meta": meta})
}
```

Call `SuccessWithMeta` only when actual metadata exists; otherwise call `Success`. Initialize list DTOs with `make([]SomeDTO, 0, len(items))` so empty lists marshal to `[]`.

For error details, add a small `FieldError` struct with `field`, `code`, `message` JSON tags. One implementation option is `ErrorWithDetails(..., details []FieldError)` and have the existing `Error` delegate to it with nil. Include `details` only when nonempty. Both helpers must use `AbortWithStatusJSON`, preserving middleware abort behavior. Only use these error helpers for HTTP 4xx/5xx.

Do not add a response-rewriting middleware: it can interfere with streaming, redirects, Swagger and WebSocket upgrades. Keep domain errors and use case result types unchanged.

## 4. Edit each runtime response

### Auth, health and profiles

| File / function | Current payload | How to change |
|---|---|---|
| `internal/presentation/http/handler/health.go` / `Health` | `{"status":"ok"}` | `response.Success(c, http.StatusOK, gin.H{"status":"ok"})`. Result is `status=success` outside and `data.status=ok` inside. |
| `internal/presentation/http/handler/auth.go` / `RegisterHandler.Handle` | `{"user":toUserDTO(user)}` | Pass `toUserDTO(user)` directly to `Success` with 201. Remove the outer `user` key. |
| `internal/presentation/http/handler/login.go` / `Handle` | Token fields plus `user` | Pass the existing complete `gin.H` payload to `Success` with 200. Keep `user` inside login data because login returns multiple related values. |
| `internal/presentation/http/handler/refresh.go` / `Handle` | Access token and expiry | Pass the existing complete payload to `Success` with 200. Do not add a refresh token. |
| `internal/presentation/http/handler/profile.go` / `GetPrivate`, `GetPublic`, `Update` | `{"user":profileDTO}` | Pass the matching profile DTO directly to `Success` with 200. Preserve private/public field restrictions and nullable fields. |

Add the `response` import where missing; keep `gin` imports where handler parameters or payloads still use it. Keep login/refresh `Cache-Control` headers before writing the response.

Registration validation needs an explicit decision within this migration: adopt the example above with top-level `error.code=invalid_input`, and put a stable field code in `details[0].code`. Currently the handler derives the public code from `ve.Message`; replace that string transformation with a presentation-level mapping of the known `auth.ValidationError` values. Suggested codes:

| Current validation value | Detail code |
|---|---|
| `ErrFirstNameRequired`, `ErrFirstNameEmpty` | `required` |
| `ErrFirstNameTooLong`, `ErrLastNameTooLong`, `ErrPasswordTooLong` | `too_long` |
| `ErrFirstNameInvalid`, `ErrLastNameInvalid`, `ErrPasswordInvalid` | `invalid_characters` |
| `ErrEmailRequired`, `ErrPasswordRequired` | `required` |
| `ErrEmailInvalid` | `invalid_email` |
| `ErrPasswordTooShort` | `too_short` |

Keep `ve.Field` and the safe existing `ve.Message`. Use `invalid_input` as the fallback detail code for an unknown validation value. The domain currently returns the first validation error: return one detail, not fabricated multiple errors. Correct the existing password-too-long wording in the presentation mapping to say **72 UTF-8 bytes**, not 72 characters. JSON decoding failures in `readJSON` may keep a general error without field details. This changes registration validation codes intentionally; update its tests and schema accordingly.

### Discovery, conversations and chat

| File / function | How to change |
|---|---|
| `internal/presentation/http/handler/discovery.go` / `Search` | Send `items` directly as `data`. Move `has_more` and `next_after_id` into `meta.pagination`. |
| `internal/presentation/http/handler/discovery.go` / `OpenDirect` | Pass the inner object containing `id` and counterpart `user` as `data`. Remove only the outer `conversation` wrapper. Preserve 201-created / 200-reused. |
| `internal/presentation/http/handler/conversations.go` / `List` | Replace `c.JSON(200, result)` with `response.Success(c, 200, result)`. List remains unpaginated; no artificial metadata. |
| `internal/presentation/http/handler/messages.go` / `Send` | Pass `toMessageDTO(message)` directly as `data`, preserving 201. Leave `chatError` mappings intact; shared error helper adds status. |
| `internal/presentation/http/handler/history.go` / `Get` | Send `items` directly as `data`. Move all three pagination fields under `meta.pagination`. |
| `internal/presentation/http/handler/read.go` / `Mark` | Pass the current object containing `conversation_id` and `last_read_message_id` as `data`. |

History example at the call site:

```go
response.SuccessWithMeta(c, http.StatusOK, items, gin.H{
    "pagination": gin.H{
        "has_more": page.HasMore,
        "next_before_id": page.NextBeforeID,
        "next_after_id": page.NextAfterID,
    },
})
```

User search includes only `has_more` and `next_after_id`. History includes both cursors, retaining null for the inactive direction and existing terminal-page semantics.

### Error boundaries to inspect

| File | Required handling |
|---|---|
| `internal/presentation/http/middleware/auth.go` | Existing `response.Error` calls automatically gain `status=fail`. Verify aborted requests never reach protected handlers. No new JWT logic. |
| `internal/presentation/websocket/handler.go` | Its explicit HTTP 401/403/405 responses automatically gain `status=fail`. Do not wrap `coderws.Accept`, write JSON after upgrade, or change frames. |
| `internal/presentation/http/router.go` | Optional separate follow-up for JSON `NoRoute`/`NoMethod` and panic recovery. Current router does not configure these; do not claim every framework/transport error is JSON after changing the helper. |

Errors generated by the WebSocket library for malformed upgrade headers, Go HTTP parsing, CORS, or a reverse proxy are outside the application helper. Keep this boundary documented. Normalizing those is separate work, not part of this envelope migration.

## 5. Update tests alongside each handler group

Existing tests often search for a substring, which can pass even if the envelope is wrong. Decode JSON and assert its structure, required status and absent obsolete keys. Do not compare JSON object key order.

| Existing file | Specific update |
|---|---|
| `internal/presentation/http/handler/auth_test.go` | Replace the exact `{"user":...}` success body assertion with decoded `status`/`data`; update registration validation codes/details, 409 fail and 500 error expectations. |
| `internal/presentation/http/handler/login_test.go` | Read tokens, expiries and user under `data`; assert 401 fail, 500 error and no-store. Keep secret-leak checks. |
| `internal/presentation/http/handler/refresh_test.go` | Read access token/expiry under `data`; assert no refresh token, 401 fail, 500 error, no-store. |
| `internal/presentation/http/handler/profile_test.go` | Replace outer `user` lookup with `data`; retain public-field privacy and null assertions; assert fail/error classification. |
| `internal/presentation/http/middleware/auth_test.go` | Assert unauthenticated response status is fail and execution is aborted. Test-only success handlers need no blanket rewrite. |
| `internal/presentation/http/auth_integration_test.go` | Wrap login and refresh decoding structs in `Data`; keep real token verification and test-only actor endpoint behavior. |
| `internal/presentation/http/test/discovery_test.go` | Replace `items` with `data`; move cursor decoding into `Meta.Pagination`; decode open-direct response from `data`. |
| `internal/presentation/http/test/conversations_test.go` | Replace raw-array decoding and literal `[]` assertion with envelope decoding and `data: []`; retain membership/unread assertions. |
| `internal/presentation/http/test/chat_test.go` | Update shared page response type, send result wrapper, read marker wrapper and all cursor accesses. Keep history/read/idempotency expectations. |
| `internal/presentation/websocket/acceptance_test.go` | Change ONLY REST send response `message` and REST history `items` decoding to `data`; keep frame decoding unchanged. |
| `internal/presentation/websocket/bidirectional_test.go` | Change REST retry/fallback response `message` wrapper to `data`; keep WebSocket event assertions. |
| `internal/presentation/websocket/handler_test.go` | Extend HTTP handshake-error decoding/assertions with `status=fail`; retain valid 101 handshake test. |
| `internal/presentation/http/swagger/contract_test.go` | Update registration error-code contract test; add envelope schema/example checks for every application REST response and HTTP handshake errors. |
| `internal/presentation/http/swagger/handler_test.go` | Preserve raw OpenAPI/UI behavior and Postman route coverage; extend checks for revised collection examples/assertions if useful. |
| `internal/presentation/http/swagger/websocket_contract_test.go` | Keep WS frame schemas unchanged. Adjust only assertions that actually depend on HTTP response schemas. |

Add `internal/presentation/http/handler/health_test.go` for `data.status=ok`, since health currently has no dedicated handler test. `optional_field_test.go`, domain/data tests, and other WebSocket lifecycle tests should not require response-model changes.

Suggested updated history decoder in tests:

```go
var body struct {
    Status string `json:"status"`
    Data []messageResponse `json:"data"`
    Meta struct {
        Pagination struct {
            HasMore bool `json:"has_more"`
            Before *int64 `json:"next_before_id"`
            After *int64 `json:"next_after_id"`
        } `json:"pagination"`
    } `json:"meta"`
}
```

## 6. Synchronize OpenAPI and Postman

### `internal/presentation/http/swagger/openapi.json`

- Keep raw resource schemas (`User`, `Participant`, `PrivateProfile`, `PublicProfile`, `Message`, `DirectConversation`, `ConversationSummary`) unchanged; WS schemas reuse some of them.
- Update `HealthResponse`, `UserResponse`, `PrivateProfileResponse`, `PublicProfileResponse`, `LoginResponse`, `RefreshResponse`, `DirectConversationResponse`, `MessageResponse`, and `MarkReadResponse` to require `status` and `data`.
- `LoginResponse.data` contains user and tokens; `RefreshResponse.data` contains access token and expiry. Other single-resource responses reference the resource directly from `data`.
- Update `UserSearchPage` and `MessagePage` to `status`, array `data`, and `meta.pagination`. Add separate metadata schemas for search and history so their required/null fields remain accurate.
- Replace the inline array response for `GET /api/conversations` with a success envelope containing an array of `ConversationSummary`.
- Use `enum: ["success"]` for success status (the document uses OpenAPI 3.0, so do not use JSON Schema `const`).
- Add/reuse concrete fail and server-error response schemas with their matching status enums. Update every inline/component 4xx/5xx schema reference and example, including `/ws` HTTP rejection examples.
- Revise `RegisterValidationError` / `RegisterErrorResponse` for `invalid_input` and typed optional details; remove the old prose-derived code list after updating its tests.
- Make `data` and `error` mutually exclusive in response schemas, for example with separate closed envelope schemas. Keep `meta` optional except for the existing paginated routes, where it is required.
- Keep pagination cursor nullability, all status codes, auth security declarations and request schemas.
- Keep the `WS*` message schemas unchanged. Do not globally wrap every schema containing `data` or `message`.
- Update all response examples and bump the contract's version to identify the breaking response revision. A version field change does not introduce a new route prefix.

### `docs/chat-app-rest.postman_collection.json`

Update each success assertion and its variable-copy instructions:

| Old access | New access |
|---|---|
| Health `body.status === "ok"` | `body.status === "success"` and `body.data.status === "ok"` |
| Register/profile `body.user` | `body.data` |
| Login `body.access_token`, `body.user` | `body.data.access_token`, `body.data.user` |
| Refresh `body.access_token` | `body.data.access_token` |
| Search/history `body.items` | `body.data` |
| Search/history top-level cursors | `body.meta.pagination.<cursor>` |
| Direct conversation `body.conversation.id` | `body.data.id` |
| REST send `body.message.id` | `body.data.id` |
| Conversation list `pm.response.json()` | `pm.response.json().data` |
| Mark-read `body.last_read_message_id` | `body.data.last_read_message_id` |
| `body.error.code` | Same location; additionally assert `body.status === "fail"` or `"error"` |

Prefer naming the full object `body` and the payload `data` in scripts to avoid accidentally checking `data.status`. Preserve manual variable management: the collection deliberately does not automatically overwrite IDs or tokens. Add shape checks for profile requests if absent. Use placeholders only in saved token examples.

## 7. Documentation file checklist

| File | What to update during implementation |
|---|---|
| `docs/CONTRACTS.md` | General response rules, error table/examples, all 13 REST operation success shapes (including health), pagination and reconnect parsing paths. Describe field details and the handshake HTTP boundary. |
| `docs/DECISIONS.md` | Record the selected custom status/data/meta/error envelope, breaking change, frontend exclusion and unchanged socket protocol. Distinguish adopted contract from pending implementation. |
| `docs/ARCHITECTURE.md` | Explain that handlers/helpers own serialization; use cases and repositories return existing domain values/errors. |
| `README.md` (backend README) | Update sample response parsing, login token retrieval and REST catch-up instructions. |
| `docs/README.md` | Link this manual guide; describe migration state accurately. |
| `docs/WEBSOCKET_PLAN.md` | Update only REST fallback/catch-up references to `data` and `meta.pagination` if needed; retain all socket examples. |
| `docs/MVP_PLAN.md` | Adjust current acceptance wording such as bare conversation arrays when necessary; retain historical milestones. |
| `docs/BACKLOG.md` | Track small implementation steps and actual validation results. Keep implementation TODO until implemented and verified. |

Inspect the repository-root `README.md` for concrete response examples; edit only if it contains paths/shapes made stale by this migration. Do not rewrite the original six-week roadmap as if it were the active contract.

`docs/chat-app-websocket.asyncapi.json` keeps its socket payloads. Review descriptions for REST references, but do not apply the REST envelope to AsyncAPI messages. `schema.dbml`, SQL migrations, deployment files and seed scripts need no response change.

## 8. Suggested implementation order and verification

1. Add shared success/error helpers and focused tests. Verify 4xx/5xx classification, null/list behavior and absence of conflicting fields.
2. Migrate health/auth handlers and their tests, including registration details and auth integration response parsing.
3. Migrate profile handlers and privacy tests.
4. Migrate discovery/conversation handlers and integration response decoders.
5. Migrate send/history/read handlers and tests, including REST readers inside WebSocket tests and handshake HTTP assertions.
6. Synchronize OpenAPI, Postman and current documentation. Finish with complete tests and a diff review.

From `backend/`, run focused checks as each group is ready:

```powershell
go test ./internal/presentation/http/response ./internal/presentation/http/handler ./internal/presentation/http/middleware
go test ./internal/presentation/http/...
go test ./internal/presentation/websocket/...
go test ./...
go vet ./...
```

Format only modified Go files using `gofmt -w` with explicit paths. Integration tests require the isolated `chat_app_test` database. Existing `internal/testutil/database.go` uses `TEST_DATABASE_URL` or derives that DB from local configuration, creates private schemas and cleans them up. Missing DB access is a verification failure to report, not a passing skipped test. Never reset development data to run these checks.

The Swagger Go tests parse JSON and check contract details; they do not execute Postman JavaScript. Import the updated collection and exercise it against the changed backend using dedicated test accounts. Record manual or collection-run results separately from Go results. Check all 13 REST operation shapes, especially empty lists, terminal pagination, login/refresh no-store, registration details, HTTP 401/409/500, and unchanged WebSocket command/event payloads.

Use these searches to locate remaining old serializers and parsers (results require inspection; legitimate nested DTO/WS fields must remain):

```powershell
rg -n 'c\.JSON|AbortWithStatusJSON|response\.Error' internal/presentation
rg -n 'json:"(user|message|conversation|items)"' internal/presentation -g '*test.go'
rg -n 'body\.(user|message|conversation|items|access_token|has_more)|pm\.response\.json\(\)' docs/chat-app-rest.postman_collection.json
git diff --check
git diff --name-only
```

Completion criteria: application serializers, response decoders, schemas, examples and Postman assertions agree; full required checks pass; frontend/domain/data/WS frame behavior remains untouched; no partial migration is marked done. A failed or unrun integration/manual check must be stated explicitly.

## 9. Implementation status

Runtime handlers, shared helpers, tests, OpenAPI, Postman and current backend documentation now use this contract. Frontend, domain/data behavior, database migrations and post-upgrade WebSocket frames remain unchanged. Follow-up on 2026-10-07: existing isolated `chat_app_test` became available; full normal/race suites and `go vet ./...` pass. Empty-list checks no longer depend on JSON object-key ordering, and missing profile envelope assertions were added to Postman. Manual Postman execution remains unrun.
