# Phase 0 Research: PR Diff Review

**Feature**: 005-pr-diff-review  
**Date**: 2026-04-01

## 1. GitHub API Coverage in go-github/v67

### Available Methods

| GitHub REST Endpoint | go-github/v67 Method | Notes |
|---|---|---|
| `GET /pulls/{number}/files` | `PullRequestsService.ListFiles` | Returns `[]*CommitFile` with `Patch` (unified diff text) |
| `POST /pulls/{number}/reviews` | `PullRequestsService.CreateReview` | Creates review; `Event: ""` or `"PENDING"` creates a pending review |
| `POST /pulls/{number}/reviews/{id}/events` | `PullRequestsService.SubmitReview` | Submits pending review with verdict |
| `DELETE /pulls/{number}/reviews/{id}` | `PullRequestsService.DeletePendingReview` | Discards pending review |
| `GET /pulls/{number}/reviews/{id}/comments` | `PullRequestsService.ListReviewComments` | Lists comments on a specific review |

### Gap: Add Comment to Existing Pending Review

**`POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}/comments` is NOT implemented in go-github/v67.**

This endpoint is required for adding individual draft comments to an already-created pending review (all comments after the first one). It must be called via a raw `http.Client`. The existing codebase already uses this pattern in `internal/github/` (e.g., suggestion commits use raw HTTP). The raw call is straightforward:

```go
// POST body: {"path": "...", "body": "...", "line": N, "side": "RIGHT"}
// Auth: same bearer token as rest of client
// URL: https://api.github.com/repos/{owner}/{repo}/pulls/{number}/reviews/{reviewID}/comments
```

### Comment Positioning Strategy

Two mutually exclusive positioning styles exist. We use the **comfort-fade style** (line numbers) rather than legacy diff position numbers:

| Field | Type | Description |
|---|---|---|
| `line` | `*int` | Line number in the file (end line, or only line for single-line) |
| `side` | `*string` | `"LEFT"` (old version) or `"RIGHT"` (new version) |
| `start_line` | `*int` | Start line for multi-line comment range |
| `start_side` | `*string` | Side for the start line |

Using `position` (legacy diff-position integer) is mutually exclusive with these fields. We use comfort-fade exclusively for clarity and forward compatibility.

### DraftReviewComment struct (go-github)

```go
type DraftReviewComment struct {
    Path      *string `json:"path,omitempty"`
    Position  *int    `json:"position,omitempty"`   // NOT used (legacy)
    Body      *string `json:"body,omitempty"`
    StartSide *string `json:"start_side,omitempty"` // "LEFT" or "RIGHT"
    Side      *string `json:"side,omitempty"`        // "LEFT" or "RIGHT"
    StartLine *int    `json:"start_line,omitempty"`
    Line      *int    `json:"line,omitempty"`
}
```

### ListFiles Response (CommitFile struct)

```go
type CommitFile struct {
    SHA              *string
    Filename         *string
    Additions        *int
    Deletions        *int
    Changes          *int
    Status           *string  // "added", "removed", "modified", "renamed", "copied", "changed", "unchanged"
    Patch            *string  // raw unified diff text (may be nil for binary files)
    BlobURL          *string
    PreviousFilename *string  // populated for renamed files
}
```

## 2. Diff Parsing Strategy

**Decision: Parse unified diff in Go backend.**

Rationale:
- Consistent with existing pattern (Go does all data transformation, frontend only renders)
- Testable at the unit level with 90% coverage requirement (constitution Principle II)
- Unified diff format is well-defined and parseable with a state machine
- Structured `ParsedDiffDTO` returned to frontend avoids parsing logic in TypeScript

### Unified Diff Format (relevant subset)

```
@@ -10,7 +12,9 @@ func Foo() {       ← hunk header
 unchanged line                           ← context (position on both sides)
-deleted line                             ← left side only
+added line                               ← right side only
 another context line
```

Hunk header format: `@@ -<old_start>,<old_count> +<new_start>,<new_count> @@`

Each line in the hunk:
- ` ` (space prefix) → context line, appears on both sides
- `-` prefix → deleted line, left side only
- `+` prefix → added line, right side only
- `\ No newline at end of file` → meta line, skip

Side-by-side rendering pairs context lines and maps deletions (left) to additions (right) where possible (GitHub-style pairing).

## 3. Existing Codebase Patterns

### Wails Binding Pattern

All Go methods on `App` struct in `app.go` are auto-exposed to the frontend via Wails. Existing methods follow this signature pattern:

```go
func (a *App) MethodName(param Type) (ReturnType, error)
```

Frontend calls via `wailsjs/go/main/App.MethodName(param)` returning a Promise.

### Navigation Pattern

- `App.vue` owns `currentPage` ref and `selectedPRItem`
- `ReviewPage.vue` currently shows conversation view
- We add a `prView` toggle inside `ReviewPage.vue` (`'conversation' | 'files'`)
- No Vue Router — in-app state switching only

### Composable Pattern

- Composables in `frontend/src/composables/`
- Per-instance (not singleton) for review state: `useReview.ts` model
- New `useDiffReview.ts` follows same pattern

### Auth/HTTP Pattern

`internal/github/client.go` holds the GitHub client. Raw HTTP calls use the authenticated HTTP client directly (same token). The `App.ghClient` field holds `*github.Client`; its underlying transport can be used for raw calls.

## 4. Architecture Decisions

### View Toggle Placement
Toggle lives inside `ReviewPage.vue` — not at App.vue level. `ReviewPage` owns:
```typescript
const prView = ref<'conversation' | 'files'>('conversation')
```
Extensible: future views (e.g., `'checks'`) added as additional tab values.

### Diff Parsing Location
Parse in Go backend. `GetFileDiff(path string)` returns `ParsedDiffDTO` with structured hunks and lines. Frontend renders what it receives — no diff parsing in TypeScript.

### Pending Review State
- `review_id` stored in `App` struct (in-memory, not persisted)
- `int64` of `0` means no pending review exists
- First draft comment: `CreateReview` with `Event: "PENDING"` → captures `review_id`
- Subsequent comments: raw HTTP `POST .../reviews/{review_id}/comments`
- `GetPendingReview()` returns current pending state including `review_id`

### File Loading
On-demand: `GetFileDiff(path)` called only when user selects a file. `GetPRFiles()` returns the lightweight list (no patch content) used for the sidebar.

### Side-by-Side Rendering
Frontend `DiffHunk.vue` pairs lines into a two-column table:
- Context lines: same content on both sides
- Delete + Add sequences: paired row-by-row (first delete with first add, etc.), surplus lines paired with empty cells
- Empty hunk separators between collapsed sections

## 5. Open Questions Resolved

| Question | Resolution |
|---|---|
| go-github support for adding to pending review | **No** — requires raw HTTP for `POST .../reviews/{id}/comments` |
| Positioning style (position vs. line/side) | **Comfort-fade** (`line` + `side`) — cleaner, forward-compatible |
| Diff parsing location | **Go backend** — testable, consistent with existing pattern |
| Binary file handling | Show placeholder from `CommitFile` metadata; `Patch` is nil for binary files |
| Stale thread detection | `PullRequestComment.OriginalLine != Line` or GitHub marks outdated in response |
