// Package auth implements GitHub OAuth Device Flow authentication.
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitura/internal/logger"
	"gitura/internal/model"
)

const (
	// tokenURL is the GitHub endpoint for polling the device flow token.
	tokenURL = "https://github.com/login/oauth/access_token"

	// statusPending indicates the user has not yet authorized the device.
	statusPending = "authorization_pending"

	// statusExpired indicates the device code has expired.
	statusExpired = "expired_token"

	// statusSlowDown indicates the polling interval should be increased.
	statusSlowDown = "slow_down"
)

// tokenResponse is the raw JSON from GitHub's access_token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

// PollForToken polls GitHub's token endpoint for the result of a device flow.
// It returns a PollResult with status "pending", "complete", "expired", or "error".
func PollForToken(deviceCode, clientID string) (model.PollResult, string, error) {
	if deviceCode == "" || clientID == "" {
		return model.PollResult{Status: "error", Error: "missing device code or client ID"}, "", fmt.Errorf("auth: deviceCode and clientID must not be empty")
	}

	logger.L.Debug("polling for token", "url", tokenURL)

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return model.PollResult{Status: "error", Error: err.Error()}, "", fmt.Errorf("auth: failed to build token poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.L.Error("token poll request failed", "err", err)
		return model.PollResult{Status: "error", Error: err.Error()}, "", fmt.Errorf("auth: token poll request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.L.Debug("token poll response", "http_status", resp.StatusCode)
	return decodeTokenResponse(resp)
}

// PollForTokenWith polls using a custom HTTPPoster for testability.
// Returns the PollResult, the access token (non-empty only on "complete"), and any error.
func PollForTokenWith(poster HTTPPoster, deviceCode, clientID string) (model.PollResult, string, error) {
	if deviceCode == "" || clientID == "" {
		return model.PollResult{Status: "error", Error: "missing device code or client ID"}, "", fmt.Errorf("auth: deviceCode and clientID must not be empty")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	resp, err := poster.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return model.PollResult{Status: "error", Error: err.Error()}, "", fmt.Errorf("auth: token poll request failed: %w", err)
	}
	defer resp.Body.Close()

	return decodeTokenResponse(resp)
}

// decodeTokenResponse parses an *http.Response from the token endpoint.
func decodeTokenResponse(resp *http.Response) (model.PollResult, string, error) {
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		logger.L.Error("failed to decode token response", "err", err)
		return model.PollResult{Status: "error", Error: "decode error"}, "", fmt.Errorf("auth: failed to decode token response: %w", err)
	}

	logger.L.Debug("token response decoded", "has_token", tr.AccessToken != "", "error_field", tr.Error)

	if tr.AccessToken != "" {
		return model.PollResult{Status: "complete"}, tr.AccessToken, nil
	}

	switch tr.Error {
	case statusPending:
		return model.PollResult{Status: "pending"}, "", nil
	case statusSlowDown:
		// GitHub requires the poll interval to increase by 5 seconds on slow_down.
		// Return Interval so the caller can reschedule accordingly.
		return model.PollResult{Status: "pending", Interval: 5}, "", nil
	case statusExpired:
		return model.PollResult{Status: "expired"}, "", nil
	default:
		msg := tr.Error
		if msg == "" {
			msg = "unknown error"
		}
		logger.L.Warn("unexpected token response error", "error_field", msg)
		return model.PollResult{Status: "error", Error: msg}, "", fmt.Errorf("auth: %s", msg)
	}
}
