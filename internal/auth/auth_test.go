package auth_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/auth"
)

// fixtureResponse creates an *http.Response from a fixture file.
func fixtureResponse(t *testing.T, statusCode int, fixturePath string) *http.Response {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "read fixture %s", fixturePath)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// errorPoster always returns a network error.
type errorPoster struct{ err error }

func (e *errorPoster) Post(_ string, _ string, _ io.Reader) (*http.Response, error) {
	return nil, e.err
}

// staticPoster returns a fixed response.
type staticPoster struct {
	resp *http.Response
}

func (s *staticPoster) Post(_ string, _ string, _ io.Reader) (*http.Response, error) {
	return s.resp, nil
}

// --- StartDeviceFlow tests ---

func TestStartDeviceFlow_Success_ReturnsDeviceFlowInfo(t *testing.T) {
	poster := &staticPoster{resp: fixtureResponse(t, 200, "../../tests/fixtures/auth/device_code_success.json")}
	info, err := auth.StartDeviceFlowWith(poster, "test-client-id")
	require.NoError(t, err)
	assert.Equal(t, "WDJB-MJHT", info.UserCode)
	assert.Equal(t, "https://github.com/login/device", info.VerificationURI)
	assert.Equal(t, 900, info.ExpiresIn)
	assert.Equal(t, 5, info.Interval)
	assert.NotEmpty(t, info.DeviceCode)
}

func TestStartDeviceFlow_EmptyClientID_ReturnsError(t *testing.T) {
	_, err := auth.StartDeviceFlowWith(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clientID")
}

func TestStartDeviceFlow_NetworkError_ReturnsError(t *testing.T) {
	poster := &errorPoster{err: io.ErrUnexpectedEOF}
	_, err := auth.StartDeviceFlowWith(poster, "test-client-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device code request failed")
}

func TestStartDeviceFlow_NonOKStatus_ReturnsError(t *testing.T) {
	poster := &staticPoster{resp: &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
	}}
	_, err := auth.StartDeviceFlowWith(poster, "test-client-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// --- PollForToken tests ---

func TestPollForToken_Complete_ReturnsTokenAndComplete(t *testing.T) {
	poster := &staticPoster{resp: fixtureResponse(t, 200, "../../tests/fixtures/auth/token_complete.json")}
	result, token, err := auth.PollForTokenWith(poster, "device-code-abc", "test-client-id")
	require.NoError(t, err)
	assert.Equal(t, "complete", result.Status)
	assert.NotEmpty(t, token)
}

func TestPollForToken_Pending_ReturnsPending(t *testing.T) {
	poster := &staticPoster{resp: fixtureResponse(t, 200, "../../tests/fixtures/auth/token_pending.json")}
	result, token, err := auth.PollForTokenWith(poster, "device-code-abc", "test-client-id")
	require.NoError(t, err)
	assert.Equal(t, "pending", result.Status)
	assert.Empty(t, token)
}

func TestPollForToken_Expired_ReturnsExpired(t *testing.T) {
	poster := &staticPoster{resp: fixtureResponse(t, 200, "../../tests/fixtures/auth/token_expired.json")}
	result, token, err := auth.PollForTokenWith(poster, "device-code-abc", "test-client-id")
	require.NoError(t, err)
	assert.Equal(t, "expired", result.Status)
	assert.Empty(t, token)
}

func TestPollForToken_NetworkError_ReturnsError(t *testing.T) {
	poster := &errorPoster{err: io.ErrUnexpectedEOF}
	result, _, err := auth.PollForTokenWith(poster, "device-code-abc", "test-client-id")
	require.Error(t, err)
	assert.Equal(t, "error", result.Status)
}

func TestPollForToken_MissingArgs_ReturnsError(t *testing.T) {
	_, _, err := auth.PollForTokenWith(nil, "", "test-client-id")
	require.Error(t, err)
}
