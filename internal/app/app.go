// Package app holds the Wails-bound App: every exported App method is a frontend binding.
package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/google/go-github/v93/github"
	"github.com/wailsapp/wails/v3/pkg/application"

	"gitura/internal/db"
	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/settings"
)

// MainWindow names the app's only window.
const MainWindow = "main"

func init() {
	application.RegisterEvent[model.RunResult]("command:run:pending")
	application.RegisterEvent[model.RunResult]("command:run:complete")
	application.RegisterEvent[model.RunOutput]("command:run:output")
	application.RegisterEvent[model.LoadProgress]("pr:load-progress")
	application.RegisterEvent[application.Void]("launch")
	application.RegisterEvent[application.Void]("auth:device-flow-complete")
	application.RegisterEvent[application.Void]("auth:device-flow-expired")
	application.RegisterEvent[application.Void]("local:changed")
}

// App is the main application struct exposed to Wails.
type App struct {
	ctx context.Context

	// clientID is the GitHub OAuth app client ID used for the device flow.
	clientID string

	// Authenticated GitHub client; nil until auth is complete.
	ghClient *github.Client

	// Cached PR data populated by LoadPR.
	prOwner       string
	prRepo        string
	prNumber      int
	prCache       *model.PullRequestSummary
	threads       []model.CommentThreadDTO
	issueComments []model.IssueCommentDTO
	reviews       []model.ReviewDTO

	// prFilesCache holds the raw CommitFile list from the GitHub ListFiles API.
	// Populated lazily by GetPRFiles; cleared on LoadPR.
	prFilesCache []*github.CommitFile
	// viewedFiles holds the paths the user marked as viewed on GitHub; filled by GetViewedFiles.
	viewedFiles map[string]bool

	// Pending review state. pendingReviewID is 0 when no pending review exists.
	pendingReviewID int64
	pendingComments []model.DraftCommentDTO
	// pendingHeadSHA is the commit the drafts' line numbers refer to; the PR head can move before submit.
	pendingHeadSHA string

	// ignoredCommenters is the persisted list of commenters to filter from review threads.
	// Lazily loaded on first use via loadConfig.
	ignoredCommenters []model.IgnoredCommenterDTO

	// commands is the persisted list of named CLI commands.
	// Lazily loaded alongside ignoredCommenters via loadConfig.
	commands []model.CommandDTO

	// defaultCommandName is the command name to use as the primary action.
	// Empty string means no default is set.
	defaultCommandName string

	appearance model.AppearanceDTO
	// keybindings maps shortcut action ids to keys; see settings.Config.Keybindings.
	keybindings map[string]string

	// runCancels maps run IDs to their context cancel functions, allowing
	// in-flight commands to be stopped via CancelRun.
	runCancelsMu sync.Mutex
	runCancels   map[string]context.CancelFunc

	// queries is the sqlc-generated DB query interface for app-managed state.
	// Opened during startup; nil if the DB could not be initialised.
	queries *db.Queries

	// local is set while reviewing a local checkout instead of a PR; see StartLocalReview.
	local *localReview

	// launch is the PR or local checkout to open once signed in; a second launch sets it from another goroutine.
	launch atomic.Pointer[launchTarget]

	// Device-flow state, held in memory only during the OAuth flow.
	deviceCode string
	authToken  string
}

// New creates an App that authenticates with the given GitHub OAuth app client ID.
func New(clientID string) *App {
	return &App{
		clientID:   clientID,
		runCancels: make(map[string]context.CancelFunc),
	}
}

// Configure wires a's asset proxy into opts; it is a function, not a method, so Wails doesn't bind it.
func Configure(opts *application.Options, a *App) {
	opts.Assets.Middleware = assetProxyMiddleware(a)
}

// ServiceStartup is called by Wails when the application starts; Wails does not bind it.
// The context is cancelled on shutdown.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	logger.L.Info("app started", "client_id", a.clientID)

	// Open the SQLite state database.
	// os.UserStateDir is not available in this build; use UserCacheDir instead.
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		logger.L.Warn("could not resolve cache dir; per-PR state will be unavailable", "err", err)
	} else {
		sqlDB, err := db.Open(cacheDir)
		if err != nil {
			logger.L.Warn("could not open state DB; per-PR state will be unavailable", "err", err)
		} else {
			a.queries = db.New(sqlDB)
		}
	}

	// Pre-warm config so the first LoadPR call is fast.
	if err := a.loadConfig(); err != nil {
		logger.L.Warn("failed to pre-load config", "err", err)
	}
	return nil
}

// loadConfig loads the full settings config from disk if not yet loaded.
// It is safe to call multiple times; subsequent calls are no-ops.
func (a *App) loadConfig() error {
	if a.ignoredCommenters != nil {
		return nil
	}
	cfg, err := settings.Load()
	if err != nil {
		a.ignoredCommenters = []model.IgnoredCommenterDTO{}
		a.commands = []model.CommandDTO{}
		return err
	}
	a.ignoredCommenters = cfg.IgnoredCommenters
	a.commands = cfg.Commands
	a.defaultCommandName = cfg.DefaultCommandName
	a.appearance = cfg.Appearance
	a.keybindings = cfg.Keybindings
	logger.L.Debug("config loaded",
		"ignored_commenters", len(a.ignoredCommenters),
		"commands", len(a.commands),
	)
	return nil
}

// initGHClient sets up the authenticated GitHub client from a token.
func (a *App) initGHClient(token string) error {
	client, err := ghapi.NewClient(token)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	a.authToken = token
	a.ghClient = client
	logger.L.Debug("github client initialised")
	return nil
}

// emit sends a named Wails event to the frontend; data must match the type registered in init.
func (a *App) emit(name string, data ...any) {
	logger.L.Debug("emitting event", "event", name)
	if wails := application.Get(); wails != nil {
		wails.Event.Emit(name, data...)
	}
}

// mainWindow returns the app window; ok is false before the Wails app exists (e.g. in tests).
func mainWindow() (application.Window, bool) {
	wails := application.Get()
	if wails == nil {
		return nil, false
	}
	return wails.Window.GetByName(MainWindow)
}
