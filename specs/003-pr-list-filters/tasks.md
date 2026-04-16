# Tasks: Open PR List with Filters

**Input**: Design documents from `/specs/003-pr-list-filters/`  
**Branch**: `003-pr-list-filters`  
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Tests**: Included per constitution requirement (Principle II: 80% coverage minimum, 90% for `internal/github`).

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete sibling tasks)
- **[Story]**: Which user story this task belongs to (US1–US5)
- Exact file paths included in all task descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new shadcn-vue UI primitives required across all user story phases.

- [ ] T001 Add shadcn-vue `checkbox` primitive to `frontend/src/components/ui/checkbox/`
- [ ] T002 [P] Add shadcn-vue `input` primitive to `frontend/src/components/ui/input/`
- [ ] T003 [P] Add shadcn-vue `select` primitive to `frontend/src/components/ui/select/`
- [ ] T004 [P] Add shadcn-vue `switch` primitive to `frontend/src/components/ui/switch/`
- [ ] T005 [P] Add shadcn-vue `skeleton` primitive to `frontend/src/components/ui/skeleton/`

**Checkpoint**: All UI primitives available — no story implementation can start without them.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Go models, search helper, and Wails binding that ALL user stories depend on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T006 Add `PRListItem`, `PRListFilters`, and `PRListResult` structs to `internal/model/model.go`
- [ ] T007 Create `internal/github/pr_search.go` with `SearchOpenPRs(ctx, client, login, filters)` — query construction, involvement-type fan-out (up to 3 queries), pagination (PerPage:100, loop until NextPage==0), HTMLURL deduplication, sort by UpdatedAt desc
- [ ] T008 Add `RateLimitError` and `AbuseRateLimitError` detection to `SearchOpenPRs` in `internal/github/pr_search.go` — return `PRListResult.RateLimitReset` (RFC3339) on rate limit
- [ ] T009 Add `ListOpenPRs(filters model.PRListFilters) model.PRListResult` exported method to `app.go` — validates filters, checks auth, delegates to `github.SearchOpenPRs`
- [ ] T010 Run `wails generate module` to regenerate `wailsjs/go/main/App.d.ts` and `wailsjs/go/main/models.ts` with `ListOpenPRs`, `PRListFilters`, `PRListResult`, `PRListItem`
- [ ] T011 Write unit tests for `SearchOpenPRs` in `internal/github/pr_search_test.go` — covering: all involvement flags true (3 queries issued), deduplication, rate limit error mapped to `RateLimitReset`, `draft:false` appended by default, `draft:true` when IncludeDrafts, repo/org/author/date qualifiers appended when set, validation error when all involvement flags false

**Checkpoint**: Backend builds (`go build ./...`), bindings regenerated, tests pass (`go test ./internal/github/... -v`). Frontend type-check passes (`pnpm type-check`).

---

## Phase 3: User Story 1 — View All Open PRs (Priority: P1) 🎯 MVP

**Goal**: Replace the `PRPage.vue` stub with a live PR list showing all open PRs for the authenticated user (all involvement types combined), sorted by most recently updated.

**Independent Test**: Navigate to the PR page while authenticated → a scrollable list of open PRs appears showing title, `owner/repo`, author login, and time since opened; list is sorted by most recently updated first; an appropriate empty state appears if the user has no open PRs.

### Implementation for User Story 1

