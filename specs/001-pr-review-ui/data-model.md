# Data Model: PR Review UI

**Feature**: 001-pr-review-ui
**Date**: 2026-03-30
**Source**: spec.md entities + research.md GitHub API field mapping

---

## Domain Entities

### PullRequest

The top-level entity representing a GitHub pull request loaded into the app.

| Field | Type | Source | Notes |
|---|---|---|---|
| `ID` | `int64` | GitHub API | Internal GitHub PR ID |
| `Number` | `int` | GitHub API | PR number (shown in UI, used in API paths) |
| `Title` | `string` | GitHub API | PR title |
| `State` | `string` | GitHub API | `open`, `closed`, `merged` |
| `Body` | `string` | GitHub API | PR description (Markdown) |
| `HeadBranch` | `string` | GitHub API | Source branch name |
| `BaseBranch` | `string` | GitHub API | Target branch name |
| `HeadSHA` | `string` | GitHub API | Latest commit SHA on head branch |
| `HTMLURL` | `string` | GitHub API | URL to PR on GitHub.com |
| `Owner` | `string` | user input | Repo owner (org or user) |
| `Repo` | `string` | user input | Repository name |
| `CommentCount` | `int` | derived | Total non-ignored review comments |
| `UnresolvedCount` | `int` | derived | Count of unresolved threads |

**State transitions**: PullRequest is loaded once per session and refreshed on explicit
user action. It is read-only from the app's perspective (the app does not create or
merge PRs).

---

### ReviewComment

A single line-level or file-level comment made as part of a pull request review.
Maps to GitHub's Pull Request Review Comment object.

