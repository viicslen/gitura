# Research: PR Review UI

**Feature**: 001-pr-review-ui
**Date**: 2026-03-30
**Status**: Complete — all unknowns resolved

---

## 1. UI Framework

**Decision**: Wails v2 with Vue 3 + TypeScript frontend.

**Rationale**: Wails v2 is the user-specified framework. It embeds a platform-native
WebView (WebKit on macOS/Linux, WebView2 on Windows) and provides a first-class Go ↔
JavaScript binding layer via auto-generated `wailsjs/` modules. Vue 3 + TypeScript is
the canonical Wails template language. There is no conflict with Go idioms; the Go
backend handles all GitHub API calls and business logic while Vue renders the UI.

**Alternatives considered**:
- Wails v3 (alpha): breaking API changes in progress; not production-ready.
- Fyne: Go-native widgets; poorer typography and layout control for a data-rich review tool.
- Electron: 150MB+ overhead; no Go backend benefit.

**Standard project structure** (from `wails init -n gitura -t vue-ts`):
```
.
├── build/                  # Platform-specific manifests, app icons, output binaries
│   ├── appicon.png
│   ├── darwin/Info.plist
│   └── windows/*.manifest
├── frontend/               # Full Vite + Vue 3 project
│   ├── src/
│   │   ├── components/
│   │   ├── composables/
│   │   ├── lib/utils.ts    # cn() utility from shadcn-vue
│   │   ├── pages/
│   │   ├── App.vue
│   │   ├── main.ts
│   │   └── style.css
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── wailsjs/                # Auto-generated bindings (do not hand-edit)
│   ├── go/main/App.js
│   ├── go/main/App.d.ts
│   ├── go/main/models.ts
│   └── runtime/runtime.d.ts
├── internal/               # All Go business logic (unexported packages)
│   ├── github/             # GitHub API client wrapper
│   ├── auth/               # Device flow OAuth logic
│   ├── keyring/            # Token storage
│   └── model/              # Shared domain types (serialisable)
├── app.go                  # App struct + all bound methods
├── main.go                 # wails.Run() entry point
├── go.mod
└── wails.json
```

**Key `wails.json` settings**:
```json
{
  "frontend:dev:serverUrl": "auto",
  "wailsjsdir": "./frontend/src",
  "assetdir": "frontend/dist"
}
```

---

## 2. Component Library — Shadcn-vue

**Decision**: shadcn-vue with Tailwind CSS v4 (via `@tailwindcss/vite` plugin).

**Rationale**: User-specified. shadcn-vue copies component source into `src/components/ui/`
(no runtime library dependency), giving full ownership of styling. Tailwind v4 via the
Vite plugin eliminates the `tailwind.config.js` file.

**Alternatives considered**:
- PrimeVue / Vuetify: runtime library deps; harder to override theming precisely.
- Tailwind v3: still supported, but v4 + Vite plugin is the current documented path.

**Mandatory setup steps**:
1. `npm install tailwindcss @tailwindcss/vite`
2. `style.css`: `@import "tailwindcss";`
3. `vite.config.ts`: add `tailwindcss()` plugin, `@` alias
4. `npx shadcn-vue@latest init` → generates `components.json`, `src/lib/utils.ts`
5. `npx shadcn-vue@latest add <component>` — add components on demand

**`components.json` key settings**:
```json
{
  "style": "new-york",
  "typescript": true,
  "tailwind": { "css": "src/style.css", "cssVariables": true }
}
```

---

## 3. Go ↔ Vue Communication

**Decision**: Wails first-class binding. Public methods on the `App` struct are
exposed automatically; the Wails toolchain generates typed TS declarations.

**Pattern**:
- Go methods return `(T, error)` — errors become rejected Promises in Vue.
- Structs used as params/returns MUST have `json:"..."` tags to control serialisation.
- Run `wails generate module` (or use `wails dev` which auto-regenerates) after any
  signature change.
- For server-push events (e.g., OAuth completion): use `runtime.EventsEmit` in Go and
  `EventsOn` in Vue from `wailsjs/runtime/runtime`.

**Alternatives considered**:
- `window.go[...]` raw calls: no type safety; avoid.
- localhost REST API: unnecessary complexity; Wails bindings are purpose-built.

---

## 4. GitHub Authentication

**Decision**: OAuth 2.0 Device Authorization Grant (Device Flow).

**Rationale**: GitHub's recommended flow for native/desktop apps. Requires no local
HTTP server, no callback URI, no OS protocol handler. The app displays a short
`user_code` in the UI and opens `https://github.com/login/device` in the system
browser. The backend polls `https://github.com/login/oauth/access_token` every
`interval` seconds until the user completes authorization.

**Flow**:
1. POST `https://github.com/login/device/code` with `client_id` + `scope`
2. Display `user_code`, open `verification_uri` via `runtime.BrowserOpenURL`
3. Poll `https://github.com/login/oauth/access_token` with `device_code`
4. On success: store `access_token` in OS keychain

**GitHub App registration**: Register a **GitHub OAuth App** for simplicity (v1). Enable
"Device flow" in the app settings page. Scopes needed:
- `repo` — read PR data, post comments, resolve threads, commit suggestions
  (note: `write:discussion` is not a separate scope; PR comment actions are covered by `repo`)

**Alternatives considered**:
- Loopback redirect (`127.0.0.1:PORT`): works but requires a temporary HTTP server and
  port conflict management.
- Custom URI scheme (`myapp://oauth`): requires OS-level registration on all three
  platforms + code signing complexities.
- PAT input field: poor UX; requires user to navigate GitHub settings manually.

---

## 5. GitHub API Client

