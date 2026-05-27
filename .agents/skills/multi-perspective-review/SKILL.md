---
name: multi-perspective-review
description: Use to run a complete cross-cutting code review on the current change — dispatches nine reviewer sub-agents (security, performance, coding-conventions, maintainability, testability, reliability, scalability, compliance, refactoring) in parallel, aggregates their findings into a prioritized report, AND saves the report + actionable checklist as a dated history file under `doc/review/`. Triggers when the user asks for a "full review", "multi-angle review", "release-readiness check", or "review this branch / PR / diff".
---

# Multi-perspective code review

Run all nine reviewer agents in parallel against the current change, merge their reports into one ordered list ranked by severity, and persist the result as a dated review log under `doc/review/`. This is the integration point — each individual reviewer is opinionated and narrow; this skill is what makes them usable together AND what makes review findings durable instead of ephemeral chat output.

This skill complements `.Codex/agents/go-reviewer.md` (which is repo-specific Go idioms and runs separately). Don't dispatch `go-reviewer` from this skill — it overlaps with `coding-conventions-reviewer` enough that running both adds noise. The user can run `go-reviewer` separately when they want it.

## When to use

Use when the user asks for any of:
- "full review" / "multi-angle review" / "全観点レビュー" / "総点検"
- "release readiness" / "ship-ready check"
- "review this branch / PR / diff"
- "I'm about to merge — sanity check"

If the user names ONE perspective ("security review please", "perf review only"), do NOT use this skill — dispatch only that one agent directly. This skill is for when they want all of them.

## Reviewer roster (9 agents, parallel)

| Agent (`subagent_type`) | What it owns |
|---|---|
| `security-reviewer` | AuthN/Z, injection, secrets, transport, prompt injection |
| `performance-reviewer` | N+1, allocations, blocking I/O, fan-out cost |
| `coding-conventions-reviewer` | Cross-cutting style: naming, layout, imports, comments |
| `maintainability-reviewer` | Function/file size, complexity, duplication, dead code (diagnoses *symptoms*) |
| `testability-reviewer` | Seams, DI, TDD compliance, behavior coverage |
| `reliability-reviewer` | Error propagation, timeouts, retries, panics, shutdown |
| `scalability-reviewer` | Shared state, single-writer, pool sizing, fan-out at N |
| `compliance-reviewer` | PII, audit trail, retention, secret handling |
| `refactoring-reviewer` | Concrete commit-sized transformations with before/after sketches (prescribes *treatments*) |

`maintainability-reviewer` and `refactoring-reviewer` are paired — the first names the smell ("function too long, leaky abstraction"), the second names the move ("extract X from lines 22-34, no test changes"). Run both. They disagree usefully.

Each agent's full prompt lives at `.Codex/agents/<name>.md`.

## Step 1: Identify the change set

Before dispatching, capture *what* is being reviewed and pass that to every agent. Agents are stateless and don't see your conversation context; the brief is what they get.

Decide the scope based on what the user said:

| User said | Scope |
|---|---|
| "review this branch" / "the current branch" | `git diff <main-branch>...HEAD` (find main-branch via `git status` or default to `master`/`main`) |
| "review this PR" / `gh pr ...` | `gh pr diff <num>` content |
| "review my staged changes" / "what I have right now" | `git diff --staged` AND `git diff` (unstaged) AND `git status` (untracked) |
| "review modules/auth/src/service/login.go" | the path(s) the user named |
| Nothing specific | Ask before dispatching — wasting 9 agents on the wrong scope is the worst failure mode. |

Capture the diff once via `Bash`. Don't re-fetch it inside each agent; pass it as text in the prompt.

## Step 2: Build a shared brief

Construct a single brief that every reviewer receives. The brief is:

```
WHAT TO REVIEW
==============

Scope: <one-line describing the change set, e.g. "current branch vs master, 4 files">

Files changed:
<list from `git diff --name-status`>

Diff:
<the actual diff body — pasted, not referenced by path>

Context:
<any user-supplied context: "this is a bug fix for X" or "preparing to deploy">
```

