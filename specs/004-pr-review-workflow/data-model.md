# Data Model: PR Deep Review Workflow

**Feature**: 004-pr-review-workflow  
**Date**: 2026-03-31  
**Status**: Complete — extends spec 001 data-model.md  
**Source**: spec.md entities + research.md decisions

All entities from spec 001 `data-model.md` are **inherited unchanged** unless noted.
This document records additions and extensions only.

---

## Inherited Entities (spec 001 — unchanged)

| Entity | Change |
|---|---|
| `User` | None |
| `ReviewComment` | None |
| `CommentThread` | None |
| `Suggestion` | None |
| `IgnoredCommenter` | None |
| `AuthState` | None |

---

## Modified Entities

### PullRequestSummary

Extends spec 001 definition. Added field:

| Field | Type | Source | Notes |
|---|---|---|---|
| `IsDraft` | `bool` | GitHub API | True if PR is in draft state. Required for FR-013 status banner. |

All other fields (`ID`, `Number`, `Title`, `State`, `Body`, `HeadBranch`, `BaseBranch`,
`HeadSHA`, `NodeID`, `HTMLURL`, `Owner`, `Repo`, `CommentCount`, `UnresolvedCount`)
are unchanged from spec 001.

> **Note**: `IsDraft` is already present in `PRListItem` (from spec 003) but was absent
> from `PullRequestSummary`. It is needed here because `LoadPullRequest` returns this
> type and the review view's status banner (FR-013) must distinguish draft from open.

**Current Go struct** (`internal/model/model.go`): Already has all spec 001 fields.
**Required change**: Add `IsDraft bool \`json:"is_draft"\`` field.

---

## New Entities

### ReviewLoadInput

Frontend-only. The data packet transferred from `PRPage` → `App.vue` → `ReviewPage`
when a user clicks a PR row. Not serialized to Go.

| Field | Type | Source | Notes |
|---|---|---|---|
| `number` | `number` | PRListItem | PR number |
| `owner` | `string` | PRListItem | Repo owner |
| `repo` | `string` | PRListItem | Repo name |
| `title` | `string` | PRListItem | PR title (display immediately) |
| `htmlUrl` | `string` | PRListItem | GitHub URL |
| `isDraft` | `boolean` | PRListItem | Draft flag (display immediately) |
| `headBranch` | `string` | PRListItem | Head branch (display immediately; not yet in PRListItem — see note) |

> **Note on `headBranch`**: `PRListItem` (spec 003) does not currently carry `head_branch`.
> For the review view, `LoadPullRequest` will fetch this. The immediate display shows
> title + number from `ReviewLoadInput`; branch info shows after `LoadPullRequest` resolves.
> `PRListItem` does not need to be changed for v1.

**TypeScript type** (frontend/src/types or inline in composable):
```typescript
export interface ReviewLoadInput {
  number:   number
  owner:    string
  repo:     string
  title:    string
  htmlUrl:  string
  isDraft:  boolean
}
```

---

### ReviewNavState

Frontend-only reactive state managed by `useReview.ts` composable. Not serialized.

| Field | Type | Default | Notes |
|---|---|---|---|
| `threads` | `CommentThreadDTO[]` | `[]` | Full thread list (all resolved states) |
| `showResolved` | `boolean` | `false` | Resets to false on new PR load |
| `currentIndex` | `number` | `0` | Index into the active queue |
| `loading` | `boolean` | `false` | True while `LoadPullRequest` is in-flight |
| `error` | `string` | `''` | Non-empty on load failure |

**Derived computed state**:

| Name | Derivation |
|---|---|
| `queue` | `threads` filtered by `showResolved` (when false, exclude resolved threads) |
| `currentThread` | `queue[currentIndex]` or `null` |
| `isAtEnd` | `currentIndex >= queue.length - 1` (triggers "All comments reviewed" state) |
| `canGoBack` | `currentIndex > 0` |
| `canGoForward` | `!isAtEnd` |

