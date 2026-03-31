// Package auth implements GitHub OAuth Device Flow authentication.
package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitura/internal/logger"
	"gitura/internal/model"
)

const (
	// deviceCodeURL is the GitHub endpoint to initiate the device flow.
	deviceCodeURL = "https://github.com/login/device/code"

	// requiredScope is the OAuth scope needed for PR operations.
	requiredScope = "repo"
)

// deviceCodeResponse is the raw JSON from GitHub's device/code endpoint.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// HTTPPoster abstracts the HTTP POST operation for testability.
type HTTPPoster interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

// StartDeviceFlow initiates GitHub OAuth device flow for the given clientID.
// It returns the DeviceFlowInfo required for the user to authenticate.
// GitHub returns form-encoded data by default; Accept: application/json requests JSON.
func StartDeviceFlow(clientID string) (model.DeviceFlowInfo, error) {
	if clientID == "" {
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: clientID must not be empty")
	}

	logger.L.Debug("requesting device code", "url", deviceCodeURL, "scope", requiredScope)

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", requiredScope)

	req, err := http.NewRequest(http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: failed to build device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.L.Error("device code request failed", "err", err)
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: device code request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logger.L.Debug("device code response", "status", resp.StatusCode)
	return decodeDeviceCodeResponse(resp)
}

// StartDeviceFlowWith initiates the device flow using a custom HTTPPoster.
// This variant is used in tests to inject a mock HTTP client.
// Note: tests supply JSON fixtures directly so no Accept header is needed.
func StartDeviceFlowWith(poster HTTPPoster, clientID string) (model.DeviceFlowInfo, error) {
	if clientID == "" {
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: clientID must not be empty")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", requiredScope)

	resp, err := poster.Post(deviceCodeURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: device code request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeDeviceCodeResponse(resp)
}

// decodeDeviceCodeResponse parses an *http.Response into a DeviceFlowInfo.
func decodeDeviceCodeResponse(resp *http.Response) (model.DeviceFlowInfo, error) {
	if resp.StatusCode != http.StatusOK {
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: unexpected status %d from device code endpoint", resp.StatusCode)
	}

	var dcr deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		logger.L.Error("failed to decode device code response", "err", err)
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: failed to decode device code response: %w", err)
	}

	if dcr.DeviceCode == "" || dcr.UserCode == "" {
		logger.L.Error("incomplete device code response", "device_code_empty", dcr.DeviceCode == "", "user_code_empty", dcr.UserCode == "")
		return model.DeviceFlowInfo{}, fmt.Errorf("auth: incomplete device code response from GitHub")
	}

	logger.L.Debug("device code response decoded",
		"user_code", dcr.UserCode,
		"expires_in", dcr.ExpiresIn,
		"interval", dcr.Interval,
	)

	return model.DeviceFlowInfo{
		DeviceCode:      dcr.DeviceCode,
		UserCode:        dcr.UserCode,
		VerificationURI: dcr.VerificationURI,
		ExpiresIn:       dcr.ExpiresIn,
		Interval:        dcr.Interval,
	}, nil
}
