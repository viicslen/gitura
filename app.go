// Package main is the entry point for the gitura desktop application.
// It wires together the Wails runtime with the App struct.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitura/internal/auth"
	"gitura/internal/db"
	githubclient "gitura/internal/github"
	"gitura/internal/keyring"
	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/runner"
	"gitura/internal/settings"
)

// App is the main application struct exposed to Wails.
// Business methods are added in subsequent phases.
type App struct {
	ctx context.Context

	// Authenticated GitHub client; nil until auth is complete.
	ghClient *github.Client

	// Cached PR data populated by LoadPullRequest.
	prOwner  string
	prRepo   string
	prNumber int
	prCache  *model.PullRequestSummary
	threads  []model.CommentThreadDTO

	// prFilesCache holds the raw CommitFile list from the GitHub ListFiles API.
	// Populated lazily by GetPRFiles; cleared on LoadPullRequest.
	prFilesCache []*github.CommitFile

	// Pending review state. pendingReviewID is 0 when no pending review exists.
	pendingReviewID int64
	pendingComments []model.DraftCommentDTO

	// ignoredCommenters is the persisted list of commenters to filter from review threads.
	// Lazily loaded on first use via loadConfig.
	ignoredCommenters []model.IgnoredCommenterDTO

	// commands is the persisted list of named CLI commands.
	// Lazily loaded alongside ignoredCommenters via loadConfig.
	commands []model.CommandDTO

	// defaultCommandID is the ID of the command to use as the primary action.
	// Empty string means no default is set.
	defaultCommandID string

	// runCancels maps run IDs to their context cancel functions, allowing
	// in-flight commands to be stopped via CancelRun.
	runCancelsMu sync.Mutex
	runCancels   map[string]context.CancelFunc

	// queries is the sqlc-generated DB query interface for app-managed state.
	// Opened during startup; nil if the DB could not be initialised.
	queries *db.Queries

	// Device-flow state, held in memory only during the OAuth flow.
	deviceCode string
	authToken  string
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{
		runCancels: make(map[string]context.CancelFunc),
	}
}

// startup is called by Wails when the application starts.
// The context is stored for use with Wails runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.L.Info("app started", "client_id", githubClientID)

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

	// Pre-warm config so the first LoadPullRequest call is fast.
	if err := a.loadConfig(); err != nil {
		logger.L.Warn("failed to pre-load config", "err", err)
	}
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
	a.defaultCommandID = cfg.DefaultCommandID
	logger.L.Debug("config loaded",
		"ignored_commenters", len(a.ignoredCommenters),
		"commands", len(a.commands),
	)
	return nil
}

// domReady is called by Wails after the frontend DOM is ready.
func (a *App) domReady(_ context.Context) {
	logger.L.Debug("dom ready")
}

// beforeClose is called by Wails just before the window closes.
// Return true to prevent the close; false to allow it.
func (a *App) beforeClose(_ context.Context) bool {
	logger.L.Debug("before close")
	return false
}

// initGHClient sets up the authenticated GitHub client from a token.
func (a *App) initGHClient(token string) {
	a.authToken = token
	a.ghClient = githubclient.NewClient(token)
	logger.L.Debug("github client initialised")
}

// emit sends a named Wails event to the frontend.
func (a *App) emit(name string, data ...interface{}) {
	logger.L.Debug("emitting event", "event", name)
	runtime.EventsEmit(a.ctx, name, data...)
}

// clientID returns the GitHub OAuth app client ID.
// The value is baked in at build time via -ldflags "-X main.githubClientID=...".
// Falls back to a placeholder for local dev builds.
var githubClientID = "Ov23liFakeClientIDDev"

// clientID returns the GitHub OAuth app client ID.
func clientID() string {
	return githubClientID
}

// StartDeviceFlow initiates GitHub OAuth device flow.
// Stores device_code in memory; returns display data to the frontend.
func (a *App) StartDeviceFlow() (model.DeviceFlowInfo, error) {
	logger.L.Info("StartDeviceFlow called")
	info, err := auth.StartDeviceFlow(clientID())
	if err != nil {
		logger.L.Error("StartDeviceFlow failed", "err", err)
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: %w", err)
	}
	a.deviceCode = info.DeviceCode
	logger.L.Info("device flow started",
		"user_code", info.UserCode,
		"verification_uri", info.VerificationURI,
		"expires_in", info.ExpiresIn,
		"interval", info.Interval,
	)
	return info, nil
}

