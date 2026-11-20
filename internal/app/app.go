// Package app holds the Wails-bound App: every exported App method is a frontend binding.
package app

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

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
	// commits is filled lazily by GetPRTimeline; cleared on LoadPR.
	commits []model.CommitDTO

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

	// configMu orders setters' read-change-save on settings.toml.
	configMu sync.Mutex
	// cfg is the cached settings.toml, nil until loadConfig. A snapshot is swapped whole and never
	// changed, so readers need no lock; read it once through conf() per use.
	cfg atomic.Pointer[settings.Config]

	// repoIndex caches gitlocal.FindRepos over projectDirs; repoIndexAt is when it was built.
	repoIndexMu sync.Mutex
	repoIndex   map[string]string
	repoIndexAt time.Time

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
	// focus hides navigation away from the launched review; set once by ConfigureLaunch.
	focus bool

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
	if a.cfg.Load() != nil {
		return nil
	}
	cfg, err := settings.Load()
	if err != nil {
		a.cfg.Store(&settings.Config{IgnoredCommenters: []model.IgnoredCommenterDTO{}, Commands: []model.CommandDTO{}})
		return err
	}
	a.applyConfig(cfg)
	logger.L.Debug("config loaded",
		"ignored_commenters", len(cfg.IgnoredCommenters),
		"commands", len(cfg.Commands),
	)
	return nil
}

// conf returns the cached settings snapshot, empty before loadConfig. Callers must not change it.
func (a *App) conf() *settings.Config {
	if c := a.cfg.Load(); c != nil {
		return c
	}
	return &settings.Config{}
}

// applyConfig caches cfg, dropping the repo index when the project folders changed.
func (a *App) applyConfig(cfg settings.Config) {
	old := a.cfg.Swap(&cfg)
	if old != nil && !slices.Equal(old.ProjectDirs, cfg.ProjectDirs) {
		a.repoIndexMu.Lock()
		a.repoIndex, a.repoIndexAt = nil, time.Time{}
		a.repoIndexMu.Unlock()
	}
}

// updateConfig applies fn to the settings on disk, not the in-memory copy, so a change another
// window saved meanwhile is kept, then refreshes the in-memory copy from the result.
// Errors from fn come back as is; others are prefixed "settings:".
func (a *App) updateConfig(fn func(*settings.Config) error) (settings.Config, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg, err := settings.Update(fn)
	if err != nil {
		return settings.Config{}, err
	}
	a.applyConfig(cfg)
	return cfg, nil
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