- [ ] T012 [US1] Create `frontend/src/composables/usePRFilters.ts` — module-level singleton refs (`includeAuthor`, `includeAssignee`, `includeReviewer` all default `true`; `includeDrafts` default `false`; `repo`, `org`, `author`, `updatedAfter` default `""`), `toPayload()` snapshot, `clearAllFilters()`, watch setup with stop-handle cleanup, `isOnlyActiveInvolvement(type)` helper
- [ ] T013 [US1] Replace stub in `frontend/src/pages/PRPage.vue` — add `<script setup>` with `usePRFilters` wired to `fetchPRs`, call `ListOpenPRs` on mount, reactive `loading`, `result`, `error` state
- [ ] T014 [US1] Add PR list rendering to `frontend/src/pages/PRPage.vue` — scrollable list using `ScrollArea` component, each row shows title, `owner/repo`, author login, relative time since opened (use `Intl.RelativeTimeFormat` or simple "X days ago" helper), sorted by `updated_at` desc (already sorted by backend)
- [ ] T015 [US1] Add loading skeleton to `frontend/src/pages/PRPage.vue` — show 5 `Skeleton` rows while `loading` is true
- [ ] T016 [US1] Add empty states to `frontend/src/pages/PRPage.vue` — "No open PRs" state (no active filters, no results) vs. "No PRs match filters" state (active filters, no results); use `Card` with descriptive text and `lucide-vue-next` icon
- [ ] T017 [US1] Add error state to `frontend/src/pages/PRPage.vue` — generic network/auth error shown via `vue-sonner` toast and inline error card with user-actionable retry button
- [ ] T018 [US1] Add rate limit error display to `frontend/src/pages/PRPage.vue` — when `result.rate_limit_reset` is non-empty, show inline alert with reset time formatted as local time string; no auto-retry
- [ ] T019 [US1] Make each PR row in `frontend/src/pages/PRPage.vue` clickable — navigate to the in-app PR detail/review view (emit event or use router push with `owner`, `repo`, `number` params extracted from `html_url`)
- [ ] T020 [US1] Write integration test in `tests/integration/pr_list_test.go` — using recorded HTTP fixture, verify `ListOpenPRs` with default filters returns `PRListItem` slice with correct field mapping (`Number`, `Title`, `Owner`, `Repo`, `AuthorLogin`, `CreatedAt`, `UpdatedAt`, `HTMLURL`, `IsDraft`)

**Checkpoint**: User Story 1 fully functional — authenticated user sees live PR list on the PR page. `go test ./... ` passes.

---

## Phase 4: User Story 2 — Filter by Repository (Priority: P2)

**Goal**: Add a repository filter control so users can narrow the list to a specific repository. Each change issues a new API query.

**Independent Test**: With the PR list loaded, select a repository from the repo filter → only PRs from that repository appear; clear the filter → all PRs reappear.

### Implementation for User Story 2

- [ ] T021 [US2] Add `setRepo(v: string)` action and `repo` ref to `frontend/src/composables/usePRFilters.ts` (already scaffolded in T012, just verify it triggers the watch correctly)
- [ ] T022 [US2] Add repository filter `Select` control to the filter bar in `frontend/src/pages/PRPage.vue` — options populated from the `owner/repo` values of currently loaded `result.items` (deduplicated, sorted alphabetically); selecting an option calls `filters.setRepo(value)` which triggers a new fetch
- [ ] T023 [US2] Add "clear repo filter" button/icon next to the repo `Select` in `frontend/src/pages/PRPage.vue` — visible only when `filters.repo` is non-empty; calls `filters.setRepo('')`
- [ ] T024 [US2] Verify `SearchOpenPRs` in `internal/github/pr_search.go` appends `repo:OWNER/REPO` qualifier when `filters.Repo` is non-empty (covered by T011 unit test — verify test case exists)

**Checkpoint**: Repo filter operational — selecting a repo issues a new query and filters results correctly.

---

## Phase 5: User Story 3 — Filter by Organization (Priority: P2)

**Goal**: Add an organization filter control so users can scope the list to a single GitHub org.

**Independent Test**: Select an organization from the org filter → only PRs from repositories owned by that organization appear; combining with the repo filter (US2) applies AND logic.

### Implementation for User Story 3

- [ ] T025 [US3] Add `setOrg(v: string)` action and `org` ref to `frontend/src/composables/usePRFilters.ts`
- [ ] T026 [US3] Add organization filter `Select` control to the filter bar in `frontend/src/pages/PRPage.vue` — options populated from the `owner` values of currently loaded `result.items` where the owner is an org (heuristic: include all unique owners); selecting calls `filters.setOrg(value)` which triggers new fetch
- [ ] T027 [US3] Add "clear org filter" icon/button to `frontend/src/pages/PRPage.vue` — visible only when `filters.org` is non-empty; calls `filters.setOrg('')`
- [ ] T028 [US3] Verify `SearchOpenPRs` in `internal/github/pr_search.go` appends `org:ORG` qualifier when `filters.Org` is non-empty and AND-composes correctly with repo filter (covered by T011 — verify test case exists)

**Checkpoint**: Org filter operational; combining org + repo filter narrows results correctly (AND logic).

---

## Phase 6: User Story 4 — Filter by Author (Priority: P3)

**Goal**: Add a text input for filtering PRs by a specific GitHub author login. Input is debounced (300ms) to avoid per-keystroke queries.

**Independent Test**: Enter a GitHub username in the author filter → only PRs authored by that user appear; clearing the input restores all PRs.

