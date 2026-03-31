# Research: Open PR List with Filters

**Feature**: 003-pr-list-filters  
**Date**: 2026-03-31  
**Status**: Complete — all unknowns resolved

---

## 1. GitHub Search API — Querying PRs

### Decision
Use `client.Search.Issues()` from go-github/v67 with a GitHub Search query string. Each filter change triggers a new query with all active filters compiled into the query string.

### Method Signature
```go
func (s *SearchService) Issues(
    ctx context.Context,
    query string,
    opts *SearchOptions,
) (*IssuesSearchResult, *Response, error)
```

**`SearchOptions`**: `Sort` ("updated"), `Order` ("desc"), embedded `ListOptions` (`PerPage: 100`, `Page: N`).  
**`IssuesSearchResult`**: `Total *int`, `IncompleteResults *bool`, `Issues []*github.Issue`.

### Query String Construction
Qualifiers are space-separated (never `+`). All qualifiers are AND-ed implicitly.

| Qualifier | Example |
|---|---|
| `is:pr is:open` | always present (restricts to open PRs) |
| `author:LOGIN` | authored by user |
| `assignee:LOGIN` | assigned to user |
| `review-requested:LOGIN` | review requested from user |
| `repo:OWNER/REPO` | repository filter |
| `org:ORG` | organization filter |
| `author:LOGIN` (filter) | author filter (different from involvement) |
| `created:>=DATE` | date filter (opened after) |
| `updated:>=DATE` | date filter (updated after) |
| `draft:false` | exclude drafts (default) |
| `draft:true` | include drafts |
| `archived:false` | always include (spec requirement) |

### Key Finding: OR Across Involvement Types
The GitHub Search API **does not support OR across different qualifier types** natively. The confirmed production pattern (AndiDog/workboard, spiffcs/triage) is:

> Run **separate queries** for each active involvement type, then **deduplicate by HTMLURL**.

```go
// Pseudocode for involvement type query construction
queries := []string{}
if filters.IncludeAuthor {
    queries = append(queries, buildBaseQuery(filters) + fmt.Sprintf(` author:"%s"`, login))
}
if filters.IncludeAssignee {
    queries = append(queries, buildBaseQuery(filters) + fmt.Sprintf(` assignee:"%s"`, login))
}
if filters.IncludeReviewer {
    queries = append(queries, buildBaseQuery(filters) + fmt.Sprintf(` review-requested:"%s"`, login))
}
// Run each query, deduplicate results by HTMLURL
```

**Rationale**: The alternative `involves:LOGIN` qualifier covers author/assignee/mentioned/commenter but NOT review-requested. Running 3 separate queries is the only way to get full coverage.

**Alternatives considered**:
- `involves:LOGIN` — rejected: misses review-requested
- Client-side merge of a single `involves:` query with a separate `review-requested:` query — overly complex; separate 3 queries + dedup is simpler and more predictable

---

## 2. Extracting Issue Fields

Key field access patterns on `*github.Issue` returned from Search:

```go
issue.GetTitle()              // string
issue.GetNumber()             // int — PR number within repo
issue.GetState()              // string — "open"
issue.GetHTMLURL()            // string — https://github.com/owner/repo/pull/123
issue.GetRepositoryURL()      // string — https://api.github.com/repos/owner/repo
issue.GetUser().GetLogin()    // string — author login
issue.GetCreatedAt().Time     // time.Time
issue.GetUpdatedAt().Time     // time.Time
issue.GetDraft()              // bool — false if nil
issue.IsPullRequest()         // bool — true when PullRequestLinks != nil
```

**Owner/repo extraction** (from RepositoryURL):
```go
func repoFromURL(repoURL string) (owner, repo string) {
    trimmed := strings.TrimPrefix(repoURL, "https://api.github.com/repos/")
    parts := strings.SplitN(trimmed, "/", 3)
    if len(parts) < 2 { return "", "" }
    return parts[0], parts[1]
}
```

