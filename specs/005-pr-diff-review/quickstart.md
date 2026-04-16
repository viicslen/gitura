# Quickstart: PR Diff Review

**Feature**: 005-pr-diff-review  
**Date**: 2026-04-01

A guide for implementing this feature. Covers the key integration points, non-obvious patterns, and gotchas.

---

## Prerequisites

- Branch `005-pr-diff-review` already checked out
- Go 1.25, `wails` CLI, and Node.js installed
- Familiar with existing `app.go` pattern and composable pattern

---

## Step 1: Add new model types

Add all new Go types to `internal/model/model.go`. Add them after the existing types:

```go
// PRFileDTO, DiffLineDTO, DiffHunkDTO, ParsedDiffDTO,
// DraftCommentDTO, PendingReviewDTO, ReviewSubmitDTO, ReviewSubmitResult
```

See `specs/005-pr-diff-review/data-model.md` for full struct definitions.

After adding types, run `wails generate module` to regenerate TypeScript bindings in `wailsjs/`.

---

## Step 2: Implement diff fetching (`internal/github/diff.go`)

Create `internal/github/diff.go` with two exported functions:

### `ListPRFiles`
```go
func ListPRFiles(ctx context.Context, client *github.Client, owner, repo string, number int) ([]*github.CommitFile, error)
```
Paginates through `PullRequestsService.ListFiles` until all files are collected.

### `ParseUnifiedDiff`
```go
func ParseUnifiedDiff(filename, status, patch string) model.ParsedDiffDTO
```
Parses the raw unified diff text into `ParsedDiffDTO`. This is the critical function that must reach 90% test coverage.

**Parsing rules**:
1. Split `patch` by `\n`
2. Lines starting with `@@` → new hunk; parse `@@ -old_start,old_lines +new_start,new_lines @@` with regex `^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`
3. Lines starting with `-` → `DiffLineDelete`; increment `oldLineNo`, leave `NewNo: 0`
4. Lines starting with `+` → `DiffLineAdd`; increment `newLineNo`, leave `OldNo: 0`
5. Lines starting with ` ` → `DiffLineContext`; increment both
6. Lines starting with `\` → skip (e.g., `\ No newline at end of file`)
7. Empty patch string → `IsBinary: true`, empty hunks

---

## Step 3: Implement review submission (`internal/github/review_submit.go`)

Create `internal/github/review_submit.go` with:

### `CreatePendingReview`
```go
func CreatePendingReview(ctx context.Context, client *github.Client, owner, repo string, number int, commitSHA string, comment model.DraftCommentDTO) (int64, error)
```
Calls `PullRequestsService.CreateReview` with `Event: ""` (pending) and the first comment. Returns the new `review_id`.

### `AddCommentToPendingReview`
```go
func AddCommentToPendingReview(ctx context.Context, client *github.Client, owner, repo string, number int, reviewID int64, comment model.DraftCommentDTO) error
```
Uses raw HTTP — `POST /repos/{owner}/{repo}/pulls/{number}/reviews/{reviewID}/comments`. Uses `client.Client()` to get the underlying `*http.Client` with auth. See existing raw HTTP pattern in `internal/github/`.

### `SubmitReview`
```go
func SubmitReview(ctx context.Context, client *github.Client, owner, repo string, number int, reviewID int64, req model.ReviewSubmitDTO) (model.ReviewSubmitResult, error)
```
Calls `PullRequestsService.SubmitReview`.

### `DeletePendingReview`
```go
func DeletePendingReview(ctx context.Context, client *github.Client, owner, repo string, number int, reviewID int64) error
```
Calls `PullRequestsService.DeletePendingReview`.

---

## Step 4: Add Wails bindings (`app.go`)

Add new fields to `App` struct:
```go
pendingReviewID int64
pendingComments []model.DraftCommentDTO
prFilesCache    []*github.CommitFile
prCommitSHA     string
```

Populate `prCommitSHA` inside `LoadPullRequest` from `pr.GetHead().GetSHA()`.

Add 6 new methods following the exact signatures in `specs/005-pr-diff-review/contracts/wails-bindings.md`.

Run `wails generate module` after adding all methods.

---

## Step 5: Modify `ReviewPage.vue`

Add view toggle at the top of the page:

```vue
<script setup lang="ts">
const prView = ref<'conversation' | 'files'>('conversation')
</script>

<template>
  <div class="flex flex-col h-full">
    <ViewToggle v-model="prView" :options="[
      { value: 'conversation', label: 'Conversation' },
      { value: 'files', label: 'Files changed' },
    ]" />
    <ConversationView v-if="prView === 'conversation'" ... />
    <DiffReviewView v-else-if="prView === 'files'" ... />
  </div>
</template>
```

The existing conversation content moves into a `ConversationView` component (or stays inline with `v-if`). Both views are preserved with `v-show` or `<KeepAlive>` to satisfy FR-005 (state preservation on tab switch).

---

## Step 6: Build frontend components

Build in this order (each depends on the previous):

1. **`ViewToggle.vue`** — uses `shadcn-vue` `Tabs` component; accepts `modelValue` + `options[]`
2. **`DiffFileSidebar.vue`** — file list; emits `select(path)` event; shows status badges
3. **`DiffLine.vue`** — single line with hover affordance for comment trigger
4. **`DiffHunk.vue`** — renders paired left/right columns of `DiffLineDTO[]`
5. **`InlineCommentForm.vue`** — anchored form; emits `submit-immediate` and `submit-draft`
6. **`DiffFileView.vue`** — orchestrates hunks + comment forms for one file
7. **`DiffReviewView.vue`** — top-level; integrates sidebar + file view + submit panel
8. **`useDiffReview.ts`** — composable wrapping all `App.*` diff calls

---

## Key Gotchas

### Wails bindings regeneration
Run `wails generate module` every time you add or change methods on `App`. Without this, the frontend TypeScript types will be stale and callers will fail at runtime.

### Raw HTTP for pending review comments
The second (and subsequent) draft comment must use raw HTTP — go-github does not expose `POST .../reviews/{id}/comments`. Use the same pattern as existing raw HTTP calls in the codebase. The authenticated HTTP client is available via `a.ghClient.Client()`.

### Comfort-fade vs. position
Do NOT use the `position` field (legacy diff position integer). Use `line` + `side` (comfort-fade style). Mixing them causes go-github to return `ErrMixedCommentStyles` before the API call is made.

### Binary files
`CommitFile.Patch` is `nil` for binary files. `ParseUnifiedDiff` should detect this (`patch == ""` or `patch == nil`) and return `ParsedDiffDTO{IsBinary: true}` — no hunks, no error.

### Pending review state on reload
`pendingReviewID` is in-memory only. If the app restarts, the pending review still exists on GitHub but the app loses track of it. This is an accepted limitation for this iteration (out of scope to persist).

### Side-by-side pairing
Context lines appear on both left and right. For delete/add sequences, pair them row-by-row (first delete + first add in the same table row). Surplus deletes or adds fill the opposing cell with an empty styled placeholder.

### Test coverage requirement
`internal/github/diff.go` (specifically `ParseUnifiedDiff`) must reach **90% line coverage** per the constitution. Write table-driven tests covering: normal hunks, pure-add files, pure-delete files, renames, binary files, files with `\ No newline at end of file`, multi-hunk files.

---

## Running locally

```bash
# Backend tests (must pass before frontend work)
go test ./internal/github/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep diff  # must be ≥ 90%

# Full test suite
go test ./...

# Dev server (hot reload)
wails dev
```