// PollDeviceFlow polls GitHub for token completion.
// On success, saves the token to the keyring, initialises the GitHub client,
// and emits the "auth:device-flow-complete" event to the frontend.
func (a *App) PollDeviceFlow() (model.PollResult, error) {
	if a.deviceCode == "" {
		logger.L.Warn("PollDeviceFlow called with no active device flow")
		return model.PollResult{Status: "error", Error: "no active device flow"}, fmt.Errorf("auth: no active device flow; call StartDeviceFlow first")
	}

	result, token, err := auth.PollForToken(a.deviceCode, clientID())
	logger.L.Debug("poll result", "status", result.Status, "token_received", len(token) > 0, "err", err)

	if err != nil && result.Status != "expired" {
		logger.L.Error("PollForToken error", "err", err)
		return result, fmt.Errorf("auth: %w", err)
	}

	if result.Status == "complete" {
		prefix := ""
		if len(token) > 10 {
			prefix = token[:10]
		}
		logger.L.Info("token received", "prefix", prefix+"...")
		if saveErr := keyring.SaveToken(token); saveErr != nil {
			logger.L.Warn("keyring save failed — token held in memory only", "err", saveErr)
		} else {
			logger.L.Info("token saved to keyring")
		}
		a.initGHClient(token)
		a.deviceCode = ""
		a.emit("auth:device-flow-complete")
		logger.L.Info("auth complete")
	}

	if result.Status == "expired" {
		logger.L.Warn("device flow expired")
		a.deviceCode = ""
		a.emit("auth:device-flow-expired")
	}

	return result, nil
}

// GetAuthState returns the current authentication status.
// If the in-memory client is set, it uses that. Otherwise it loads from keyring.
func (a *App) GetAuthState() (model.AuthState, error) {
	logger.L.Debug("GetAuthState called", "has_client", a.ghClient != nil)

	// Fast path: in-memory client already set (e.g. just completed device flow).
	if a.ghClient != nil {
		logger.L.Debug("using in-memory github client")
		ghUser, _, err := a.ghClient.Users.Get(a.ctx, "")
		if err != nil {
			logger.L.Warn("in-memory client Users.Get failed — clearing client", "err", err)
			a.ghClient = nil
			a.authToken = ""
		} else {
			logger.L.Info("GetAuthState: authenticated via memory", "login", ghUser.GetLogin())
			return model.AuthState{
				IsAuthenticated: true,
				Login:           ghUser.GetLogin(),
				AvatarURL:       ghUser.GetAvatarURL(),
			}, nil
		}
	}

	logger.L.Debug("loading token from keyring")
	token, err := keyring.LoadToken()
	if err != nil || token == "" {
		logger.L.Info("no token in keyring", "err", err)
		return model.AuthState{IsAuthenticated: false}, nil
	}

	prefix := ""
	if len(token) > 10 {
		prefix = token[:10]
	}
	logger.L.Debug("keyring token prefix", "prefix", prefix+"...")

	logger.L.Debug("verifying keyring token via GitHub API")
	client := githubclient.NewClient(token)
	ghUser, _, err := client.Users.Get(a.ctx, "")
	if err != nil {
		logger.L.Warn("keyring token invalid", "err", err)
		return model.AuthState{IsAuthenticated: false}, nil
	}

	a.initGHClient(token)
	logger.L.Info("GetAuthState: authenticated via keyring", "login", ghUser.GetLogin())
	return model.AuthState{
		IsAuthenticated: true,
		Login:           ghUser.GetLogin(),
		AvatarURL:       ghUser.GetAvatarURL(),
	}, nil
}

// Logout removes the stored token and clears in-memory auth state.
func (a *App) Logout() error {
	logger.L.Info("Logout called")
	if err := keyring.DeleteToken(); err != nil {
		logger.L.Error("keyring delete failed", "err", err)
		return fmt.Errorf("keyring: %w", err)
	}
	a.ghClient = nil
	a.authToken = ""
	a.deviceCode = ""
	logger.L.Info("logged out")
	return nil
}

// LoadPullRequest fetches PR metadata and all review threads, caches them in memory,
// and returns a PullRequestSummary with CommentCount and UnresolvedCount computed
// from the filtered thread list.
//
// Progress events ("pr:load-progress") are emitted during the paginated GraphQL fetch.
// Ignored-commenter threads are filtered out before caching.
//
// Error prefixes: "auth:" — no client; "notfound:" — PR absent; "github:" — API error.
func (a *App) LoadPullRequest(owner, repo string, number int) (model.PullRequestSummary, error) {
	logger.L.Info("LoadPullRequest called", "owner", owner, "repo", repo, "number", number)

	if a.ghClient == nil {
		return model.PullRequestSummary{}, fmt.Errorf("auth: not authenticated")
	}
	if err := a.loadConfig(); err != nil {
		logger.L.Warn("LoadPullRequest: failed to load ignored commenters", "err", err)
	}

	pr, err := githubclient.FetchPR(a.ctx, a.ghClient, owner, repo, number)
	if err != nil {
		logger.L.Error("FetchPR failed", "err", err)
		return model.PullRequestSummary{}, err
	}

	httpClient := githubclient.NewHTTPClient(a.authToken)
	threads, err := githubclient.FetchReviewThreads(
		a.ctx, httpClient, owner, repo, number,
		func(loaded, total int) {
			a.emit("pr:load-progress", map[string]int{"loaded": loaded, "total": total})
		},
	)
	if err != nil {
		logger.L.Error("FetchReviewThreads failed", "err", err)
		return model.PullRequestSummary{}, err
	}

	// Build ignored-login set for O(1) lookup.
	ignoredSet := make(map[string]struct{}, len(a.ignoredCommenters))
	for _, ic := range a.ignoredCommenters {
		ignoredSet[ic.Login] = struct{}{}
	}

	filtered := filterIgnoredThreads(threads, ignoredSet)

	commentCount := len(filtered)
	unresolvedCount := countUnresolved(filtered)

	pr.CommentCount = commentCount
	pr.UnresolvedCount = unresolvedCount
	pr.Owner = owner
	pr.Repo = repo

	// Cache for subsequent GetCommentThreads / GetThread calls.
	a.prOwner = owner
	a.prRepo = repo
	a.prNumber = number
	a.prCache = pr
	a.threads = filtered
	// Clear diff and review caches for the new PR.
	a.prFilesCache = nil
	a.pendingReviewID = 0
	a.pendingComments = nil

	logger.L.Info("LoadPullRequest complete",
		"threads", len(filtered),
		"unresolved", unresolvedCount,
	)
	return *pr, nil
}

