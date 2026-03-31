# Implementation Plan: PR Deep Review Workflow

**Branch**: `004-pr-review-workflow` | **Date**: 2026-03-31 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/004-pr-review-workflow/spec.md`

## Summary

Build the PR deep review workflow: clicking a PR in the existing list (spec 003) navigates
in-app to a full review view. The view loads PR review threads fresh via the GitHub GraphQL
API (for resolved state + thread node IDs) while reusing cached PR metadata from the list.
Users navigate threads one-by-one with diff hunk context, reply, resolve/unresolve, and
commit suggestion blocks. A status banner flags draft/non-open PRs. An ignored-commenters
list (persisted to JSON on disk) filters bot noise server-side. All new Go logic lives
under `internal/github/` and `internal/settings/`; all new Vue components under
`frontend/src/pages/` and `frontend/src/components/`.

## Technical Context

**Language/Version**: Go 1.25; TypeScript 5.x (strict mode)  
**Primary Dependencies**: Wails v2.11, go-github/v67 (PR metadata + REST write ops), raw HTTP
GraphQL (review threads + resolve/unresolve — no new Go dependency), golang.org/x/oauth2,
go-keyring; Vue 3, VueUse ^14.2.1, shadcn-vue (reka-ui + radix-vue), lucide-vue-next  
**Storage**: `os.UserConfigDir()/gitura/ignored_commenters.json` (ignored commenters only);
all PR/comment data in-memory per session  
**Testing**: `go test` + `testify`; `httptest` for HTTP fixture recording; 80% line
coverage minimum; 90% for `internal/github/` and `internal/settings/`  
**Target Platform**: macOS / Linux / Windows (Wails WebView desktop app; no web deployment)  
**Project Type**: Desktop application  
**Performance Goals**: Comments visible ≤ 3 s (SC-001); thread navigation ≤ 1 s (SC-002);
reply confirmed ≤ 5 s (SC-003); suggestion commit ≤ 10 s (SC-004)  
**Constraints**: < 150 MB memory; diff hunk render ≤ 500 ms; no new Go module dependencies
beyond those already in go.mod (GraphQL via raw HTTP POST)  
**Scale/Scope**: Single PR session; tested against 200 comments (SC-008)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Pre-Design Status | Notes |
|---|---|---|
| I. Code Quality (gofmt, golint, cyclomatic ≤ 10) | PASS | Enforced by `.golangci.yml`; all new functions split to keep complexity ≤ 10 |
| II. Testing (80%/90% coverage, fixtures, TDD) | PASS | GraphQL responses mocked via `httptest` fixtures; `testify` assertions |
| III. UX Consistency (single design language, ARIA) | PASS | shadcn-vue is the only component system; all interactive elements need ARIA + keyboard nav |
| IV. Performance (load ≤ 2 s, diff ≤ 500 ms, mem < 150 MB) | PASS | Single in-memory cache; GraphQL paginates up to 100 threads per page |
| Technology Stack | PASS | No new module dependencies; GraphQL via `net/http` + `encoding/json` |
| Development Workflow | PASS | Branch `004-pr-review-workflow`; CI gates; UI screenshots required |

**Post-Design Re-check**: See `research.md` § Constitution Check.

## Project Structure

### Documentation (this feature)

```text
specs/004-pr-review-workflow/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── wails-bindings.md  # Phase 1 output
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code

```text
# Go backend — new files
internal/
├── github/
│   ├── client.go          # existing — NewClient()
│   ├── pr_search.go       # existing — SearchOpenPRs()
│   ├── pr.go              # NEW — FetchPRMetadata()
│   ├── comments.go        # NEW — FetchReviewThreads() via GraphQL
│   ├── resolve.go         # NEW — ResolveThread(), UnresolveThread() via GraphQL mutations
│   └── suggestion.go      # NEW — CommitSuggestion() via Git Contents API
└── settings/
    └── settings.go        # NEW — LoadIgnoredCommenters(), SaveIgnoredCommenters()

# Go backend — changed files
app.go                     # add: LoadPullRequest, GetCommentThreads, GetThread,
                           #      ReplyToComment, ResolveThread, UnresolveThread,
                           #      CommitSuggestion, GetIgnoredCommenters,
                           #      AddIgnoredCommenter, RemoveIgnoredCommenter
                           # extend App struct: ignoredCommenters []model.IgnoredCommenterDTO

internal/model/model.go    # add IsDraft bool to PullRequestSummary

# Frontend — new files
frontend/src/
├── pages/
│   └── ReviewPage.vue               # NEW — top-level review view
├── composables/
│   └── useReview.ts                 # NEW — thread list, nav state, show-resolved toggle
└── components/
    ├── CommentSummaryList.vue        # NEW — summary list (author, file, 200-char excerpt)
    ├── CommentDetailPanel.vue        # NEW — full comment body + diff hunk + actions
    ├── DiffHunkView.vue              # NEW — unified diff renderer with highlighted line
    ├── ReplyComposer.vue             # NEW — reply textarea + submit (draft-preserving)
    ├── SuggestionBlock.vue           # NEW — suggestion display + commit button
    └── PRStatusBanner.vue            # NEW — draft/closed/merged banner

# Frontend — changed files
frontend/src/App.vue                 # extend currentPage to 'pr'|'settings'|'review';
                                     # add selectedPRItem ref; handle open/close-review;
                                     # wrap PRPage in <KeepAlive> for scroll+filter preservation
frontend/src/pages/SettingsPage.vue  # add IgnoredCommenters management section

# Tests — new files
internal/github/comments_test.go
internal/github/resolve_test.go
internal/github/suggestion_test.go
internal/github/pr_test.go
internal/settings/settings_test.go
tests/fixtures/graphql/             # NEW — recorded GraphQL response fixtures
```

## Complexity Tracking

> No constitution violations — no entries required.
