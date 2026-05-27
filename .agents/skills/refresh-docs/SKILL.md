---
name: refresh-docs
description: Use when the user asks to update / refresh / sync the project's README.md and AGENTS.md files (top-level + per-module). Locates every doc with `find`, classifies each by path → concern, and rewrites stale sections so the docs match current code (handlers, schema, Make targets, design-doc pointers).
---

# Refresh project docs (README.md / AGENTS.md)

This repo carries documentation at four levels:

| Level | Files | Concern |
|---|---|---|
| Repo root | `README.md`, `AGENTS.md` | Project intro (humans) + agent-facing master index (cross-cutting standards). |
| Per-module | `modules/<svc>/README.md`, `modules/<svc>/AGENTS.md` | Service intro + module-local agent guidance. `<svc>` ∈ `audit`, `auth`, `queue`, `shared`. |
| Sub-area | `deploy/k8s/README.md` | k8s layout walkthrough (cross-service composition). |
| Rules | `.Codex/rules/*.md` | NOT updated by this skill — those are owned by humans (linked from AGENTS.md). |

The skill exists because these files drift fast: a new endpoint, sqlc query, gRPC RPC, or Make target lands and the docs lag. Stale docs lie to future agents and slow new humans down.

## Scope rules

- **Update**, don't rewrite from scratch. Preserve existing structure when accurate.
- **Don't duplicate** content between top-level and nested files. Top-level AGENTS.md owns repo-wide conventions; per-module AGENTS.md owns module-local facts. If the same statement appears in both, delete it from the nested file and link to the canonical one.
- **Never touch** files under `.Codex/rules/`, `.Codex/skills/`, or `.Codex/agents/`. Those are governed separately.
- **Never touch** generated content: `infra/database/db/*.go`, `route/grpc/*.pb.go`, `atlas.sum`. README/Codex files only describe these — they don't include their content.

## Step 1: Locate the targets

Run `find` from the repo root, exactly as the user specified:

```bash
find . -maxdepth 6 -type f \( -iname "README.md" -o -iname "AGENTS.md" \) \
  -not -path "./node_modules/*" -not -path "./.git/*" -not -path "./.Codex/*" | sort
```

Expected results today (2026-05-07):

- `./README.md`
- `./AGENTS.md`
- `./deploy/k8s/README.md`
- `./modules/audit/README.md`, `./modules/audit/AGENTS.md`
- `./modules/auth/README.md`,  `./modules/auth/AGENTS.md`
- `./modules/queue/README.md`, `./modules/queue/AGENTS.md`
- `./modules/shared/README.md`,`./modules/shared/AGENTS.md`

If the list differs, that itself is a finding — either a doc was deleted (probably accidental) or a new module landed without docs (gap to fill).

## Step 2: Classify each hit

Map each path to the per-file concern below. Read the file, then read the listed sources of truth, then update only the sections that are actually stale.

### `./README.md` — project intro (humans)

**Concern**: First-time visitor's orientation. NOT a substitute for AGENTS.md.

**Sources of truth**:
- `compose.yml` (which services exist locally)
- `Makefile` `MODS`, `K8S_IMAGES`, top-level help target — for what someone can actually run
- `modules/*/README.md` — for the one-liner per module

**Keep it short**: what the project is, which services are inside, where to start (AGENTS.md for agents, per-module READMEs for service-level details).

### `./AGENTS.md` — agent-facing master index

**Concern**: Cross-cutting facts that apply to every module. The single load-point pre-loaded into agent context.

**Sources of truth** (read these and reconcile section-by-section):
- `Makefile` (Common Commands section must match real targets)
- `modules/go.work` and each `modules/*/src/go.mod` (Go version, module names)
- `compose.yml` (service list, ports)
- `.golangci.yml` (lint rules summary in §15 of coding-standards.md)
- `modules/*/src/route/...` (HTTP vs gRPC per service)
- `.devcontainer/Dockerfile` (toolchain versions)

**Sections that drift fastest**:
- Repository Layout (when a new module lands)
- Toolchain (when versions bump)
- Common Commands (when Makefile targets change)
- Conventions Worth Knowing (when a new pattern is established — e.g. the gRPC consumer pattern)

The detailed rules live in `.Codex/rules/coding-standards.md`, `testing.md`, `kubernetes-conventions.md` — keep AGENTS.md as a high-level index that references them, not a duplicate.

### `./deploy/k8s/README.md` — k8s walkthrough

**Concern**: Layout, namespace mapping, image list, kind workflow.

**Sources of truth**:
- `deploy/k8s/dev/kustomization.yaml`
- `modules/*/deploy/k8s/{base,overlays/dev}/`
- `K8S_IMAGES` in `Makefile`
- `.Codex/rules/kubernetes-conventions.md` — but this README must complement, not duplicate, the rules file.

### `./modules/<svc>/README.md` — service intro

**Concern**: What this service is, current implementation status, how to run it locally, where the design lives.