// filterIgnoredThreads returns threads whose root comment author is not in ignored.
// Threads with no comments are always included.
func filterIgnoredThreads(threads []model.CommentThreadDTO, ignored map[string]struct{}) []model.CommentThreadDTO {
	filtered := make([]model.CommentThreadDTO, 0, len(threads))
	for _, t := range threads {
		if len(t.Comments) == 0 {
			filtered = append(filtered, t)
			continue
		}
		if _, ok := ignored[t.Comments[0].AuthorLogin]; !ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// countUnresolved returns the number of unresolved threads in the slice.
func countUnresolved(threads []model.CommentThreadDTO) int {
	n := 0
	for _, t := range threads {
		if !t.Resolved {
			n++
		}
	}
	return n
}

// GetCommentThreads returns the cached review threads for the loaded PR.
// When includeResolved is false, resolved threads are excluded.
// Returns "notfound:" error if no PR has been loaded yet.
func (a *App) GetCommentThreads(includeResolved bool) ([]model.CommentThreadDTO, error) {
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}
	if includeResolved {
		return a.threads, nil
	}
	result := make([]model.CommentThreadDTO, 0, len(a.threads))
	for _, t := range a.threads {
		if !t.Resolved {
			result = append(result, t)
		}
	}
	return result, nil
}

// GetThread returns a single review thread by its root comment ID.
// Returns "notfound:thread" if no thread with that ID exists in the cache.
func (a *App) GetThread(rootID int64) (model.CommentThreadDTO, error) {
	if a.prCache == nil {
		return model.CommentThreadDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}
	for _, t := range a.threads {
		if t.RootID == rootID {
			return t, nil
		}
	}
	return model.CommentThreadDTO{}, fmt.Errorf("notfound:thread %d", rootID)
}

// ReplyToComment posts a reply to an existing review thread.
// threadRootID identifies the thread by its root comment's database ID.
// Returns the new CommentDTO on success and appends it to the cached thread.
// Error prefixes: "validation:" — empty body; "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) ReplyToComment(threadRootID int64, body string) (model.CommentDTO, error) {
	if strings.TrimSpace(body) == "" {
		return model.CommentDTO{}, fmt.Errorf("validation:body required")
	}
	if a.ghClient == nil {
		return model.CommentDTO{}, fmt.Errorf("auth: not authenticated")
	}

	// Find thread in cache.
	threadIdx := -1
	for i, t := range a.threads {
		if t.RootID == threadRootID {
			threadIdx = i
			break
		}
	}
	if threadIdx == -1 {
		return model.CommentDTO{}, fmt.Errorf("notfound:thread %d", threadRootID)
	}

	comment, _, err := a.ghClient.PullRequests.CreateCommentInReplyTo(a.ctx, a.prOwner, a.prRepo, a.prNumber, body, threadRootID)
	if err != nil {
		return model.CommentDTO{}, fmt.Errorf("github: create comment: %w", err)
	}

	dto := model.CommentDTO{
		ID:           comment.GetID(),
		InReplyToID:  threadRootID,
		Body:         comment.GetBody(),
		AuthorLogin:  comment.GetUser().GetLogin(),
		AuthorAvatar: comment.GetUser().GetAvatarURL(),
		CreatedAt:    comment.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
		IsSuggestion: strings.Contains(comment.GetBody(), "```suggestion"),
	}

	// Append to cached thread.
	a.threads[threadIdx].Comments = append(a.threads[threadIdx].Comments, dto)

	logger.L.Info("ReplyToComment complete", "thread_root_id", threadRootID, "new_comment_id", dto.ID)
	return dto, nil
}

