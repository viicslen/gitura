# Tasks: PR Deep Review Workflow

**Feature**: 004-pr-review-workflow  
**Input**: Design documents from `/specs/004-pr-review-workflow/`  
**Branch**: `004-pr-review-workflow`

**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/wails-bindings.md ✓, quickstart.md ✓

**Tests**: Included — constitution II requires 80% coverage (90% for `internal/github/` and `internal/settings/`); integration tests use `httptest` fixtures per constitution.

**Organization**: 6 user stories (P1–P6) mapped to phases 3–8. Each phase independently deliverable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story this task belongs to (US1–US6)
- Exact file paths included in all descriptions

---

## Phase 1: Setup

**Purpose**: Test fixture directory structure; no code yet.

- [ ] T001 Create `tests/fixtures/graphql/` directory for recorded GraphQL response fixtures

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before ANY user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T002 Add `IsDraft bool \`json:"is_draft"\`` field to `PullRequestSummary` struct in `internal/model/model.go`
- [ ] T003 [P] Implement `internal/settings/settings.go` — export `Load() ([]model.IgnoredCommenterDTO, error)`, `Save([]model.IgnoredCommenterDTO) error`, `ConfigDir() (string, error)`; atomic write via temp-file rename; create `gitura/` subdir if absent; empty list writes `[]`
- [ ] T004 [P] Write unit tests in `internal/settings/settings_test.go` — cover `Load` (missing file returns empty slice), `Save` (round-trip), `ConfigDir` (returns OS-appropriate path), duplicate-add no-op; use `t.TempDir()`; target 90% coverage
- [ ] T005 [P] Implement `internal/github/pr.go` — `FetchPR(ctx, client, owner, repo string, number int) (*model.PullRequestSummary, error)` wrapping `go-github PullRequests.Get`; map `IsDraft` from `pr.GetDraft()`; return `notfound:` error on HTTP 404
- [ ] T006 [P] Write unit tests in `internal/github/pr_test.go` — success response, 404 returns `notfound:` prefix, network error; use `httptest` fixture JSON recorded from GitHub API
- [ ] T007 Implement `internal/github/comments.go` — `FetchReviewThreads(ctx context.Context, token, owner, repo string, number int, progressFn func(loaded, total int)) ([]model.CommentThreadDTO, error)`; POST to `https://api.github.com/graphql`; paginate via `pageInfo.hasNextPage`/`endCursor`; map GraphQL nodes to `CommentThreadDTO` + `CommentDTO`; detect `IsSuggestion` from ` ```suggestion` in body; set `InReplyToID` from `replyTo.databaseId`; call `progressFn` after each page
- [ ] T008 Write unit tests in `internal/github/comments_test.go` using `httptest.NewServer` — single-page result, multi-page pagination, empty result (no threads), GraphQL error array, HTTP 401; record fixture JSON in `tests/fixtures/graphql/`; target 90% coverage
- [ ] T009 Add `ignoredCommenters []model.IgnoredCommenterDTO` field to `App` struct in `app.go`; implement `loadIgnoredCommenters()` private helper that lazy-loads from `settings.Load()` on first call (call from `startup` to pre-warm)
- [ ] T010 Extend `frontend/src/App.vue` — change `currentPage` type to `'pr' | 'settings' | 'review'`; add `selectedPRItem = ref<ReviewLoadInput | null>(null)`; add `handleOpenReview(item: ReviewLoadInput)` and `handleCloseReview()`; wrap `<PRPage>` in `<KeepAlive>`; add `<ReviewPage>` conditional using `v-if="currentPage === 'review' && selectedPRItem"`

**Checkpoint**: Foundation ready — all user story phases can now begin.

---

## Phase 3: User Story 1 — Review Comment Summary List (Priority: P1) 🎯 MVP

**Goal**: Clicking a PR in the list navigates to a review view showing all comments as a
summary list (author, file path, 200-char excerpt). Show-resolved toggle and ignored-commenter
filter applied. Back navigation restores list state with no re-fetch.

**Independent Test**: Click any open PR → verify review view loads within 3s, comment list
shows author + file + excerpt per thread, ignored-commenter comments absent, resolved threads
hidden by default, back restores list scroll/filters.

- [ ] T011 [US1] Add `LoadPullRequest(owner, repo string, number int) (model.PullRequestSummary, error)` to `app.go` — calls `github.FetchPR` + `github.FetchReviewThreads`; filters out threads whose root comment author is in `a.ignoredCommenters`; stores result in `a.prCache` and `a.threads`; emits `pr:load-progress` events from progress callback; computes `CommentCount` and `UnresolvedCount`
- [ ] T012 [US1] Add `GetCommentThreads(includeResolved bool) ([]model.CommentThreadDTO, error)` and `GetThread(rootID int64) (model.CommentThreadDTO, error)` to `app.go` — `GetCommentThreads` filters `a.threads` by `Resolved` when `includeResolved` is false; `GetThread` looks up by `RootID`; both return `notfound:` error if no PR loaded
- [ ] T013 [US1] Run `wails generate module` to regenerate `wailsjs/go/main/App.d.ts` and `wailsjs/go/main/models.ts` after T011/T012 changes; verify `PullRequestSummary` interface in generated TS includes `is_draft`
- [ ] T014 [US1] Implement `frontend/src/composables/useReview.ts` — export `useReview(prItem: ReviewLoadInput)`; reactive state: `threads ref<CommentThreadDTO[]>`, `prSummary ref<PullRequestSummary|null>`, `loading ref<boolean>`, `error ref<string>`, `showResolved ref<boolean>` (reset to `false` on each `loadPR` call); implement `loadPR()` calling `LoadPullRequest` then `GetCommentThreads(showResolved.value)`; listen for `pr:load-progress` event; export `toggleShowResolved()` which re-fetches threads with new toggle value
- [ ] T015 [P] [US1] Implement `frontend/src/components/CommentSummaryList.vue` — props: `{ threads: CommentThreadDTO[], currentIndex: number, showResolved: boolean }`; render scrollable list; each row: author avatar (`<img>`) + login, file path, 200-char body excerpt with ellipsis, "Resolved" badge (muted styling) when `thread.resolved && showResolved`; highlight active row by `currentIndex`; emit `select(index: number)` on row click; keyboard: Up/Down arrows move selection, Enter selects; `role="listbox"` + `aria-activedescendant`
- [ ] T016 [US1] Implement `frontend/src/pages/ReviewPage.vue` — props: `{ prItem: ReviewLoadInput }`; emits: `close-review`; use `useReview(prItem)` composable; layout: top bar (back button, PR title + number, show-resolved toggle, comment count), left panel (`CommentSummaryList`), right panel (detail placeholder until T021); handle `select` event from list to update `currentIndex`; show loading spinner while `loading`; show error alert when `error` non-empty; show empty state when `queue` is empty
- [ ] T017 [US1] Add click handler to PR row in `frontend/src/pages/PRPage.vue` — emit `open-review` with `ReviewLoadInput` constructed from `PRListItem`; add cursor-pointer styling to rows; ensure keyboard Enter on focused row also triggers emit
- [ ] T018 [US1] Run `go test ./internal/settings/... ./internal/github/...` and verify T004/T006/T008 tests pass; fix any failures

**Checkpoint**: User Story 1 fully functional — navigate to review, see comment list, go back with state intact.

---

## Phase 4: User Story 2 — Comment-by-Comment Navigation with Diff Context (Priority: P2)

**Goal**: Click a comment in the summary list to see its full body and diff hunk in a detail
panel. Step forward/backward through comments. End-of-queue shows "All comments reviewed".
Target line highlighted in diff. Show-resolved toggle gates the nav queue.

**Independent Test**: Click a comment → verify full body + diff hunk displayed → navigate
Next/Prev through all threads → verify end-of-queue message when last comment reached.

- [ ] T019 [US2] Extend `frontend/src/composables/useReview.ts` — add `currentIndex ref<number>` (reset to 0 on new PR load); computed `queue` (threads filtered by `showResolved`); computed `currentThread` (`queue[currentIndex] ?? null`); computed `isAtEnd`, `canGoBack`, `canGoForward`; export `goNext()`, `goPrev()`; clamp `currentIndex` to `queue.length - 1` when `showResolved` toggles
- [ ] T020 [P] [US2] Implement `frontend/src/components/DiffHunkView.vue` — props: `{ diffHunk: string, highlightLine?: number }`; render in `<pre class="font-mono text-xs overflow-x-auto">`; split hunk by `\n`; color lines: `+` prefix → green bg, `-` prefix → red bg, ` ` prefix → neutral; highlight the `highlightLine`-th non-header line with a yellow left border; `aria-label="Diff context"` on container
- [ ] T021 [US2] Implement `frontend/src/components/CommentDetailPanel.vue` — props: `{ thread: CommentThreadDTO | null, isAtEnd: boolean }`; shows "Select a comment to begin" placeholder when `thread` is null; when thread present: author avatar + login + timestamp for each comment in thread, comment body (Markdown rendered as plain text for v1), `DiffHunkView` below root comment; "All comments reviewed" message when `isAtEnd && thread` is last; resolve/unresolve button placeholder (wired in Phase 5); reply area placeholder (wired in Phase 5)
- [ ] T022 [US2] Update `frontend/src/pages/ReviewPage.vue` — integrate `CommentDetailPanel` in right panel; wire `goNext`/`goPrev` to Prev/Next buttons in bottom bar; disable Next when `!canGoForward`, Prev when `!canGoBack`; keyboard: Right arrow → `goNext`, Left arrow → `goPrev` (when detail panel focused); show "All comments reviewed" end-state when `isAtEnd` and `queue.length > 0`

**Checkpoint**: User Stories 1 + 2 work independently — full read-only review session functional.

---

## Phase 5: User Story 3 — Reply to and Resolve Comment Threads (Priority: P3)

**Goal**: Submit a reply to a comment thread from the detail panel. Resolve or unresolve
threads via remote API. Optimistic resolve update with rollback on failure. Draft preserved
on reply failure.

**Independent Test**: Submit a reply → verify appears in thread remotely. Click Resolve →
verify thread summary updates immediately, state confirmed on GitHub. On network error →
verify draft preserved and error shown.

- [ ] T023 [US3] Implement `internal/github/resolve.go` — `ResolveThread(ctx context.Context, token, threadNodeID string) error` and `UnresolveThread(ctx context.Context, token, threadNodeID string) error`; POST to `https://api.github.com/graphql` with `resolveReviewThread` / `unresolveReviewThread` mutations; return `github:` prefixed error when response has `errors` array or non-200 status
- [ ] T024 [P] [US3] Write unit tests in `internal/github/resolve_test.go` using `httptest` — `ResolveThread` success, `UnresolveThread` success, GraphQL error response, HTTP 422; record fixture JSON in `tests/fixtures/graphql/`; target 90% coverage
- [ ] T025 [US3] Add `ReplyToComment(threadRootID int64, body string) (model.CommentDTO, error)` to `app.go` — validate non-empty `body` (return `validation:body required`); find thread in `a.threads`; call `go-github PullRequests.CreateComment` with `InReplyTo = threadRootID`; append returned `CommentDTO` to cached thread `Comments`; return new `CommentDTO`
- [ ] T026 [US3] Add `ResolveThread(threadRootID int64) error` and `UnresolveThread(threadRootID int64) error` to `app.go` — look up `NodeID` from `a.threads`; call `resolve.ResolveThread` / `resolve.UnresolveThread`; on success update cached thread `Resolved` field; return `notfound:thread` if rootID absent
- [ ] T027 [US3] Run `wails generate module` after T025/T026 changes to update `wailsjs/go/main/App.d.ts`
- [ ] T028 [P] [US3] Implement `frontend/src/components/ReplyComposer.vue` — props: `{ threadRootId: number }`; emits: `reply-sent(comment: CommentDTO)`; `<textarea>` with placeholder "Write a reply…"; submit button disabled while `submitting ref`; on error: show inline error message, keep draft text (do NOT clear textarea); `aria-label` on textarea; keyboard: Ctrl+Enter submits
- [ ] T029 [US3] Wire reply and resolve/unresolve in `frontend/src/components/CommentDetailPanel.vue` — add `ReplyComposer` below thread comments; on `reply-sent` event append comment to thread display; add Resolve/Unresolve button (toggle based on `thread.resolved`); emit `resolve(rootId)` / `unresolve(rootId)` events; handle in `ReviewPage.vue` via `useReview` optimistic helpers `resolveThread()` / `unresolveThread()` with rollback on error

