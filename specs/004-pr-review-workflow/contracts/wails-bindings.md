# UI Contracts: Go ↔ Vue Wails Bindings

**Feature**: 004-pr-review-workflow  
**Date**: 2026-03-31  
**Status**: Complete — extends spec 001 `contracts/wails-bindings.md`

All spec 001 bindings are **inherited unchanged**. This document records the new and
modified bindings required by feature 004. At implementation time, all bindings live
on the `App` struct in `app.go`. Run `wails generate module` after any signature change.

---

## Auth Bindings (spec 001 — unchanged)

`StartDeviceFlow`, `PollDeviceFlow`, `GetAuthState`, `Logout` — see spec 001 contracts.

---

## Pull Request Bindings

### LoadPullRequest *(modified from spec 001)*

Fetches PR metadata from GitHub REST API and fetches all review threads via GraphQL.
Caches both in-memory on the `App` struct. Returns a summary for immediate display.

```go
func (a *App) LoadPullRequest(owner, repo string, number int) (model.PullRequestSummary, error)
```

**Changed**: `PullRequestSummary` gains `IsDraft bool \`json:"is_draft"\`` field (see
data-model.md).

**Behaviour**:
1. GET `/repos/{owner}/{repo}/pulls/{number}` via go-github → populate `prCache`
2. Execute `FetchReviewThreads` GraphQL query (paginated) → populate `threads`
3. Apply ignored-commenter filter to `threads` before caching
4. Return `PullRequestSummary` (includes `State`, `IsDraft`, `CommentCount`,
   `UnresolvedCount` computed from filtered threads)

**Error cases**:
- `notfound:` — PR does not exist
- `github:` — GitHub API error (network, rate limit, 4xx/5xx)
- `auth:` — no authenticated client

**Progress event** (emitted during paginated GraphQL fetch):
```
pr:load-progress  payload: { loaded: int, total: int }
```
(`total` is -1 if unknown until pagination completes.)

---

### GetCommentThreads *(spec 001 — unchanged signature)*

Returns threads from the in-memory cache with optional resolved filter applied.
Must call `LoadPullRequest` first; returns `notfound:` error if no PR is loaded.

```go
func (a *App) GetCommentThreads(includeResolved bool) ([]model.CommentThreadDTO, error)
```

**Behaviour**: Returns cached `threads`. When `includeResolved` is false, filters out
threads where `Resolved == true`. Ignored-commenter filter is already applied at load
time; no additional filtering here.

---

### GetThread *(spec 001 — unchanged)*

```go
func (a *App) GetThread(rootID int64) (model.CommentThreadDTO, error)
```

Returns a single thread by its root comment ID from the in-memory cache.
Returns `notfound:` if no thread with that root ID exists.

---

## Comment Action Bindings

### ReplyToComment *(spec 001 — unchanged signature)*

```go
func (a *App) ReplyToComment(threadRootID int64, body string) (model.CommentDTO, error)
```

**Behaviour**:
1. Validate `body` is non-empty (return `validation:` error if empty)
2. Call `POST /repos/{owner}/{repo}/pulls/{number}/comments` with `in_reply_to` =
   `threadRootID` via go-github `PullRequests.CreateComment`
3. Append returned `CommentDTO` to the cached thread
4. Return the new `CommentDTO`

**Error cases**:
- `validation:body required` — empty body
- `github:` — GitHub API error
- `notfound:thread` — thread root ID not in cache

---

### ResolveThread *(spec 001 — unchanged signature)*

```go
func (a *App) ResolveThread(threadRootID int64) error
```

**Behaviour**:
1. Look up `NodeID` from cached thread with given `rootID`
2. Execute GraphQL mutation `resolveReviewThread(input: {threadId: NodeID})`
3. On success: update cached thread `Resolved = true`
4. On failure: return `github:` error (frontend rolls back optimistic update)

---

### UnresolveThread *(spec 001 — unchanged signature)*

```go
func (a *App) UnresolveThread(threadRootID int64) error
```

Symmetric to `ResolveThread`. Executes `unresolveReviewThread` mutation.

---

### CommitSuggestion *(spec 001 — unchanged signature)*

```go
func (a *App) CommitSuggestion(commentID int64, commitMessage string) (model.SuggestionCommitResult, error)
```

**Behaviour**:
1. Find comment in cached threads; validate `IsSuggestion == true`
2. Extract `SuggestionText` from comment body
3. GET `/repos/{owner}/{repo}/contents/{path}?ref={headBranch}` → file content + blob SHA
4. Apply patch (replace hunk lines with suggestion lines)
5. PUT `/repos/{owner}/{repo}/contents/{path}` with updated content + blob SHA + `commitMessage`
6. Return `SuggestionCommitResult{CommitSHA, HTMLURL}`

**Error cases**:
- `validation:not-a-suggestion` — comment has no suggestion block
- `github:conflict` — file SHA mismatch (file changed since load); descriptive message
- `github:` — any other GitHub API error