// ResolveThread marks a review thread as resolved.
// threadRootID identifies the thread by its root comment's database ID.
// Error prefixes: "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) ResolveThread(threadRootID int64) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}

	threadIdx := -1
	for i, t := range a.threads {
		if t.RootID == threadRootID {
			threadIdx = i
			break
		}
	}
	if threadIdx == -1 {
		return fmt.Errorf("notfound:thread %d", threadRootID)
	}

	nodeID := a.threads[threadIdx].NodeID
	httpClient := githubclient.NewHTTPClient(a.authToken)
	if err := githubclient.ResolveThread(a.ctx, httpClient, nodeID); err != nil {
		return err
	}

	a.threads[threadIdx].Resolved = true
	logger.L.Info("ResolveThread complete", "thread_root_id", threadRootID)
	return nil
}

// UnresolveThread marks a review thread as unresolved.
// threadRootID identifies the thread by its root comment's database ID.
// Error prefixes: "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) UnresolveThread(threadRootID int64) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}

	threadIdx := -1
	for i, t := range a.threads {
		if t.RootID == threadRootID {
			threadIdx = i
			break
		}
	}
	if threadIdx == -1 {
		return fmt.Errorf("notfound:thread %d", threadRootID)
	}

	nodeID := a.threads[threadIdx].NodeID
	httpClient := githubclient.NewHTTPClient(a.authToken)
	if err := githubclient.UnresolveThread(a.ctx, httpClient, nodeID); err != nil {
		return err
	}

	a.threads[threadIdx].Resolved = false
	logger.L.Info("UnresolveThread complete", "thread_root_id", threadRootID)
	return nil
}

// CommitSuggestion applies the suggestion block in a review comment to the PR
// branch and creates a commit.
//
// commentID is the database ID of the comment that contains the suggestion.
// The method finds the comment across all cached threads, resolves the file
// path from its parent thread, and delegates to suggestion.CommitSuggestion.
//
// Error prefixes: "auth:" — no client; "notfound:comment" — unknown comment;
// "validation:" — not a suggestion; "github:conflict" — SHA conflict;
// "github:" — other API errors.
func (a *App) CommitSuggestion(commentID int64, commitMessage string) (model.SuggestionCommitResult, error) {
	if a.ghClient == nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("notfound:comment %d — no PR loaded", commentID)
	}

	// Find the comment and its parent thread.
	var (
		found      model.CommentDTO
		threadPath string
	)
	for _, t := range a.threads {
		for _, c := range t.Comments {
			if c.ID == commentID {
				found = c
				threadPath = t.Path
				goto done
			}
		}
	}
done:
	if found.ID == 0 {
		return model.SuggestionCommitResult{}, fmt.Errorf("notfound:comment %d", commentID)
	}

	result, err := githubclient.CommitSuggestion(
		a.ctx,
		a.ghClient,
		a.prOwner,
		a.prRepo,
		a.prCache.HeadBranch,
		threadPath,
		found,
		commitMessage,
	)
	if err != nil {
		logger.L.Error("CommitSuggestion failed", "comment_id", commentID, "err", err)
		return model.SuggestionCommitResult{}, err
	}

	logger.L.Info("CommitSuggestion complete", "comment_id", commentID, "commit_sha", result.CommitSHA)
	return result, nil
}

// GetIgnoredCommenters returns the current list of ignored-commenter entries.
// The list is lazily loaded from disk on the first call.
// Error prefix: none — returns empty slice on load failure.
func (a *App) GetIgnoredCommenters() ([]model.IgnoredCommenterDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	result := make([]model.IgnoredCommenterDTO, len(a.ignoredCommenters))
	copy(result, a.ignoredCommenters)
	return result, nil
}

// AddIgnoredCommenter adds a GitHub login to the ignored-commenters list.
// Silently no-ops if the login is already present.
// Error prefixes: "validation:" — empty login; "settings:" — save failure.
func (a *App) AddIgnoredCommenter(login string) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return fmt.Errorf("validation: login required")
	}
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	for _, ic := range a.ignoredCommenters {
		if ic.Login == login {
			return nil // already present — no-op
		}
	}
	a.ignoredCommenters = append(a.ignoredCommenters, model.IgnoredCommenterDTO{
		Login:   login,
		AddedAt: time.Now().UTC(),
	})
	if err := settings.Save(settings.Config{IgnoredCommenters: a.ignoredCommenters, Commands: a.commands, DefaultCommandID: a.defaultCommandID}); err != nil {
		// Roll back in-memory change.
		a.ignoredCommenters = a.ignoredCommenters[:len(a.ignoredCommenters)-1]
		return fmt.Errorf("settings: save: %w", err)
	}
	logger.L.Info("AddIgnoredCommenter", "login", login)
	return nil
}

// RemoveIgnoredCommenter removes a GitHub login from the ignored-commenters list.
// Silently no-ops if the login is not present.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveIgnoredCommenter(login string) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	idx := -1
	for i, ic := range a.ignoredCommenters {
		if ic.Login == login {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil // not present — no-op
	}
	updated := make([]model.IgnoredCommenterDTO, 0, len(a.ignoredCommenters)-1)
	updated = append(updated, a.ignoredCommenters[:idx]...)
	updated = append(updated, a.ignoredCommenters[idx+1:]...)
	if err := settings.Save(settings.Config{IgnoredCommenters: updated, Commands: a.commands, DefaultCommandID: a.defaultCommandID}); err != nil {
		return fmt.Errorf("settings: save: %w", err)
	}
	a.ignoredCommenters = updated
	logger.L.Info("RemoveIgnoredCommenter", "login", login)
	return nil
}

