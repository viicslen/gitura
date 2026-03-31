# Quickstart: PR Deep Review Workflow

**Feature**: 004-pr-review-workflow  
**Branch**: `004-pr-review-workflow`  
**Date**: 2026-03-31

This guide gives an implementer the ordered build sequence, critical contracts, and key
gotchas. Read `research.md` for decision rationale and `contracts/wails-bindings.md` for
full method signatures.

---

## Prerequisites

- Branch `004-pr-review-workflow` checked out
- `wails dev` runs without error (existing auth + PR list works)
- `go test ./...` passes
- Familiar with spec 001 data-model.md (all entities are inherited)

---

## Build Order

Build in this sequence — each step depends on the one before.

### Step 1 — Model extension (`internal/model/model.go`)

Add `IsDraft bool \`json:"is_draft"\`` to `PullRequestSummary`. This unblocks all
downstream steps that use the full PR summary.

```go
type PullRequestSummary struct {
    // ... existing fields ...
    IsDraft  bool   `json:"is_draft"`
}
```

Verify: `go build ./...` passes.

---

### Step 2 — Settings package (`internal/settings/settings.go`)

Implement ignored-commenter persistence. This is a standalone package with no GitHub
API dependency — build and test it first.

**Exports**:
```go
func Load() ([]model.IgnoredCommenterDTO, error)
func Save(entries []model.IgnoredCommenterDTO) error
func ConfigDir() (string, error)  // returns os.UserConfigDir()/gitura
```

**Key details**:
- Create `gitura/` subdir if absent (`os.MkdirAll`)
- Atomic write: write to `ignored_commenters.json.tmp` then `os.Rename`
- Empty list → write `[]` (not omit file)

**Test file**: `internal/settings/settings_test.go`  
Coverage target: 90%

---

### Step 3 — PR metadata fetch (`internal/github/pr.go`)

Fetch a single PR from REST. This is a thin wrapper around go-github.

```go
// FetchPR fetches PR metadata for a single pull request.
func FetchPR(ctx context.Context, client *github.Client, owner, repo string, number int) (*model.PullRequestSummary, error)
```

Maps `github.PullRequest` → `model.PullRequestSummary`. Sets `IsDraft` from
`pr.GetDraft()`. Returns `notfound:` error on HTTP 404.

**Test file**: `internal/github/pr_test.go` — use `httptest` fixture.

---

### Step 4 — GraphQL review thread fetch (`internal/github/comments.go`)

This is the most complex new Go file. Fetches all review threads via GraphQL and maps
them to `[]model.CommentThreadDTO`.

```go
// FetchReviewThreads fetches all PR review threads via GitHub GraphQL API.
// Pagination is handled internally. Progress is reported via progressFn.
func FetchReviewThreads(
    ctx context.Context,
    token string,
    owner, repo string,
    number int,
    progressFn func(loaded, total int),
) ([]model.CommentThreadDTO, error)
```

**GraphQL endpoint**: `POST https://api.github.com/graphql`  
**Auth header**: `Authorization: bearer {token}`  
**Request body**: `{"query": "...", "variables": {"owner":..., "repo":..., "number":..., "after":...}}`  
**Pagination**: Loop until `pageInfo.hasNextPage == false`. Pass `endCursor` as `after`.

**Mapping** (GraphQL node → `CommentThreadDTO`):
- `thread.ID` → `NodeID`
- `thread.IsResolved` → `Resolved`
- First comment's `DatabaseId` → `RootID`
- First comment's `Path` / `Line` → `Path` / `Line` on the DTO
- Each `graphQLComment` → `CommentDTO` (see data-model.md for GraphQL struct shapes)

**IsSuggestion detection**: Check if `comment.Body` contains ` ```suggestion` (backtick-suggestion).

**InReplyToID**: Set from `comment.ReplyTo.DatabaseId` (0 if `ReplyTo` is nil).

**Test file**: `internal/github/comments_test.go`  
Use `httptest.NewServer` to serve fixture JSON responses. Test: single page, multi-page,
empty result, error response.  
Coverage target: 90%

---

### Step 5 — Resolve/unresolve mutations (`internal/github/resolve.go`)

```go
// ResolveThread executes the resolveReviewThread GraphQL mutation.
func ResolveThread(ctx context.Context, token, threadNodeID string) error

