<div align="center">

<img src="frontend/src/assets/images/logo.svg" alt="" width="96" height="96">

# gitura

**Review GitHub pull requests from a native desktop app.**

Load a PR, walk its review comments one by one, reply, resolve, commit suggestions,
and hand a comment straight to your coding agent — without a browser tab in sight.

[![CI](https://github.com/viicslen/gitura/actions/workflows/ci.yml/badge.svg)](https://github.com/viicslen/gitura/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/viicslen/gitura)](https://github.com/viicslen/gitura/releases/latest)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](#license)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Wails](https://img.shields.io/badge/Wails-v3-DF0000)](https://v3.wails.io)

<img src="https://github.com/viicslen/gitura/raw/assets/pr-screenshots/readme-review-dark.png" alt="gitura review view" width="900">

</div>

## Screenshots

| Pull requests | Files changed |
| ------------- | ------------- |
| ![Pull request list](https://github.com/viicslen/gitura/raw/assets/pr-screenshots/readme-pull-requests.png) | ![Files changed](https://github.com/viicslen/gitura/raw/assets/pr-screenshots/readme-files-changed.png) |
| Filter by involvement, repository, organisation, author, or date | Split diff with syntax highlighting and inline comments |

| Settings | Light theme |
| -------- | ----------- |
| ![Settings](https://github.com/viicslen/gitura/raw/assets/pr-screenshots/readme-settings.png) | ![Light theme](https://github.com/viicslen/gitura/raw/assets/pr-screenshots/readme-review-light.png) |
| Fonts and text size, ignored commenters, CLI commands to run on a comment, and keybindings | Dark, light, and system themes |

## Features

- **Pull request list** — everything you author, are assigned, or are asked to review, filtered by repository, organisation, author, or last-updated date
- **Side rail and title strip** — a frameless window with a slim rail for the PR list, Overview, Conversation and Files views (badged with open reviews, unresolved threads and unviewed files), and a title strip with the PR's state, Refresh and Open on GitHub
- **Overview** — description, labels, general comments and reviews; reviews with open conversations come first
- **Conversation view** — step through review comments one at a time with diff hunk context and thread replies
- **Files changed view** — JetBrains-style side-by-side diff with curved connectors mapping each changed block; added and deleted files open as a single full-width panel, and moved or renamed files say so
- **Viewed files** — tick files as viewed (synced with GitHub), jump to the next one, and hide viewed or test files from the list
- **Diff view options** — show or hide inline comments and toggle line wrapping; Vue and Svelte files are highlighted per template, script and style block
- **GitHub-flavoured comments** — alerts, tables, `<picture>` images, and task lists you can tick straight from the app
- **Review in the diff** — click or drag line numbers (Shift+click extends) to comment on either side; threads, pending drafts, replies and resolve/unresolve sit inline under their lines
- **Batch reviews** — draft comments, then submit them together as comment, approval, or changes requested (approve and request changes are off on your own PRs)
- **Reply, resolve, unresolve** — posted straight to GitHub with optimistic UI updates
- **Commit suggestions** — accept a GitHub suggestion block and commit it to the PR branch
- **Run commands** — pipe a comment into any CLI (e.g. an `opencode` agent) from the comment itself and watch its output stream live in the run panel
- **Ignored commenters** — filter out CI bot noise by username
- **Configurable shortcuts** — rebind the keys that move between files and threads in Settings → Keyboard
- **Open a PR from the command line** — `gitura 123` or `gitura my-branch` opens that PR straight away; the repository comes from the current directory's `origin` remote, or pass `--repo owner/repo`. A running window is reused unless you pass `--new-window`. `--focus` opens it in its own window with only that review: no PR list, nothing to navigate away to, and the main window is left alone. `gitura --local [path]` reviews the local changes of a checkout instead (default: the current directory)
- **Dark, light, and system themes**
- **Readable type** — Inter and JetBrains Mono by default; switch to Atkinson Hyperlegible or system fonts and scale the text size in Settings

## Install

### Download a release

Prebuilt binaries for Linux, macOS, and Windows are attached to every
[release](https://github.com/viicslen/gitura/releases/latest), with SHA-256 checksums.

### Nix

```sh
nix run github:viicslen/gitura
```

### Build from source

See [Development](#development).

## Requirements

### Runtime

| Platform | Dependencies                        |
| -------- | ----------------------------------- |
| macOS    | None beyond the app bundle          |
| Linux    | `libgtk-4-1`, `libwebkitgtk-6.0-4`, `libsecret-1` |
| Windows  | None beyond the app bundle          |

### Environment

```sh
GITURA_GITHUB_CLIENT_ID=<your GitHub OAuth App client ID>
```

This variable is read at **build/dev time** and injected into the binary via `-ldflags`.
Copy `.env.example` to `.env`, fill in the value, and use the `just dev` / `just build`
recipes below.

### GitHub OAuth App

Create an OAuth App at **GitHub → Settings → Developer settings → OAuth Apps** with:

- **Authorization callback URL**: `http://localhost` (Device Flow does not use a callback, but GitHub requires a value)
- Copy the **Client ID** into `GITURA_GITHUB_CLIENT_ID`

No client secret is needed — the app uses the
[Device Authorization Grant](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow).

The requested scope is **`repo`**, required for the GraphQL mutations that resolve and
unresolve review threads on private repositories.

## Authentication

Sign-in uses GitHub Device Flow:

1. Click **Sign in with GitHub** — a user code is displayed in the app
2. Visit the verification URL (opened automatically in your browser)
3. Enter the code and authorize the app
4. The app polls GitHub and stores the token in your OS keychain on success

The token lives in the OS native keychain — never on disk, never in logs.

## Development

### Prerequisites

- Go 1.27+
- [Bun](https://bun.sh) (the frontend workspace is Bun-managed)
- The [Wails v3](https://v3.wails.io) CLI, pinned as a Go tool in `go.mod`: the `just` recipes and Taskfiles call `wails3`, so outside the Nix shell run `go install tool` once to put that version on your `PATH`
- On Linux: `libgtk-4-dev`, `libwebkitgtk-6.0-dev`, `libsecret-1-dev`

A Nix flake is provided — `nix develop` (or `direnv allow`) gives you all of the above, plus a `wails3` wrapper for the pinned CLI.

### Common tasks

```sh
just dev       # hot-reload dev server (injects GITHUB_CLIENT_ID from .env)
just build     # build for the current platform → bin/
just bindings  # regenerate frontend/src/bindings/ after changing a bound method or its types
```

Builds run through the Taskfiles (`Taskfile.yml`, `build/`), which skip steps whose inputs haven't changed.

```sh
# CI-equivalent checks
CGO_ENABLED=1 go test -race -count=1 -coverprofile=coverage.out ./internal/...
CGO_ENABLED=1 GOFLAGS='-tags=dev' golangci-lint run
cd frontend && bun run typecheck && bun run test && bun run build
```

> **Note**: build each platform natively; the app needs cgo (SQLite, and GTK/WebKit on Linux).

## Project structure

```text
.
├── main.go                 # Wails application entry point: window options, app icon
├── Taskfile.yml            # Wails build/dev/package tasks (includes build/**/Taskfile.yml)
├── assets.go, assets_dev.go # Embedded frontend/dist (production) or none (dev, build tag)
├── internal/
│   ├── app/                # App, bound as a Wails service; one file per domain (auth, pr, threads, pending_review, …)
│   ├── auth/               # GitHub OAuth 2.0 Device Flow
│   ├── db/                 # SQLite app state (sqlc.yaml + generated queries)
│   ├── fonts/              # System font listing
│   ├── ghapi/              # GitHub REST + GraphQL API client
│   ├── gitlocal/           # Local git working-tree diffs
│   ├── keyring/            # OS keychain token storage (go-keyring)
│   ├── logger/             # Structured slog logger (GITURA_LOG_LEVEL)
│   ├── model/              # Shared DTO types (Go ↔ Vue boundary)
│   ├── runner/             # CLI command execution for review comments
│   ├── settings/           # User preferences (settings.toml)
│   └── unidiff/            # Unified-diff hunk parser
├── frontend/src/
│   ├── components/         # comment/, diff/, markdown/, pr/, run/, shell/, ui/ (shadcn-vue)
│   ├── composables/        # useAuth, useCommentThreads, useDiffReview, usePRFilters, …
│   ├── lib/                # Framework-free helpers (markdown, highlight, fileTree, …)
│   ├── pages/              # AuthPage, PRListPage, PRReviewPage, LocalReviewPage, SettingsPage
│   ├── types/              # Shared frontend types
│   └── bindings/           # Generated Wails bindings (`just bindings`; do not edit)
└── build/                  # config.yml, platform Taskfiles, icons and packaging assets used by `wails3`
```

Go test fixtures live next to their package in `testdata/`.

## License

Copyright (C) 2026 Victor R. Licensed under [AGPL-3.0](LICENSE).
