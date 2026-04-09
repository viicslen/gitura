# gitura Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-31

## Active Technologies
- Go 1.25; TypeScript 5.x (strict mode) + Wails v2.11, go-github/v67 (Search.Issues), golang.org/x/oauth2, go-keyring; Vue 3, VueUse ^14.2.1, shadcn-vue (reka-ui + radix-vue), lucide-vue-next (003-pr-list-filters)
- N/A — all data fetched live from GitHub REST API; filter state in-memory only (003-pr-list-filters)
- Go 1.25; TypeScript 5.x (strict mode) + Wails v2.11, go-github/v67 (PR metadata + REST write ops), raw HTTP (004-pr-review-workflow)
- `os.UserConfigDir()/gitura/ignored_commenters.json` (ignored commenters only); (004-pr-review-workflow)

- **Language**: Go 1.22+
- **UI Framework**: Wails v2 (WebView-based desktop app)
- **Frontend**: Vue 3 + TypeScript (Vite)
- **Component Library**: shadcn-vue + Tailwind CSS v4
- **GitHub API Client**: `github.com/google/go-github/v67`
- **Auth**: GitHub OAuth 2.0 Device Flow
- **Token Storage**: `github.com/zalando/go-keyring`
- **Testing**: `go test` + `testify`; `httptest` for HTTP fixture recording
- **Linting**: `golangci-lint` with `.golangci.yml`

## Project Structure

```text
.
├── build/                  # Wails build output + platform manifests
│   ├── appicon.png
│   ├── darwin/Info.plist
│   └── windows/*.manifest
├── frontend/               # Vue 3 + Vite frontend
│   ├── src/
│   │   ├── components/     # App-specific components
│   │   ├── components/ui/  # shadcn-vue components (owned source)
│   │   ├── composables/    # Vue composables
│   │   ├── lib/utils.ts    # cn() utility
│   │   ├── pages/          # Route-level views
│   │   └── style.css       # Tailwind v4 import
│   ├── vite.config.ts
│   └── package.json
├── wailsjs/                # Auto-generated Wails bindings (do not edit)
│   ├── go/main/App.d.ts
│   ├── go/main/models.ts
│   └── runtime/runtime.d.ts
├── internal/               # Go business logic (unexported)
│   ├── github/             # GitHub API client wrapper
│   ├── auth/               # OAuth device flow
│   ├── keyring/            # Token persistence
│   └── model/              # Shared domain types
├── app.go                  # App struct + Wails-bound methods
├── main.go                 # wails.Run() entry point
├── go.mod
├── wails.json
└── .golangci.yml
tests/
├── integration/
└── unit/
```

## Commands

```bash
# Development (hot-reload)
wails dev

# Frontend package scripts (run from ./frontend)
bun install
bun run dev
bun run build

# Run Go tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Lint
golangci-lint run

# Build (current platform)
wails build

# Regenerate Wails JS bindings
wails generate module
```

## Package Manager

- Frontend commands MUST use **Bun** (`bun run ...`) for this repo.
- Reason: the frontend workspace is Bun-managed (`frontend/bun.lock` present).
- Do not use `npm run ...` in normal workflows.

## Code Style

Go: Follow idiomatic Go conventions enforced by `gofmt`, `golint`, `go vet`.
- Every exported symbol MUST have a doc comment
- Cyclomatic complexity per function MUST NOT exceed 10
- Test names MUST follow `TestFunctionName_Scenario_ExpectedOutcome`

Frontend: TypeScript strict mode. shadcn-vue components are the single source of
UI primitives — no ad-hoc inline styles.

## Recent Changes
- 004-pr-review-workflow: Added Go 1.25; TypeScript 5.x (strict mode) + Wails v2.11, go-github/v67 (PR metadata + REST write ops), raw HTTP
- 003-pr-list-filters: Added Go 1.25; TypeScript 5.x (strict mode) + Wails v2.11, go-github/v67 (Search.Issues), golang.org/x/oauth2, go-keyring; Vue 3, VueUse ^14.2.1, shadcn-vue (reka-ui + radix-vue), lucide-vue-next

- 001-pr-review-ui: Added Wails+Vue+shadcn-vue stack, go-github client, device flow auth

<!-- MANUAL ADDITIONS START -->
## App State Storage

There are three distinct categories of app state, each with a designated storage mechanism:

### 1. Credentials
Sensitive secrets (e.g. OAuth tokens, API keys) MUST be stored in the **system keyring** via `go-keyring`. Never write credentials to disk in plaintext.

### 2. User-Editable Configuration
User preferences and settings MUST be stored in a **TOML config file** at `ConfigDir()/settings.toml` (see `internal/settings/settings.go`). The config directory is resolved via `os.UserConfigDir()` (Go stdlib):
  - Linux: `$XDG_CONFIG_HOME/gitura/` (default `~/.config/gitura/`)
  - macOS: `~/Library/Application Support/gitura/`
  - Windows: `%AppData%\gitura\`

This file is human-readable and may be edited directly by the user.

### 3. Non-User-Editable App State
App-managed state not intended for direct user editing (e.g. caches, internal flags, derived data) MUST be stored in a **SQLite database** using **sqlc** for type-safe query generation. Store the database file under `os.UserCacheDir()/gitura/` (note: `os.UserStateDir` is not available in this Go build), which resolves to:
  - Linux: `$XDG_CACHE_HOME/gitura/` (default `~/.cache/gitura/`)
  - macOS: `~/Library/Caches/gitura/`
  - Windows: `%LocalAppData%\gitura\`

## Handling Blockers

When a blocker is encountered during implementation, **do not use workarounds**. Stop and ask the user how to proceed.

Workarounds are prohibited, including but not limited to:

- Using `any` type in TypeScript instead of defining proper types
- Adding `// @ts-ignore`, `// @ts-expect-error`, or `eslint-disable` comments to suppress errors
- Using non-null assertions (`!`) to silence TypeScript nullability errors
- Casting through `as unknown as T` to bypass type safety
- Using `interface{}` or `any` in Go instead of concrete types
- Ignoring errors with `_` in Go (e.g., `result, _ := ...`)
- Trying to work around a missing or incompatible dependency instead of adding/upgrading it
- Hardcoding values that should be retrieved from config, API, or environment
- Duplicating logic to avoid a refactor or a missing abstraction

If you hit a blocker, state clearly what the blocker is and ask the user for a decision before proceeding.

## Documentation Maintenance

After every non-fix code change (new features, refactors, structural changes):

- **README.md**: Evaluate if user-facing content needs updating (features, usage, setup, screenshots).
- **AGENTS.md**: Evaluate if developer-facing content needs updating (project structure, technologies, commands, code style).

Do not update docs for bug fixes unless the fix changes behavior that was previously documented.
<!-- MANUAL ADDITIONS END -->
