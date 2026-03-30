---
description: "Task list for PR Review UI — 001-pr-review-ui"
---

# Tasks: PR Review UI

**Input**: Design documents from `specs/001-pr-review-ui/`
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/wails-bindings.md ✅

**Tests**: Not explicitly requested. Unit tests are included only for Go internal packages
per the constitution (80%/90% coverage requirement). No frontend test tasks generated.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US6)

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Initialize the Wails + Vue + Go project with all tooling in place.

- [x] T001 Initialize Wails v2 project with `wails init -n gitura -t vue-ts` and commit generated scaffold
- [x] T002 [P] Add Go module dependencies: `go get github.com/google/go-github/v67 github.com/zalando/go-keyring github.com/stretchr/testify`
- [x] T003 [P] Install frontend dependencies and init shadcn-vue: `npm install tailwindcss @tailwindcss/vite clsx tailwind-merge` then `npx shadcn-vue@latest init` with style=new-york, cssVariables=true
- [x] T004 [P] Configure `frontend/vite.config.ts` with `@tailwindcss/vite` plugin, `@` path alias, and `server.port = 5173`
- [x] T005 [P] Replace `frontend/src/style.css` with `@import "tailwindcss";` and verify Tailwind v4 builds
- [x] T006 [P] Create `.golangci.yml` at repo root enforcing `gofmt`, `golint`, `go vet`, cyclomatic complexity ≤ 10
- [x] T007 [P] Configure `wails.json`: set `wailsjsdir: "./frontend/src"`, `frontend:dev:serverUrl: "auto"`, `assetdir: "frontend/dist"`
- [x] T008 [P] Create `tests/fixtures/` directory and add `.gitkeep`; create `tests/integration/` and `tests/unit/` directories

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before any user story can be implemented.
**CRITICAL**: All Phase 3+ work is blocked until this phase is complete.

- [x] T009 Create `internal/model/model.go` with all shared DTOs: `PullRequestSummary`, `CommentThreadDTO`, `CommentDTO`, `AuthState`, `DeviceFlowInfo`, `PollResult`, `IgnoredCommenterDTO`, `SuggestionCommitResult` — all fields must have `json:"..."` tags
- [x] T010 Create `internal/keyring/keyring.go` implementing `SaveToken(token string) error`, `LoadToken() (string, error)`, `DeleteToken() error` using `github.com/zalando/go-keyring` with service key `com.gitura.app`
- [x] T011 Create `internal/keyring/keyring_test.go` with unit tests for all three functions using a mock keyring interface
- [x] T012 Create `internal/github/client.go` implementing `NewClient(token string) *github.Client` factory returning an authenticated `go-github` client; document exported function
- [x] T013 Create `app.go` with `App` struct, `startup(ctx)`, `domReady(ctx)`, `beforeClose(ctx)` lifecycle methods, and a stub `main.go` calling `wails.Run()` — no business methods yet
- [x] T014 Add shadcn-vue components needed by all stories: `npx shadcn-vue@latest add button card badge separator scroll-area toast dialog`; verify files appear in `frontend/src/components/ui/`
- [x] T015 Create `frontend/src/App.vue` with simple page-level routing logic: show `AuthPage` when not authenticated, `PRPage` when authenticated, with a nav link to `SettingsPage`
- [x] T016 [P] Create `frontend/src/pages/AuthPage.vue` as a skeleton (empty card, no logic yet)
- [x] T017 [P] Create `frontend/src/pages/PRPage.vue` as a skeleton (empty card, no logic yet)
- [x] T018 [P] Create `frontend/src/pages/SettingsPage.vue` as a skeleton (empty card, no logic yet)

**Checkpoint**: `wails dev` starts, shows an empty authenticated/unauthenticated shell. `go build ./...` passes. `go vet ./...` passes.

---

## Phase 3: Authentication — US0 (Foundational Auth, blocks US1+)

**Goal**: Users can authenticate with GitHub via OAuth Device Flow; auth state persists
across app restarts.

**Independent Test**: Launch app without stored token → device flow UI appears with a
`user_code` → authenticate in browser → app transitions to PR input screen → restart app
→ app skips auth screen and goes directly to PR input screen.