**Checkpoint**: Full interactive review session — read, navigate, reply, resolve.

---

## Phase 6: User Story 4 — Accept and Commit GitHub Suggestion Blocks (Priority: P4)

**Goal**: When a comment contains a ` ```suggestion` block, show an "Accept Suggestion" action.
Clicking it commits the change to the PR branch. SHA conflict and other errors show descriptive
messages per FR-018.

**Independent Test**: Open a PR with suggestion comments → verify "Accept Suggestion" button
visible → click it → verify new commit on PR branch with applied change. On conflict: verify
descriptive error, no partial commit.

- [ ] T030 [US4] Implement `internal/github/suggestion.go` — `CommitSuggestion(ctx context.Context, client *github.Client, owner, repo, headBranch string, comment model.CommentDTO, commitMessage string) (model.SuggestionCommitResult, error)`; validate `comment.IsSuggestion` (return `validation:not-a-suggestion`); parse suggestion fenced block from `comment.Body`; use `go-github RepositoriesService.GetContents` to fetch file content + blob SHA; apply patch (replace hunk lines); use `go-github RepositoriesService.UpdateFile` to commit; handle HTTP 409 as `github:conflict` with descriptive message
- [ ] T031 [P] [US4] Write unit tests in `internal/github/suggestion_test.go` using `httptest` — success commit, HTTP 409 returns `github:conflict`, not-a-suggestion returns `validation:` error, missing `@@` hunk header, multi-line suggestion; record fixture JSON; target 90% coverage
- [ ] T032 [US4] Add `CommitSuggestion(commentID int64, commitMessage string) (model.SuggestionCommitResult, error)` to `app.go` — find comment in `a.threads` by `commentID`; call `suggestion.CommitSuggestion`; on success mark cached comment metadata (no DTO field change needed in v1)
- [ ] T033 [US4] Run `wails generate module` after T032 changes
- [ ] T034 [P] [US4] Implement `frontend/src/components/SuggestionBlock.vue` — props: `{ comment: CommentDTO }`; shown only when `comment.is_suggestion`; parse and display suggestion diff lines from body fenced block in styled `<pre>`; "Accept Suggestion" button; shows inline commit-message input on click; confirm button calls `CommitSuggestion`; shows success indicator on commit; emits `committed(result: SuggestionCommitResult)`; `aria-label` on all interactive elements
- [ ] T035 [US4] Integrate `SuggestionBlock` in `frontend/src/components/CommentDetailPanel.vue` — render `SuggestionBlock` for root comment when `is_suggestion`; on `committed` event show commit SHA link; on error display descriptive message from `github:conflict` or other prefixes per FR-018

**Checkpoint**: Full suggestion workflow — detect, display, commit.

---

## Phase 7: User Story 5 — Status Banner for Non-Open PRs (Priority: P5)

**Goal**: Show a prominent banner when the loaded PR is draft, closed, or merged. All comment
interactions remain available.

**Independent Test**: Open a draft PR → verify "Draft" banner. Open a merged PR → verify
"Merged" banner. Open an open PR → verify no banner.

- [ ] T036 [P] [US5] Implement `frontend/src/components/PRStatusBanner.vue` — props: `{ summary: PullRequestSummary | null }`; shows banner when `summary.is_draft` → "Draft", `summary.state === 'merged'` → "Merged", `summary.state === 'closed'` → "Closed"; no banner for open non-draft PRs; uses shadcn-vue `Alert` component with appropriate variant; `role="alert"` for accessibility
- [ ] T037 [US5] Wire `PRStatusBanner` at the top of `frontend/src/pages/ReviewPage.vue` immediately below the top bar, above the comment list; pass `prSummary` as prop

**Checkpoint**: Non-open PR states visually identified.

---

## Phase 8: User Story 6 — Ignored-Commenters Management (Priority: P6)

**Goal**: Users can add/view/remove GitHub usernames in an ignored-commenters list via
Settings. Persists across restarts.

**Independent Test**: Add bot username in Settings → open PR with bot comments → verify hidden.
Restart app → verify list persists. Remove username → verify comments reappear.

- [ ] T038 [US6] Add `GetIgnoredCommenters() ([]model.IgnoredCommenterDTO, error)`, `AddIgnoredCommenter(login string) error`, `RemoveIgnoredCommenter(login string) error` to `app.go` — `GetIgnoredCommenters` returns `a.ignoredCommenters` (lazy-loaded); `Add` validates non-empty login, silently no-ops if duplicate, appends to `a.ignoredCommenters` and calls `settings.Save`; `Remove` removes by login and calls `settings.Save`
- [ ] T039 [US6] Run `wails generate module` after T038 changes
- [ ] T040 [US6] Add ignored-commenters management section to `frontend/src/pages/SettingsPage.vue` — heading "Ignored Commenters"; scrollable list of current entries showing login + `added_at` date + "Remove" button per entry; text input + "Add" button (validate non-empty, trim whitespace); calls `GetIgnoredCommenters` on mount, `AddIgnoredCommenter` on add, `RemoveIgnoredCommenter` on remove; show inline error if add fails; empty state "No ignored commenters"; all interactive elements have ARIA labels

**Checkpoint**: Full ignored-commenter workflow — add, view, remove, persists.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Accessibility, error handling completeness, lint, test coverage verification, build.

- [ ] T041 [P] Audit and add missing ARIA labels, roles, and keyboard navigation to `frontend/src/components/CommentSummaryList.vue`, `CommentDetailPanel.vue`, `DiffHunkView.vue`, `ReplyComposer.vue`, `SuggestionBlock.vue`, `ReviewPage.vue` per FR spec and constitution III
- [ ] T042 [P] Verify all error/loading/empty states are handled in `ReviewPage.vue` — loading spinner during `LoadPullRequest`, error alert with retry button on load failure, empty-queue state with "No review comments" message; ensure consistent styling with existing `PRPage.vue` patterns
- [ ] T043 [P] Run `golangci-lint run` and fix all reported issues in new Go files (`internal/github/pr.go`, `internal/github/comments.go`, `internal/github/resolve.go`, `internal/github/suggestion.go`, `internal/settings/settings.go`, `app.go`)
- [ ] T044 Run `go test -coverprofile=coverage.out ./...` and verify overall ≥ 80%, `internal/github/` ≥ 90%, `internal/settings/` ≥ 90%; fix coverage gaps
- [ ] T045 Run `wails build` and verify a clean production build succeeds with no compile errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **blocks all user stories**
- **US1 (Phase 3)**: Depends on Phase 2 completion
- **US2 (Phase 4)**: Depends on Phase 3 completion (uses composable from T014)
- **US3 (Phase 5)**: Depends on Phase 4 completion (wires into `CommentDetailPanel` from T021)
- **US4 (Phase 6)**: Depends on Phase 5 completion (wires into `CommentDetailPanel` from T029)
- **US5 (Phase 7)**: Depends on Phase 2 (needs `IsDraft` from T002); can run alongside Phase 6
- **US6 (Phase 8)**: Depends on Phase 2 (needs settings from T003); can run alongside Phases 6–7
- **Polish (Phase 9)**: Depends on all user story phases complete

### User Story Dependencies

| Story | Depends On | Blocks |
|---|---|---|
| US1 (P1) | Phase 2 complete | US2 |
| US2 (P2) | US1 composable (T014) | US3 |
| US3 (P3) | US2 detail panel (T021) | US4 |
| US4 (P4) | US3 detail panel (T029) | — |
| US5 (P5) | Phase 2 (T002) | — |
| US6 (P6) | Phase 2 (T003, T009) | — |

### Within Each Story

- `[P]` tasks within a phase can start simultaneously
- Backend (Go) before `wails generate module` before frontend binding imports
- Composable before page before component wiring

---

## Parallel Execution Examples

### Phase 2 (Foundational)
```
Run in parallel:
  T003  internal/settings/settings.go
  T004  internal/settings/settings_test.go
  T005  internal/github/pr.go
  T006  internal/github/pr_test.go
