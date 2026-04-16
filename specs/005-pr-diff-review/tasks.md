# Tasks: PR Diff Review

**Input**: Design documents from `/specs/005-pr-diff-review/`  
**Branch**: `005-pr-diff-review`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/wails-bindings.md ✅, quickstart.md ✅

**Tests**: Unit tests included for Go backend packages (constitution mandates ≥ 90% on diff-parsing packages). Frontend tests not in scope for this iteration.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1–US5)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new model types that all subsequent phases depend on.

- [ ] T001 Add PRFileDTO, DiffLineDTO, DiffHunkDTO, ParsedDiffDTO, DraftCommentDTO, PendingReviewDTO, ReviewSubmitDTO, ReviewSubmitResult types to `internal/model/model.go` (see `specs/005-pr-diff-review/data-model.md` for full struct definitions)

---

## Phase 2: Foundational (Go Backend)

**Purpose**: Implement all Go packages and Wails bindings before any frontend work can begin.

**⚠️ CRITICAL**: No frontend user story work can begin until this phase is complete (Wails bindings must be generated before frontend TypeScript types exist).

- [ ] T002 [P] Implement `ListPRFiles` (paginated `PullRequestsService.ListFiles`) and `ParseUnifiedDiff` (unified diff parser: hunk header regex, line classification for context/add/delete, binary detection) in `internal/github/diff.go`
- [ ] T003 [P] Write table-driven unit tests in `internal/github/diff_test.go` covering: normal hunks, pure-add file, pure-delete file, renamed file, binary file (empty patch), `\ No newline at end of file`, multi-hunk file, zero-line hunk — must reach **≥ 90% line coverage**
- [ ] T004 [P] Implement `CreatePendingReview`, `AddCommentToPendingReview` (raw HTTP `POST .../reviews/{id}/comments`), `SubmitReview`, and `DeletePendingReview` in `internal/github/review_submit.go`
- [ ] T005 [P] Write unit tests in `internal/github/review_submit_test.go` using `httptest` fixtures for each of the four functions; cover success and error paths
- [ ] T006 Add `pendingReviewID int64`, `pendingComments []model.DraftCommentDTO`, `prFilesCache []*github.CommitFile`, `prCommitSHA string` fields to `App` struct; populate `prCommitSHA` from `pr.GetHead().GetSHA()` in `LoadPullRequest` in `app.go`
- [ ] T007 Add `GetPRFiles`, `GetFileDiff`, `AddDraftComment`, `PostImmediateComment`, `GetPendingReview`, `SubmitReview`, `DiscardPendingReview` Wails-bound methods to `app.go` (full signatures and preconditions in `specs/005-pr-diff-review/contracts/wails-bindings.md`)
- [ ] T008 Run `wails generate module` to regenerate `wailsjs/go/main/App.d.ts` and `wailsjs/go/models.ts` with new types and methods
- [ ] T009 Verify `go build ./...` passes cleanly and `go test ./internal/github/... -coverprofile=coverage.out` confirms `diff.go` reaches ≥ 90% line coverage

**Checkpoint**: Go backend complete. All Wails bindings available. Frontend development can begin.

---

## Phase 3: User Story 1 — Switch Between PR Views (Priority: P1) 🎯 MVP

**Goal**: Add a multi-value view toggle to `ReviewPage.vue` that switches between the existing conversation view and a new (initially empty) "Files changed" slot. State is preserved on switch.

**Independent Test**: Open any PR, verify the toggle appears with "Conversation" and "Files changed" options, switch between them, confirm conversation view content is preserved on return, confirm active tab is visually distinguished.

- [ ] T010 [US1] Create `frontend/src/components/ViewToggle.vue` using shadcn-vue `Tabs` component; accept `modelValue` (string) and `options: { value: string; label: string }[]` props; emit `update:modelValue`; keyboard navigation and ARIA labels required
- [ ] T011 [US1] Modify `frontend/src/pages/ReviewPage.vue`: add `prView = ref<'conversation' | 'files'>('conversation')`; render `<ViewToggle>` at top; wrap existing conversation content in `v-show="prView === 'conversation'"`; render `<DiffReviewView>` placeholder with `v-show="prView === 'files'"` (component stubbed, implemented in Phase 4)

**Checkpoint**: US1 complete and independently testable. Toggle works; conversation view preserved.

---

## Phase 4: User Story 2 — Browse and Navigate Changed Files (Priority: P2)