### Implementation

- [x] T019 Create `internal/auth/deviceflow.go` implementing `StartDeviceFlow(clientID string) (model.DeviceFlowInfo, error)` — POSTs to `https://github.com/login/device/code`, returns user code and verification URI
- [x] T020 Create `internal/auth/poll.go` implementing `PollForToken(deviceCode, clientID string, interval int) (model.PollResult, error)` — POSTs to `https://github.com/login/oauth/access_token`, returns status `pending/complete/expired/error`
- [x] T021 Create `internal/auth/auth_test.go` with fixture-based tests for `StartDeviceFlow` and `PollForToken` covering success, pending, expired, and network-error cases; fixtures in `tests/fixtures/auth/`
- [x] T022 Bind `StartDeviceFlow() (model.DeviceFlowInfo, error)` method on `App` struct in `app.go` — delegates to `internal/auth`; stores device code in-memory
- [x] T023 Bind `PollDeviceFlow() (model.PollResult, error)` method on `App` struct — calls `internal/auth.PollForToken`; on `complete` status saves token via `internal/keyring.SaveToken`; emits `auth:device-flow-complete` Wails event
- [x] T024 Bind `GetAuthState() (model.AuthState, error)` method on `App` struct — loads token from keyring; if present calls GitHub `/user` endpoint and returns `IsAuthenticated: true` with login and avatar
- [x] T025 Bind `Logout() error` method on `App` struct — calls `internal/keyring.DeleteToken` and clears in-memory state
- [x] T026 Create `frontend/src/composables/useAuth.ts` exposing `authState`, `startDeviceFlow()`, `pollDeviceFlow()`, `logout()`; imports from `wailsjs/go/main/App`; subscribes to `auth:device-flow-complete` event via `wailsjs/runtime`
- [x] T027 Implement `frontend/src/pages/AuthPage.vue` — shows device flow user code in a card, "Open GitHub" button, polling spinner, success/error states; uses `useAuth` composable and shadcn-vue Button/Card components
- [x] T028 Wire `frontend/src/App.vue` routing: call `GetAuthState` on mount; navigate to `AuthPage` if not authenticated, `PRPage` if authenticated; handle `auth:device-flow-complete` event to transition

**Checkpoint**: Full device flow works end-to-end. `go test ./internal/auth/...` passes. `golangci-lint run ./internal/auth/...` passes.

---

## Phase 4: User Story 1 — Browse and Triage PR Comments (Priority: P1)

**Goal**: User can input a PR (owner/repo/number), load it, and see all review comments
in a filterable summary list.

**Independent Test**: Enter any public or private (with token access) PR → summary list
renders all review comment threads with author, file path, and body excerpt. Ignored
commenters are absent from the list.

### Implementation

