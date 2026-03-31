# Data Model: Open PR List with Filters

**Feature**: 003-pr-list-filters  
**Date**: 2026-03-31

---

## Overview

This feature introduces 3 new Go model types (in `internal/model/model.go`) and 1 new Vue composable (`usePRFilters.ts`). The existing `PullRequestSummary` model is NOT modified — it remains the detail-view DTO used by the future PR review page.

---

## Go Backend Models

### `PRListItem` — New

A lightweight DTO representing a single PR row in the list view. Deliberately excludes fields only needed in the detail view (branch names, body, comment counts, NodeID).

```go
// PRListItem is a lightweight DTO for a single row in the PR list view.
// It carries only the fields required by FR-002 plus navigation data.
type PRListItem struct {
    Number      int    `json:"number"`
    Title       string `json:"title"`
    Owner       string `json:"owner"`       // GitHub org or user owning the repo
    Repo        string `json:"repo"`        // repository name (without owner prefix)
    AuthorLogin string `json:"author_login"`
    CreatedAt   string `json:"created_at"`  // RFC3339 — for "time since opened"
    UpdatedAt   string `json:"updated_at"`  // RFC3339 — used for default sort
    HTMLURL     string `json:"html_url"`    // canonical GitHub URL for navigation
    IsDraft     bool   `json:"is_draft"`
}
```

**Fields**:

| Field | Source | Notes |
|---|---|---|
| `Number` | `issue.GetNumber()` | PR number within the repo |
| `Title` | `issue.GetTitle()` | |
| `Owner` | Parsed from `issue.GetRepositoryURL()` | Strips `https://api.github.com/repos/` prefix |
| `Repo` | Parsed from `issue.GetRepositoryURL()` | |
| `AuthorLogin` | `issue.GetUser().GetLogin()` | |
| `CreatedAt` | `issue.GetCreatedAt().Time.Format(time.RFC3339)` | |
| `UpdatedAt` | `issue.GetUpdatedAt().Time.Format(time.RFC3339)` | |
| `HTMLURL` | `issue.GetHTMLURL()` | Also used as deduplication key |
| `IsDraft` | `issue.GetDraft()` | Safe on nil (returns false) |

**Validation rules**:
- `Number > 0`
- `Title` non-empty
- `Owner` and `Repo` non-empty (if parsing fails, item is skipped with a log warning)
- `HTMLURL` non-empty

---

### `PRListFilters` — New

The filter state sent from the Vue frontend to the Go backend on each `ListOpenPRs` call. Serialized as a JSON object via Wails RPC.

```go
// PRListFilters carries the active filter state from the frontend to the
// Go backend. All string fields are empty when the filter is inactive.
// At least one of IncludeAuthor, IncludeAssignee, IncludeReviewer must be true.
type PRListFilters struct {
    IncludeAuthor   bool   `json:"include_author"`   // include PRs authored by the user
    IncludeAssignee bool   `json:"include_assignee"` // include PRs assigned to the user
    IncludeReviewer bool   `json:"include_reviewer"` // include PRs where user is review-requested
    Repo            string `json:"repo"`             // "owner/repo" format, or "" for all
    Org             string `json:"org"`              // GitHub org login, or "" for all
    Author          string `json:"author"`           // filter by PR author login, or ""
    UpdatedAfter    string `json:"updated_after"`    // RFC3339 datetime, or "" for no date filter
    IncludeDrafts   bool   `json:"include_drafts"`   // false = exclude drafts (default)
}
```

**Validation rules** (enforced in `ListOpenPRs` before issuing queries):
- At least one of `IncludeAuthor`, `IncludeAssignee`, `IncludeReviewer` must be `true`; if all are false, return error
- `Repo` when non-empty must contain exactly one `/` separator
- `UpdatedAfter` when non-empty must parse as RFC3339

**Default state** (matches FR-001, FR-015):
```go
PRListFilters{
    IncludeAuthor:   true,
    IncludeAssignee: true,
    IncludeReviewer: true,
    IncludeDrafts:   false,
}
```

---

### `PRListResult` — New

