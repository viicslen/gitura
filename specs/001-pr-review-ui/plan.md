# Implementation Plan: PR Review UI

**Branch**: `001-pr-review-ui` | **Date**: 2026-03-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/001-pr-review-ui/spec.md`

## Summary

Build a Wails v2 desktop application (Go backend + Vue 3 frontend) that lets developers
review GitHub pull requests without leaving their desktop. The app authenticates via
GitHub OAuth 2.0 Device Flow, retrieves PR review comments via the GitHub REST API,
and provides a summary list view and one-by-one navigation with actions: reply, resolve,
commit suggestion, and filter by ignored commenter.

## Technical Context

**Language/Version**: Go 1.22+; TypeScript 5.x (strict)
**Primary Dependencies**:
- Wails v2 (desktop app framework + Go↔JS bindings)
- Vue 3 + Vite (frontend)
- shadcn-vue + Tailwind CSS v4 (component library)
- `github.com/google/go-github/v67` (GitHub REST API)
- `github.com/zalando/go-keyring` (OS token storage)

**Storage**: OS native keychain (tokens); local JSON file (ignored commenters);
in-memory cache (PR data per session)

**Testing**: `go test` + `testify`; `httptest` for HTTP fixture recording; no live API
calls in CI

**Target Platform**: Desktop — macOS, Linux, Windows

**Project Type**: Desktop GUI app

**Performance Goals**: PR comment list (50 comments) loads in ≤ 3s; comment navigation
transition ≤ 300ms; diff render (5 000-line file) ≤ 500ms; memory < 150MB

**Constraints**: No offline mode; requires active GitHub token; Linux requires
`libwebkit2gtk` + `libsecret`; Wails v2 cannot cross-compile (build each OS natively)

**Environment Variables**:
- `GITURA_GITHUB_CLIENT_ID` (required): GitHub OAuth App client ID. App MUST fail at
  startup with a descriptive error if this variable is not set.

**Auth Strategy**: GitHub OAuth 2.0 Device Flow only (v1). PAT input is not supported.

**OAuth Scope**: `repo` — required for GraphQL resolve/unresolve mutations on private
repositories. `public_repo` is insufficient for private repo thread mutations.

**Token Storage**: OS native keychain via `go-keyring`. Token is NEVER written to disk,
logged, or transmitted to the frontend.

**Keychain Unavailable**: If `go-keyring` returns an error indicating the keychain is
unavailable (e.g., no `libsecret` daemon on Linux), the app MUST show a modal error
with setup instructions and exit gracefully.

**Token Revocation**: A `401` response from any GitHub API call MUST trigger token
deletion from keychain and redirect to auth screen.

**Scale/Scope**: Single-user desktop app; one PR reviewed at a time per session

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Pre-Design | Post-Design |
|---|---|---|
| I. Code Quality (gofmt, golint, complexity ≤ 10, doc comments) | PASS — enforced by golangci-lint in CI | PASS — internal packages keep functions focused |
| II. Testing (80%/90% coverage, fixtures, TDD, naming convention) | PASS — httptest fixtures planned for all GitHub API calls | PASS — fixture files in `tests/fixtures/`; `testify` assertions |
| III. UX Consistency (single design language, keyboard nav, ARIA, error/loading/empty states) | PASS — shadcn-vue is the single component system | PASS — all views share shadcn-vue primitives; empty states defined in spec |
| IV. Performance (load ≤ 2s app, ≤ 500ms diff, < 150MB, cache API) | PASS — Wails WebView is lightweight; in-memory cache prevents duplicate fetches | PASS — in-memory cache per session; pagination prevents large memory spikes |

No violations. No complexity tracking required.

## Project Structure

### Documentation (this feature)

```text
specs/001-pr-review-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── wails-bindings.md  # Go↔Vue method contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
.
├── app.go                       # App struct + all Wails-bound public methods
├── main.go                      # wails.Run() entry point + app lifecycle hooks
├── go.mod
├── go.sum
├── wails.json
├── .golangci.yml
├── build/
│   ├── appicon.png
│   ├── darwin/Info.plist
│   └── windows/*.manifest
├── internal/
│   ├── model/                   # Shared serialisable domain types (DTOs)
│   │   └── model.go             # PullRequestSummary, CommentThreadDTO, CommentDTO,
│   │                            #   AuthState, DeviceFlowInfo, IgnoredCommenterDTO, etc.
│   ├── auth/                    # GitHub OAuth 2.0 Device Flow logic
│   │   ├── deviceflow.go        # StartDeviceFlow, PollDeviceFlow
│   │   └── deviceflow_test.go
│   ├── github/                  # GitHub API client wrapper
│   │   ├── client.go            # NewClient, authenticated client factory
│   │   ├── pr.go                # LoadPullRequest, GetCommentThreads, GetThread
│   │   ├── comments.go          # ReplyToComment, ResolveThread, UnresolveThread
│   │   ├── suggestion.go        # CommitSuggestion (Git Contents API path)
│   │   └── *_test.go            # httptest fixture-based tests (≥ 90% coverage)
│   ├── keyring/                 # Token persistence via go-keyring
│   │   ├── keyring.go           # SaveToken, LoadToken, DeleteToken
│   │   └── keyring_test.go
│   └── settings/                # Ignored-commenter list persistence
│       ├── settings.go          # Load, Save, Add, Remove
│       └── settings_test.go
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── CommentList.vue  # Summary list view of comment threads
│   │   │   ├── CommentDetail.vue # One-by-one comment view (with diff hunk)
│   │   │   ├── ReplyForm.vue    # Reply compose + submit
│   │   │   ├── SuggestionBlock.vue # Renders suggestion diff + commit button
│   │   │   └── PRInput.vue      # PR URL / number input field
│   │   ├── components/ui/       # shadcn-vue copied components
│   │   ├── composables/
│   │   │   ├── useAuth.ts       # Auth state, StartDeviceFlow, Poll, Logout
│   │   │   ├── usePR.ts         # LoadPullRequest, GetCommentThreads, navigation
│   │   │   └── useSettings.ts   # Ignored commenters CRUD
│   │   ├── lib/utils.ts         # cn() utility
│   │   ├── pages/
│   │   │   ├── AuthPage.vue     # Device flow UI (user code display)
│   │   │   ├── PRPage.vue       # Main PR review page (list + detail)
│   │   │   └── SettingsPage.vue # Ignored-commenter management
│   │   ├── App.vue              # Root component + routing
│   │   ├── main.ts
│   │   └── style.css            # Tailwind v4 import
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── wailsjs/                     # Auto-generated — do not hand-edit
│   ├── go/main/App.d.ts
│   ├── go/main/models.ts
│   └── runtime/runtime.d.ts
└── tests/
    ├── fixtures/                # Recorded HTTP responses for GitHub API tests
    │   ├── pr_response.json
    │   ├── comments_response.json
    │   └── ...
    ├── integration/             # End-to-end Go tests (fixture-based, no live API)
    └── unit/                    # Pure unit tests for logic without I/O
```

**Structure Decision**: Single Wails project. Go business logic in `internal/` packages
(unexported, testable in isolation). `app.go` is a thin adapter that delegates to
`internal/` and exposes the Wails-bound API. Frontend in `frontend/` per Wails
convention; `wailsjs/` is auto-generated and gitignored or committed as-is.

## Complexity Tracking

No constitution violations. No justifications required.