| Field | Type | Source | Notes |
|---|---|---|---|
| `ID` | `int64` | GitHub API | Unique comment ID |
| `InReplyToID` | `int64` | GitHub API | 0 if root comment; parent ID if reply |
| `ThreadID` | `int64` | derived | Root comment ID for the thread |
| `Body` | `string` | GitHub API | Comment text (Markdown) |
| `Author` | `User` | GitHub API | Commenter identity |
| `Path` | `string` | GitHub API | File path the comment is on |
| `Line` | `int` | GitHub API | Line number in the diff |
| `Side` | `string` | GitHub API | `LEFT` or `RIGHT` |
| `DiffHunk` | `string` | GitHub API | Raw unified diff hunk for context |
| `CreatedAt` | `time.Time` | GitHub API | Creation timestamp |
| `UpdatedAt` | `time.Time` | GitHub API | Last updated timestamp |
| `HTMLURL` | `string` | GitHub API | Link to comment on GitHub.com |
| `Resolved` | `bool` | derived | True if parent thread is resolved |
| `IsSuggestion` | `bool` | derived | True if body contains ` ```suggestion ` block |
| `SuggestionText` | `string` | derived | Extracted suggestion content (if applicable) |

**Validation rules**:
- `Body` MUST NOT be empty when posting a new comment or reply.
- `Path` and `Line` are required for root comments; optional for replies (replies
  inherit the thread's location).

---

### CommentThread

A logical grouping of a root ReviewComment and all its replies. Resolved state is
tracked at the thread level.

| Field | Type | Source | Notes |
|---|---|---|---|
| `RootID` | `int64` | derived | ID of the root comment |
| `NodeID` | `string` | GitHub API | GraphQL global ID (`PullRequestReviewThread` node); required for resolve/unresolve mutations |
| `Comments` | `[]ReviewComment` | derived | Root + replies in chronological order |
| `Resolved` | `bool` | GitHub API | Thread resolved state |
| `Path` | `string` | derived | From root comment |
| `Line` | `int` | derived | From root comment |
| `Author` | `User` | derived | Root comment author |

**State transitions**:
```
Unresolved → Resolved   (user triggers "Resolve thread")
Resolved   → Unresolved (user triggers "Unresolve thread")
```
Resolution is persisted to GitHub immediately; local state updates optimistically.

---

### User

Represents a GitHub user (commenter, authenticated user).

| Field | Type | Source | Notes |
|---|---|---|---|
| `Login` | `string` | GitHub API | GitHub username |
| `AvatarURL` | `string` | GitHub API | Profile picture URL |
| `HTMLURL` | `string` | GitHub API | Profile page URL |

---

### Suggestion

A proposed code change embedded in a ReviewComment body. Extracted client-side.

| Field | Type | Source | Notes |
|---|---|---|---|
| `CommentID` | `int64` | derived | Parent comment ID |
| `FilePath` | `string` | derived | From parent comment's `Path` |
| `OriginalText` | `string` | derived | Current file content at the affected lines |
| `ReplacementText` | `string` | derived | Content inside the suggestion fenced block |
| `CommitStatus` | `string` | app state | `pending`, `committed`, `failed` |
| `CommitSHA` | `string` | GitHub API | Populated after successful commit |

**Extraction rule**: A comment body contains a suggestion if it includes a fenced code
block with the language identifier `suggestion`:
````
```suggestion
replacement lines
```
````
All lines in the diff hunk above the suggestion block are the lines being replaced.

**SHA conflict detection**: Before committing a suggestion, the Go backend MUST fetch
the current file blob SHA from the GitHub Contents API immediately before the commit
request. The cached `HeadSHA` from the PR load is used only as a reference; the
live file SHA determines whether the file has been modified since load. If the
fetched SHA differs from what was used to construct the suggestion, the commit MUST
be aborted with a `github:conflict` error and the user informed that the file has
changed.

---

### IgnoredCommenter

A persisted username entry whose comments are hidden from all views.

| Field | Type | Source | Notes |
|---|---|---|---|
| `Login` | `string` | user input | GitHub username to ignore |
| `AddedAt` | `time.Time` | app | Timestamp when added |

**Storage**: Persisted as a JSON file at the OS-appropriate config directory
(`os.UserConfigDir()/gitura/ignored_commenters.json`). Not synced across devices.

**Validation**: `Login` MUST be a non-empty string. Duplicates are silently de-duped.

---

### AuthState

Represents the current authentication status of the app.

| Field | Type | Source | Notes |
|---|---|---|---|
| `IsAuthenticated` | `bool` | derived | True if a valid token is stored |
| `Login` | `string` | GitHub API | Authenticated user's GitHub username; empty if not authed |
| `AvatarURL` | `string` | GitHub API | Authenticated user's avatar URL; empty if not authed |
| `Token` | `string` | keyring | Stored in OS keychain; never sent to frontend |
| `DeviceCode` | `string` | temp | Used during OAuth device flow polling |
| `UserCode` | `string` | temp | Shown to user during device flow |
| `VerificationURI` | `string` | temp | GitHub device authorization URL |

**Note**: `Token` is NEVER included in Wails binding responses to the frontend.
The frontend only receives `IsAuthenticated`, `Login`, and `AvatarURL` — as flat
string fields. There is no nested `User` struct in the frontend-facing DTO.

---

## Entity Relationships

```
AuthState
  └── User (authenticated user)

PullRequest
  ├── Owner (string)
  ├── Repo (string)
  └── CommentThreads []CommentThread
        └── Comments []ReviewComment
              ├── Author (User)
              └── Suggestion? (optional, derived)

IgnoredCommenter (standalone, applied as a filter at query time)
```

---

## Filtering Rules

When loading comments for display, the following filter is applied in-process:

1. **Ignored commenters**: Any `ReviewComment` where `Author.Login` is in the
   `IgnoredCommenter` list is excluded from all views.
2. **Resolved threads**: Hidden from the list view and one-by-one navigation by
   default. Toggle opt-in shows them with visual distinction.
3. **Replies**: Root comments (`InReplyToID == 0`) are shown as thread headers;
   replies are nested within their thread in the detail view.

---

## Persistence Summary

| Data | Storage | Scope |
|---|---|---|
| GitHub access token | OS keychain (go-keyring) | Per-machine |
| Ignored commenters | JSON file (`UserConfigDir/gitura/`) | Per-machine |
| PR data, comments | In-memory cache only | Per-session |
| Auth device flow state | In-memory only | Per-flow |