**Draft detection**: The `draft` field IS populated by the Search API for PR results. Use `issue.GetDraft()` (safe on nil pointer). Also filter at query time with `draft:false` for better performance.

---

## 3. Rate Limit Handling

Two error types from go-github:

```go
var rateLimitErr *github.RateLimitError
var abuseErr *github.AbuseRateLimitError

switch {
case errors.As(err, &rateLimitErr):
    resetAt := rateLimitErr.Rate.Reset.Time   // time.Time
    // Return resetAt to frontend for display
case errors.As(err, &abuseErr):
    retryAfter := abuseErr.GetRetryAfter()    // *time.Duration
    // Return hint to frontend
}
```

**Search API rate limit**: 30 requests/min (authenticated). With 3 involvement types and up to ~10 pagination calls each, a single full fetch could use up to ~30 requests. Rate limiting is a real concern for users with many PRs.

**Decision**: Return rate limit error as a structured error to the frontend with the reset time included. No auto-retry. The frontend will display: "Rate limit reached. Try again after HH:MM:SS."

---

## 4. Pagination

```go
opts := &github.SearchOptions{
    Sort:  "updated",
    Order: "desc",
    ListOptions: github.ListOptions{PerPage: 100},
}
for {
    result, resp, err := client.Search.Issues(ctx, query, opts)
    // handle err
    all = append(all, result.Issues...)
    if resp.NextPage == 0 { break }
    opts.Page = resp.NextPage
}
```

**GitHub cap**: Search API returns at most 1000 results total (10 pages × 100). `result.IncompleteResults` will be `true` if GitHub timed out internally.

**Decision**: Paginate fully up to the 1000-item cap per involvement type query. If `IncompleteResults` is true, surface a warning. After deduplication across 3 queries, the practical result set will be well under 1000 for the vast majority of users.

---

## 5. Vue Filter State Composable

### Decision
Use a **module-level singleton composable** (`usePRFilters.ts`) following the existing `useAuth.ts` pattern:
- Filter state `ref`s declared at module scope (outside the exported function)
- Survives route navigation within the session — no need for Pinia or `provide/inject`
- Watches added fresh on each mount but cleaned up via stored `WatchStopHandle`s

### Debouncing Strategy

| Input type | Debounce | Rationale |
|---|---|---|
| Free-text author filter | 300ms (`useDebounceFn` from VueUse) | Prevents per-keystroke API calls |
| Checkbox/toggle (involvement, drafts) | None | Discrete, intentional user action |
| Select (repo, org, date range) | None | Single-click, discrete selection |

**Tool**: `useDebounceFn` from `@vueuse/core` (already in `package.json ^14.2.1`).

### "At Least One Involvement Type" Constraint
Enforced in the composable's `toggleInvolvementType` action:
```typescript
if (next.has(type) && next.size === 1) return  // silent early return
```
UI communicates via `:disabled` on the last active checkbox.

### Alternatives considered
- **Pinia store** — rejected: overkill for in-memory filter state with no cross-feature sharing requirement
- `useStorage` / `localStorage` — rejected: spec explicitly states filter state is not persisted to disk
- **Option A** (call composable in App.vue once) — viable but couples layout to PR feature; rejected in favour of Option B
- **Option B** (stop previous watches on re-mount) — selected: self-contained, follows existing `useAuth.ts` pattern

---

## 6. New Model Types Required

The existing `PullRequestSummary` struct covers single-PR detail view. For the list view, a **new, lighter DTO** is needed:

