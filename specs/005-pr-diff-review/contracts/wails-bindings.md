# Wails Bindings Contract: PR Diff Review

**Feature**: 005-pr-diff-review  
**Date**: 2026-04-01

All methods are on the `App` struct in `app.go` and auto-exposed to the frontend via Wails. The frontend calls them via the generated `wailsjs/go/main/App.ts` bindings.

---

## New Methods

### `GetPRFiles() ([]model.PRFileDTO, error)`

Returns the list of files changed in the currently loaded PR. Does not include diff content. Used to populate the sidebar.

**Precondition**: `LoadPullRequest` must have been called.  
**GitHub API**: `PullRequestsService.ListFiles` (paginated; collects all pages)  
**Error cases**: No PR loaded → `"no PR loaded"` error. API failure → wrapped error.

```typescript
// Frontend call
import { GetPRFiles } from '../../wailsjs/go/main/App'
const files: main.PRFileDTO[] = await GetPRFiles()
```

---

### `GetFileDiff(path string) (model.ParsedDiffDTO, error)`

Fetches and parses the diff for a single file. Returns structured hunks and lines suitable for direct rendering. Binary files return `is_binary: true` with empty `hunks`.

**Precondition**: `LoadPullRequest` must have been called.  
**GitHub API**: Re-uses data from `ListFiles` (patch field). Calls `ListFiles` if not cached, extracts the specific file's patch.  
**Error cases**: File not found in PR → error. Binary file → returns `ParsedDiffDTO{IsBinary: true}`, no error.

```typescript
// Frontend call
import { GetFileDiff } from '../../wailsjs/go/main/App'
const diff: main.ParsedDiffDTO = await GetFileDiff('src/foo/bar.go')
```

---

### `AddDraftComment(comment model.DraftCommentDTO) (model.PendingReviewDTO, error)`

Adds a comment to the pending review batch. If no pending review exists yet, creates one on GitHub first (`CreateReview` with `Event: "PENDING"`). Subsequent calls append to the existing pending review via raw HTTP `POST .../reviews/{reviewID}/comments`.

**Precondition**: `LoadPullRequest` must have been called.  
**GitHub API**:
- First call: `PullRequestsService.CreateReview(ctx, owner, repo, number, &PullRequestReviewRequest{Event: "PENDING", Comments: [comment]})`
- Subsequent calls: `POST /repos/{owner}/{repo}/pulls/{number}/reviews/{reviewID}/comments` (raw HTTP)

Returns the updated `PendingReviewDTO` including all pending comments and the `review_id`.

**Side effect**: Sets `a.pendingReviewID` on first call.

```typescript
// Frontend call
import { AddDraftComment } from '../../wailsjs/go/main/App'
const pending: main.PendingReviewDTO = await AddDraftComment({
  path: 'src/foo/bar.go',
  body: 'Consider extracting this',
  line: 42,
  side: 'RIGHT',
})
```

---

### `PostImmediateComment(comment model.DraftCommentDTO) (model.CommentDTO, error)`

Posts a standalone inline comment immediately (not as a draft review comment). The comment is immediately visible on GitHub.

**Precondition**: `LoadPullRequest` must have been called.  
**GitHub API**: `PullRequestsService.CreateComment` (maps to `POST /pulls/{number}/comments`)  
**Returns**: `CommentDTO` (existing type) with comment ID, body, author, HTML URL.

```typescript
// Frontend call
import { PostImmediateComment } from '../../wailsjs/go/main/App'
const comment: main.CommentDTO = await PostImmediateComment({
  path: 'src/foo/bar.go',
  body: 'Looks good!',
  line: 10,
  side: 'RIGHT',
})
```

---

### `GetPendingReview() (model.PendingReviewDTO, error)`

Returns the current pending review state. If no pending review exists, returns `PendingReviewDTO{ReviewID: 0, HasPending: false}`.

**Note**: The pending review list is maintained in-memory (`a.pendingComments`). GitHub's API does not provide a reliable way to list draft comments on a pending review without knowing the review ID.

```typescript
// Frontend call
import { GetPendingReview } from '../../wailsjs/go/main/App'
const pending: main.PendingReviewDTO = await GetPendingReview()
```

---

### `SubmitReview(req model.ReviewSubmitDTO) (model.ReviewSubmitResult, error)`

Submits the pending review with a verdict. Publishes all draft comments and records the review event on GitHub.

**Precondition**: A pending review must exist (`pendingReviewID != 0`).  
**GitHub API**: `PullRequestsService.SubmitReview(ctx, owner, repo, number, reviewID, &PullRequestReviewRequest{Event: verdict, Body: body})`  
**Side effect**: Clears `a.pendingReviewID` and `a.pendingComments` on success.

If no pending review exists but the user wants to submit just a verdict + body, creates a new review directly with the appropriate event (no prior pending comments).

```typescript
// Frontend call
import { SubmitReview } from '../../wailsjs/go/main/App'
const result: main.ReviewSubmitResult = await SubmitReview({
  verdict: 'APPROVE',
  body: 'Looks great!',
})
```

---

### `DiscardPendingReview() error`

Discards the current pending review on GitHub. Clears local state.

**Precondition**: A pending review must exist. If none exists, returns nil (no-op).  
**GitHub API**: `PullRequestsService.DeletePendingReview(ctx, owner, repo, number, reviewID)`  
**Side effect**: Clears `a.pendingReviewID` and `a.pendingComments`.

```typescript
// Frontend call
import { DiscardPendingReview } from '../../wailsjs/go/main/App'
await DiscardPendingReview()
```

---

## Modified Methods

### `LoadPullRequest` — no signature change

The existing method is called before any diff methods. It must additionally:
- Cache `prCommitSHA` (HEAD commit SHA) from the PR data — needed for `CreateReview`

No signature change; the frontend call remains the same.

---

## Unchanged Methods (used by diff review view)

| Method | Used for |
|---|---|
| `GetCommentThreads(includeResolved bool)` | Loading other reviewers' threads when toggle is on |
| `GetAuthState()` | Checking auth before attempting API calls |

---

## New App Struct Fields

```go
// Added to App struct in app.go:
pendingReviewID   int64              // 0 = no pending review
pendingComments   []model.DraftCommentDTO
prCommitSHA       string             // HEAD commit SHA of loaded PR
prFilesCache      []*github.CommitFile // cached from ListFiles
```

---

## Error Conventions

All new methods follow the existing pattern:
- Return `(T, error)` — never panic
- Error messages are lowercase, no trailing period
- Wrap underlying errors with `fmt.Errorf("operation: %w", err)`
- Frontend surfaces errors via existing error handling in composables

---

## Frontend Bindings Location

Generated TypeScript bindings will appear at:
```
wailsjs/go/main/App.d.ts   ← method signatures
wailsjs/go/models.ts        ← all DTO types
```

Run `wails generate module` after adding new methods to regenerate bindings.
