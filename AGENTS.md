# gitura Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-30

## Active Technologies

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

## Code Style

Go: Follow idiomatic Go conventions enforced by `gofmt`, `golint`, `go vet`.
- Every exported symbol MUST have a doc comment
- Cyclomatic complexity per function MUST NOT exceed 10
- Test names MUST follow `TestFunctionName_Scenario_ExpectedOutcome`

Frontend: TypeScript strict mode. shadcn-vue components are the single source of
UI primitives — no ad-hoc inline styles.

## Recent Changes

- 001-pr-review-ui: Added Wails+Vue+shadcn-vue stack, go-github client, device flow auth

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
