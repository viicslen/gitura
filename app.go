// Package main is the entry point for the gitura desktop application.
// It wires together the Wails runtime with the App struct.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v67/github"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitura/internal/auth"
	githubclient "gitura/internal/github"
	"gitura/internal/keyring"
	"gitura/internal/logger"
	"gitura/internal/model"
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

	// Device-flow state, held in memory only during the OAuth flow.
	deviceCode string
	authToken  string
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts.
// The context is stored for use with Wails runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.L.Info("app started", "client_id_set", os.Getenv("GITURA_GITHUB_CLIENT_ID") != "")
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

// clientID returns the GitHub OAuth app client ID from the environment.
func clientID() string {
	id := os.Getenv("GITURA_GITHUB_CLIENT_ID")
	if id == "" {
		id = "Ov23liFakeClientIDDev" // placeholder for dev builds
	}
	return id
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