- [ ] T029 Create `internal/github/pr.go` implementing `LoadPullRequest(ctx, client, owner, repo string, number int) (model.PullRequestSummary, error)` — fetches PR via go-github, maps to `PullRequestSummary` DTO
- [ ] T030 Create `internal/github/comments.go` implementing `GetCommentThreads(ctx, client, owner, repo string, number int) ([]model.CommentThreadDTO, error)` — paginates through all review comments (`PerPage: 100`), groups by thread, applies ignored-commenter filter, detects suggestion blocks
- [ ] T031 [P] Create `tests/fixtures/github/pr_get.json` and `tests/fixtures/github/comments_list.json` by recording responses from GitHub API (or manually crafting representative fixtures with ≥ 3 threads, mixed resolved/unresolved, one suggestion)
- [ ] T032 Create `internal/github/pr_test.go` and `internal/github/comments_test.go` with httptest fixture tests covering: normal load, zero comments (empty-state), pagination (simulate 2 pages), ignored-commenter filtering, suggestion detection
- [ ] T033 Bind `LoadPullRequest(owner, repo string, number int) (model.PullRequestSummary, error)` on `App` struct in `app.go` — delegates to `internal/github/pr.go`; caches result in `App` fields; emits `pr:load-progress` events during paginated comment fetch
- [ ] T034 Bind `GetCommentThreads(includeResolved bool) ([]model.CommentThreadDTO, error)` on `App` struct — returns cached threads; applies resolved filter based on `includeResolved` param
- [ ] T035 Bind `GetThread(rootID int64) (model.CommentThreadDTO, error)` on `App` struct — returns single thread from cache by root ID; returns `notfound:` error if absent
- [ ] T036 Create `frontend/src/composables/usePR.ts` exposing: `prSummary`, `threads`, `loading`, `error`, `loadPR(owner, repo, number)`, `refreshThreads(includeResolved)`, `currentThread`, `goToThread(rootID)`; imports from `wailsjs/go/main/App`
- [ ] T037 Create `frontend/src/components/PRInput.vue` — input fields for owner, repo name, PR number (or URL parser auto-splitting), Load button, loading state, error display; uses shadcn-vue Input/Button/Card
- [ ] T038 Create `frontend/src/components/CommentList.vue` — renders a scrollable list of `CommentThreadDTO` items; each row shows author avatar + login, file path + line, resolved badge, body excerpt (150 chars); empty-state card when list is empty; "Show resolved" toggle; click row → emits `select-thread` event
- [ ] T039 Implement `frontend/src/pages/PRPage.vue` — composes `PRInput` + `CommentList`; on PR load success shows list; handles `pr:load-progress` event with a progress bar; handles network error with retry button

**Checkpoint**: `go test ./internal/github/...` passes. User can load a real PR and see the full comment list with no console errors.

---

## Phase 5: User Story 2 — Navigate Comments One-by-One (Priority: P2)

**Goal**: User can step through comment threads sequentially with next/previous controls,
seeing full text and diff hunk per thread.

**Independent Test**: With a loaded PR, press Next/Previous to step through all unresolved
threads; each shows full body, diff hunk, author, and reply thread; "all reviewed" message
appears after last thread.

### Implementation

- [ ] T040 Extend `frontend/src/composables/usePR.ts` to add sequential navigation state: `navigationQueue` (filtered, ordered thread list), `currentIndex`, `navigateNext()`, `navigatePrev()`, `isAtEnd` computed; skips resolved threads unless `includeResolved` is set
- [ ] T041 Create `frontend/src/components/CommentDetail.vue` — full-page detail view for one `CommentThreadDTO`: renders diff hunk in a monospaced code block, root comment + replies in chronological order, each reply showing avatar + login + body + timestamp, Next/Previous navigation buttons, keyboard shortcut hints (←/→), "all reviewed" empty state at end
- [ ] T042 Integrate `CommentDetail` into `frontend/src/pages/PRPage.vue` — toggle between list view and detail view; selecting a thread in `CommentList` navigates to detail at that index; breadcrumb/back link returns to list
- [ ] T043 Add keyboard navigation support in `PRPage.vue`: `ArrowRight`/`j` → next, `ArrowLeft`/`k` → prev, `Escape` → back to list; use `document.addEventListener` in `onMounted` / `onUnmounted`

**Checkpoint**: Tab/keyboard navigation through comments works. No regressions to list view (US1 still passes its independent test).

---

## Phase 6: User Story 3 — Reply to a Review Comment (Priority: P2)

**Goal**: User can compose and submit a reply to any thread; reply is posted to GitHub
and reflected immediately in the thread.

**Independent Test**: Submit a reply to one thread → reply appears in thread within the
app → verify reply also appears on GitHub PR via browser.

### Implementation

- [ ] T044 Add `ReplyToComment(ctx, client, owner, repo string, threadRootID int64, body string) (model.CommentDTO, error)` to `internal/github/comments.go` — uses go-github `PullRequestsService.CreateCommentReply`; validates non-empty body; maps response to `CommentDTO`
- [ ] T045 Add `tests/fixtures/github/reply_post.json` fixture; extend `internal/github/comments_test.go` with tests for `ReplyToComment` covering: success, empty body validation error, network error, permission error
- [ ] T046 Bind `ReplyToComment(threadRootID int64, body string) (model.CommentDTO, error)` on `App` struct in `app.go` — delegates to `internal/github`; on success appends new `CommentDTO` to in-memory thread cache
- [ ] T047 Create `frontend/src/components/ReplyForm.vue` — textarea (shadcn-vue Textarea), Submit button with loading state, character count, empty-body validation message, preserved text on submission failure, error alert with retry option
- [ ] T048 Integrate `ReplyForm` into `CommentDetail.vue` — shown below thread replies; on successful submit calls `usePR` to append reply to current thread state without full reload

