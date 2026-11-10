# gitura Development Guidelines

## Technologies

- **Language**: Go 1.26+
- **UI Framework**: Wails v2 (WebView-based desktop app)
- **Frontend**: Vue 3 + TypeScript (Vite)
- **Component Library**: shadcn-vue + Tailwind CSS v4
- **GitHub API Client**: `github.com/google/go-github/v67`
- **Auth**: GitHub OAuth 2.0 Device Flow
- **Token Storage**: `github.com/zalando/go-keyring`
- **Testing**: `go test` + `testify`; `httptest` for HTTP fixture recording
- **Linting**: `golangci-lint` with `.golangci.yml`

## Project Structure

See the tree in README.md. Root holds only what tooling requires there: `main.go` and `assets*.go` (`//go:embed` cannot reach parent directories, so they must sit beside `frontend/dist` and `build/`), `wails.json`, `go.mod`, release-please, Nix, lint and just configs.

- Wails-bound methods live on `App` in `internal/app`; only binding methods may be exported on `App`, since Wails exposes every exported method to the frontend.
- Go test fixtures go in the package's `testdata/`.
- Frontend components are grouped by feature folder under `frontend/src/components/`.

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

# Regenerate sqlc queries after editing internal/db/query.sql or schema.sql
sqlc generate -f internal/db/sqlc.yaml

# Regenerate build/appicon.png and build/windows/icon.ico from frontend/src/assets/images/logo.svg
just icons
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

## Post-Change Validation

After any code change, run the same checks used by CI before handing off work.

- **Go tests (CI-equivalent)**: `CGO_ENABLED=1 GOFLAGS='-tags=webkit2_41' go test -race -count=1 -coverprofile=coverage.out ./internal/...`
- **Go lint (CI-equivalent)**: `CGO_ENABLED=1 GOFLAGS='-tags=dev,webkit2_41' golangci-lint run`
- **Frontend check (CI-equivalent)**: from `frontend/`, run `bun run build`

If `golangci-lint` is not installed locally, run:

- `CGO_ENABLED=1 GOFLAGS='-tags=dev,webkit2_41' go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run`

If you are in a Nix-enabled environment and validation/build commands fail due to missing system dependencies (e.g., WebKit/GTK/libsecret), run the commands inside the repo `shell.nix` environment:

- `nix-shell --run "<your command>"`
<!-- MANUAL ADDITIONS END -->