// ListOpenPRs fetches all open pull requests involving the authenticated user
// using a single GitHub Search query (involves: qualifier). The returned items
// are tagged with IsAuthor, IsAssignee, IsReviewer for client-side filtering.
// Only IncludeDrafts from the filters struct affects the server-side query;
// all other filter fields are applied in the frontend.
// All errors are surfaced via the returned PRListResult.Error field.
func (a *App) ListOpenPRs(filters model.PRListFilters) (model.PRListResult, error) {
	logger.L.Debug("ListOpenPRs called", "include_drafts", filters.IncludeDrafts)

	if a.ghClient == nil {
		logger.L.Warn("ListOpenPRs called without authenticated client")
		return model.PRListResult{Error: "not authenticated"}, nil
	}

	authState, err := a.GetAuthState()
	if err != nil || !authState.IsAuthenticated {
		logger.L.Warn("ListOpenPRs: auth check failed", "err", err)
		return model.PRListResult{Error: "not authenticated"}, nil
	}

	result, err := githubclient.SearchOpenPRs(a.ctx, a.ghClient, authState.Login, filters)
	if err != nil {
		logger.L.Error("SearchOpenPRs error", "err", err)
		return model.PRListResult{Error: fmt.Sprintf("search failed: %v", err)}, nil
	}

	logger.L.Info("ListOpenPRs complete",
		"items", len(result.Items),
		"incomplete", result.IncompleteResults,
		"error", result.Error,
	)
	return result, nil
}

// GetPRFiles returns the list of files changed in the currently loaded PR.
// Results are cached; the cache is cleared when LoadPullRequest is called.
// Error prefix: "notfound:" if no PR has been loaded.
func (a *App) GetPRFiles() ([]model.PRFileDTO, error) {
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}
	if a.prFilesCache == nil {
		rawFiles, err := githubclient.FetchPRFilesRaw(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
		if err != nil {
			return nil, err
		}
		a.prFilesCache = rawFiles
	}
	result := make([]model.PRFileDTO, 0, len(a.prFilesCache))
	for _, f := range a.prFilesCache {
		result = append(result, githubclient.CommitFileToPRFileDTO(f))
	}
	return result, nil
}

// GetFileDiff fetches and parses the diff for a single file in the loaded PR.
// Binary files return ParsedDiffDTO{IsBinary: true} with no hunks, no error.
// Error prefix: "notfound:" if no PR is loaded or the file is not in the diff.
func (a *App) GetFileDiff(path string) (model.ParsedDiffDTO, error) {
	if a.ghClient == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}
	if a.prFilesCache == nil {
		rawFiles, err := githubclient.FetchPRFilesRaw(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
		if err != nil {
			return model.ParsedDiffDTO{}, err
		}
		a.prFilesCache = rawFiles
	}
	for _, f := range a.prFilesCache {
		if f.GetFilename() == path {
			return githubclient.ParseCommitFileDiff(f), nil
		}
	}
	return model.ParsedDiffDTO{}, fmt.Errorf("notfound: file %q not in PR diff", path)
}

// GetPendingReview returns the current in-memory pending review state.
// Returns PendingReviewDTO{HasPending: false} when no pending review exists.
func (a *App) GetPendingReview() (model.PendingReviewDTO, error) {
	if len(a.pendingComments) == 0 {
		return model.PendingReviewDTO{HasPending: false}, nil
	}
	comments := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(comments, a.pendingComments)
	return model.PendingReviewDTO{
		Comments:   comments,
		HasPending: true,
	}, nil
}