### Implementation for User Story 4

- [ ] T029 [US4] Add `setAuthor(v: string)` action and `author` ref to `frontend/src/composables/usePRFilters.ts` — the watch on `author` uses `useDebounceFn(fetchPRs, 300)` from `@vueuse/core` (300ms debounce for free-text input)
- [ ] T030 [US4] Add author filter `Input` control to the filter bar in `frontend/src/pages/PRPage.vue` — placeholder "Filter by author login"; bound with `v-model` to `filters.author`; show clear icon when non-empty
- [ ] T031 [US4] Verify `SearchOpenPRs` in `internal/github/pr_search.go` appends `author:LOGIN` qualifier when `filters.Author` is non-empty (note: this is the PR-author filter qualifier, distinct from the involvement-type `author:login` qualifier — confirm query construction is correct)

**Checkpoint**: Author filter operational with 300ms debounce — typing a login narrows results without per-keystroke API hammering.

---

## Phase 7: User Story 5 — Filter by Date Range (Priority: P3)

**Goal**: Add a date range selector (preset options: Last 7 days, Last 30 days, Last 90 days, Any time) that restricts results to PRs updated within the chosen window.

**Independent Test**: Select "Last 7 days" → only PRs updated in the past 7 days appear; selecting "Any time" restores the full list; if no PRs fall within the range, the "no results match filters" empty state is shown.

### Implementation for User Story 5

- [ ] T032 [US5] Add `setUpdatedAfter(v: string)` action and `updatedAfter` ref to `frontend/src/composables/usePRFilters.ts` — `v` is an RFC3339 datetime string or `""` for "Any time"; the watch triggers immediately (no debounce, discrete selection)
- [ ] T033 [US5] Add date range `Select` control to the filter bar in `frontend/src/pages/PRPage.vue` — options: "Any time" (`""`), "Last 7 days", "Last 30 days", "Last 90 days"; selecting an option computes the RFC3339 cutoff date and calls `filters.setUpdatedAfter(value)`
- [ ] T034 [US5] Verify `SearchOpenPRs` in `internal/github/pr_search.go` appends `updated:>=DATE` qualifier when `filters.UpdatedAfter` is non-empty (covered by T011 — verify test case exists)

**Checkpoint**: All 5 user stories functional; all filters composable via AND logic; session-persistent filter state verified by navigating away and back to the PR page.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Involvement type toggles, include-drafts toggle, ARIA labels, and final validation.

- [ ] T035 [P] Add involvement type toggle controls to the filter bar in `frontend/src/pages/PRPage.vue` — three `Checkbox` components labelled "Author", "Assignee", "Reviewer"; bound to `filters.includeAuthor/Assignee/Reviewer`; last active checkbox is `:disabled` (use `filters.isOnlyActiveInvolvement(type)`); each toggle triggers a new fetch immediately
- [ ] T036 [P] Add include-drafts `Switch` toggle to the filter bar in `frontend/src/pages/PRPage.vue` — label "Include drafts"; bound to `filters.includeDrafts`; toggling triggers a new fetch
- [ ] T037 [P] Add ARIA labels to all filter controls in `frontend/src/pages/PRPage.vue` — `aria-label` on `Checkbox` (e.g., "Filter by author involvement"), `Select` (`aria-label="Filter by repository"`), `Input` (`aria-label="Filter by PR author"`), `Switch` (`aria-label="Include draft pull requests"`)
- [ ] T038 Add "Clear all filters" button to the filter bar in `frontend/src/pages/PRPage.vue` — visible only when `filters.hasActiveFilters` is true; calls `filters.clearAllFilters()`; after clearing, triggers a fresh fetch with default filters
- [ ] T039 Add `incomplete_results` warning banner to `frontend/src/pages/PRPage.vue` — shown when `result.incomplete_results` is true; text: "Results may be incomplete — GitHub returned partial data. Try narrowing your filters."
- [ ] T040 [P] Run `golangci-lint run` and fix all lint errors in new Go files (`internal/model/model.go`, `internal/github/pr_search.go`, `app.go`)
- [ ] T041 [P] Run `go test -coverprofile=coverage.out ./internal/github/...` and verify `pr_search.go` reaches ≥ 90% line coverage; add missing test cases as needed in `internal/github/pr_search_test.go`
- [ ] T042 [P] Run `pnpm type-check` in `frontend/` and fix any TypeScript strict-mode errors
- [ ] T043 Run `wails build` and verify the app compiles and PR list page is functional end-to-end

