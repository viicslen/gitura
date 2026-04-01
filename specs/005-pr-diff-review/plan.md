# Implementation Plan: PR Diff Review

**Branch**: `005-pr-diff-review` | **Date**: 2026-04-01 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/005-pr-diff-review/spec.md`

## Summary

Adds a side-by-side PR diff review view to Gitura's existing PR detail page. A multi-value view toggle (segmented control) inside `ReviewPage.vue` switches between the existing conversation view and a new "Files changed" diff view. Files are listed in a sidebar and loaded on-demand; diffs are parsed in Go and rendered as side-by-side split tables. Reviewers can leave inline comments posted immediately or batched into a pending review submitted with a verdict (Approve / Request Changes / Comment). Existing reviewer threads can optionally be toggled on.

## Technical Context

**Language/Version**: Go 1.25; TypeScript 5.x (strict mode)  
**Primary Dependencies**: Wails v2.11, go-github/v67, golang.org/x/oauth2, go-keyring; Vue 3, VueUse ^14.2.1, shadcn-vue (reka-ui + radix-vue), lucide-vue-next  
**Storage**: In-memory only (pending review ID + comment list); no file persistence  
**Testing**: `go test` + `testify`; `httptest` for HTTP fixture recording  
**Target Platform**: Desktop (macOS/Linux/Windows) via Wails WebView  
**Project Type**: Desktop application  
**Performance Goals**: File diff render ≤ 500ms for 5000-line files (SC-001, SC-004); inline comment form open < 1s (SC-002)  
**Constraints**: diff-parsing package ≥ 90% line coverage (Constitution II); shadcn-vue only for UI primitives (Constitution III); keyboard nav + ARIA labels required (Constitution III)  
**Scale/Scope**: Up to ~100 changed files per PR; diffs up to 5000 lines

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Code Quality | PASS | All exported symbols will have doc comments; cyclomatic complexity ≤ 10 per function; `ParseUnifiedDiff` state machine kept simple via helper functions |
| II. Testing | PASS | `internal/github/diff.go` (diff parsing) targeted at 90% coverage; all new packages have unit tests; integration fixtures for new API calls |
| III. UX Consistency | PASS | `ViewToggle` uses shadcn-vue Tabs; keyboard nav via Tabs component; ARIA labels on all interactive diff elements; error/loading/empty states handled |
| IV. Performance | PASS | Files loaded on demand (FR-011); diff rendering ≤ 500ms enforced by collapsing unchanged hunks by default; no full-PR pre-loading |

## Project Structure

### Documentation (this feature)

```text
specs/005-pr-diff-review/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── wails-bindings.md  # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks — not yet created)
```

### Source Code

```text
# Go backend
internal/
├── model/
│   └── model.go                  # MODIFY: add PRFileDTO, DiffHunkDTO, DiffLineDTO,
│                                 #   ParsedDiffDTO, DraftCommentDTO, PendingReviewDTO,
│                                 #   ReviewSubmitDTO, ReviewSubmitResult
├── github/
│   ├── diff.go                   # NEW: ListPRFiles, ParseUnifiedDiff
│   └── review_submit.go          # NEW: CreatePendingReview, AddCommentToPendingReview,
│                                 #   SubmitReview, DeletePendingReview
app.go                            # MODIFY: add pendingReviewID, pendingComments,
│                                 #   prFilesCache, prCommitSHA fields;
│                                 #   add GetPRFiles, GetFileDiff, AddDraftComment,
│                                 #   PostImmediateComment, GetPendingReview,
│                                 #   SubmitReview, DiscardPendingReview methods

# Go tests
internal/github/diff_test.go      # NEW: table-driven unit tests for ParseUnifiedDiff
internal/github/review_submit_test.go  # NEW: unit tests with httptest fixtures

# Auto-generated (run wails generate module)
wailsjs/go/main/App.d.ts          # REGENERATE
wailsjs/go/models.ts              # REGENERATE

