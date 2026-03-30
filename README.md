# gitura

A desktop application for reviewing GitHub pull requests without leaving your terminal workflow. Authenticate once via GitHub OAuth, load any PR, and triage review comments — reply, resolve, or commit suggestions — from a native desktop UI.

## Features

- **Summary list view** — all review comments with author, file path, and excerpt
- **One-by-one navigation** — step through comments with full diff hunk context and thread replies
- **Reply** — post replies to comment threads directly to GitHub
- **Resolve / unresolve** — mark threads resolved on GitHub with optimistic UI updates
- **Commit suggestions** — accept and commit GitHub suggestion blocks to the PR branch
- **Ignored commenters** — filter out CI bot noise by username; persists across sessions

## Requirements

### Runtime

| Platform | Dependencies |
|---|---|
| macOS | None beyond the app bundle |
| Linux | `libwebkit2gtk-4.0`, `libsecret-1` |
| Windows | None beyond the app bundle |

### Environment

```
GITURA_GITHUB_CLIENT_ID=<your GitHub OAuth App client ID>
```

This variable must be set before launching the app. The app will exit with a descriptive error if it is missing.

### GitHub OAuth App

Create an OAuth App at **GitHub → Settings → Developer settings → OAuth Apps** with:
- **Authorization callback URL**: `http://localhost` (Device Flow does not use a callback, but GitHub requires a value)
- Copy the **Client ID** into `GITURA_GITHUB_CLIENT_ID`

No client secret is needed — the app uses the [Device Authorization Grant](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow) (no browser redirect required).

The OAuth scope requested is **`repo`**, required for GraphQL mutations that resolve and unresolve review threads on private repositories.

## Development

### Prerequisites

- Go 1.22+
- Node.js 20+
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- On Linux: `libwebkit2gtk-4.0-dev`, `libsecret-1-dev`

### Run in dev mode (hot reload)

```bash
export GITURA_GITHUB_CLIENT_ID=<your client ID>
wails dev
```

This starts a Vite dev server for the frontend with hot reload. A browser-accessible dev server is also available at `http://localhost:34115`.

### Run Go tests

```bash
CGO_ENABLED=0 go test ./...
```

### Lint

```bash
golangci-lint run
```

### Build (current platform)

```bash
wails build
```

The binary is placed in `build/bin/`.

> **Note**: Wails v2 does not support cross-compilation. Build each platform natively.

## Project Structure

```
.
├── app.go                  # Wails-bound App methods (auth, PR, comments, settings)
├── main.go                 # wails.Run() entry point
├── internal/
│   ├── auth/               # GitHub OAuth 2.0 Device Flow
│   ├── github/             # GitHub REST + GraphQL API client
│   ├── keyring/            # OS keychain token storage (go-keyring)
│   ├── model/              # Shared DTO types (Go ↔ Vue boundary)
│   └── settings/           # Ignored-commenter list persistence
├── frontend/
│   └── src/
│       ├── components/     # App-specific Vue components
│       ├── components/ui/  # shadcn-vue primitives
│       ├── composables/    # useAuth, usePR, useSettings
│       └── pages/          # AuthPage, PRPage, SettingsPage
├── specs/                  # Feature specifications and contracts
└── tests/
    └── fixtures/           # Recorded GitHub API responses for offline tests
```

## Authentication

Sign-in uses GitHub Device Flow:

1. Click **Sign in with GitHub** — a user code is displayed in the app
2. Visit the verification URL (opened automatically in your browser)
3. Enter the code and authorize the app
4. The app polls GitHub and stores the token in your OS keychain on success

The token is stored in the OS native keychain and never written to disk or logged.

## License

MIT