**Sources of truth** (per service):
- `modules/<svc>/docs/system-design.md` — link from the README, summarize Phase status
- `modules/<svc>/src/cmd/*/main.go` — entry points
- `modules/<svc>/src/route/` — what's actually exposed
- `Makefile` — which `make <svc>-*` targets exist (note: `queue` has no top-level meta target; it ships inside `make audit`)

**Required sections** (template — keep ordering):

```markdown
# <svc>

<one-paragraph what-it-is + link to docs/system-design.md>

## 状況 / Status

| 項目 | 状態 |
|---|---|
| <feature> | ✅ / ⏳ Phase X.Y |

## ディレクトリ / Layout

<tree showing the canonical layout for this service>

## 開発 / Development

<the make targets that actually exist for this service>

## 関連ドキュメント / Related

- 設計の一次資料 — docs/system-design.md
- リポジトリ規約 — ../../.Codex/rules/coding-standards.md (+ testing.md, kubernetes-conventions.md)
- ルートの開発手順 — ../../AGENTS.md
```

### `./modules/<svc>/AGENTS.md` — module-local agent guidance

**Concern**: Things an agent needs that are TRUE FOR THIS MODULE ONLY. Don't repeat the top-level AGENTS.md's cross-cutting rules.

**Required sections** (lean — under ~80 lines):

```markdown
# AGENTS.md (modules/<svc>)

This file extends the repo-root AGENTS.md with guidance specific to the `<svc>` module. The repo-root file remains the source of truth for cross-cutting conventions.

## What this module is

<2-3 line summary; link to docs/system-design.md>

## Layout (module-local)

<only the parts that differ from a "standard" service module — e.g. audit has cmd/api + cmd/worker, queue has no DB yet, shared has no cmd>

## Where to start reading

<ordered list of 3-5 files that orient an agent: design doc, entry point, route, schema/proto>

## Module-local conventions

<anything specific: audit's append-only invariant, queue's UnimplementedQueueServer embed, auth's chi-only HTTP layer, shared's "no per-service deps" rule, etc.>

## Make targets specific to this module

<grep Makefile for `<svc>-*` and list them>

## When proto / schema changes

<for audit/queue: how to regen the .pb.go; for audit/auth: how to regen sqlc; for shared: N/A>
```

The shared module's AGENTS.md is structurally different — see the existing file for the right shape (no design doc, no proto, no migrations).

## Step 3: Verify before writing

For each file the skill is about to update:

1. **Read the current file** (Read tool).
2. **Read the listed sources of truth** for that scope.
3. **Diff in your head**: what claims in the doc are no longer true? What new facts in the code are absent from the doc?
4. **Write only the deltas** — use Edit for surgical fixes, Write only when restructuring or replacing a stub.

Don't churn working content. If a section is correct, don't touch it.

## Step 4: Sanity check

After updating:

```bash
# All claimed make targets must exist:
make help 2>/dev/null | head -40
grep -E '^[a-z].*-?(proto|sqlc|migrate|up|down):' Makefile

# All claimed file paths in docs must exist:
grep -roE 'modules/[a-z]+/[^[:space:])"]+\.(go|sql|proto|yaml|md)' README.md AGENTS.md modules/*/README.md modules/*/AGENTS.md \
  | awk -F: '{print $2}' | sort -u | while read p; do test -e "$p" || echo "MISSING: $p"; done
```

Any line in the second command's output is a broken reference to fix before reporting done.

## What this skill is NOT for

- **Authoring `.Codex/rules/*.md`** — those are owned by humans; the skill `add-error-type` and others may touch specific rules but only their narrow scope.
- **Generating system-design.md** — design docs are authored by humans. The skill links to them and quotes Phase status, not the other way around.
- **Initial scaffolding of a new module's docs** — bigger task. Use `/init` (the built-in init skill) for that, then this skill for upkeep.

## Reference: today's per-file ownership map

| File | Owner concern | Notes |
|---|---|---|
| `README.md` | Humans landing on the repo | Was empty until 2026-05-07 — keep it short, point to AGENTS.md and per-module READMEs. |
| `AGENTS.md` | Agent master index | ~100 lines. Don't bloat — link to `.Codex/rules/*.md`. |
| `deploy/k8s/README.md` | k8s walkthrough | ~450 lines, comprehensive. Mostly stable. |
| `modules/audit/README.md` | audit service intro | Was 1-line stub until 2026-05-07. |
| `modules/audit/AGENTS.md` | audit module-local agent guidance | New 2026-05-07. |
| `modules/auth/README.md` | auth service intro | Was 5-line stub until 2026-05-07. |
| `modules/auth/AGENTS.md` | auth module-local agent guidance | New 2026-05-07. |
| `modules/queue/README.md` | queue service intro | Most developed since 2026-05-07. Don't churn. |
| `modules/queue/AGENTS.md` | queue module-local agent guidance | New 2026-05-07. |
| `modules/shared/README.md` | shared library intro | New 2026-05-07. |
| `modules/shared/AGENTS.md` | shared module-local agent guidance | New 2026-05-07. |
