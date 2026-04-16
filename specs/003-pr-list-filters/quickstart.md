# Quickstart: Open PR List with Filters

**Feature**: 003-pr-list-filters  
**Date**: 2026-03-31

---

## Prerequisites

- Go 1.25+ installed
- Node.js 18+ and pnpm installed
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- `golangci-lint` installed (for linting)
- A GitHub OAuth App with Device Flow enabled (Client ID already configured in app)

---

## Dev Environment Setup

```bash
# 1. Switch to the feature branch
git checkout 003-pr-list-filters

# 2. Install frontend dependencies (if not already done)
cd frontend && pnpm install && cd ..

# 3. Start Wails dev server (hot-reloads both Go and Vue)
wails dev
```

The app will open in a WebView window. Auth via the existing GitHub Device Flow before accessing the PR list.

---

## Key Files to Implement

### Backend (Go)

| File | Change |
|---|---|
| `internal/model/model.go` | Add `PRListItem`, `PRListFilters`, `PRListResult` structs |
| `internal/github/pr_search.go` | New file: `SearchOpenPRs(ctx, client, login, filters)` helper |
| `app.go` | Add `ListOpenPRs(filters PRListFilters) PRListResult` Wails method |

### Frontend (Vue/TypeScript)

| File | Change |
|---|---|
| `frontend/src/composables/usePRFilters.ts` | New file: filter state singleton composable |
| `frontend/src/pages/PRPage.vue` | Replace stub: full PR list UI with filters |
| `frontend/src/components/ui/checkbox/` | Add shadcn-vue checkbox primitive |
| `frontend/src/components/ui/input/` | Add shadcn-vue input primitive |
| `frontend/src/components/ui/select/` | Add shadcn-vue select primitive |
| `frontend/src/components/ui/switch/` | Add shadcn-vue switch primitive |
| `frontend/src/components/ui/skeleton/` | Add shadcn-vue skeleton primitive |

### Auto-Generated (do not edit)

| File | Trigger |
|---|---|
| `wailsjs/go/main/App.d.ts` | `wails generate module` |
| `wailsjs/go/main/models.ts` | `wails generate module` |

---

## Implementation Order

### Step 1: Add Go Models
Edit `internal/model/model.go` to add `PRListItem`, `PRListFilters`, `PRListResult`.

### Step 2: Implement Search Helper
Create `internal/github/pr_search.go`:

```go
// SearchOpenPRs runs up to 3 GitHub Search queries (one per active involvement
// type), deduplicates results by HTMLURL, and returns sorted PRListItems.
func SearchOpenPRs(
    ctx context.Context,
    client *github.Client,
    login string,
    filters model.PRListFilters,
) (model.PRListResult, error)
```

Key implementation notes:
- Build query string: always include `is:pr is:open archived:false`
- Append `draft:false` unless `filters.IncludeDrafts` is true
- Append `repo:OWNER/REPO` if `filters.Repo` is non-empty
- Append `org:ORG` if `filters.Org` is non-empty
- Append `author:AUTHOR` if `filters.Author` is non-empty
- Append `updated:>=DATE` if `filters.UpdatedAfter` is non-empty
- For each active involvement type, add the involvement qualifier (`author:login`, `assignee:login`, `review-requested:login`) to the base query
- Paginate each query with `PerPage: 100`, `Sort: "updated"`, `Order: "desc"`
- Deduplicate by `issue.GetHTMLURL()`
- Sort final slice by `UpdatedAt` descending
- Detect rate limit with `errors.As(err, &rateLimitErr)` → return `RateLimitReset`

### Step 3: Wire Wails Method
Add to `app.go`:
```go
func (a *App) ListOpenPRs(filters model.PRListFilters) (model.PRListResult, error) {
    if a.ghClient == nil {
        return model.PRListResult{Error: "not authenticated"}, nil
    }
    authState, err := a.GetAuthState()
    if err != nil || !authState.IsAuthenticated {
        return model.PRListResult{Error: "not authenticated"}, nil
    }
    return github.SearchOpenPRs(a.ctx, a.ghClient, authState.Login, filters)
}
```

### Step 4: Regenerate Wails Bindings
```bash
wails generate module
```

### Step 5: Add shadcn-vue Primitives
Add `checkbox`, `input`, `select`, `switch`, `skeleton` components to `frontend/src/components/ui/`. Follow the existing pattern in that directory (copy from shadcn-vue docs, they use reka-ui which is already in `package.json`).

### Step 6: Implement Filter Composable
Create `frontend/src/composables/usePRFilters.ts` with module-level singleton refs following the `useAuth.ts` pattern.

### Step 7: Build PR List Page
Replace the stub in `frontend/src/pages/PRPage.vue` with the full implementation:
- Filter bar (involvement toggles, repo/org/author/date selects, include-drafts switch)
- PR list (scrollable rows with title, repo, author, age)
- Loading skeleton
- Empty states (no PRs vs. no filter matches)
- Error state (network error, token expiry, rate limit with reset time)

---

## Testing

```bash
# Run all Go tests
go test ./...

# Run tests with coverage (must reach 80% per constitution)
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint
golangci-lint run

# Frontend type-check
cd frontend && pnpm type-check
```

### Test Files to Create

| Test file | Coverage target |
|---|---|
| `internal/github/pr_search_test.go` | `SearchOpenPRs` — all filter combinations, rate limit, pagination |
| `internal/model/model_test.go` | `PRListFilters` validation (if validation logic extracted to model) |
| `tests/integration/pr_list_test.go` | End-to-end via recorded HTTP fixtures (no live API) |

Test names must follow: `TestFunctionName_Scenario_ExpectedOutcome`

Example:
```go
func TestSearchOpenPRs_RateLimited_ReturnsRateLimitReset(t *testing.T) { ... }
func TestSearchOpenPRs_AllFiltersActive_BuildsCorrectQuery(t *testing.T) { ... }
func TestSearchOpenPRs_AllInvolvementTypesActive_DeduplicatesResults(t *testing.T) { ... }
```

---

## Build

```bash
# Build for current platform
wails build

# Output: build/bin/gitura (Linux), build/bin/gitura.app (macOS), build/bin/gitura.exe (Windows)
```