// SyncPendingReview queries GitHub for any PENDING review on the current PR
// that belongs to the authenticated user, and restores in-memory state from it.
// This recovers pending-review state across app restarts. If in-memory state
// already exists it is returned immediately without a network call.
// GitHub only returns PENDING reviews to their own author, so any PENDING entry
// in the list is guaranteed to belong to us.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) SyncPendingReview() (model.PendingReviewDTO, error) {
	if a.ghClient == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}

	// If we already have local state, return it directly.
	if len(a.pendingComments) > 0 {
		return a.GetPendingReview()
	}

	// List all reviews; PENDING reviews are only visible to their author,
	// so any PENDING entry belongs to the authenticated user.
	reviews, _, err := a.ghClient.PullRequests.ListReviews(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, nil,
	)
	if err != nil {
		return model.PendingReviewDTO{}, fmt.Errorf("github: list reviews: %w", err)
	}

	var reviewID int64
	for _, r := range reviews {
		if r.GetState() == "PENDING" {
			reviewID = r.GetID()
			break
		}
	}
	if reviewID == 0 {
		return model.PendingReviewDTO{HasPending: false}, nil
	}

	a.pendingReviewID = reviewID

	// Fetch the comments for this review to populate the local cache.
	rawComments, _, err := a.ghClient.PullRequests.ListReviewComments(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, reviewID, nil,
	)
	if err != nil {
		// Non-fatal: we have the review ID; comment list will just be empty.
		logger.L.Warn("SyncPendingReview: failed to fetch review comments", "err", err)
		a.pendingComments = nil
	} else {
		a.pendingComments = make([]model.DraftCommentDTO, 0, len(rawComments))
		for _, c := range rawComments {
			a.pendingComments = append(a.pendingComments, model.DraftCommentDTO{
				Path:      c.GetPath(),
				Body:      c.GetBody(),
				Line:      c.GetLine(),
				Side:      c.GetSide(),
				StartLine: c.GetStartLine(),
				StartSide: c.GetStartSide(),
			})
		}
	}

	dtos := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(dtos, a.pendingComments)
	logger.L.Info("SyncPendingReview: restored pending review", "review_id", reviewID, "comments", len(dtos))
	return model.PendingReviewDTO{
		ReviewID:   reviewID,
		Comments:   dtos,
		HasPending: true,
	}, nil
}

// AddDraftComment adds a comment to the local pending review batch.
// Comments are stored in-memory only; no GitHub API calls are made during
// the draft phase. All comments are submitted together by SubmitReview.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded.
func (a *App) AddDraftComment(comment model.DraftCommentDTO) (model.PendingReviewDTO, error) {
	if a.ghClient == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}

	a.pendingComments = append(a.pendingComments, comment)
	logger.L.Info("AddDraftComment (local)", "path", comment.Path, "line", comment.Line, "total", len(a.pendingComments))

	comments := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(comments, a.pendingComments)
	return model.PendingReviewDTO{
		Comments:   comments,
		HasPending: true,
	}, nil
}

// PostImmediateComment posts a standalone inline comment immediately (not as a
// draft review comment). The comment is immediately visible on GitHub.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) PostImmediateComment(comment model.DraftCommentDTO) (model.CommentDTO, error) {
	if a.ghClient == nil {
		return model.CommentDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.CommentDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}

	commitSHA := a.prCache.HeadSHA
	prComment := &github.PullRequestComment{
		Path:     &comment.Path,
		Body:     &comment.Body,
		CommitID: &commitSHA,
		Line:     &comment.Line,
		Side:     &comment.Side,
	}
	if comment.StartLine > 0 {
		prComment.StartLine = &comment.StartLine
		prComment.StartSide = &comment.StartSide
	}

	created, _, err := a.ghClient.PullRequests.CreateComment(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, prComment,
	)
	if err != nil {
		return model.CommentDTO{}, fmt.Errorf("github: post immediate comment: %w", err)
	}

	logger.L.Info("PostImmediateComment", "path", comment.Path, "line", comment.Line, "id", created.GetID())
	return model.CommentDTO{
		ID:           created.GetID(),
		Body:         created.GetBody(),
		AuthorLogin:  created.GetUser().GetLogin(),
		AuthorAvatar: created.GetUser().GetAvatarURL(),
		CreatedAt:    created.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

// SubmitReview submits all pending draft comments as a single review.
// If an existing GitHub pending review was synced, it is deleted first so
// there is no conflict when creating the new submitted review.
// All locally-buffered comments are sent together with the verdict in one
// CreateReview call.
// Side effect: clears pendingReviewID and pendingComments on success.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) SubmitReview(req model.ReviewSubmitDTO) (model.ReviewSubmitResult, error) {
	if a.ghClient == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("notfound: no PR loaded; call LoadPullRequest first")
	}

	// If we synced an existing GitHub pending review, delete it first.
	// We recreate it as a submitted review with all local comments below.
	if a.pendingReviewID != 0 {
		_, _, delErr := a.ghClient.PullRequests.DeletePendingReview(
			a.ctx, a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID,
		)
		if delErr != nil {
			logger.L.Warn("SubmitReview: failed to delete old pending review (continuing)", "review_id", a.pendingReviewID, "err", delErr)
		}
		a.pendingReviewID = 0
	}

	draftComments := make([]*github.DraftReviewComment, len(a.pendingComments))
	for i, c := range a.pendingComments {
		draftComments[i] = draftCommentToGitHub(c)
	}

	commitSHA := a.prCache.HeadSHA
	review, _, err := a.ghClient.PullRequests.CreateReview(
		a.ctx, a.prOwner, a.prRepo, a.prNumber,
		&github.PullRequestReviewRequest{
			CommitID: &commitSHA,
			Body:     &req.Body,
			Event:    &req.Verdict,
			Comments: draftComments,
		},
	)
	if err != nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("github: submit review: %w", err)
	}

	a.pendingComments = nil
	logger.L.Info("SubmitReview complete", "review_id", review.GetID(), "verdict", req.Verdict)
	return model.ReviewSubmitResult{
		ReviewID: review.GetID(),
		HTMLURL:  review.GetHTMLURL(),
	}, nil
}

