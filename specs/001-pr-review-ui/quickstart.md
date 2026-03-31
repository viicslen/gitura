# Quickstart: PR Review UI Development

**Feature**: 001-pr-review-ui
**Date**: 2026-03-30

---

## Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.22+ | https://go.dev/dl/ |
| Node.js | 20+ (LTS) | https://nodejs.org |
| Wails CLI | v2.x | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| npm | 10+ | bundled with Node.js |

**Platform dependencies**:
- **Linux**: `sudo apt install libwebkit2gtk-4.0-dev libsecret-1-dev build-essential`
  (Debian/Ubuntu). Use `-tags webkit2_41` flag if on Ubuntu 24.04+.
- **macOS**: Xcode Command Line Tools (`xcode-select --install`)
- **Windows**: WebView2 runtime (auto-downloaded on first run)

---

## 1. Clone and Initialize

```bash
git clone <repo-url> gitura
cd gitura
git checkout 001-pr-review-ui
```

---

## 2. Install Dependencies

```bash
# Go dependencies
go mod download

# Frontend dependencies
cd frontend && npm install && cd ..
```

---

## 3. Register a GitHub OAuth App

1. Go to https://github.com/settings/developers → "OAuth Apps" → "New OAuth App"
2. Set:
   - **Application name**: Gitura (dev)
   - **Homepage URL**: http://localhost
   - **Authorization callback URL**: http://localhost (device flow does not use this)
3. Enable "Device Flow" in the app settings after creation
4. Copy the **Client ID** (no client secret needed for device flow)
5. Set the environment variable:
   ```bash
   export GITHUB_CLIENT_ID=your_client_id_here
   ```

---

## 4. Run in Development Mode

```bash
wails dev
```

This starts:
- The Vite dev server (hot-reload for Vue changes)
- The Wails app window (auto-reloads Go backend on file change)
- Regenerates `wailsjs/` bindings when Go method signatures change

The app window opens automatically. Go backend logs appear in the terminal.

---

## 5. Validate Core Flows

### Authentication
1. Click "Sign in with GitHub" in the app
2. A short code (e.g., `WDJB-MJHT`) is displayed and the browser opens to `github.com/login/device`
3. Enter the code on GitHub
4. Return to the app — it should show your GitHub username and avatar

### Load a PR
1. Enter a GitHub PR URL or `owner/repo#number` in the input field
2. The comment list renders within 3 seconds
3. CI bot comments (if any ignored commenters are set) should be absent

### Reply to a comment
1. Click any comment in the list to open it
2. Type a reply and submit
3. Open the PR on GitHub.com — the reply should be visible

### Resolve a thread
1. Click "Resolve" on a comment thread
2. Thread should grey out / move to resolved section
3. Refresh on GitHub.com — thread shows as resolved

### Commit a suggestion
1. Open a PR comment containing a ` ```suggestion ` block
2. Click "Commit suggestion"
3. Confirm — a commit SHA should appear
4. Check the PR branch on GitHub — the commit is present

### Ignored commenters
1. Open Settings
2. Add a GitHub username (e.g., a CI bot's login)
3. Return to a PR where that user commented — their comments are hidden

---

## 6. Run Tests

```bash
# All tests
go test ./...

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Frontend tests (if configured)
cd frontend && npm test
```

Coverage targets per constitution:
- Overall: ≥ 80%
- `internal/github/` package: ≥ 90%

---

## 7. Build for Production

```bash
# Current platform
wails build

# macOS universal binary (run on macOS only)
wails build -platform darwin/universal

# Windows (run on Windows only)
wails build -platform windows/amd64

# Linux (run on Linux only)
wails build -platform linux/amd64
```

Output binary is placed in `build/bin/`.

---

## 8. Lint

```bash
golangci-lint run
```

Lint config is at `.golangci.yml` in the repo root. All lint errors block merge.

---

## Troubleshooting

| Problem | Solution |
|---|---|
| `wails dev` fails with WebKit error on Linux | Install `libwebkit2gtk-4.0-dev`; or use `-tags webkit2_41` on Ubuntu 24.04+ |
| Token not saving on Linux | Ensure GNOME Keyring or KWallet is running; set `GITURA_GITHUB_TOKEN` env var as fallback |
| `wailsjs/` bindings out of date | Run `wails generate module` or restart `wails dev` |
| GitHub API 401 errors | Run `Logout()` from the app and re-authenticate |
| Device flow "expired" | Start a new device flow — codes expire after ~15 minutes |