Keep the brief under ~50KB. If the diff is bigger, split by directory and dispatch per-directory parallel runs (most diffs aren't this big).

If the user only mentioned files (no `git diff` available), include `Read`-pulled file contents instead.

## Step 3: Dispatch all nine in parallel — one message, nine tool calls

This is the whole point of the skill. Use ONE message containing nine `Agent` tool calls. The runtime executes them concurrently; sequential dispatch defeats the design.

For each call:
- `subagent_type`: the reviewer name (`security-reviewer`, etc.)
- `description`: one short line, e.g. `"Security review of branch"`
- `prompt`: the shared brief above + a one-line reminder of the agent's specific lens, e.g. `"Apply your security checklist."`. Don't restate the agent's full prompt — they already have it.

Each agent reads the brief, applies its own prompt, and returns its own report format. You do not edit those reports — preserve them verbatim in the aggregated output.

## Step 4: Aggregate and prioritize

Receive the nine reports. Build a single deliverable:

### 4a. Top-line verdict

Aggregate the nine verdicts into one. Mapping:

| Aggregated | When |
|---|---|
| `block-merge` | Any reviewer returned a hard-block verdict (security `block-merge`, reliability `dangerous`, compliance `blocking`) |
| `needs-changes` | Multiple reviewers flagged blocking findings, OR one reviewer flagged a serious-but-not-block-merge issue |
| `minor-cleanup` | Only style / drift / non-blocking findings across reviewers |
| `ship-it` | All reviewers came back clean |

The refactoring reviewer's verdicts (`ship-as-is` / `small-improvements` / `substantial-refactor-warranted`) feed the aggregate but never escalate it on their own — refactoring proposals are advisory, not blocking. A `substantial-refactor-warranted` verdict from refactoring alongside otherwise-clean reports yields aggregated `minor-cleanup`, not `needs-changes`.

### 4b. Severity-ordered finding list

Merge per-agent findings into one list, ordered:

1. **Block-merge findings** (security, reliability, compliance hard-blocks)
2. **Production-impact findings** (performance hot-path, scalability replica-coherence, reliability page-worthy)
3. **Quality findings** (maintainability, testability, coding-conventions structural drift)
4. **Refactoring opportunities** (refactoring-reviewer's commit-sized proposals — advisory, ordered by impact ÷ cost)
5. **Drift / nits** (style, minor cleanup)

For each finding, format:
```
[<severity>] [<reviewer>] <path:line> — <finding> — <fix>
```

Deduplicate: if two reviewers flagged the same `path:line` for related reasons, collapse with both reviewer tags. Common pairs to expect: `[maintainability + refactoring]` (symptom + transformation), `[testability + refactoring]` (extracted function deserves a unit test), `[coding-conventions + refactoring]` (rename to match repo).

### 4c. Cross-reviewer patterns

When 3+ reviewers flag findings that share a theme (e.g. "this service mixes responsibilities" surfaces from coding-conventions + maintainability + testability simultaneously), call it out as a *pattern*, separate from individual findings. The pattern is more actionable than the parts.

### 4d. Reports section (per-reviewer, verbatim)

After the aggregated view, include each agent's raw report under a heading. The user may want to drill into a specific reviewer's reasoning without re-running it.

## Step 5: Persist the review log to `doc/review/`

Reviews are durable history, not chat artifacts. Always write the aggregated report to a dated file before echoing the summary to chat. The file is what future contributors grep when they want to know "why is this code shaped this way" or "what did we decide about X last quarter".

### File path

```
doc/review/<YYYY-MM-DD>-<scope-slug>.md
```

- **Date** — today's date (UTC), `YYYY-MM-DD`.
- **Scope slug** — kebab-case theme of the change. Examples:
  - `doc-refresh` — for /refresh-docs follow-ups
  - `auth-jwt-bearer` — for a feature commit on auth
  - `audit-handler-impl` — for code-side work on audit
  - `pr-42` — when the scope is a specific GitHub PR
- **Disambiguation** — if the path already exists (multiple reviews same day), append the next integer: `2026-05-08-doc-refresh-2.md`.

If `doc/review/` does not yet exist, create it with `mkdir -p doc/review`.

### File template

Write this exact shape — an actionable checklist FIRST so the user can act on it without reading the full record, then the analysis, then the verbatim per-reviewer reports for archival.

```markdown
# Multi-perspective review — <YYYY-MM-DD> <short-scope>

| | |
|---|---|
| **日付** | <YYYY-MM-DD> |
| **対象ブランチ / PR** | <branch / PR# / specific paths> |
| **発火元** | </multi-perspective-review or which skill chain> |
| **変更概要** | <2-3 lines: what changed and why> |
| **総合判定** | <block-merge / needs-changes / minor-cleanup / ship-it> |

---

## 1. 対応アイテム一覧 (チェックリスト)

### 1.1 doc-only / scope-included — この変更内で対応する

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| A | <one-line> | <reviewer(s)> | 高 / 中 / 低 |
| B | ... | ... | ... |

### 1.2 code-side / 別 PR でフォローアップ

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| H | <one-line> | <reviewer(s)> | ... |

### 1.3 Refactoring proposals (advisory — open follow-up PR)

`refactoring-reviewer` の提案。各項目は単一 commit サイズの transformation。Cost / Test impact 込みで一覧化。

| # | Refactoring | path:line | Cost | Test impact |
|---|---|---|---|---|
| R1 | <name> | <path:line> | S / M | none / ... |

---

## 2. 横断パターン (cross-reviewer)

- <theme> — flagged by <reviewers> across <paths>

---

## 3. Top findings (severity-ordered)

```
[block-merge]      [security]  <path:line> — <finding> — <fix>
[production-impact][reliability] <path:line> — ...
...
```

---

## 4. Per-reviewer reports (verbatim)

9 sub-agent (security / performance / coding-conventions / maintainability / testability / reliability / scalability / compliance / refactoring) を並列 dispatch した結果。各 agent の verbatim 報告を保存する。

### 4.1 Security — `<verdict>`

> <verbatim report>

### 4.2 Performance — `<verdict>`

> ...

### 4.3 Coding conventions — `<verdict>`

> ...

### 4.4 Maintainability — `<verdict>`

> ...

### 4.5 Testability — `<verdict>`

> ...

### 4.6 Reliability — `<verdict>`

> ...

### 4.7 Scalability — `<verdict>`

> ...

### 4.8 Compliance — `<verdict>`

> ...

### 4.9 Refactoring — `<verdict>`

> ...
```

The actionable checklist (§1) is the load-bearing section — keep it scannable. Group items in §1.1 for "the user can act on this in this conversation/PR" and §1.2 for "needs separate follow-up". Place `refactoring-reviewer`'s proposals in §1.3 — they are advisory and follow a different format (cost / test-impact columns).

### What NOT to persist (PII / secret hygiene)

`doc/review/` is committed to git and surfaces in `git log` / `git blame` indefinitely. Before writing the file, scan the brief AND the agent reports for:

- Real email addresses (use `[email]` placeholder)
- Phone numbers, postal addresses, government IDs (`[pii]`)
- Credentials: API keys, OAuth tokens, JWT-shaped strings, AWS access keys (`AKIA*` / `ASIA*`), private-key headers (`-----BEGIN ...PRIVATE KEY-----`), passwords, DB connection strings with embedded credentials (`[redacted]`)
- Internal-only endpoint URLs (replace host with `[internal]`)
- Customer or user data appearing in test fixtures

If found, replace with the placeholder before writing. The original review brief and agent reports stay in conversation context (ephemeral) — only the redacted form lands in `doc/review/`. If the agent reports themselves quote something sensitive, edit the verbatim block to redact before persisting (this is the one place where editing agent output is sanctioned).

When in doubt: keep it out of the file, surface it only in the chat summary, and link to a non-public artifact (Slack thread, Linear issue) for follow-up.

### Retention

- `doc/review/` is append-only by convention; old files are historical record. Do NOT amend or delete past entries during a normal review run — they describe the project at a point in time.
- A periodic cleanup pass (every ~6 months) MAY prune review files older than 12 months provided the conclusions have been folded into `.Codex/rules/*` or per-module `AGENTS.md`. That pruning is a separate, explicitly-requested task — not something this skill does.

### When the file write fails

If `Write` fails (permission, disk full, etc.), surface the error in the chat reply alongside the summary. Don't silently drop the persistence. The chat output is the fallback record; tell the user the file path you tried so they can re-create the file from the chat content if they want.

## Step 6: Echo a condensed summary to chat

After writing the file, output a condensed deliverable to stdout — just enough that the user can decide what to act on without re-opening the file.

Structure:

```markdown
# Multi-perspective review

**Verdict:** <block-merge | needs-changes | minor-cleanup | ship-it>
**Scope:** <one line>
**Full record:** `doc/review/<YYYY-MM-DD>-<scope-slug>.md`

## 対応アイテム

### doc-only / scope-included
- A: ...
- B: ...

### code-side / 別 PR
- H: ...

### Refactoring proposals (advisory)
- R1: <one-line> (<path:line>, <cost>)

## Top findings (severity-ordered, top 10)

[block-merge] [security] modules/auth/src/service/login.go:42 — ...
[production-impact] [reliability] modules/queue/src/route/grpc/server.go:88 — ...
...

## Cross-reviewer patterns

- <pattern> — flagged by <reviewers> across <paths>
```

Do NOT paste all 9 verbatim reports into the chat — those live in the file. Chat output is the index; the file is the record.

## Failure modes

- **A reviewer agent returns garbled / off-topic output.** Note it in the aggregated report ("security-reviewer returned a malformed report — consider re-running") rather than silently dropping. The user should know coverage was incomplete.
- **A reviewer agent times out / errors.** Surface the error. Don't paper over it.
- **The brief was too small (e.g. user pasted a one-liner).** Reviewers will produce thin reports; that's expected. The skill's value comes from the *parallel* dispatch, not from forcing depth that isn't there.
- **The user asked for one perspective.** This skill is wrong; dispatch one agent directly. Bail out of the skill early.

## When NOT to use this skill

- Single-perspective requests (use the named agent directly).
- Tiny one-line typo fixes (the agents will spend more tokens producing the report than the change is worth).
- Pre-commit / pre-push automation (the pre-commit hook runs `make fmt && make vet && make lint`; `make test` runs pre-push — those are mechanical and cheap. This skill is for thoughtful human-in-the-loop review).
- When the user is mid-implementation and just wants a sanity check on one file — dispatch `coding-conventions-reviewer` or `maintainability-reviewer` alone.

## Reference: agent files

- `.Codex/agents/security-reviewer.md`
- `.Codex/agents/performance-reviewer.md`
- `.Codex/agents/coding-conventions-reviewer.md`
- `.Codex/agents/maintainability-reviewer.md`
- `.Codex/agents/testability-reviewer.md`
- `.Codex/agents/reliability-reviewer.md`
- `.Codex/agents/scalability-reviewer.md`
- `.Codex/agents/compliance-reviewer.md`
- `.Codex/agents/refactoring-reviewer.md`

If any of these files don't exist, the skill cannot run — flag the missing agent and stop.

## Reference: history directory

- `doc/review/` — review log archive. Each file is a dated snapshot of one `/multi-perspective-review` run. Earliest example: `doc/review/2026-05-08-doc-refresh-multi-perspective.md` (the run that established the format).