**Goal**: The "Files changed" view shows a sidebar listing all changed files; clicking a file loads and displays its diff in split/side-by-side format with collapsed unchanged hunks; next/previous navigation steps through files.

**Independent Test**: Open a PR with ≥ 3 changed files; enter "Files changed" view; verify sidebar lists all files with status badges and line counts; click each file and confirm its diff renders (context collapsed, only changed hunks visible); expand a collapsed hunk; use next/previous buttons to step through files.

- [ ] T012 [P] [US2] Create `frontend/src/composables/useDiffReview.ts`: reactive state for `files`, `filesLoading`, `currentFile`, `currentDiff`, `diffLoading`; methods wrapping `GetPRFiles()` and `GetFileDiff(path)`; `selectFile(path)` and `nextFile()` / `prevFile()` helpers
- [ ] T013 [P] [US2] Create `frontend/src/components/DiffFileSidebar.vue`: renders `PRFileDTO[]` list; shows filename, status badge (added/modified/removed/renamed), additions/deletions counts; emits `select(path: string)`; highlights currently selected file; keyboard accessible
- [ ] T014 [P] [US2] Create `frontend/src/components/DiffLine.vue`: renders a single side-by-side diff table row for one `DiffLineDTO` pair; left cell (old side) and right cell (new side); context lines shown on both sides; delete lines on left only; add lines on right only; empty placeholder cell for surplus lines; no comment affordance yet (added in US3)
- [ ] T015 [US2] Create `frontend/src/components/DiffHunk.vue`: renders `DiffHunkDTO` as a `<table>`; iterates lines using `DiffLine`; adds a collapsible context bar between hunks (shows "N hidden lines", expands on click); hunk header displayed
- [ ] T016 [US2] Create `frontend/src/components/DiffFileView.vue`: accepts `ParsedDiffDTO` prop; renders file header (filename, status, additions/deletions, binary placeholder); iterates and renders `DiffHunk` components; handles binary file placeholder
- [ ] T017 [US2] Create `frontend/src/components/DiffReviewView.vue`: layout with `DiffFileSidebar` on left and `DiffFileView` on right; previous/next file navigation buttons; loading and empty states; uses `useDiffReview` composable; calls `GetPRFiles()` on mount
- [ ] T018 [US2] Replace stub in `frontend/src/pages/ReviewPage.vue` with real `<DiffReviewView>` component import; confirm files load when switching to "Files changed" tab

**Checkpoint**: US2 complete. Read-only diff review fully functional: sidebar, file loading, split diff rendering, hunk collapse/expand, file navigation.

---

## Phase 5: User Story 3 — Leave Inline Comments (Priority: P3)

**Goal**: Reviewer can click a diff line to open a comment form, type a comment, and post it immediately (standalone) or add it to a pending review batch. Multi-line selection supported.

**Independent Test**: Open any diff; hover a line and verify "+" affordance appears; click it; type a comment; click "Add single comment"; verify the comment appears on GitHub. Then open another line, click "Start a review"; verify pending review indicator appears in the UI.

- [ ] T019 [P] [US3] Update `frontend/src/components/DiffLine.vue`: add hover state that reveals a "+" icon affordance on the right-side cell; emit `open-comment({ path, line, side })` on click; emit `range-select({ path, startLine, endLine, side })` for multi-line (mousedown + mousemove + mouseup on consecutive lines)
- [ ] T020 [P] [US3] Create `frontend/src/components/InlineCommentForm.vue`: textarea for comment body; "Add single comment" button (calls `PostImmediateComment`); "Start a review" / "Add review comment" button (calls `AddDraftComment`); cancel button; emits `submitted` and `cancelled`; anchored visually below the target line
- [ ] T021 [P] [US3] Extend `frontend/src/composables/useDiffReview.ts`: add `pendingReview` state; add `addDraftComment(comment)` wrapping `AddDraftComment()`; add `postImmediateComment(comment)` wrapping `PostImmediateComment()`; update `pendingReview` ref after each draft comment
- [ ] T022 [US3] Integrate `InlineCommentForm` into `frontend/src/components/DiffFileView.vue`: track `activeCommentTarget` ref; show/hide form below the clicked line; handle `submitted` and `cancelled` events; pass correct `path`, `line`, `side` to form
- [ ] T023 [US3] Add multi-line drag selection state to `frontend/src/components/DiffHunk.vue`: track `dragStart` and `dragEnd` line numbers; highlight selected range; open `InlineCommentForm` with `start_line` and `line` range on drag release