**Checkpoint**: All polish complete — lint clean, coverage gate met, build succeeds.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately; T001–T005 all parallelizable
- **Phase 2 (Foundational)**: Depends on Phase 1 completion — BLOCKS all user story phases
  - T006 before T007 (models before search helper)
  - T007 before T008 (rate limit detection extends search helper)
  - T006+T007+T008 before T009 (Wails method depends on both)
  - T009 before T010 (bindings depend on method existing)
  - T011 can run in parallel with T009/T010
- **Phase 3 (US1)**: Depends on Phase 2 — T012 before T013 (composable before page)
- **Phases 4–7 (US2–US5)**: Each depends on Phase 3 (PR page scaffolding must exist); US2–US5 can proceed in parallel with each other
- **Phase 8 (Polish)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P1)**: Foundational complete → can start
- **US2 (P2)**: US1 complete (repo `Select` options drawn from loaded items) → can start
- **US3 (P2)**: US1 complete → can start; US2 and US3 are parallel
- **US4 (P3)**: US1 complete → can start; independent of US2/US3
- **US5 (P3)**: US1 complete → can start; independent of US2/US3/US4

### Parallel Opportunities

- T001–T005 (Phase 1): All parallel
- T007, T011: Parallel (search helper + its tests)
- T009, T010: Sequential (method → bindings)
- T013–T019 (US1 implementation): T012 first, then T013–T019 mostly sequential (same file)
- T022–T024 (US2), T025–T028 (US3): Parallel with each other after T012
- T029–T031 (US4), T032–T034 (US5): Parallel with each other after T012
- T035, T036, T037, T038, T039 (Polish): T035–T037 parallel; T038, T039 depend on page structure

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Step 1: Model first
Task: "T006 — Add PRListItem, PRListFilters, PRListResult to internal/model/model.go"

# Step 2: Search helper + tests in parallel
Task: "T007+T008 — Create internal/github/pr_search.go with SearchOpenPRs"
Task: "T011 — Write unit tests in internal/github/pr_search_test.go"

# Step 3: Wails method + binding regeneration (sequential)
Task: "T009 — Add ListOpenPRs to app.go"
Task: "T010 — Run wails generate module"
```

## Parallel Example: User Stories 2–5 (after US1 complete)

```bash
# All four can start simultaneously:
Task: "T021–T024 — Repository filter (US2)"
Task: "T025–T028 — Organization filter (US3)"
Task: "T029–T031 — Author filter (US4)"
Task: "T032–T034 — Date range filter (US5)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Add UI primitives (T001–T005)
2. Complete Phase 2: Go models + search helper + Wails method + bindings + unit tests (T006–T011)
3. Complete Phase 3: Replace PR page stub with live list (T012–T020)
4. **STOP and VALIDATE**: Navigate to PR page — live PR list appears with title, repo, author, age; empty state works; error/rate-limit states work; PR row click navigates to detail view
5. Ship MVP — users can see their open PRs immediately

### Incremental Delivery

1. Setup + Foundational (Phase 1–2) → Backend ready
2. US1 (Phase 3) → **MVP: Live PR list** — validate and demo
3. US2 + US3 in parallel (Phases 4–5) → Repo/Org filters — validate each independently
4. US4 + US5 in parallel (Phases 6–7) → Author/Date filters — validate each independently
5. Polish (Phase 8) → Involvement toggles, drafts, ARIA, lint, coverage gate

### Parallel Team Strategy

With multiple developers (after Phase 2 complete):

- Developer A: US1 (Phase 3) — unblocks everyone else
- Developer B: US2 + US3 (Phases 4–5) — starts after US1 scaffold exists (T012–T013 done)
- Developer C: US4 + US5 (Phases 6–7) — same dependency

---

## Notes

- [P] tasks operate on different files or have no incomplete sibling dependencies
- [Story] label maps each task to the spec user story for traceability
- `wails generate module` (T010) MUST be re-run after any change to exported `App` methods
- Constitution Principle II requires 90% coverage for `internal/github/pr_search.go` (critical package)
- The involvement-type fan-out (up to 3 queries) is the most complex part of T007 — keep each query builder as a small named function to stay under cyclomatic complexity ≤ 10 (Principle I)
- Filter state persists within the session via module-level refs in `usePRFilters.ts` — no router or Pinia needed
- Repo/org filter options are populated from the current result set, not from the GitHub API directly