### Decision: New `PRListItem` model
```go
// PRListItem is a lightweight DTO for a single row in the PR list view.
type PRListItem struct {
    Number      int    `json:"number"`
    Title       string `json:"title"`
    Owner       string `json:"owner"`
    Repo        string `json:"repo"`
    AuthorLogin string `json:"author_login"`
    CreatedAt   string `json:"created_at"`   // RFC3339
    UpdatedAt   string `json:"updated_at"`   // RFC3339
    HTMLURL     string `json:"html_url"`
    IsDraft     bool   `json:"is_draft"`
}

// PRListFilters carries the filter state from the frontend to the Go backend.
type PRListFilters struct {
    IncludeAuthor   bool   `json:"include_author"`
    IncludeAssignee bool   `json:"include_assignee"`
    IncludeReviewer bool   `json:"include_reviewer"`
    Repo            string `json:"repo"`    // "owner/repo" or ""
    Org             string `json:"org"`     // org login or ""
    Author          string `json:"author"`  // GitHub login or ""
    UpdatedAfter    string `json:"updated_after"`  // RFC3339 or ""
    IncludeDrafts   bool   `json:"include_drafts"`
}

// PRListResult is the Wails method return type for ListOpenPRs.
type PRListResult struct {
    Items           []PRListItem `json:"items"`
    RateLimitReset  string       `json:"rate_limit_reset,omitempty"` // RFC3339 — set on rate limit error
    IncompleteResults bool       `json:"incomplete_results,omitempty"`
    Error           string       `json:"error,omitempty"`
}
```

**Rationale**: `PullRequestSummary` carries `CommentCount`, `UnresolvedCount`, `NodeID`, branch info — fields only needed in the detail view. A separate lightweight DTO keeps list-view serialization minimal.

---

## 7. Wails Method Design

### Decision: Single `ListOpenPRs` method on `App`
```go
// ListOpenPRs fetches open pull requests for the authenticated user matching
// the given filters. It runs up to 3 separate GitHub Search queries (one per
// active involvement type) and deduplicates results by PR HTMLURL.
func (a *App) ListOpenPRs(filters model.PRListFilters) (model.PRListResult, error)
```

**Rationale**:
- Single Wails call from frontend per filter change — simpler than orchestrating multiple calls in JS
- Backend handles query construction, multi-query fan-out, deduplication, and error normalization
- Fits the existing `App`-method pattern (synchronous Wails RPC, blocking on goroutine)

**Alternatives considered**:
- Separate backend calls per involvement type in frontend — rejected: exposes API complexity to Vue
- GraphQL API — rejected: REST Search API is sufficient; avoids adding GraphQL client complexity
- Caching layer in Go — rejected: spec says every filter change issues a new query; caching would contradict FR-013

---

## 8. New shadcn-vue Components Needed

The existing shadcn-vue primitives (`badge/`, `button/`, `card/`, `dialog/`, `scroll-area/`, `separator/`, `sonner/`) cover most needs. The following are missing:

| Component needed | Available? | Plan |
|---|---|---|
| `Checkbox` | Not yet added | Add shadcn-vue `checkbox` primitive (uses reka-ui) |
| `Select` / `Combobox` | Not yet added | Add shadcn-vue `select` primitive for repo/org/date filters |
| `Input` | Not yet added | Add shadcn-vue `input` primitive for author text filter |
| `Toggle` / `Switch` | Not yet added | Add shadcn-vue `switch` primitive for include-drafts toggle |
| `Skeleton` | Not yet added | Add shadcn-vue `skeleton` for loading state rows |

**Installation**: shadcn-vue primitives are added by copying component source into `frontend/src/components/ui/` (owned source model — no CLI required, they are already installed via `reka-ui`/`radix-vue` in `package.json`).

---

## All Unknowns Resolved

| Unknown | Resolution |
|---|---|
| OR across involvement types in GitHub Search | Run 3 separate queries, deduplicate by HTMLURL |
| Rate limit detection | `errors.As(err, &rateLimitErr)` → return `rateLimitErr.Rate.Reset.Time` |
| Draft field availability in Search | Confirmed populated; also filter with `draft:false` qualifier |
| Repository name extraction | Parse `RepositoryURL` string (strip API prefix) |
| Filter state persistence within session | Module-level ref singleton in composable (same pattern as `useAuth`) |
| Debouncing strategy | 300ms for free-text, none for discrete toggles/selects |
| DTO design for list vs. detail | Separate lightweight `PRListItem` + `PRListFilters` + `PRListResult` |
| Wails method shape | Single `ListOpenPRs(filters PRListFilters) PRListResult` method |
