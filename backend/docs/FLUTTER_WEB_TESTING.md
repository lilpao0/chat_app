# Flutter Web testing and Google Cloud handoff

The backend supports browser REST CORS and one-use WebSocket connection tickets. This guide describes the new code; deploy that code before testing these endpoints against an older hosted revision. Runtime verification is recorded in [BACKLOG.md](BACKLOG.md).

## Backend configuration

Frontend origin assumed for this handoff: `http://localhost:5173`. Confirm the browser's `location.origin`. Start Flutter Web with a fixed port, for example `flutter run -d chrome --web-port 5173`.

Set on the running API:

```dotenv
WEB_ALLOWED_ORIGINS=http://localhost:5173
```

Multiple origins are comma-separated. Scheme, hostname and port must match exactly; add `http://127.0.0.1:5173` explicitly if needed. No wildcard, URL path or trailing slash is accepted. Empty configuration disables cross-origin browser access and browser tickets. Mobile Bearer handshakes remain available.

Cloud Run URL confirmed by a read-only service lookup on 2026-10-07:

```text
HTTP base: https://chat-api-bzhivzj3sq-as.a.run.app
WS base:   wss://chat-api-bzhivzj3sq-as.a.run.app
```

The hosted revision's preflight was checked on 2026-10-07 and still omitted Authorization. Local code changes do not update that revision. The operator must release the changed backend and set `WEB_ALLOWED_ORIGINS` on Cloud Run. Do not assume `.env.example` configures a hosted service. The existing deployment definitions already cap the service at one instance; the hub and pending-ticket store are process-local. Keep that ceiling until a shared atomic ticket store and realtime broadcast layer exist. Outstanding tickets may be lost during restart or revision changes; request a new one.

For an already authorized operator, the environment-only update command is:

```powershell
gcloud run services update chat-api --project vinai-lab --region asia-southeast1 --update-env-vars 'WEB_ALLOWED_ORIGINS=http://localhost:5173'
```

This command changes the service revision and does not deploy the new binary by itself. Release through the existing approved deployment workflow, preserve secret bindings and other environment variables, and verify the final revision. This implementation session does not execute deployment or cloud configuration writes.

Check Cloud Run/proxy request logging before enabling tickets: redact the `ticket` query parameter, or exclude `/ws` request URLs from access logs where redaction is unavailable. The application does not add request-URL logging. Tickets are credentials; use HTTPS/WSS, never a long-lived JWT in a query string. Browser pages loaded over HTTPS require WSS.

## Preflight verification

```powershell
curl.exe -i -X OPTIONS https://chat-api-bzhivzj3sq-as.a.run.app/api/conversations -H 'Origin: http://localhost:5173' -H 'Access-Control-Request-Method: GET' -H 'Access-Control-Request-Headers: authorization,content-type'
```

Expected after release: HTTP 204, `Access-Control-Allow-Origin: http://localhost:5173`, allowed headers containing Authorization and Content-Type, no Bearer token required. Repeat against `/api/auth/refresh` and `/api/ws/tickets`. Approved-origin application 401/400/500 responses must also carry CORS headers. A rejected origin may yield an unreadable browser CORS error.

If direct API and proxy-facing responses differ, inspect proxy routing/header handling. A WebSocket upgrade must reach the Go server with its query string and Origin intact.

## Accounts and test setup

Use dedicated A/B accounts registered through the existing REST collection, then open/reuse their direct conversation. Set unique test emails and passwords locally; registration uses `first_name`, optional `last_name`, `email`, and `password`. Account C can verify authorization rejection.

Existing `run-seed.ps1` names `alice@test.com` and `bob@test.com`, but no hosted-account availability or passwords are asserted by this guide. Seed passwords do not reset existing accounts. Never copy credentials from a local development DB into Cloud Run automatically. Supply credentials privately after confirming login in the intended test environment.

The automated PostgreSQL acceptance tests create A/B/C fixtures in isolated `chat_app_test` schemas and clean them up. Those temporary credentials are not persistent FE accounts.

## Browser console example

Run this from an approved-origin page. Keep credentials/tokens in memory during testing; do not print them or save them in shared snippets.

