# UI Contracts: Go ↔ Vue Wails Bindings

**Feature**: 001-pr-review-ui
**Date**: 2026-03-30

This document defines the Go method signatures exposed to the Vue frontend via Wails
bindings, and the corresponding TypeScript types auto-generated in `wailsjs/`. These
are the contracts between the Go backend and the Vue UI.

All methods live on the `App` struct in `app.go`. All types that cross the boundary
MUST have `json:"..."` tags on every field. Errors are returned as rejected Promises
in the frontend.

---

## Auth Bindings

### StartDeviceFlow

Initiates GitHub OAuth device flow. Returns display data for the UI. Does NOT block.

```go
func (a *App) StartDeviceFlow() (DeviceFlowInfo, error)

type DeviceFlowInfo struct {
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`       // seconds
    Interval        int    `json:"interval"`          // polling interval seconds
}
```

**Frontend usage**: Display `user_code` to user, open `verification_uri` in browser,
then call `PollDeviceFlow` on a timer.

---

### PollDeviceFlow

Polls GitHub for token completion. Returns empty string if still pending, token on
success (token is stored in keychain by Go; frontend receives success signal only).

```go
func (a *App) PollDeviceFlow() (PollResult, error)

type PollResult struct {
    Status string `json:"status"` // "pending" | "complete" | "expired" | "error"
}
```

**Frontend usage**: Call every `DeviceFlowInfo.Interval` seconds until `status !=
"pending"`. On `"complete"`, call `GetAuthState`.

---

### GetAuthState

Returns current authentication status and user info.

```go
func (a *App) GetAuthState() (AuthState, error)

type AuthState struct {
    IsAuthenticated bool   `json:"is_authenticated"`
    Login           string `json:"login"`      // empty if not authenticated
    AvatarURL       string `json:"avatar_url"` // empty if not authenticated
}
```

---

### Logout

Removes the stored token from the OS keychain and clears in-memory state.

```go
func (a *App) Logout() error
```

---

## Pull Request Bindings

### LoadPullRequest

Fetches a pull request and all its review comments from GitHub. Caches in-memory
for the session.

```go
func (a *App) LoadPullRequest(owner, repo string, number int) (PullRequestSummary, error)

type PullRequestSummary struct {
    Number          int    `json:"number"`
    Title           string `json:"title"`
    State           string `json:"state"`
    HeadBranch      string `json:"head_branch"`
    BaseBranch      string `json:"base_branch"`
    HTMLURL         string `json:"html_url"`
    CommentCount    int    `json:"comment_count"`     // total after ignore filter
    UnresolvedCount int    `json:"unresolved_count"`
}
```

---

### GetCommentThreads

Returns all comment threads for the currently loaded PR, with ignore filter and
resolved filter applied.

```go
func (a *App) GetCommentThreads(includeResolved bool) ([]CommentThreadDTO, error)

type CommentThreadDTO struct {
    RootID   int64          `json:"root_id"`
    NodeID   string         `json:"node_id"`   // GraphQL global ID for resolve/unresolve
    Path     string         `json:"path"`
    Line     int            `json:"line"`
    Resolved bool           `json:"resolved"`
    Comments []CommentDTO   `json:"comments"`
}

type CommentDTO struct {
    ID           int64  `json:"id"`
    InReplyToID  int64  `json:"in_reply_to_id"`
    Body         string `json:"body"`
    AuthorLogin  string `json:"author_login"`
    AuthorAvatar string `json:"author_avatar"`
    DiffHunk     string `json:"diff_hunk"`
    CreatedAt    string `json:"created_at"` // RFC3339
    IsSuggestion bool   `json:"is_suggestion"`
}
```

---

### GetThread

Returns a single thread by root comment ID.

```go
func (a *App) GetThread(rootID int64) (CommentThreadDTO, error)
```

---

## Comment Action Bindings

### ReplyToComment

Posts a reply to a comment thread on GitHub.

```go
func (a *App) ReplyToComment(threadRootID int64, body string) (CommentDTO, error)
```

**Validation**: Returns error if `body` is empty.

---

### ResolveThread

Marks a comment thread as resolved on GitHub.

```go
func (a *App) ResolveThread(threadRootID int64) error
```

---

### UnresolveThread

Marks a comment thread as unresolved on GitHub.

```go
func (a *App) UnresolveThread(threadRootID int64) error
```

---

### CommitSuggestion

Applies and commits a suggestion from a comment to the PR's head branch.

```go
func (a *App) CommitSuggestion(commentID int64, commitMessage string) (SuggestionCommitResult, error)

type SuggestionCommitResult struct {
    CommitSHA string `json:"commit_sha"`
    HTMLURL   string `json:"html_url"`
}
```

**Validation**: Returns error if comment does not contain a suggestion block.

---

## Settings Bindings

### GetIgnoredCommenters

Returns the current ignored-commenter list.

```go
func (a *App) GetIgnoredCommenters() ([]IgnoredCommenterDTO, error)

type IgnoredCommenterDTO struct {
    Login   string `json:"login"`
    AddedAt string `json:"added_at"` // RFC3339
}
```

---

### AddIgnoredCommenter

Adds a username to the ignored-commenter list and persists it.

```go
func (a *App) AddIgnoredCommenter(login string) error
```

**Validation**: Returns error if `login` is empty. Silently no-ops if already present.

---

### RemoveIgnoredCommenter

Removes a username from the ignored-commenter list.

```go
func (a *App) RemoveIgnoredCommenter(login string) error
```

---

## Events (Go → Vue Push)

Wails runtime events emitted by Go; consumed in Vue with `EventsOn`.

| Event Name | Payload Type | When emitted |
|---|---|---|
| `auth:device-flow-complete` | `AuthState` | Device flow polling succeeds |
| `auth:device-flow-expired` | `{}` | Device code expires before authorization |
| `pr:load-progress` | `{ loaded: int, total: int }` | During paginated comment fetch |

---

## Error Codes

All errors returned from Go methods are strings. The frontend distinguishes error
categories by prefix convention:

| Prefix | Meaning |
|---|---|
| `auth:` | Authentication / token error |
| `github:` | GitHub API error (includes HTTP status) |
| `validation:` | Input validation failure |
| `keyring:` | OS credential store error |
| `notfound:` | Requested resource not found |