---

## Settings Bindings

### GetIgnoredCommenters *(spec 001 — unchanged signature)*

```go
func (a *App) GetIgnoredCommenters() ([]model.IgnoredCommenterDTO, error)
```

Reads from `os.UserConfigDir()/gitura/ignored_commenters.json`. Returns empty slice if
file does not exist.

---

### AddIgnoredCommenter *(spec 001 — unchanged signature)*

```go
func (a *App) AddIgnoredCommenter(login string) error
```

Validates non-empty login. Silently no-ops if already present. Persists to disk.

---

### RemoveIgnoredCommenter *(spec 001 — unchanged signature)*

```go
func (a *App) RemoveIgnoredCommenter(login string) error
```

Removes entry from list. No-ops if not present. Persists to disk.

---

## Events (Go → Vue Push)

Wails runtime events emitted by Go; consumed in Vue with `EventsOn`.

| Event Name | Payload Type | When emitted |
|---|---|---|
| `auth:device-flow-complete` | `AuthState` | Device flow polling succeeds |
| `auth:device-flow-expired` | `{}` | Device code expires |
| `pr:load-progress` | `{ loaded: number, total: number }` | During paginated GraphQL thread fetch |

---

## Error Codes

All errors returned from Go methods are strings with prefix convention:

| Prefix | Meaning |
|---|---|
| `auth:` | Authentication / token error |
| `github:` | GitHub API error (includes HTTP status or sub-code like `conflict`) |
| `validation:` | Input validation failure |
| `keyring:` | OS credential store error |
| `notfound:` | Requested resource not found |

**New sub-codes for 004**:

| Error | Trigger |
|---|---|
| `github:conflict` | Suggestion commit aborted due to file SHA mismatch |
| `validation:not-a-suggestion` | CommitSuggestion called on non-suggestion comment |
| `notfound:thread` | Thread root ID not found in in-memory cache |

---

## Full TypeScript Contract (post `wails generate module`)

After adding new methods to `app.go` and running `wails generate module`, the following
types will be auto-generated in `wailsjs/go/main/models.ts`:

```typescript
export namespace main {
  // Modified (IsDraft added)
  export interface PullRequestSummary {
    id:               number
    number:           number
    title:            string
    state:            string    // "open" | "closed" | "merged"
    body:             string
    head_branch:      string
    base_branch:      string
    head_sha:         string
    node_id:          string
    html_url:         string
    owner:            string
    repo:             string
    comment_count:    number
    unresolved_count: number
    is_draft:         boolean   // NEW
  }

  // Unchanged from spec 001
  export interface CommentThreadDTO {
    root_id:  number
    node_id:  string
    comments: CommentDTO[]
    resolved: boolean
    path:     string
    line:     number
  }

  export interface CommentDTO {
    id:            number
    in_reply_to_id: number
    body:          string
    author_login:  string
    author_avatar: string
    diff_hunk:     string
    created_at:    string   // RFC3339
    is_suggestion: boolean
  }

  export interface SuggestionCommitResult {
    commit_sha: string
    html_url:   string
  }

  export interface IgnoredCommenterDTO {
    login:    string
    added_at: string   // RFC3339
  }
}
```

---

## Frontend Usage Patterns

### Loading the review view

```typescript
// frontend/src/composables/useReview.ts
import { LoadPullRequest, GetCommentThreads, ReplyToComment,
         ResolveThread, UnresolveThread, CommitSuggestion } from '@/wailsjs/go/main/App'
import type { CommentThreadDTO, PullRequestSummary } from '@/wailsjs/go/main/models'

const threads = ref<CommentThreadDTO[]>([])
const prSummary = ref<PullRequestSummary | null>(null)
const loading = ref(false)
const error = ref('')

async function loadPR(owner: string, repo: string, number: number): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    prSummary.value = await LoadPullRequest(owner, repo, number)
    threads.value = await GetCommentThreads(false)  // initially exclude resolved
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}
```

### Optimistic resolve/unresolve

```typescript
async function resolveThread(rootID: number): Promise<void> {
  // Optimistic update
  const thread = threads.value.find(t => t.root_id === rootID)
  if (!thread) return
  thread.resolved = true
  try {
    await ResolveThread(rootID)
  } catch {
    thread.resolved = false  // rollback
  }
}
```

### Navigation queue

```typescript
const showResolved = ref(false)
const currentIndex = ref(0)

const queue = computed(() =>
  showResolved.value ? threads.value : threads.value.filter(t => !t.resolved)
)
const currentThread = computed(() => queue.value[currentIndex.value] ?? null)
const isAtEnd = computed(() => currentIndex.value >= queue.value.length - 1)
const canGoForward = computed(() => !isAtEnd.value)
const canGoBack = computed(() => currentIndex.value > 0)

function goNext() { if (canGoForward.value) currentIndex.value++ }
function goPrev() { if (canGoBack.value) currentIndex.value-- }
```