```javascript
const httpBase = 'https://chat-api-bzhivzj3sq-as.a.run.app';
const wsBase = 'wss://chat-api-bzhivzj3sq-as.a.run.app';
let accessToken, refreshToken;

async function request(path, options = {}, retry = true) {
  const headers = new Headers(options.headers);
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
  const res = await fetch(httpBase + path, { ...options, headers });
  const body = await res.json();
  if (res.status === 401 && retry && refreshToken) {
    const refreshed = await fetch(httpBase + '/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken })
    });
    const result = await refreshed.json();
    if (!refreshed.ok) throw new Error(result.error?.code ?? 'login_required');
    accessToken = result.data.access_token;
    return request(path, options, false);
  }
  if (!res.ok) throw new Error(body.error?.code ?? `http_${res.status}`);
  return body;
}

async function login(email, password) {
  const res = await fetch(httpBase + '/api/auth/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  });
  const body = await res.json();
  if (!res.ok) throw new Error(body.error?.code ?? 'login_failed');
  accessToken = body.data.access_token;
  refreshToken = body.data.refresh_token;
}

async function connect() {
  const { data } = await request('/api/ws/tickets', { method: 'POST' });
  const socket = new WebSocket(wsBase + '/ws?ticket=' + encodeURIComponent(data.ticket));
  return socket;
}

// Supply dedicated credentials through your local test UI/console.
// await login(email, password);
// const { data: conversations } = await request('/api/conversations');
// const socket = await connect();
// socket.addEventListener('message', event => handleFrame(JSON.parse(event.data)));
// socket.addEventListener('open', () => socket.send(JSON.stringify({
//   type: 'send_message', request_id: crypto.randomUUID(),
//   data: { conversation_id: conversationId, content: 'Browser test' }
// })));
```

`connect()` returns a CONNECTING socket; register listeners immediately and send only after `open`. Browser supplies Origin automatically. Do not add Origin manually to fetch, put Authorization into the WS URL, or send a body to the ticket endpoint. Use Flutter's equivalent browser-compatible WebSocket client; HTTP calls still send Bearer normally.

Socket frames retain `type`, `request_id`, `data`/`error`. `message_sent` confirms persistence; `new_message` is broadcast to every online member device. Their arrival order is unspecified. Merge both by message ID. Use a fresh UUID per logical send and retain it with the original content for unresolved retries.

## Expiration and reconnect procedure

1. After disconnect, apply bounded exponential backoff with jitter; obtain a fresh ticket for every connection attempt. Never reuse a ticket, even after a failed upgrade.
2. If ticket issuance returns HTTP 401 `unauthenticated`, refresh the access token once. If refresh fails with `invalid_refresh_token`, return to login. Network errors and 429 need backoff rather than repeated refresh.
3. Browser WebSocket cannot read failed-handshake JSON or HTTP status. Handle `error`/`close`; 1006 often signals a failed upgrade/network failure. Use REST ticket issuance errors, expiry timestamps, or server diagnostics to investigate.
4. Open the socket and buffer live events before fetching REST history. Starting from the last completed REST checkpoint, request `/api/conversations/{id}/messages?after_id={checkpoint}` until `meta.pagination.has_more` is false, following `meta.pagination.next_after_id`.
5. Read messages from `data`, merge fetched pages and buffered events by message ID, then update the checkpoint from successfully completed REST synchronization. Do not derive it only from the largest live-event ID.
6. Replay unresolved sends using their original UUID and unchanged payload. Socket close 1008 can mean access-token expiry or other policy violations; inspect its reason and avoid treating every 1008 as refresh-needed.

Ticket expiration only limits admission; connected sockets use original access-token expiry. Reloading the page does not preserve this sample's in-memory tokens/checkpoint. Production client storage and UI are owned by the frontend team.

## Acceptance evidence to return

Record actual `location.origin`, backend URL/revision and browser version. Verify A/B login, list, ticket, socket open, both-direction sends, non-member rejection, offline catch-up, used/expired ticket rejection, and access-token refresh/reconnect. Postman and backend Go tests are useful evidence, but only a real Flutter Web/browser run proves browser CORS enforcement and FE integration.