**Checkpoint**: Replies post to GitHub. Empty replies are blocked client-side and server-side. Text is preserved on network failure.

---

## Phase 7: User Story 4 — Resolve a Review Comment (Priority: P2)

**Goal**: User can resolve/unresolve a thread; resolved state is reflected on GitHub and
in the app; resolved threads are excluded from navigation by default.

**Independent Test**: Resolve one thread → it becomes visually distinct in list view →
it is skipped in next/prev navigation → "Show resolved" toggle reveals it with a resolved
indicator → unresolve it → it re-enters navigation queue.

### Implementation

- [ ] T049 Add `ResolveThread(ctx, client, owner, repo string, threadRootID int64) error` and `UnresolveThread(...)` to `internal/github/comments.go` — uses GitHub GraphQL `resolveReviewThread` / `unresolveReviewThread` mutations (go-github does not expose these; use `client.Client().Do()` with raw GraphQL POST to `https://api.github.com/graphql`)
- [ ] T050 Add `tests/fixtures/github/resolve_thread.json` and `unresolve_thread.json` fixtures; add tests for both mutations covering success and permission-denied error cases
- [ ] T051 Bind `ResolveThread(threadRootID int64) error` and `UnresolveThread(threadRootID int64) error` on `App` struct — on success update resolved state of cached thread optimistically
- [ ] T052 Update `CommentList.vue` to visually distinguish resolved threads: use a muted/strikethrough style and a "Resolved" badge (shadcn-vue Badge); "Show resolved" toggle filters the list reactively
- [ ] T053 Add Resolve/Unresolve button to `CommentDetail.vue` — calls `ResolveThread`/`UnresolveThread` via `usePR`; updates local state on success; on resolution during navigation auto-advances to next unresolved thread
- [ ] T054 Extend `usePR.ts` `navigationQueue` computed to exclude resolved threads unless `includeResolved` is toggled; add `toggleShowResolved()` action

**Checkpoint**: Resolve/unresolve round-trips to GitHub. `go test ./internal/github/...` still passes. Navigation queue updates reactively.

---

## Phase 8: User Story 5 — Commit a Suggested Change (Priority: P3)

**Goal**: User can accept and commit a GitHub suggestion block directly from the app.

**Independent Test**: On a PR with a suggestion comment, click "Commit suggestion" →
confirm dialog appears → commit succeeds → commit SHA displayed → branch updated on GitHub.

### Implementation

- [ ] T055 Create `internal/github/suggestion.go` implementing `CommitSuggestion(ctx, client, owner, repo, headBranch, headSHA string, commentID int64, diffHunk, body, commitMessage string) (model.SuggestionCommitResult, error)` — parses suggestion block from comment body, reads current file via Contents API, applies replacement lines, writes back via Contents API PUT with commit message
- [ ] T056 Add `tests/fixtures/github/file_get.json` and `file_put.json` fixtures; create `internal/github/suggestion_test.go` testing: successful commit, non-suggestion comment error, merge-conflict-detected error, file-not-found error
- [ ] T057 Bind `CommitSuggestion(commentID int64, commitMessage string) (model.SuggestionCommitResult, error)` on `App` struct — delegates to `internal/github/suggestion.go`; uses cached PR head branch and SHA
- [ ] T058 Create `frontend/src/components/SuggestionBlock.vue` — renders the suggestion diff (before/after using a simple two-column diff or unified diff display), "Commit suggestion" button; shown only when `comment.is_suggestion === true`
- [ ] T059 Add commit confirmation `Dialog` (shadcn-vue Dialog) to `SuggestionBlock.vue` — shows commit message input pre-filled with `"Accept suggestion from @{author}"`, Confirm/Cancel buttons, loading state during commit, success state showing commit SHA link
- [ ] T060 Integrate `SuggestionBlock` into `CommentDetail.vue` — render it between the diff hunk and the reply form when `comment.is_suggestion` is true; update thread state on successful commit (mark comment resolved)