# Vue frontend
frontend/src/
├── pages/
│   └── ReviewPage.vue            # MODIFY: add prView toggle, render DiffReviewView
├── components/
│   ├── ViewToggle.vue            # NEW: segmented tab control
│   ├── DiffReviewView.vue        # NEW: top-level diff review container
│   ├── DiffFileSidebar.vue       # NEW: file list sidebar
│   ├── DiffFileView.vue          # NEW: diff renderer for one file
│   ├── DiffHunk.vue              # NEW: single hunk side-by-side table
│   ├── DiffLine.vue              # NEW: single diff line + comment affordance
│   └── InlineCommentForm.vue     # NEW: anchored inline comment form
└── composables/
    └── useDiffReview.ts          # NEW: diff session state composable
```

## Implementation Phases

### Phase A — Go backend (no frontend dependency)

**A1. Data types** (`internal/model/model.go`)
- Add all 8 new types from `data-model.md`
- Run `go build ./...` to verify

**A2. Diff parsing** (`internal/github/diff.go` + `diff_test.go`)
- `ListPRFiles`: paginate `PullRequestsService.ListFiles`
- `ParseUnifiedDiff`: state machine; hunk header regex; line classification
- Tests: normal, pure-add, pure-delete, rename, binary, no-newline, multi-hunk
- Gate: ≥ 90% line coverage before proceeding

**A3. Review submit** (`internal/github/review_submit.go` + `review_submit_test.go`)
- `CreatePendingReview`: `PullRequestsService.CreateReview` with `Event: "PENDING"`
- `AddCommentToPendingReview`: raw HTTP `POST .../reviews/{id}/comments`
- `SubmitReview`: `PullRequestsService.SubmitReview`
- `DeletePendingReview`: `PullRequestsService.DeletePendingReview`

**A4. Wails bindings** (`app.go`)
- Add 4 new struct fields
- Populate `prCommitSHA` in `LoadPullRequest`
- Add 7 new bound methods per `contracts/wails-bindings.md`
- Run `wails generate module`
- Run `go build ./...`

### Phase B — Frontend (depends on Phase A bindings)

**B1. `ViewToggle.vue`** — shadcn-vue Tabs; `v-model`; keyboard nav; ARIA

**B2. Modify `ReviewPage.vue`** — add `prView` ref; render toggle + conditional views; `<KeepAlive>` or `v-show` for state preservation

**B3. `useDiffReview.ts`** composable — wraps `GetPRFiles`, `GetFileDiff`, `AddDraftComment`, `PostImmediateComment`, `GetPendingReview`, `SubmitReview`, `DiscardPendingReview`

**B4. `DiffFileSidebar.vue`** — file list with status badges, additions/deletions counts; emits `select`; keyboard navigable

**B5. `DiffLine.vue`** — renders one table row; hover affordance (+ icon); click to open comment form; multi-line drag selection

**B6. `DiffHunk.vue`** — paired left/right columns; context lines, add/delete pairing; collapsed context bar (expandable)

**B7. `DiffFileView.vue`** — iterates hunks; manages which comment form is open; passes existing threads when toggle is on

**B8. `InlineCommentForm.vue`** — anchored form; "Add single comment" and "Start review"/"Add review comment" buttons; handles both posting modes

**B9. `DiffReviewView.vue`** — top-level layout: sidebar + file view + submit panel; next/previous navigation; "Show reviewer comments" toggle; review submit panel (verdict + body)

### Phase C — Polish and tests

**C1. Error/loading/empty states** — all three states on all components

**C2. Binary/rename handling** — binary placeholder component; rename display in sidebar and file header

**C3. Stale thread display** — "Outdated" badge on threads with `original_line != line`

**C4. Keyboard navigation** — file navigation with keyboard shortcuts; tab order on comment form

**C5. Run full test suite** — `go test ./... -coverprofile=coverage.out`; verify diff.go ≥ 90%

## Complexity Tracking

No constitution violations. The raw HTTP call for `POST .../reviews/{id}/comments` is necessary because go-github/v67 does not implement this endpoint; the raw HTTP pattern is already established in the codebase.