The return type of `ListOpenPRs`. A union of the happy path (items) and error paths (rate limit, network error, incomplete results) in a single flat struct to keep the Wails RPC contract simple.

```go
// PRListResult is the return type of ListOpenPRs. On success, Items is populated
// and Error is empty. On error, Items may be empty and Error describes the failure.
// RateLimitReset is set (RFC3339) when the GitHub API rate limit was exhausted.
// IncompleteResults is true when GitHub returned incomplete_results=true.
type PRListResult struct {
    Items             []PRListItem `json:"items"`
    RateLimitReset    string       `json:"rate_limit_reset,omitempty"` // RFC3339
    IncompleteResults bool         `json:"incomplete_results,omitempty"`
    Error             string       `json:"error,omitempty"`
}
```

**State transitions**:

```
ListOpenPRs called
  ├─ Auth not ready              → PRListResult{Error: "not authenticated"}
  ├─ Validation failure          → PRListResult{Error: "..."}
  ├─ GitHub API rate limited     → PRListResult{RateLimitReset: "<RFC3339>", Error: "rate limit"}
  ├─ Network / other error       → PRListResult{Error: "<message>"}
  └─ Success                     → PRListResult{Items: [...], IncompleteResults: bool}
```

---

## Frontend Composable

### `usePRFilters` — New

**Location**: `frontend/src/composables/usePRFilters.ts`

#### TypeScript Interfaces

```typescript
/** Which involvement roles to include in results. At least one must be active. */
export interface PRListFilters {
    includeAuthor:   boolean
    includeAssignee: boolean
    includeReviewer: boolean
    repo:            string   // "owner/repo" or ""
    org:             string   // org login or ""
    author:          string   // GitHub login or ""
    updatedAfter:    string   // ISO 8601 date string or ""
    includeDrafts:   boolean
}
```

#### Module-Level Singleton State

```typescript
// Declared at module scope — survives route navigation
const includeAuthor   = ref<boolean>(true)
const includeAssignee = ref<boolean>(true)
const includeReviewer = ref<boolean>(true)
const repo            = ref<string>('')
const org             = ref<string>('')
const author          = ref<string>('')
const updatedAfter    = ref<string>('')
const includeDrafts   = ref<boolean>(false)
```

#### Exported Actions

| Action | Description |
|---|---|
| `toggleAuthor()` | Toggle `includeAuthor`; no-op if it would deselect all three |
| `toggleAssignee()` | Toggle `includeAssignee`; same guard |
| `toggleReviewer()` | Toggle `includeReviewer`; same guard |
| `setRepo(v: string)` | Set repo filter (empty = clear) |
| `setOrg(v: string)` | Set org filter (empty = clear) |
| `setAuthor(v: string)` | Set author filter (empty = clear); debounced watch |
| `setUpdatedAfter(v: string)` | Set date filter |
| `toggleIncludeDrafts()` | Toggle include/exclude drafts |
| `clearAllFilters()` | Reset all to defaults |
| `toPayload(): PRListFilters` | Snapshot current state for Wails call |

#### Constraint

- `toggleAuthor/Assignee/Reviewer`: silently returns early if removing the type would leave all three inactive.
- UI communicates this via `:disabled` on the last remaining active toggle.

---

## Existing Models (Unchanged)

| Type | File | Status |
|---|---|---|
| `PullRequestSummary` | `internal/model/model.go` | Unchanged — used by future detail view |
| `CommentThreadDTO` | `internal/model/model.go` | Unchanged |
| `CommentDTO` | `internal/model/model.go` | Unchanged |
| `AuthState` | `internal/model/model.go` | Unchanged |

---

## Entity Relationships

```
AuthState (existing)
    │ (authenticated user login flows into filter queries)
    ▼
PRListFilters ──────────────────────► ListOpenPRs (Wails method)
    │                                       │
    │                              up to 3 GitHub Search queries
    │                              (one per active involvement type)
    │                                       │ dedup by HTMLURL
    │                                       ▼
    └──────────────────────────────── PRListResult
                                           │
                                      PRListItem[]
                                           │
                                     (click row)
                                           │
                                           ▼
                                    PullRequestSummary (existing detail view)
```