**Checkpoint**: `go test ./internal/github/...` passes including suggestion tests. Suggestion commits appear on the PR branch on GitHub.

---

## Phase 9: User Story 6 — Manage Ignored Commenters (Priority: P3)

**Goal**: User maintains a persistent ignored-commenter list; comments from those users
are hidden across all views without requiring an app restart.

**Independent Test**: Add a username via Settings → return to loaded PR → that user's
comments are absent from list and navigation → remove username → comments reappear.

### Implementation

- [ ] T061 Create `internal/settings/settings.go` implementing: `Load() ([]model.IgnoredCommenterDTO, error)`, `Add(login string) error`, `Remove(login string) error` — persists to `os.UserConfigDir()/gitura/ignored_commenters.json`; `Add` silently de-dupes; `Remove` is a no-op if not present
- [ ] T062 Create `internal/settings/settings_test.go` with unit tests for `Load`, `Add`, `Remove` using a temp directory; cover empty list, duplicate add, remove of absent entry
- [ ] T063 Bind `GetIgnoredCommenters() ([]model.IgnoredCommenterDTO, error)`, `AddIgnoredCommenter(login string) error`, `RemoveIgnoredCommenter(login string) error` on `App` struct — delegate to `internal/settings`; after Add/Remove, re-apply ignored filter to cached threads and return updated summary counts
- [ ] T064 Create `frontend/src/composables/useSettings.ts` exposing: `ignoredCommenters`, `loadIgnoredCommenters()`, `addIgnoredCommenter(login)`, `removeIgnoredCommenter(login)`; imports from `wailsjs/go/main/App`
- [ ] T065 Implement `frontend/src/pages/SettingsPage.vue` — section "Ignored Commenters": text input + Add button to add new username, list of current ignored usernames each with a Remove button, empty-state message when list is empty; uses shadcn-vue Input/Button/Badge/Card
- [ ] T066 Wire instant reactivity in `frontend/src/pages/PRPage.vue`: after `addIgnoredCommenter` or `removeIgnoredCommenter` returns, call `refreshThreads` to reload the comment list without full PR reload; update `prSummary.comment_count`

**Checkpoint**: `go test ./internal/settings/...` passes. Adding/removing ignored commenters takes effect without app restart.

---

## Phase 10: Polish and Cross-Cutting Concerns

**Purpose**: Error handling hardening, keyboard accessibility, performance, and
final quality gates.

- [ ] T067 [P] Add global error boundary in `frontend/src/App.vue` — catch unhandled Promise rejections from Wails binding calls; display a toast notification (shadcn-vue Toast) with error prefix categorization (`auth:`, `github:`, `validation:`, `notfound:`, `keyring:`)
- [ ] T068 [P] Add ARIA labels and roles to `CommentList.vue`, `CommentDetail.vue`, `ReplyForm.vue`, `SuggestionBlock.vue` — `aria-label` on all icon-only buttons, `role="list"` + `role="listitem"` on comment lists, `aria-live="polite"` on status messages
- [ ] T069 [P] Add loading skeletons (shadcn-vue Skeleton) to `CommentList.vue` and `CommentDetail.vue` for the PR load progress state; wire `pr:load-progress` event to show `{loaded}/{total}` count during paginated fetch
- [ ] T070 [P] Implement virtual scrolling or pagination in `CommentList.vue` for PRs with 200+ comments — use `scroll-area` with windowed rendering or add a "Load more" button revealing 50 threads at a time
- [ ] T071 Add `frontend/src/components/EmptyState.vue` — reusable component with icon slot, title, and description; use in `CommentList.vue` (no comments), `CommentDetail.vue` (all reviewed), `SettingsPage.vue` (no ignored commenters)
- [ ] T072 [P] Run `go test ./... -coverprofile=coverage.out` and verify: `internal/github/` ≥ 90%, all other packages ≥ 80%; address any gap
- [ ] T073 [P] Run `golangci-lint run ./...` and fix all reported issues
- [ ] T074 Run `quickstart.md` validation end-to-end: authenticate, load a real PR, navigate comments, reply, resolve, commit a suggestion, add/remove an ignored commenter
- [ ] T075 [P] Update `AGENTS.md` with final source tree, any commands added, and any deviations from the plan

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **Auth (Phase 3)**: Depends on Phase 2 — BLOCKS Phases 4+
- **US1 (Phase 4)**: Depends on Phase 3 (needs auth client) — BLOCKS Phases 5, 6, 7, 8
- **US2 (Phase 5)**: Depends on Phase 4 (needs loaded PR + threads)
- **US3 (Phase 6)**: Depends on Phase 4 (needs loaded PR + threads)
- **US4 (Phase 7)**: Depends on Phase 4; recommended after Phase 5 (navigation queue integration)
- **US5 (Phase 8)**: Depends on Phase 4; independent from US2/US3/US4
- **US6 (Phase 9)**: Depends on Phase 4; independent from US2–US5
- **Polish (Phase 10)**: Depends on all desired user stories complete