// UnresolveThread executes the unresolveReviewThread GraphQL mutation.
func UnresolveThread(ctx context.Context, token, threadNodeID string) error
```

Both use the same GraphQL transport as Step 4. Return `github:` prefixed error on
non-200 status or GraphQL `errors` array non-empty.

**Test file**: `internal/github/resolve_test.go`  
Test: success, graphQL error response, HTTP error.

---

### Step 6 — Suggestion commit (`internal/github/suggestion.go`)

```go
// CommitSuggestion applies a suggestion patch and commits it to the PR branch.
func CommitSuggestion(
    ctx context.Context,
    client *github.Client,
    owner, repo string,
    headBranch string,
    comment model.CommentDTO,
    commitMessage string,
) (model.SuggestionCommitResult, error)
```

**Steps** (within this function):
1. Parse suggestion block from `comment.Body` → extract `replacementLines`
2. Parse hunk header from `comment.DiffHunk` to identify the line range being replaced
3. GET `/repos/{owner}/{repo}/contents/{comment.Path}?ref={headBranch}` → file + blob SHA
4. Apply patch: replace lines `[startLine, endLine]` in file content with `replacementLines`
5. PUT `/repos/{owner}/{repo}/contents/{comment.Path}` — body: `{message, content (base64), sha (blob SHA), branch}`
6. Return `SuggestionCommitResult{CommitSHA, HTMLURL}`

**SHA conflict**: If step 3 blob SHA is stale (file was modified externally), step 5
returns HTTP 409 — return `github:conflict` error.

**Error**: If `comment.IsSuggestion == false`, return `validation:not-a-suggestion`.

**Test file**: `internal/github/suggestion_test.go`  
Test: successful commit, SHA conflict (409), not-a-suggestion, diff parse edge cases.

---

### Step 7 — Wire up `app.go`

Add to `App` struct:
```go
type App struct {
    // ... existing fields ...
    ignoredCommenters []model.IgnoredCommenterDTO
}
```

Add all new Wails-bound methods (see `contracts/wails-bindings.md` for full signatures):
- `LoadPullRequest` — calls `pr.FetchPR` + `comments.FetchReviewThreads`; applies
  ignored filter; caches in `a.prCache` + `a.threads`; emits `pr:load-progress`
- `GetCommentThreads` — returns filtered view of `a.threads`
- `GetThread` — looks up single thread from `a.threads`
- `ReplyToComment` — calls go-github REST; appends to cached thread
- `ResolveThread` — calls `resolve.ResolveThread`; updates `a.threads` on success
- `UnresolveThread` — calls `resolve.UnresolveThread`; updates `a.threads`
- `CommitSuggestion` — calls `suggestion.CommitSuggestion`
- `GetIgnoredCommenters` — returns `a.ignoredCommenters` (loaded from disk lazily on
  first call)
- `AddIgnoredCommenter` / `RemoveIgnoredCommenter` — mutate `a.ignoredCommenters`
  and call `settings.Save`

After adding methods: `wails generate module`

---

### Step 8 — Frontend composable (`frontend/src/composables/useReview.ts`)

Implement `useReview(prItem: ReviewLoadInput)` composable:

**Reactive state**: `threads`, `prSummary`, `loading`, `error`, `showResolved`,
`currentIndex` (see data-model.md `ReviewNavState`)

**Computed**: `queue`, `currentThread`, `isAtEnd`, `canGoBack`, `canGoForward`

**Actions**: `loadPR`, `goNext`, `goPrev`, `toggleShowResolved`, `resolveThread`,
`unresolveThread`, `reply`, `commitSuggestion`

**Show-resolved toggle rule**: Reset `currentIndex` to 0 when toggling, clamp to
`queue.length - 1` if current index is now out of bounds.

**Listen for progress event**:
```typescript
import { EventsOn } from '@/wailsjs/runtime/runtime'
EventsOn('pr:load-progress', (data: { loaded: number; total: number }) => {
  loadProgress.value = data
})
```

---

### Step 9 — Frontend pages & components

Build in this order (each depends on composable from Step 8):

1. **`PRStatusBanner.vue`** — simplest: shows banner if `prSummary.state !== 'open'`
   or `prSummary.is_draft`. Props: `{ summary: PullRequestSummary | null }`.

2. **`DiffHunkView.vue`** — render `diffHunk` string in `<pre>` with monospace font.
   Highlight the target line (passed as `highlightLine?: number` prop). Lines starting
   with `+` are additions (green), `-` deletions (red), ` ` context (neutral).

3. **`SuggestionBlock.vue`** — parse and display suggestion from `CommentDTO.body`.
   Shows "Commit suggestion" button. Emits `commit` event with `{ commentId, message }`.

4. **`ReplyComposer.vue`** — `<textarea>` + submit button. Props: `threadRootId`.
   Emits `reply-sent`. On error, keeps draft text. Disable submit while in-flight.

5. **`CommentDetailPanel.vue`** — full thread view for `currentThread`. Shows all
   comments, `DiffHunkView`, `SuggestionBlock` (if applicable), `ReplyComposer`,
   and Resolve/Unresolve button. Props: `{ thread: CommentThreadDTO | null }`.

6. **`CommentSummaryList.vue`** — scrollable list of `queue` threads. Each row shows:
   author avatar + login, file path, 200-char body excerpt, "Resolved" badge if resolved.
   Clicking a row sets `currentIndex`. Highlight active row. Props: `{ queue, currentIndex }`.

7. **`ReviewPage.vue`** — top-level page. Layout: left panel `CommentSummaryList` +
   right panel `CommentDetailPanel`. Top bar: PR title, back button, `PRStatusBanner`,
   `showResolved` toggle, "X / Y comments" counter. Bottom bar: Prev / Next nav buttons +
   "All comments reviewed" end-state message when `isAtEnd && queue.length === 0 || isAtEnd`.

   Props: `{ prItem: ReviewLoadInput }`  
   Emits: `close-review`

---

### Step 10 — App.vue integration

Extend `App.vue`:
1. `currentPage` type: `'pr' | 'settings' | 'review'`
2. `selectedPRItem = ref<ReviewLoadInput | null>(null)`
3. Wrap `<PRPage>` in `<KeepAlive>`:
   ```html
   <KeepAlive>
     <PRPage
       v-if="currentPage === 'pr'"
       @open-review="handleOpenReview"
     />
   </KeepAlive>
   <ReviewPage
     v-if="currentPage === 'review' && selectedPRItem"
     :pr-item="selectedPRItem"
     @close-review="handleCloseReview"
   />
   ```
4. `handleOpenReview(item: ReviewLoadInput)`: set `selectedPRItem = item`, `currentPage = 'review'`
5. `handleCloseReview()`: set `currentPage = 'pr'` (PRPage is already alive)

---

### Step 11 — PRPage.vue: emit open-review

In `PRPage.vue`, add a click handler to each PR row that emits `open-review` with a
`ReviewLoadInput` constructed from the `PRListItem`:

```typescript
function openReview(item: PRListItem): void {
  emit('open-review', {
    number: item.number,
    owner:  item.owner,
    repo:   item.repo,
    title:  item.title,
    htmlUrl: item.html_url,
    isDraft: item.is_draft,
  } satisfies ReviewLoadInput)
}
```

---

### Step 12 — SettingsPage.vue: ignored commenters

Add an ignored-commenter management section to the existing `SettingsPage.vue`.
Shows current list with "Remove" buttons. Input field + "Add" button. Uses
`GetIgnoredCommenters`, `AddIgnoredCommenter`, `RemoveIgnoredCommenter` bindings.

---

## Key Gotchas

| Gotcha | Mitigation |
|---|---|
| GraphQL `line` field is nullable — can be `null` for outdated comments | Use `*int` in Go struct; default to 0 if nil; frontend handles 0 as "no specific line" |
| `KeepAlive` caches ALL matching components — ensure only one `PRPage` instance | Use `include` prop if needed: `<KeepAlive :include="['PRPage']">` |
| Suggestion commit: diff hunk may have no `@@` header (reply-level comments) | Validate hunk has `@@` header; return descriptive error if missing |
| `wails generate module` must run after each `app.go` signature change | Add to dev workflow; CI should verify `wailsjs/` is up to date |
| GraphQL `databaseId` for comments is an `Int` in the schema but Go needs `int64` | Use `int64` in Go structs; GraphQL Int is 32-bit but GitHub IDs are 64-bit |
| `os.UserConfigDir()` returns different paths per OS | Test with a mock `configDirFn` injected via function parameter in `settings.go` |

---

## Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep -E "(internal/github|internal/settings)"

# Frontend (if Vitest is configured)
cd frontend && npm test
```

## Linting

```bash
golangci-lint run
```

## Dev server

```bash
wails dev
```

After `LoadPullRequest` is wired, the review page is accessible by clicking any PR row
in the PR list.
