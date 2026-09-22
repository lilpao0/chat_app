# User discovery and direct chat - 2026-09-21

User decision U11 expanded the original demo-only scope. X01-X03 are complete; B01-B29 of the original plan remain complete, while B30-B34 are unfinished. No existing database migration was added, no development data was changed, and no Flutter UI was implemented.

## X01: Search

- Domain: entity.UserSearchQuery/UserSearchPage, repository.UserSearcher and usecase/user.Search. The use case trims q, validates 2-254 Unicode code points, defaults limit to 20 and caps it at 50.
- Data: PostgresUserRepository.Search uses parameterized SQL. It matches a literal case-insensitive name substring or an exact email, excludes the actor and returns only id/name/avatar_url. Ascending ID plus limit+1 implements cursor pagination.
- Presentation: protected GET /api/users parses only q/limit/after_id and returns items, has_more and nullable next_after_id. Empty/invalid q, invalid limits/cursors and unknown/duplicate parameters return 400; absent auth returns 401.
- Verified through tests in the domain use case and real PostgreSQL/HTTP integration: result pages, actor exclusion, exact email, literal % wildcard, privacy and input failures.
- Explained in Vietnamese: search does not create a conversation; the client picks a returned user ID to open one.

## X02: Open/reuse direct conversation

- Domain: DirectConversationOpener and usecase/conversation.OpenDirect validate positive, distinct actor/target IDs. Actor comes from authentication, target from the JSON body.
- Data: PostgresConversationRepository.OpenDirect uses a read-committed transaction. OpenDirectInTx takes a transaction-scoped advisory lock on sorted IDs, verifies both users, finds an exact two-member conversation or creates it with both members. The seed calls the same function after creating/reusing its demo users.
- Presentation: protected POST /api/conversations/direct accepts only user_id, returns 201 for new and 200 for reused conversation, including ID and the counterpart's public summary. Self-chat is 400, missing user 404, missing auth 401.
- Verified with 12 concurrent A↔B HTTP requests on PostgreSQL: one new conversation, all others reuse its ID. Seed reuse, another A↔C conversation, send/list integration and invalid/unauthorized requests passed. Race detector passed.
- Explained in Vietnamese: the unordered pair key prevents simultaneous creation from both directions. All application creation paths use it. Manual SQL bypassing those paths can still create duplicate pairs because no database pair registry/unique constraint was added.

## X03: Documentation and verification

U11 supersedes the former demo-only search/creation exclusion in AGENTS, DECISIONS, CONTRACTS and MVP_PLAN. README files describe the two new routes. `go test -race -count=1 ./...`, `go vet ./...` and `git diff --check` passed. PostgreSQL integration tests use private schemas in chat_app_test and clean them up. B30 still needs the mobile-only versus mobile-and-Web platform answer before selecting the WebSocket handshake.