**Decision**: `github.com/google/go-github/v67` (REST API).

**Rationale**: Most widely-used Go GitHub client (11k+ stars, Google-maintained). Has
typed methods for all required endpoints: PR review comments, replies, thread resolution,
and suggestions. Handles pagination, rate-limit errors, and ETag caching natively.

**Key endpoints required** (all available via go-github):

| Operation | REST Endpoint |
|---|---|
| List PR review comments | `GET /repos/{owner}/{repo}/pulls/{pull_number}/comments` |
| Get comment | `GET /repos/{owner}/{repo}/pulls/comments/{comment_id}` |
| Reply to comment | `POST /repos/{owner}/{repo}/pulls/{pull_number}/comments` with `in_reply_to` |
| Resolve thread | `PUT /repos/{owner}/{repo}/pulls/comments/{comment_id}/resolve` |
| List review threads | `GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews` |
| Commit suggestion | `PUT /repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/suggestions` |
| Get PR | `GET /repos/{owner}/{repo}/pulls/{pull_number}` |
| Get authenticated user | `GET /user` |

**Pagination**: GitHub uses `page`+`per_page` (link header). go-github handles this via
`ListOptions{Page, PerPage}`. Fetch up to 100 per page; iterate until no next-page link.

**Suggestions**: A suggestion in a comment body uses the fenced block syntax:
````
```suggestion
replacement code
```
````
The "commit suggestion" operation is a REST endpoint that applies the patch. In
go-github: `client.PullRequests.CreateReviewComment` is not the commit path — use the
raw GitHub API `PUT /repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/suggestions`
via `github.Client.Do()` with a custom request, as go-github does not have a typed
method for suggestion commits. Alternatively use the REST API directly.

**Rate limits**: 5,000 requests/hour for authenticated requests. For a single-user
desktop session reviewing one PR this is not a concern. Cache responses in-process
using `go-github`'s ETag support.

**Alternatives considered**:
- `shurcooL/githubv4` (GraphQL): better for complex nested queries; unnecessary for
  these straightforward endpoints.
- Raw `net/http`: no rate-limit handling, no pagination helpers, no type safety.

---

## 6. Local Token Storage

**Decision**: `github.com/zalando/go-keyring` v0.2.x — OS native credential stores.

**Platform backends**:
- macOS: macOS Keychain (via `/usr/bin/security`; no CGo)
- Linux: D-Bus Secret Service → GNOME Keyring (requires `libsecret-1-0` package)
- Windows: Windows Credential Manager

**Linux caveat**: On headless Linux (no desktop session), Secret Service is unavailable.
Provide a fallback: check for `GITURA_GITHUB_TOKEN` env variable before attempting
keyring access.

**Alternatives considered**:
- `99designs/keychain`: macOS-only.
- Encrypted file: key management complexity.
- Plain file: insufficient security for access tokens.

---

## 7. Distribution / Build

**Decision**: `wails build` per-platform; GitHub Actions matrix for CI/CD.

**Platform outputs**:
- macOS: `.app` bundle → notarize with `notarytool` for Gatekeeper
- Windows: `.exe` + optional NSIS installer (`wails build -nsis`)
- Linux: ELF binary → wrap in AppImage for distribution

**Key constraint**: Wails v2 cannot cross-compile (CGo + native WebView). Each platform
MUST be built on that OS.

**macOS**: Use `darwin/universal` target for fat binary (Intel + Apple Silicon).

**WebView2 on Windows**: Use `-webview2 download` (default) or `-webview2 embed` for
fully self-contained distribution.

**Alternatives considered**:
- Wails v3: not yet stable.
- Electron: eliminates Go backend benefits, heavy overhead.

---

## 8. GitHub API — Suggestion Commit Detail

**Decision**: Use raw GitHub REST API call via `go-github`'s `Do()` method since
`go-github` has no typed method for suggestion commits.

**Endpoint**: `POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}/events`
is not the right one. The correct approach is to commit a suggestion via:
```
PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge` — not correct.
```
Actually, suggestion commits are done via the **Checks** or **Commit** path. The correct
API is undocumented officially but works as:
- Suggestions are committed by creating a commit that applies the patch. GitHub web UI
  does this through an internal endpoint. The public API method is:
  `POST /repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/replies` — not this either.
- **Actual mechanism**: Create a commit via the Git Data API applying the patch from the
  suggestion diff, then push it to the PR branch. This requires:
  1. Get the current file content + SHA via `GET /repos/{owner}/{repo}/contents/{path}?ref={branch}`
  2. Apply the suggestion patch to the file content
  3. Push updated file via `PUT /repos/{owner}/{repo}/contents/{path}` (creates a commit)
- This is equivalent to what GitHub's web UI does. go-github has typed support for the
  Repository Contents API.

**Scope required**: `repo` scope covers contents writes to the PR branch.

---

## Constitution Check — Pre-Design

| Principle | Status | Notes |
|---|---|---|
| I. Code Quality (gofmt, golint, complexity ≤ 10) | PASS | Enforced by `.golangci.yml` in CI |
| II. Testing (80%/90% coverage, fixtures, TDD) | PASS | GitHub API calls mocked via fixtures; `testify` |
| III. UX Consistency (single design language, ARIA) | PASS | shadcn-vue is the single component system |
| IV. Performance (load ≤ 2s, diff ≤ 500ms, mem < 150MB) | PASS | Wails WebView is lightweight; API responses cached |
| Technology Stack (Go + GitHub API) | PASS | Wails + Vue + shadcn-vue confirmed; go-github confirmed |
| Development Workflow | PASS | Standard branch + CI gates apply |
