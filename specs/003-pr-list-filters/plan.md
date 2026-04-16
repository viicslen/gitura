# Implementation Plan: Open PR List with Filters

**Branch**: `003-pr-list-filters` | **Date**: 2026-03-31 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/003-pr-list-filters/spec.md`

## Summary

Replace the placeholder PR page with a live, filterable list of open GitHub pull requests relevant to the authenticated user. The backend will issue up to 3 parallel GitHub Search API queries (one per involvement type: author, assignee, review-requested), deduplicate by URL, and return a sorted `PRListResult`. The frontend will drive all API calls from a session-persistent filter composable; each filter change triggers a fresh Wails RPC call.

## Technical Context

**Language/Version**: Go 1.25; TypeScript 5.x (strict mode)  
**Primary Dependencies**: Wails v2.11, go-github/v67 (Search.Issues), golang.org/x/oauth2, go-keyring; Vue 3, VueUse ^14.2.1, shadcn-vue (reka-ui + radix-vue), lucide-vue-next  
**Storage**: N/A — all data fetched live from GitHub REST API; filter state in-memory only  
**Testing**: `go test` + `testify`; `httptest` for recorded HTTP fixtures; `pnpm type-check` for frontend  
**Target Platform**: Desktop (Wails WebView) — Linux, macOS, Windows  
**Project Type**: Desktop application (Wails v2 — Go backend + WebView frontend)  
**Performance Goals**: SC-001: PRs visible ≤ 3s on standard broadband; SC-002: filter change result ≤ 3s  
**Constraints**: No new frontend libraries; no client-side caching (every filter change = new API call per FR-013); GitHub Search API cap of 1000 results per query; 30 req/min rate limit (authenticated)  
**Scale/Scope**: Targets individual developer workflows; typical user: 5–200 open PRs across ≤ 20 repos

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I — Code Quality Standards

| Check | Status | Notes |
|---|---|---|
| Idiomatic Go (`gofmt`, `golint`, `go vet`) | PASS | New Go files will be gofmt'd; CI will lint |
| Every exported symbol has a doc comment | PASS | All new exported types and methods will carry doc comments |
| Cyclomatic complexity ≤ 10 | PASS | `SearchOpenPRs` fan-out logic will be split into helpers to stay under limit |
| No dead code / TODO comments | PASS | No carryover debt in scope of this feature |

### Principle II — Testing Standards

| Check | Status | Notes |
|---|---|---|
| Unit tests for all new packages | PASS | `internal/github/pr_search_test.go` required |
| Coverage ≥ 80% (90% for critical packages) | PASS | `pr_search.go` is a critical package; target 90% |
| Integration tests with recorded fixtures | PASS | `tests/integration/pr_list_test.go` with httptest fixtures |
| TDD preferred | PASS | Tests to be written alongside implementation |
| Test naming convention | PASS | `TestFunctionName_Scenario_ExpectedOutcome` |
| No flaky tests | PASS | No sleep/timing dependencies in tests |

### Principle III — User Experience Consistency

| Check | Status | Notes |
|---|---|---|
| Single design language (shadcn-vue) | PASS | All new UI elements use shadcn-vue primitives only |
| Keyboard navigation + ARIA labels | PASS | shadcn-vue primitives include ARIA by default; must add labels to filter inputs |
| Error/loading/empty states explicitly handled | PASS | All three states modelled in `PRListResult`; all three states rendered in `PRPage.vue` |
| Information hierarchy consistent with other views | PASS | Font scales, spacing, color tokens from Tailwind CSS v4 design tokens shared across app |
| Breaking changes update all consumers | PASS | No existing components consume `PRPage.vue`; it is a stub |

### Principle IV — Performance Requirements

| Check | Status | Notes |
|---|---|---|
| Initial load ≤ 2s (app shell) | PASS | PR list fetch is async; app shell loads before data |
| API calls paginated and cached | PARTIAL | Paginated: yes. Cached: NO — spec FR-013 explicitly requires a new query per filter change. This is a justified exception (user-driven live data) |
| Memory < 150 MB under normal workload | PASS | PR list items are lightweight DTOs; no large data held in memory |
| Performance regression > 10% blocks merge | PASS | No existing PR list baseline exists; new feature, new baseline |

**Caching Exception Justification**: FR-013 states "every filter change MUST issue a new GitHub API search query." Caching search results would contradict this requirement and could return stale data. The feature is inherently user-driven query-by-query. This exception is scoped to the PR list feature only and does not apply to other resources.

### Post-Design Re-check (Phase 1)

All checks still pass after data model design. The `PRListResult` union struct provides structured error/rate-limit handling, satisfying Principle III. The separate `PRListItem` DTO keeps serialization minimal, satisfying Principle IV.

## Project Structure

### Documentation (this feature)

```text
specs/003-pr-list-filters/
├── plan.md              # This file
├── research.md          # Phase 0 output — all unknowns resolved
├── data-model.md        # Phase 1 output — Go models + frontend composable
├── quickstart.md        # Phase 1 output — dev setup + implementation guide
├── contracts/
│   └── wails-rpc.md     # Phase 1 output — ListOpenPRs RPC contract
└── tasks.md             # Phase 2 output (/speckit.tasks command — NOT created here)
```

### Source Code (repository root)

```text
# Backend (Go)
internal/
├── github/
│   ├── client.go           # Existing — factory only (unchanged)
│   └── pr_search.go        # NEW — SearchOpenPRs helper
├── model/
│   └── model.go            # MODIFIED — add PRListItem, PRListFilters, PRListResult

app.go                      # MODIFIED — add ListOpenPRs Wails method

# Auto-generated (wails generate module — do not edit)
wailsjs/
├── go/main/
│   ├── App.d.ts            # REGENERATED — adds ListOpenPRs
│   └── models.ts           # REGENERATED — adds new DTO interfaces

# Frontend (Vue/TypeScript)
frontend/src/
├── composables/
│   └── usePRFilters.ts     # NEW — session-persistent filter state singleton
├── pages/
│   └── PRPage.vue          # REPLACED — full implementation (was stub)
└── components/ui/
    ├── checkbox/            # NEW — shadcn-vue checkbox primitive
    ├── input/               # NEW — shadcn-vue input primitive
    ├── select/              # NEW — shadcn-vue select primitive
    ├── switch/              # NEW — shadcn-vue switch primitive
    └── skeleton/            # NEW — shadcn-vue skeleton primitive

# Tests
tests/
├── integration/
│   └── pr_list_test.go     # NEW — end-to-end with recorded fixtures
internal/github/
│   └── pr_search_test.go   # NEW — unit tests for SearchOpenPRs
```

**Structure Decision**: Option 2 (web application with separate backend/frontend) applied to the Wails project structure. Backend logic lives in `internal/github/pr_search.go`; the Wails bridge is `app.go`; the frontend lives under `frontend/src/`. This matches the existing project conventions exactly.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| No client-side caching of search results (Principle IV partial) | FR-013 requires a new API query on every filter change to guarantee live data | Caching would return stale results and contradict the spec's explicit server-side filtering requirement |