**State transitions**:
```
Initial load:
  loading=true → LoadPullRequest() → loading=false, threads populated, currentIndex=0

showResolved toggled:
  queue recomputed → currentIndex clamped to queue.length-1 if out of bounds

New PR loaded (back → different PR):
  showResolved reset to false, threads=[], currentIndex=0, loading=true
```

---

## Entity Relationships (updated)

```
ReviewLoadInput (frontend-only, transient)
  └── used to call LoadPullRequest() → PullRequestSummary + ReviewNavState.threads

ReviewNavState (frontend-only, per-session)
  └── threads []CommentThreadDTO
        └── Comments []CommentDTO
              ├── Author (login, avatar)
              └── IsSuggestion (derived from body)

IgnoredCommenter (persisted, applied as filter server-side in GetCommentThreads)
```

---

## GraphQL Response Shapes (internal — not frontend-facing)

These are Go structs used internally by `internal/github/comments.go` to decode the
GraphQL response. They are NOT part of the Wails DTO surface.

```go
// graphQLReviewThreadsResponse is the top-level GraphQL response wrapper.
type graphQLReviewThreadsResponse struct {
    Data   graphQLReviewData   `json:"data"`
    Errors []graphQLError      `json:"errors"`
}

type graphQLReviewData struct {
    Repository graphQLRepository `json:"repository"`
}

type graphQLRepository struct {
    PullRequest graphQLPullRequest `json:"pullRequest"`
}

type graphQLPullRequest struct {
    ReviewThreads graphQLThreadConnection `json:"reviewThreads"`
}

type graphQLThreadConnection struct {
    PageInfo graphQLPageInfo      `json:"pageInfo"`
    Nodes    []graphQLThread      `json:"nodes"`
}

type graphQLPageInfo struct {
    HasNextPage bool   `json:"hasNextPage"`
    EndCursor   string `json:"endCursor"`
}

type graphQLThread struct {
    ID         string                  `json:"id"`
    IsResolved bool                    `json:"isResolved"`
    Comments   graphQLCommentConnection `json:"comments"`
}

type graphQLCommentConnection struct {
    Nodes []graphQLComment `json:"nodes"`
}

type graphQLComment struct {
    DatabaseID   int64              `json:"databaseId"`
    Body         string             `json:"body"`
    Author       graphQLActor       `json:"author"`
    Path         string             `json:"path"`
    Line         *int               `json:"line"`
    OriginalLine *int               `json:"originalLine"`
    DiffHunk     string             `json:"diffHunk"`
    CreatedAt    string             `json:"createdAt"` // ISO-8601
    URL          string             `json:"url"`
    ReplyTo      *graphQLReplyTo    `json:"replyTo"`
}

type graphQLActor struct {
    Login     string `json:"login"`
    AvatarURL string `json:"avatarUrl"`
}

type graphQLReplyTo struct {
    DatabaseID int64 `json:"databaseId"`
}

type graphQLError struct {
    Message string `json:"message"`
}
```

---

## Filtering Rules (extended from spec 001)

1. **Ignored commenters**: Thread excluded when the **root comment** author is in the
   ignored list. Applied in `GetCommentThreads` before returning to frontend.
2. **Resolved threads**: Excluded from `queue` (nav) and `CommentSummaryList` when
   `showResolved` is false. Shown with muted styling + "Resolved" badge when true.
3. **Empty queue**: When all threads are resolved and `showResolved` is false,
   `queue` is empty → show "All comments reviewed" message, disable navigation.

---

## Persistence Summary (extended from spec 001)

| Data | Storage | Scope |
|---|---|---|
| GitHub access token | OS keychain (go-keyring) | Per-machine |
| Ignored commenters | JSON file (`UserConfigDir/gitura/ignored_commenters.json`) | Per-machine |
| PR metadata, comment threads | In-memory (`App.prCache`, `App.threads`) | Per-session |
| Review nav state | In-memory (Vue reactive, `useReview` composable) | Per-session |
| Reply draft | In-memory (Vue ref in `ReplyComposer`) | Until component unmount |
| Auth device flow state | In-memory | Per-flow |
