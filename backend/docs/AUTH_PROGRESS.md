# Authentication handoff - 2026-09-21

The user authorized continuation until a decision needs their answer. B10-B15 are complete; B16 awaits the read-position design choice. All changes preserve the presentation/domain/data layout. No commits, deployment, or Flutter changes were made.

## B10 - Registration use case

- Added domain/usecase/auth/register.go: normalize/validate input, hash through PasswordHasher, insert through UserRepository, return PublicUser without credentials.
- Tests cover normalization, unchanged passwords, invalid input before dependencies, canceled context, duplicate email, repository errors and hashing failure.
- Vietnamese explanation: the domain coordinates interfaces and database uniqueness handles concurrent registration safely. Next was B11.

## B11 - Registration HTTP endpoint

- Added presentation handlers/response helper and main wiring for POST /api/auth/register. Uses explicit public DTO, JSON object/unknown-field validation, 16 KiB limit and 201/400/409/413/415/500 mappings.
- HTTP tests cover malformed/trailing JSON, unknown fields, wrong types, media type, size limit, duplicate email and secret-free output. PostgreSQL integration exercises simultaneous normalized duplicates: one 201, one 409, one stored account.
- Vietnamese explanation: presentation handles transport and delegates business validation. Next was B12.

## B12 - JWT adapter

- Added domain token types/issuer interface, data/auth/jwt.go, startup config and tests. Added golang-jwt/jwt/v5 v5.3.1, requiring valid HS256 signature, expiry, canonical positive subject, issuer, audience and time claims. See I05 in DECISIONS.
- Tests verify issue/verify identity and expiry; reject expired/malformed/wrong-key/wrong-algorithm/none tokens, missing expiry/subject, bad subject, issuer/audience mismatches, future issued-at/not-before. Config defaults and invalid secret/TTL are tested.
- Vietnamese explanation: tokens are signed and expire; environment owns the secret, default demo TTL is 24h, logout does not revoke old tokens. Next was B13.

## B13 - Login use case

- Added domain/usecase/auth/login.go: normalize email, find user, compare password, issue token only on success. Missing user and incorrect password return the same application error. Other failures preserve their cause.
- Fake-based tests cover success, nonexistent users, wrong passwords, invalid input, normalization, repository/hash/token failures and no issuance after failed authentication.
- Vietnamese explanation: domain coordinates three interfaces without importing data implementations. Next was B14.

## B14 - Login endpoint

- Added POST /api/auth/login and shared production router. Returns access_token, UTC expires_at, public user and no-store; uses a shared 401 credential error.
- Handler tests verify response/error mapping. Real PostgreSQL integration via httptest verifies registration/login, stored hash, token identity/expiry, no hash/plaintext leaks and identical missing-user/wrong-password responses.
- Vietnamese explanation: the client uses Bearer tokens; expiry requires login again. No refresh/server revocation. Next was B15.

## B15 - Authentication middleware

- Added presentation TokenVerifier interface, Bearer middleware and identity accessor. The production router returns a protected /api group for future chat routes; public health/auth routes remain outside it. A protected test route exists only in tests.
- Tests cover missing/malformed/duplicate/invalid/expired/valid headers, case-insensitive Bearer scheme, identity propagation and ignored client user_id. PostgreSQL integration rejects an actually expired JWT and confirms the valid token reaches the protected handler.
- Vietnamese explanation: authentication identifies the caller; later use cases must separately check conversation membership.

## Verification and remaining work

### 2026-09-23 refresh-token extension

- Login now returns access and refresh JWTs with separate expiration timestamps. `POST /api/auth/refresh` validates a refresh token and returns a new access token with `Cache-Control: no-store`.
- A required token-type claim prevents refresh tokens from authorizing protected routes and prevents access tokens from calling the refresh endpoint. `JWT_REFRESH_TTL` defaults to 720h.
- This deliberately simple flow has no persistence, rotation, reuse detection, logout endpoint, or server-side revocation; refresh tokens remain reusable until expiry.

- `go test -count=1 ./...` passed with TEST_DATABASE_URL configured for chat_app_test, including repository and full auth integration. Tests use connection-local temporary users tables derived from the active migration; persistent data is untouched.
- `go vet ./...` passed. No live network server or Flutter integration was exercised; httptest exercises the production router with real PostgreSQL, bcrypt and JWT.
- Database integration tests are mandatory. They use `TEST_DATABASE_URL` or derive the isolated `chat_app_test` URL from local `.env`, and fail rather than skip when PostgreSQL is unavailable. No credentials/tokens were printed or committed.
- Next: B16. Ask whether to use last_read_message_id alongside last_read_at and serialize per-conversation inserts before allocating message IDs (D08-D09), or retain the roadmap's timestamp-based design. Do not write a chat migration until this schema/client-contract question is answered.
