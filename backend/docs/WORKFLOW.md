# Workflow for Small Steps

**Current authorization (2026-09-21):** the user asked to continue until a question needs their decision. This supersedes one-item-per-turn stopping instructions below; retain separate implementation, checks, and Vietnamese explanations for each item.

Use this together with [AGENTS.md](AGENTS.md). The goal is for the user to understand the code being built and for the next agent to continue from the correct point. **Keep these documents in English, but give all user-facing explanations and code walkthroughs in Vietnamese.**

## 1. One work turn

| Step | Agent action | What the user receives |
|---|---|---|
| Read | Review the latest request, rules, BACKLOG, handoff notes, and existing changes | Correct scope without overwriting previous work |
| Select | Choose one eligible item; split it if it contains several large goals | One concise goal with verification criteria |
| Explain first | Explain the behavior, data flow, intended files/layers, and new concepts in Vietnamese | An understanding of why the change is needed |
| Implement | Change only what the item requires; keep the system runnable/buildable at appropriate milestones | A change that can be read and tried |
| Verify | Run checks appropriate to the changed behavior and inspect the final diff | Concrete evidence or clearly stated verification limits |
| Report completed work | State completed actions, changed files, verification results, unfinished work, and the next item in Vietnamese | A clear account of what was actually done in this small step |
| Explain afterward | Reference files/functions and explain logic and verification in Vietnamese, not just filenames | Enough understanding to describe how the code runs |
| Hand off | Update BACKLOG and the notes below; suggest the next item | A clear continuation point for the next agent |

**The user chose to end each turn after one item and its Vietnamese explanation, allowing time to read.** Wait for the next request rather than automatically continuing through the backlog. If the user explicitly assigns multiple items in a later turn, apply this cycle separately to each item. Do not ask again for authorization for operations already covered by the assigned task.

## 2. What counts as small enough?

Small examples: add HTTP port configuration; add `/health`; implement the login use case with a fake repository; connect login to HTTP once its dependencies exist.

Oversized examples: "implement all authentication," "build the entire repository layer," or "finish week 2." Split these into ordered, independently understandable sub-items. Do not generate the whole architecture merely to match a diagram.

An item may touch several layers when it implements one simple behavior. Do not force one turn per layer if that produces empty files or meaningless code. Test important branches with the change itself instead of postponing all tests until the end of the roadmap.

## 3. Required completion report and code explanation

After every small item, provide both a work-completed report and a code explanation **in Vietnamese**. An explanation alone is not enough. For documentation/setup-only items, explain the changed document/configuration and its purpose rather than inventing application behavior.

### Work-completed report

Include these concrete points in the final response, even if they were mentioned in progress updates:

1. **Completed work:** the item ID and actions actually performed.
2. **Changed files:** what was created/modified and the purpose of each relevant change.
3. **Verification results:** the checks actually run and their outcomes; distinguish passed, failed, and not run.
4. **Remaining scope:** incomplete/unverified behavior and the boundary of this step.
5. **Next item:** suggest one eligible small item, then stop rather than implementing it automatically.

Keep this report concise and specific. Do not claim completion merely because files exist.

### Code explanation

Deliver every part below **in Vietnamese**:

1. **What changed:** the behavior or foundation just added and its connection to the chat app.
2. **Where the code lives:** the responsibility of each relevant file/function and its layer.
3. **How the code runs:** input-to-output flow using a short data example; include at least one important error branch when applicable.
4. **Go concepts:** explain relevant new concepts such as structs, methods, interfaces, pointers, errors, or context.
5. **Why this approach:** give a concrete reason related to authorization, simplicity, testing, or dependency direction.
6. **How to verify:** actual commands/requests, expected results, and results the agent observed. Claim a command was run only if it was actually run.
7. **Where work stops:** unfinished work, open assumptions, and the next item. An optional learning question is welcome, but an answer is not a prerequisite for continuing.

A small change does not need a long lecture. Explain enough for the user to understand the lines that determine behavior. Expand the relevant section when the user asks for more detail.

## 4. Choose the appropriate verification level

| Change type | Appropriate checks |
|---|---|
| Documentation only | Internal links, terminology, status, and consistency; do not install tools or generate tests |
| Simple configuration/handler | Formatting/build; meaningful valid inputs and configuration/request errors |
| Use case | Tests with small fakes focused on business rules, authorization, and errors; no real DB required |
| SQL/repository/migration | Integration tests against a separate PostgreSQL test DB; constraints, rollback, ordering/pagination where relevant |
| WebSocket/goroutines | Lifecycle, disconnects, event authorization, slow clients; race detector when supported |
| Complete MVP flow | A/B chat, unauthorized user C, reconnects, and read/unread state |

Once the module exists, common baseline commands from `backend/` are `go test ./...` and `go vet ./...`; format modified Go files. Use `go test -race ./...` when concurrency is involved and the toolchain supports it. State which commands were not run; do not run tests during the documentation stage before a module exists.

Integration tests use a separate DB and separate test data. Do not delete/reset the user's active DB. Do not initialize databases, install dependencies, or download tools during explanation-only/documentation-only work.

## 5. Handoff to another agent

BACKLOG is the source of work status; DECISIONS is the source of decisions; CONTRACTS is the source of communication contracts. Do not maintain three conflicting copies of the same status. Update the contract when changing an API/schema and describe the effect on clients.

Use this template in the handoff log of [BACKLOG](BACKLOG.md), with one short entry per item. Keep the stored entry in English while explaining the results to the user in Vietnamese.

```text
Date / item:
Status: TODO | IN_PROGRESS | DONE | BLOCKED
Assigned request:
Changed: files and behavior
Reported to the user in Vietnamese: completed actions, outcomes, remaining work, next item
Explained in Vietnamese: main concepts/flows
Verified: commands or requests + results
Unverified / incomplete:
Related decisions:
Next item and prerequisites:
```

Do not mark an item DONE if required criteria are missing. Use IN_PROGRESS when stopping partway through and state exactly what remains. Use BLOCKED only for a condition that actually prevents the item from proceeding; independent items may still be addressed when covered by the user's request.

If multiple agents are used, assign independent work with explicit file ownership and have one agent integrate and verify the result. Do not let two agents edit the same file. Delegation must not expand one small item into several features.

## 6. Initial state

On 2026-09-18, only preparation documents and the backend skeleton exist. No feature implementation, server run, or application test result is available. B01 is the first implementation item the user can assign in a later turn.
