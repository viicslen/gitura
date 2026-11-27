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

	// prMu guards the loaded PR's identity above, its file state below from prFilesCache to subViewed and
	// the files it holds, and the pending review state; Wails runs bound calls concurrently, so each one
	// using that state holds prMu throughout.
	prMu sync.Mutex
	// loadGen numbers LoadPR calls, so a load that a later one overtook leaves the loaded PR alone.
	loadGen atomic.Int64
	// prFilesCache holds the raw CommitFile list from the GitHub ListFiles API.
	// Populated lazily by GetPRFiles; cleared on LoadPR.
	prFilesCache []*github.CommitFile
	// viewedFiles holds the files the user marked as viewed on GitHub; filled by GetViewedFiles.
	viewedFiles map[fileID]bool
	// prSubmodules holds the PR's submodule bumps by path, and prSubmoduleOf the bump each file
	// inside one belongs to; both are rebuilt with prFilesCache.
	prSubmodules  map[string]*prSubmodule
	prSubmoduleOf map[*github.CommitFile]*prSubmodule
	// prFilesTruncated is set when the parent PR's file list hit prFileListCap, so bumps may be missing from it.
	prFilesTruncated bool
	// subViewed caches the loaded PR's viewed marks kept in the database (submodule files no
	// submodule PR can hold): file to the fingerprint of the change marked.
	subViewed map[fileID]string
	// subReviewURLs are reviews SubmitReview already posted on submodule PRs while the parent
	// review has yet to go through.
	subReviewURLs []string
	// subCommentURLs are single comments SubmitReview already posted on submodule PRs, likewise.
	subCommentURLs []string
	// orphans are pending reviews failed submits left, on submodule PRs or the loaded one; the next submit deletes them first.
	orphans []orphanReview
	// claimed are the comments and reviews submits already count as posted, so no other draft's reconciliation adopts them.
	claimed map[claim]bool
	// uncertain maps the ID of each draft whose last send may have gone through, unconfirmed because the lookup failed,
	// to that send; the next submit looks for it where it went before doing anything else with the draft.
	uncertain map[int64]*uncertainError
	// parentAttempt is the loaded PR's review whose last send may have gone through, unconfirmed likewise, kept
	// even when it carried no drafts; the next submit looks for it before sending it again.
	parentAttempt *uncertainError
	// immediate maps each single comment PostImmediateComment sent whose outcome is unknown to that send; posting
	// the same comment again looks for it first.
	immediate map[immediateKey]*uncertainError
	// draftSeq numbers pending drafts for DraftCommentDTO.ID.
	draftSeq int64

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
	local atomic.Pointer[localReview]

	// launch is the PR or local checkout to open once signed in; a second launch sets it from another goroutine.
	launch atomic.Pointer[launchTarget]
	// focus hides navigation away from the launched review; set once by ConfigureLaunch.
	focus bool

	// Device-flow state, held in memory only during the OAuth flow.
	deviceCode string
	authToken  string
	// login is the authenticated user's login, "" until viewerLogin or GetAuthState reads it.
	login string
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
		a.cfg.CompareAndSwap(nil, &settings.Config{IgnoredCommenters: []model.IgnoredCommenterDTO{}, Commands: []model.CommandDTO{}})
		return err
	}
	// CompareAndSwap: a setter that saved while this read ran has already cached the newer config.
	a.cfg.CompareAndSwap(nil, &cfg)
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

// applyConfig caches cfg. When the project folders change, it swaps the snapshot and drops the
// repo index together under repoIndexMu, so a lookup never pairs the new folders with the old index.
// Callers hold configMu.
func (a *App) applyConfig(cfg settings.Config) {
	old := a.cfg.Load()
	if old != nil && slices.Equal(old.ProjectDirs, cfg.ProjectDirs) {
		a.cfg.Store(&cfg)
		return
	}
	a.repoIndexMu.Lock()
	defer a.repoIndexMu.Unlock()
	a.cfg.Store(&cfg)
	a.repoIndex, a.repoIndexAt = nil, time.Time{}
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
	a.login = ""
	logger.L.Debug("github client initialised")
	return nil
}

// viewerLogin returns the authenticated user's login, fetched once per client.
func (a *App) viewerLogin() (string, error) {
	if a.login == "" {
		u, _, err := a.ghClient.Users.Get(a.ctx, "")
		if err != nil {
			return "", fmt.Errorf("github: get authenticated user: %w", err)
		}
		a.login = u.GetLogin()
	}
	return a.login, nil
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