**Checkpoint**: US3 complete. Inline comments (immediate and draft) fully functional.

---

## Phase 6: User Story 4 — Submit a Formal Review with Verdict (Priority: P4)

**Goal**: When review mode is active (pending comments exist), the reviewer can open a submit panel, select a verdict (Approve / Request Changes / Comment), optionally add a body, and submit. All pending comments are published. Discard is also available.

**Independent Test**: Enter review mode by adding a draft comment; open the submit panel; verify three verdict options are shown; submit with "Approve" and verify GitHub records the review and UI confirms success. Then start a new review, add a comment, click discard, confirm warning, and verify pending state is cleared.

- [ ] T024 [P] [US4] Extend `frontend/src/composables/useDiffReview.ts`: add `submitReview(req)` wrapping `SubmitReview()`; add `discardPendingReview()` wrapping `DiscardPendingReview()`; clear `pendingReview` state on success
- [ ] T025 [P] [US4] Add pending review indicator to `frontend/src/components/DiffReviewView.vue`: show badge with pending comment count when `pendingReview.has_pending`; show "Submit review" button that opens the submit panel
- [ ] T026 [US4] Add review submit panel to `frontend/src/components/DiffReviewView.vue`: radio/button group for Approve / Request Changes / Comment verdict; optional body textarea; Submit button (calls `submitReview`); Discard button with confirmation dialog (calls `discardPendingReview`); success and error states

**Checkpoint**: US4 complete. Full review workflow: draft comments → submit verdict → publish.

---

## Phase 7: User Story 5 — View Other Reviewers' Comment Threads (Priority: P5)

**Goal**: Existing comment threads from other reviewers are hidden by default. A toggle reveals them inline anchored to their diff lines, with "Outdated" indicator for stale threads.

**Independent Test**: Open a PR that already has review comments from other users; enter diff view; confirm threads are hidden; toggle "Show reviewer comments"; verify threads appear at their diff lines; toggle off and verify they hide again.

- [ ] T027 [P] [US5] Extend `frontend/src/composables/useDiffReview.ts`: add `showOtherThreads = ref(false)`; add `toggleOtherThreads()`; load `CommentThreadDTO[]` via existing `GetCommentThreads(false)` call; expose `threadsForFile(path)` computed that filters threads by the current file path
- [ ] T028 [P] [US5] Add "Show reviewer comments" toggle button to `frontend/src/components/DiffReviewView.vue`: calls `toggleOtherThreads()`; shows active/inactive state
- [ ] T029 [US5] Render `CommentThreadDTO[]` inline in `frontend/src/components/DiffFileView.vue`: when `showOtherThreads` is true, insert thread cards below their anchored diff line (match on `thread.path` and `thread.line`); show "Outdated" badge when `thread.originalLine !== thread.line` (or thread is marked outdated)

**Checkpoint**: US5 complete. All five user stories independently functional.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Error/loading/empty states, edge case display, keyboard nav, and final build verification.

- [ ] T030 [P] Add loading skeleton, error state (with retry), and empty state to `frontend/src/components/DiffFileSidebar.vue`, `DiffFileView.vue`, and `DiffReviewView.vue`
- [ ] T031 [P] Add binary file placeholder display in `frontend/src/components/DiffFileView.vue`: show filename, status, additions/deletions metadata; message "Binary file — diff not available"
- [ ] T032 [P] Add renamed file display in `frontend/src/components/DiffFileSidebar.vue` (show old → new path) and `DiffFileView.vue` file header (show `previous_filename → filename`)
- [ ] T033 [P] Add "Outdated" badge styling to stale `CommentThreadDTO` in `frontend/src/components/DiffFileView.vue` thread rendering (distinct muted visual treatment from current threads)
- [ ] T034 [P] Add keyboard navigation to `frontend/src/components/DiffReviewView.vue`: `]` / `[` keys for next/previous file; `Escape` key to close open `InlineCommentForm`; ARIA labels on all interactive diff elements (line affordance buttons, collapse bars)
- [ ] T035 Run full test suite: `go test ./... -coverprofile=coverage.out` and confirm `internal/github/diff.go` line coverage ≥ 90%; run `wails build` to verify production build compiles without errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — **BLOCKS all frontend phases**
- **Phase 3 (US1)**: Depends on Phase 2 (Wails bindings must exist for TypeScript imports)
- **Phase 4 (US2)**: Depends on Phase 3 (DiffReviewView is integrated into ReviewPage via US1)
- **Phase 5 (US3)**: Depends on Phase 4 (adds comment affordance to existing DiffLine/DiffHunk)
- **Phase 6 (US4)**: Depends on Phase 5 (submit panel builds on pending review state from US3)
- **Phase 7 (US5)**: Depends on Phase 4 (needs DiffFileView infrastructure); independent of US3/US4
- **Phase 8 (Polish)**: Depends on all desired user stories being complete

