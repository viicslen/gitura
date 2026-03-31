# Wails RPC Contract: ListOpenPRs

**Feature**: 003-pr-list-filters  
**Method**: `App.ListOpenPRs`  
**Layer**: Go backend → Wails → TypeScript frontend

---

## Method Signature

### Go (backend)

```go
// ListOpenPRs fetches open pull requests for the authenticated user matching
// the given filter state. It issues up to 3 separate GitHub Search API queries
// (one per active involvement type: author, assignee, review-requested) and
// deduplicates results by PR HTMLURL before returning.
//
// On success, Result.Items contains PRs sorted by UpdatedAt descending.
// On rate limit exhaustion, Result.RateLimitReset is set and Result.Error is non-empty.
// On any other failure, Result.Error is non-empty and Result.Items is empty.
//
// The method is synchronous and blocks for the duration of all API queries.
// The frontend must call it from an async context.
func (a *App) ListOpenPRs(filters model.PRListFilters) (model.PRListResult, error)
```

### TypeScript (Wails-generated binding, post `wails generate module`)

```typescript
// wailsjs/go/main/App.d.ts (auto-generated — do not edit)
export function ListOpenPRs(
    filters: main.PRListFilters
): Promise<main.PRListResult>;
```

---

## Input Contract: `PRListFilters`

```typescript
// wailsjs/go/main/models.ts (auto-generated — do not edit)
export namespace main {
    export interface PRListFilters {
        include_author:   boolean;   // true = include PRs authored by logged-in user
        include_assignee: boolean;   // true = include PRs assigned to logged-in user
        include_reviewer: boolean;   // true = include PRs where user is review-requested
        repo:             string;    // "owner/repo" or "" (no filter)
        org:              string;    // GitHub org login or "" (no filter)
        author:           string;    // filter by PR author login, or "" (no filter)
        updated_after:    string;    // RFC3339 datetime or "" (no filter)
        include_drafts:   boolean;   // false = exclude drafts (default)
    }
}
```

### Validation Rules (enforced server-side)

| Rule | Error returned |
|---|---|
| All three involvement flags false | `PRListResult{Error: "at least one involvement type must be selected"}` |
| `repo` non-empty and contains no `/` | `PRListResult{Error: "invalid repo format: expected owner/repo"}` |
| `updated_after` non-empty and not RFC3339 | `PRListResult{Error: "invalid updated_after: must be RFC3339"}` |
| Not authenticated (`a.ghClient == nil`) | `PRListResult{Error: "not authenticated"}` |

### Default Values (matching spec FR-001, FR-015)

```typescript
const DEFAULT_FILTERS: PRListFilters = {
    include_author:   true,
    include_assignee: true,
    include_reviewer: true,
    repo:             "",
    org:              "",
    author:           "",
    updated_after:    "",
    include_drafts:   false,
};
```

---

## Output Contract: `PRListResult`

```typescript
export namespace main {
    export interface PRListResult {
        items:              PRListItem[];  // empty array on error or no results
        rate_limit_reset:   string;       // RFC3339 — non-empty on rate limit error only
        incomplete_results: boolean;      // true if GitHub returned incomplete_results=true
        error:              string;       // non-empty on any error
    }

    export interface PRListItem {
        number:       number;  // PR number within the repo
        title:        string;
        owner:        string;  // repo owner (org or user)
        repo:         string;  // repo name (without owner)
        author_login: string;  // GitHub login of PR author
        created_at:   string;  // RFC3339
        updated_at:   string;  // RFC3339
        html_url:     string;  // canonical GitHub PR URL
        is_draft:     boolean;
    }
}
```

### Response Scenarios

| Scenario | `items` | `error` | `rate_limit_reset` | `incomplete_results` |
|---|---|---|---|---|
| Success, results found | `[...]` | `""` | `""` | `false` (usually) |
| Success, no results | `[]` | `""` | `""` | `false` |
| Rate limit exhausted | `[]` | `"rate limit exceeded"` | `"2026-03-31T14:30:00Z"` | `false` |
| Network error | `[]` | `"network error: ..."` | `""` | `false` |
| Token expired | `[]` | `"authentication required"` | `""` | `false` |
| GitHub timeout (partial) | `[...]` | `""` | `""` | `true` |
| Not authenticated | `[]` | `"not authenticated"` | `""` | `false` |

---

## Frontend Usage Pattern

```typescript
// frontend/src/pages/PRPage.vue
import { ListOpenPRs } from '@/wailsjs/go/main/App'
import type { PRListFilters, PRListResult, PRListItem } from '@/wailsjs/go/main/models'

const loading = ref(false)
const result = ref<PRListResult | null>(null)

async function fetchPRs(filters: PRListFilters): Promise<void> {
    loading.value = true
    result.value = null
    try {
        result.value = await ListOpenPRs(filters)
    } catch (err) {
        // Wails wraps Go errors — handle unexpected JS-level errors here
        result.value = { items: [], error: String(err), rate_limit_reset: '', incomplete_results: false }
    } finally {
        loading.value = false
    }
}
```

### Error Handling in the UI

```typescript
// Derived state helpers
const isRateLimited = computed(() => !!result.value?.rate_limit_reset)
const hasError = computed(() => !!result.value?.error && !isRateLimited.value)
const hasResults = computed(() => (result.value?.items ?? []).length > 0)
const isEmpty = computed(() => !loading.value && !hasError.value && !hasResults.value)
```

---

## Wails Binding Regeneration

After adding `ListOpenPRs` to `app.go`, regenerate bindings:

```bash
wails generate module
```

This updates:
- `wailsjs/go/main/App.d.ts` — adds `ListOpenPRs` function export
- `wailsjs/go/main/models.ts` — adds `PRListFilters`, `PRListResult`, `PRListItem` interfaces

The `wailsjs/` directory is auto-generated — **do not edit manually**.