// DiscardPendingReview clears the local pending review state and, if a GitHub
// pending review was synced, deletes it from GitHub too.
// Error prefix: "auth:" — no client; "github:" — API error.
func (a *App) DiscardPendingReview() error {
	if a.pendingReviewID != 0 {
		if a.ghClient == nil {
			return fmt.Errorf("auth: not authenticated")
		}
		if _, _, err := a.ghClient.PullRequests.DeletePendingReview(
			a.ctx, a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID,
		); err != nil {
			return fmt.Errorf("github: delete pending review: %w", err)
		}
	}
	a.pendingReviewID = 0
	a.pendingComments = nil
	logger.L.Info("DiscardPendingReview: cleared state")
	return nil
}

// draftCommentToGitHub converts a DraftCommentDTO to a go-github DraftReviewComment.
func draftCommentToGitHub(c model.DraftCommentDTO) *github.DraftReviewComment {
	dc := &github.DraftReviewComment{
		Path: &c.Path,
		Body: &c.Body,
		Line: &c.Line,
		Side: &c.Side,
	}
	if c.StartLine > 0 {
		dc.StartLine = &c.StartLine
		dc.StartSide = &c.StartSide
	}
	return dc
}

// GetCommands returns the current list of configured commands.
// Error prefix: "settings:" — load failure.
func (a *App) GetCommands() ([]model.CommandDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// AddCommand adds a new command configuration.
// A UUID is generated for the command's ID. Silently no-ops if a command with
// the same name already exists.
// Error prefixes: "validation:" — missing name or command; "settings:" — save failure.
func (a *App) AddCommand(cmd model.CommandDTO) ([]model.CommandDTO, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Command = strings.TrimSpace(cmd.Command)
	if cmd.Name == "" {
		return nil, fmt.Errorf("validation: name required")
	}
	if cmd.Command == "" {
		return nil, fmt.Errorf("validation: command required")
	}
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	for _, existing := range a.commands {
		if existing.Name == cmd.Name {
			result := make([]model.CommandDTO, len(a.commands))
			copy(result, a.commands)
			return result, nil // already present — no-op
		}
	}
	cmd.ID = uuid.NewString()
	a.commands = append(a.commands, cmd)
	if err := settings.Save(settings.Config{
		IgnoredCommenters: a.ignoredCommenters,
		Commands:          a.commands,
		DefaultCommandID:  a.defaultCommandID,
	}); err != nil {
		a.commands = a.commands[:len(a.commands)-1]
		return nil, fmt.Errorf("settings: save: %w", err)
	}
	logger.L.Info("AddCommand", "name", cmd.Name, "id", cmd.ID)
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// RemoveCommand removes the command with the given ID.
// Silently no-ops if the ID is not found.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveCommand(id string) ([]model.CommandDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	idx := -1
	for i, cmd := range a.commands {
		if cmd.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		result := make([]model.CommandDTO, len(a.commands))
		copy(result, a.commands)
		return result, nil
	}
	updated := make([]model.CommandDTO, 0, len(a.commands)-1)
	updated = append(updated, a.commands[:idx]...)
	updated = append(updated, a.commands[idx+1:]...)
	// Clear the default if the removed command was the default.
	if a.defaultCommandID == id {
		a.defaultCommandID = ""
	}
	if err := settings.Save(settings.Config{
		IgnoredCommenters: a.ignoredCommenters,
		Commands:          updated,
		DefaultCommandID:  a.defaultCommandID,
	}); err != nil {
		return nil, fmt.Errorf("settings: save: %w", err)
	}
	a.commands = updated
	logger.L.Info("RemoveCommand", "id", id)
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// GetDefaultCommandID returns the ID of the user's default command, or an
// empty string if none is set.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetDefaultCommandID() (string, error) {
	if err := a.loadConfig(); err != nil {
		return "", fmt.Errorf("settings: load: %w", err)
	}
	return a.defaultCommandID, nil
}

// SetDefaultCommandID persists the given command ID as the default. Pass an
// empty string to clear the default.
//
// Error prefix: "validation:" — unknown ID; "settings:" — save failure.
func (a *App) SetDefaultCommandID(id string) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	if id != "" {
		found := false
		for _, c := range a.commands {
			if c.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("validation: unknown command ID %q", id)
		}
	}
	a.defaultCommandID = id
	if err := settings.Save(settings.Config{
		IgnoredCommenters: a.ignoredCommenters,
		Commands:          a.commands,
		DefaultCommandID:  a.defaultCommandID,
	}); err != nil {
		return err
	}
	return nil
}