### User Story Dependencies

| Story | Depends on | Notes |
|---|---|---|
| US1 (P1) | Phase 2 complete | Entry point; no other story dependency |
| US2 (P2) | US1 complete | Fills the "Files changed" slot created by US1 |
| US3 (P3) | US2 complete | Adds interactivity to the diff rendered by US2 |
| US4 (P4) | US3 complete | Builds on pending review state introduced by US3 |
| US5 (P5) | US2 complete | Independent of US3/US4; can be done after US2 |

### Within Phase 2 (Foundational)

```
T002 (diff.go) ──┬── T003 (diff_test.go) ───────┐
                 │                               ├── T009 (verify)
T004 (submit.go) ─┴── T005 (submit_test.go) ──┐ │
                                               ↓ ↓
                 T006 (app fields) → T007 (app methods) → T008 (generate)
```

T002, T003, T004, T005 can all run in parallel (different files).  
T006 follows T002 + T004. T007 follows T006. T008 follows T007. T009 follows T003 + T005 + T008.

### Within Phase 4 (US2)

T012, T013, T014 can run in parallel. T015 follows T014. T016 follows T015. T017 follows T012 + T013 + T016. T018 follows T017.

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Round 1 — all four in parallel (different files):
Task: "T002 Implement ListPRFiles and ParseUnifiedDiff in internal/github/diff.go"
Task: "T003 Write unit tests in internal/github/diff_test.go"
Task: "T004 Implement review submit functions in internal/github/review_submit.go"
Task: "T005 Write unit tests in internal/github/review_submit_test.go"

# Round 2 — sequential:
Task: "T006 Add new fields to App struct and update LoadPullRequest in app.go"
Task: "T007 Add 7 new Wails methods to app.go"
Task: "T008 Run wails generate module"
Task: "T009 Verify build and coverage"
```

## Parallel Example: Phase 4 (US2)

```bash
# Round 1 — in parallel:
Task: "T012 Create useDiffReview.ts composable"
Task: "T013 Create DiffFileSidebar.vue"
Task: "T014 Create DiffLine.vue (rendering only)"

# Round 2 — sequential:
Task: "T015 Create DiffHunk.vue"
Task: "T016 Create DiffFileView.vue"
Task: "T017 Create DiffReviewView.vue"
Task: "T018 Wire DiffReviewView into ReviewPage.vue"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002–T009) — **critical gate**
3. Complete Phase 3: US1 — View Toggle (T010–T011)
4. **STOP and VALIDATE**: toggle appears, conversation view preserved, "Files changed" tab shows placeholder
5. Merge or demo if needed

### Incremental Delivery

1. Phase 1 + 2 → Go backend + bindings ready
2. Phase 3 (US1) → Toggle works, entry point established
3. Phase 4 (US2) → Read-only diff review fully usable; significant standalone value
4. Phase 5 (US3) → Inline commenting unlocked
5. Phase 6 (US4) → Full formal review submission
6. Phase 7 (US5) → Contextual enhancement (other reviewers' threads)
7. Phase 8 → Polish and ship

### Parallel Team Strategy

With two developers after Phase 2 completes:
- Developer A: US1 → US2 → US3 (sequential; core diff + commenting path)
- Developer B: US1 → US2 → US5 (can branch off after US2 for the threads toggle)
- Merge US3 + US5 before US4

---

## Notes

- [P] tasks = different files, no shared state dependencies
- [Story] label maps each task to its user story for traceability
- `wails generate module` (T008) is a hard gate; no frontend TypeScript will compile without it
- `ParseUnifiedDiff` is the highest-risk function — write tests (T003) early and treat the 90% gate as non-negotiable (constitution Principle II)
- The raw HTTP call in `AddCommentToPendingReview` (T004) is the only non-go-github API call; test it carefully with `httptest`
- Commit after each task or logical group; stop at any checkpoint to validate independently