Then sequential:
  T007  internal/github/comments.go
  T008  internal/github/comments_test.go
  T009  app.go struct extension
  T010  App.vue navigation extension
```

### Phase 3 (US1)
```
Sequential:
  T011  app.go LoadPullRequest
  T012  app.go GetCommentThreads + GetThread
  T013  wails generate module
Then parallel:
  T014  useReview.ts composable
  T015  CommentSummaryList.vue
Then sequential:
  T016  ReviewPage.vue
  T017  PRPage.vue click handler
  T018  go test
```

### Phases 7 + 8 (US5 + US6)
```
Can run in parallel with Phase 6 (US4):
  Phase 7: T036 PRStatusBanner.vue + T037 wire in ReviewPage
  Phase 8: T038 app.go settings bindings → T039 wails gen → T040 SettingsPage.vue
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (T011–T018)
4. **STOP and VALIDATE**: Navigate to a PR, verify comment summary list loads
5. Demo read-only review summary

### Incremental Delivery

1. Setup + Foundational → `go test ./...` green
2. US1 → Clickable PR list navigates to comment summary
3. US2 → Full diff-context navigation
4. US3 → Reply + resolve/unresolve (full interactive review)
5. US4 → Suggestion commits
6. US5 + US6 → Status banner + ignored-commenters settings
7. Polish → Clean lint + coverage + build

---

## Notes

- `[P]` = different files, no intra-phase dependencies
- Each user story is independently deliverable after its phase
- Run `wails generate module` after every `app.go` signature change (T013, T027, T033, T039)
- All `httptest` fixture JSON files go in `tests/fixtures/graphql/`
- `go-keyring` and `settings.json` are unaffected by this feature — no changes to auth flow
- `wails dev` auto-regenerates bindings on `app.go` save in development mode