// RunCommands launches one or more commands in parallel against
// the same input text. For each command a pending RunResult with
// Running=true is emitted immediately via the "command:run:pending" event.
// When each process finishes, the completed result is emitted via the
// "command:run:complete" event.
//
// Returns the pending RunResult stubs (one per command) so the frontend can
// seed its run list synchronously before events arrive.
//
// Error prefix: "validation:" — empty commandIDs; "settings:" — load failure.
func (a *App) RunCommands(commandIDs []string, input string, runCtx model.RunContext) ([]model.RunResult, error) {
	if len(commandIDs) == 0 {
		return nil, fmt.Errorf("validation: at least one commandID required")
	}
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}

	// Look up the per-PR local path from the state DB (best-effort).
	localPath := ""
	if a.queries != nil {
		state, err := a.queries.GetPRState(a.ctx, db.GetPRStateParams{
			Owner:  a.prOwner,
			Repo:   a.prRepo,
			Number: int64(a.prNumber),
		})
		if err == nil {
			localPath = state.LocalPath
		}
	}

	// Build a lookup map for fast ID → command resolution.
	cmdMap := make(map[string]model.CommandDTO, len(a.commands))
	for _, c := range a.commands {
		cmdMap[c.ID] = c
	}

	pendingResults := make([]model.RunResult, 0, len(commandIDs))
	now := time.Now().UTC().Format(time.RFC3339)

	var wg sync.WaitGroup
	for _, cid := range commandIDs {
		cmd, ok := cmdMap[cid]
		if !ok {
			logger.L.Warn("RunCommands: unknown command ID", "id", cid)
			continue
		}

		runID := uuid.NewString()

		// Create a cancellable context for this run and register the cancel func.
		cancelCtx, cancel := context.WithCancel(context.Background())
		a.runCancelsMu.Lock()
		a.runCancels[runID] = cancel
		a.runCancelsMu.Unlock()

		// Emit pending immediately so the frontend can show a spinner.
		pending := model.RunResult{
			RunID:        runID,
			CommandID:    cmd.ID,
			CommandName:  cmd.Name,
			Input:        input,
			StartedAt:    now,
			Running:      true,
			ThreadRootID: runCtx.ThreadRootID,
			CommentID:    runCtx.CommentID,
		}
		pendingResults = append(pendingResults, pending)
		a.emit("command:run:pending", pending)

		wg.Add(1)
		go func(c model.CommandDTO, rID string, ctx context.Context, lp string, rc model.RunContext) {
			defer wg.Done()
			defer func() {
				a.runCancelsMu.Lock()
				delete(a.runCancels, rID)
				a.runCancelsMu.Unlock()
			}()
			result := runner.RunCommand(ctx, c, input, lp)
			result.RunID = rID
			result.Running = false
			result.ThreadRootID = rc.ThreadRootID
			result.CommentID = rc.CommentID
			a.emit("command:run:complete", result)
			logger.L.Info("run complete",
				"run_id", rID,
				"command", c.Name,
				"exit_code", result.ExitCode,
				"cancelled", result.Cancelled,
			)
		}(cmd, runID, cancelCtx, localPath, runCtx)
	}

	return pendingResults, nil
}

// CancelRun stops an in-flight command by its run ID.
// If the run has already finished or the ID is unknown, this is a no-op.
//
// Error prefix: "not_found:" — unknown or already-finished run ID.
func (a *App) CancelRun(runID string) error {
	a.runCancelsMu.Lock()
	cancel, ok := a.runCancels[runID]
	a.runCancelsMu.Unlock()
	if !ok {
		return fmt.Errorf("not_found: run %q is not in-flight", runID)
	}
	cancel()
	return nil
}

// GetPRLocalPath returns the local filesystem path associated with the current PR,
// or an empty string if none has been set.
//
// Error prefix: "db:" — database query failure.
func (a *App) GetPRLocalPath() (string, error) {
	if a.queries == nil {
		return "", nil
	}
	state, err := a.queries.GetPRState(a.ctx, db.GetPRStateParams{
		Owner:  a.prOwner,
		Repo:   a.prRepo,
		Number: int64(a.prNumber),
	})
	if err != nil {
		// sql.ErrNoRows is expected when no path has been set yet.
		return "", nil
	}
	return state.LocalPath, nil
}

// SetPRLocalPath persists a local filesystem path for the current PR.
// Pass an empty string to clear the stored path.
//
// Error prefix: "db:" — database write failure.
func (a *App) SetPRLocalPath(localPath string) error {
	if a.queries == nil {
		return fmt.Errorf("db: state database is not available")
	}
	return a.queries.UpsertPRLocalPath(a.ctx, db.UpsertPRLocalPathParams{
		Owner:     a.prOwner,
		Repo:      a.prRepo,
		Number:    int64(a.prNumber),
		LocalPath: localPath,
	})
}

// OpenFolderPicker opens a native OS folder picker dialog and returns the
// selected directory path, or an empty string if the user cancels.
func (a *App) OpenFolderPicker(title string, defaultDir string) (string, error) {
	opts := runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
	}
	selected, err := runtime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil {
		return "", err
	}
	return selected, nil
}
