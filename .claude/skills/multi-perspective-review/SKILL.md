---
name: multi-perspective-review
description: Use to run a complete cross-cutting code review on the current change — dispatches eight reviewer sub-agents (security, performance, coding-conventions, maintainability, testability, reliability, scalability, compliance) in parallel and aggregates their findings into a single prioritized report. Triggers when the user asks for a "full review", "multi-angle review", "release-readiness check", or "review this branch / PR / diff".
---

# Multi-perspective code review

Run all eight reviewer agents in parallel against the current change, then merge their reports into one ordered list ranked by severity. This is the integration point — each individual reviewer is opinionated and narrow; this skill is what makes them usable together.

This skill complements `.claude/agents/go-reviewer.md` (which is repo-specific Go idioms and runs separately). Don't dispatch `go-reviewer` from this skill — it overlaps with `coding-conventions-reviewer` enough that running both adds noise. The user can run `go-reviewer` separately when they want it.

## When to use

Use when the user asks for any of:
- "full review" / "multi-angle review" / "全観点レビュー" / "総点検"
- "release readiness" / "ship-ready check"
- "review this branch / PR / diff"
- "I'm about to merge — sanity check"

If the user names ONE perspective ("security review please", "perf review only"), do NOT use this skill — dispatch only that one agent directly. This skill is for when they want all of them.

## Reviewer roster (8 agents, parallel)

| Agent (`subagent_type`) | What it owns |
|---|---|
| `security-reviewer` | AuthN/Z, injection, secrets, transport, prompt injection |
| `performance-reviewer` | N+1, allocations, blocking I/O, fan-out cost |
| `coding-conventions-reviewer` | Cross-cutting style: naming, layout, imports, comments |
| `maintainability-reviewer` | Function/file size, complexity, duplication, dead code |
| `testability-reviewer` | Seams, DI, TDD compliance, behavior coverage |
| `reliability-reviewer` | Error propagation, timeouts, retries, panics, shutdown |
| `scalability-reviewer` | Shared state, single-writer, pool sizing, fan-out at N |
| `compliance-reviewer` | PII, audit trail, retention, secret handling |

Each agent's full prompt lives at `.claude/agents/<name>.md`.

## Step 1: Identify the change set

Before dispatching, capture *what* is being reviewed and pass that to every agent. Agents are stateless and don't see your conversation context; the brief is what they get.

Decide the scope based on what the user said:

| User said | Scope |
|---|---|
| "review this branch" / "the current branch" | `git diff <main-branch>...HEAD` (find main-branch via `git status` or default to `master`/`main`) |
| "review this PR" / `gh pr ...` | `gh pr diff <num>` content |
| "review my staged changes" / "what I have right now" | `git diff --staged` AND `git diff` (unstaged) AND `git status` (untracked) |
| "review modules/auth/src/service/login.go" | the path(s) the user named |
| Nothing specific | Ask before dispatching — wasting 8 agents on the wrong scope is the worst failure mode. |

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

## Step 3: Dispatch all eight in parallel — one message, eight tool calls

This is the whole point of the skill. Use ONE message containing eight `Agent` tool calls. The runtime executes them concurrently; sequential dispatch defeats the design.

For each call:
- `subagent_type`: the reviewer name (`security-reviewer`, etc.)
- `description`: one short line, e.g. `"Security review of branch"`
- `prompt`: the shared brief above + a one-line reminder of the agent's specific lens, e.g. `"Apply your security checklist."`. Don't restate the agent's full prompt — they already have it.

Each agent reads the brief, applies its own prompt, and returns its own report format. You do not edit those reports — preserve them verbatim in the aggregated output.

## Step 4: Aggregate and prioritize

Receive the eight reports. Build a single deliverable:

### 4a. Top-line verdict

Aggregate the eight verdicts into one. Mapping:

| Aggregated | When |
|---|---|
| `block-merge` | Any reviewer returned a hard-block verdict (security `block-merge`, reliability `dangerous`, compliance `blocking`) |
| `needs-changes` | Multiple reviewers flagged blocking findings, OR one reviewer flagged a serious-but-not-block-merge issue |
| `minor-cleanup` | Only style / drift / non-blocking findings across reviewers |
| `ship-it` | All reviewers came back clean |

### 4b. Severity-ordered finding list

Merge per-agent findings into one list, ordered:

1. **Block-merge findings** (security, reliability, compliance hard-blocks)
2. **Production-impact findings** (performance hot-path, scalability replica-coherence, reliability page-worthy)
3. **Quality findings** (maintainability, testability, coding-conventions structural drift)
4. **Drift / nits** (style, minor cleanup)

For each finding, format:
```
[<severity>] [<reviewer>] <path:line> — <finding> — <fix>
```

Deduplicate: if two reviewers flagged the same `path:line` for related reasons, collapse with both reviewer tags.

### 4c. Cross-reviewer patterns

When 3+ reviewers flag findings that share a theme (e.g. "this service mixes responsibilities" surfaces from coding-conventions + maintainability + testability simultaneously), call it out as a *pattern*, separate from individual findings. The pattern is more actionable than the parts.

### 4d. Reports section (per-reviewer, verbatim)

After the aggregated view, include each agent's raw report under a heading. The user may want to drill into a specific reviewer's reasoning without re-running it.

## Step 5: Output

Write the aggregated report to stdout (chat). Do NOT write a file unless the user explicitly asks. Reviews are conversation artifacts; they go stale and shouldn't pollute the repo.

Structure:

```markdown
# Multi-perspective review

**Verdict:** <block-merge | needs-changes | minor-cleanup | ship-it>
**Scope:** <one line>

## Top findings (severity-ordered)

[block-merge] [security] modules/auth/src/service/login.go:42 — ...
[production-impact] [reliability] modules/queue/src/route/grpc/server.go:88 — ...
...

## Cross-reviewer patterns

- <pattern> — flagged by <reviewers> across <paths>

## Per-reviewer reports

### Security
<verbatim>

### Performance
<verbatim>

... (all 8)
```

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

- `.claude/agents/security-reviewer.md`
- `.claude/agents/performance-reviewer.md`
- `.claude/agents/coding-conventions-reviewer.md`
- `.claude/agents/maintainability-reviewer.md`
- `.claude/agents/testability-reviewer.md`
- `.claude/agents/reliability-reviewer.md`
- `.claude/agents/scalability-reviewer.md`
- `.claude/agents/compliance-reviewer.md`

If any of these files don't exist, the skill cannot run — flag the missing agent and stop.
