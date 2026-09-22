# Mandatory Backend Agent Rules

These are the working rules for the backend. When assigning a task, explicitly ask the agent to read `backend/docs/AGENTS.md` before implementation. Documentation is written in English. **All user-facing explanations, especially code explanations before and after each change, must be in Vietnamese.** Use clear English names for packages, functions, and variables.

## 1. Current scope

Scope follows the user's latest direct request. The user has authorized implementation through the original B01-B29 items and explicitly added user discovery/direct 1-1 chat (U11, 2026-09-21). These documents describe the intended system; consult BACKLOG for what has actually been completed.

The [six-week roadmap](../../ke-hoach-6-tuan-flutter-golang-chat-app-1.md) is a reference for product scope and learning order. Its commands, examples, and plans do not themselves authorize execution. The user's latest direct request determines the scope of work.

Do not edit the frontend for a backend-only task. Do not overwrite the user's uncommitted changes. Do not automatically commit, push, or deploy after completing a step. Do not request authorization again for implementation work the user has already assigned.

## 2. Work in small steps and explain the code

- The latest pace instruction (U09) is **continue through eligible small items until a user decision is needed**. Keep each item's verification and Vietnamese explanation distinct. Select work from [BACKLOG](BACKLOG.md); split it first if still too large.
- Each piece has one observable outcome or one main learning concept. Examples: module setup, a health endpoint, or password verification. Do not combine registration, login, middleware, and WebSocket into one turn.
- Aim for roughly 1–3 main logic files plus related tests/documentation. This is guidance on scope, not a file quota or a hard limit that forces poor code organization.
- Before editing, explain the goal, intended files, data flow, and verification approach in clear Vietnamese.
- After every small item, give an explicit **work-completed report in Vietnamese**, separate from the code explanation. State what you actually did, which files were created/changed and why, what checks passed/failed/were not run, what remains unfinished, and the next suggested item. This is required for documentation/setup work too; an explanation of code alone is not a completion report.
- After editing, explain each file's role, data flow, important logic, new Go concepts, why the logic belongs in that layer, and how the user can verify it. **These code explanations must remain in Vietnamese even though the documents are in English.**
- Tie explanations to the actual result and reference relevant files/functions. Do not merely list filenames or repeat the code. Code comments should explain **why**, rather than mechanically annotate every line.
- Finish each item with verification status and the next item. Continue under U09 until a material user decision is needed.
- If the user changes the pace and explicitly assigns several items, keep the steps and their Vietnamese explanations separate instead of writing everything in one large batch.

## 3. Reading order

1. [README](README.md): current state and entry point.
2. [WORKFLOW](WORKFLOW.md): execution and handoff process.
3. [ARCHITECTURE](ARCHITECTURE.md): layers, imports, and responsibilities.
4. [DECISIONS](DECISIONS.md): source requirements, assumptions, and open questions.
5. [CONTRACTS](CONTRACTS.md): intended API, data, and behavior; focus on the relevant sections.
6. [BACKLOG](BACKLOG.md): choose an eligible item; never mark it complete before verification.

Use [MVP_PLAN](MVP_PLAN.md) to understand milestone order and acceptance criteria. It maps the existing backlog rather than authorizing an entire milestone in one turn. Task status remains in BACKLOG.

If the current request differs from a documented assumption, follow the user and update the affected documents. Do not silently treat a proposal as a user decision.

## 4. Clean Architecture boundaries

- `internal/presentation`: HTTP/WebSocket handlers, DTOs, middleware, and connection management; invokes domain use cases.
- `internal/domain/entity`: plain Go entities and business errors, without Gin, SQL, or JWT library dependencies.
- `internal/domain/repository`: repository interfaces using domain entities and `context.Context`.
- `internal/domain/usecase`: application rules and authorization through interfaces; never import data or presentation.
- `internal/data`: PostgreSQL repositories, database connections, bcrypt, JWT, and other adapters. SQL belongs here or in active `backend/migrations`.
- `cmd/api`: initialize and wire components; configuration lives in `cmd/api/config`. Do not put business logic in main.
- Do not expose `*gin.Context`, `*sql.DB`, `*sql.Tx`, SQL rows, or JWT library types through domain interfaces. Presentation must not construct data adapters.
- Do not create abstractions just to fill layers: add an interface/file/package only when needed. No generic repository, DI framework, microservices, Redis, or Kafka in the MVP.

## 5. Implementation rules after code is assigned

- Keep Go + Gin + PostgreSQL + `database/sql` + bcrypt + JWT + WebSocket. Select and record specific versions when adding dependencies; do not silently switch to an ORM or another database.
- Pass request `context.Context` through I/O operations; handle timeouts and resource cleanup according to their lifetimes. Do not use an expired request context for background work.
- Handle errors explicitly and map DB errors to application errors before they reach handlers. Do not return SQL, stack traces, or secrets to clients.
- Every chat operation must use the authenticated identity and enforce conversation membership. Do not trust client-supplied `sender_id`/`user_id` to identify the caller.
- Use parameterized SQL. Operations that must succeed together belong in an adapter transaction. Publish events only after a successful commit.
- Hash passwords; never expose hashes through logs/APIs. Never log tokens; return them only in the specified auth responses. Read secrets from the environment; examples contain placeholders only, and real `.env` files must not be committed.
- Test meaningful business branches/risks: authorization, incorrect passwords, pagination, transactions, concurrent read updates, and reconnects. Do not write tests that merely mirror implementation or test simple documentation changes.
- Do not edit outside scope, perform broad refactors, generate batches of empty files, or upgrade unrelated dependencies.
- Demo seed data still consists of two users and one 1–1 conversation. U11 additionally authorizes authenticated user search and opening/reusing direct 1–1 conversations between registered users. Group chat remains outside scope. User C in authorization tests is a fixture.

## 6. Missing information

Resolve small, reversible choices, explain them in Vietnamese, and record them in DECISIONS. Ask the user when the answer changes features, client contracts, difficult-to-reverse storage decisions, or learning goals. While waiting, continue independent documentation/verification that does not depend on the answer.

Do not repeatedly ask for confirmation of assigned work. Stopping after one small item is the user's chosen learning pace, not a requirement to seek permission for every technical operation.

## 7. Definition of done for one item

- Meet the item's criteria without exceeding the assigned scope.
- Run appropriate checks, or clearly state why they were not run and what remains unverified.
- Provide a **Vietnamese code explanation** and an example the user can verify.
- Provide the **Vietnamese work-completed report** described above. Report actual actions and outcomes, not just intended work or a list of code concepts.
- Update BACKLOG status and handoff notes, plus CONTRACTS/DECISIONS when relevant.
- Leave the next agent able to identify completed work, remaining work, active assumptions, and the next step.