### User Story Dependencies After Phase 4

US2, US3, US4, US5, US6 can all be developed in parallel once Phase 4 is done.
US4 should integrate after US2 for the navigation queue changes.

### Within Each Phase

- Models → Services → Bindings → Frontend composables → Frontend components
- Fixture files before tests; tests before implementation tasks when TDD preferred
- Story complete before moving to next priority

### Parallel Opportunities

All `[P]`-marked tasks within a phase can be started concurrently.
US2, US3, US4, US5, US6 implementation tasks can proceed in parallel across developers.

---

## Parallel Execution Example: Phase 4 (US1)

```bash
# Launch in parallel (different files, no cross-dependencies):
Task: "Create internal/github/pr.go"
Task: "Create tests/fixtures/github/pr_get.json + comments_list.json"
Task: "Create frontend/src/composables/usePR.ts (skeleton)"
Task: "Create frontend/src/components/PRInput.vue"
Task: "Create frontend/src/components/CommentList.vue"

# Then sequentially (depend on the above):
Task: "Create internal/github/pr_test.go + comments_test.go" (needs fixtures + pr.go)
Task: "Bind LoadPullRequest on App struct" (needs internal/github/pr.go)
Task: "Bind GetCommentThreads on App struct" (needs internal/github/comments.go)
Task: "Implement PRPage.vue" (needs CommentList + composable)
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: Auth
4. Complete Phase 4: US1 (Browse and Triage)
5. **STOP and VALIDATE**: Load a real PR and see the comment list
6. Demo: full read-only PR comment triage is functional

### Incremental Delivery

1. Setup + Foundational + Auth → Shell ready
2. Add US1 → PR comment list (read-only MVP)
3. Add US2 → One-by-one navigation
4. Add US3 + US4 → Reply + Resolve (workflow complete)
5. Add US5 → Suggestion commits
6. Add US6 → Ignored commenters
7. Polish phase → Ready for release

### Parallel Team Strategy (if staffed)

Once Phase 4 (US1) is complete:
- Developer A: US2 (navigation)
- Developer B: US3 (reply) + US4 (resolve)
- Developer C: US5 (suggestions) + US6 (settings)

---

## Notes

- `[P]` = different files, no incomplete dependencies; safe to parallelize
- `[USN]` maps each task to its user story for traceability
- Wails bindings are auto-regenerated by `wails dev` on `app.go` changes — do not hand-edit `wailsjs/`
- GraphQL mutations (US4 resolve/unresolve) require the `repo` OAuth scope and the PR's `node_id` — retrieve node ID from initial `LoadPullRequest` response and store in cache
- Suggestion commit (US5) requires reading the current file SHA before PUT — always fetch latest file metadata immediately before applying to avoid SHA conflicts
- Commit after each task or logical group
- Stop at each phase checkpoint to validate the story works independently
