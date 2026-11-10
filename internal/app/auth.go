package app

import (
	"fmt"

	"gitura/internal/auth"
	"gitura/internal/ghapi"
	"gitura/internal/keyring"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// StartDeviceFlow initiates GitHub OAuth device flow.
// Stores device_code in memory; returns display data to the frontend.
func (a *App) StartDeviceFlow() (model.DeviceFlowInfo, error) {
	logger.L.Info("StartDeviceFlow called")
	info, err := auth.StartDeviceFlow(a.clientID)
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

	result, token, err := auth.PollForToken(a.deviceCode, a.clientID)
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
	client := ghapi.NewClient(token)
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
