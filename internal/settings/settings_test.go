package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

// TestConfigDir_ReturnsNonEmptyPath verifies ConfigDir returns a non-empty OS path.
func TestConfigDir_ReturnsNonEmptyPath(t *testing.T) {
	dir, err := ConfigDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)
	assert.Equal(t, "gitura", filepath.Base(dir))
}

// TestLoad_MissingFile_ReturnsEmptySlice verifies that a missing file produces
// an empty (non-nil) slice and no error.
func TestLoad_MissingFile_ReturnsEmptySlice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // redirect config dir to tmp
	commenters, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, commenters)
	assert.Empty(t, commenters)
}

// TestSave_RoundTrip_PreservesAllFields verifies that saving and loading
// produces an identical slice.
func TestSave_RoundTrip_PreservesAllFields(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	input := []model.IgnoredCommenterDTO{
		{Login: "alice", AddedAt: now},
		{Login: "bob", AddedAt: now.Add(24 * time.Hour)},
	}

	require.NoError(t, Save(input))

	got, err := Load()
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "alice", got[0].Login)
	assert.Equal(t, "bob", got[1].Login)
	assert.True(t, got[0].AddedAt.Equal(now))
}

// TestSave_EmptySlice_WritesEmptyJSON verifies that saving nil or empty writes
// "[]" and loads back as empty (not nil).
func TestSave_EmptySlice_WritesEmptyJSON(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	require.NoError(t, Save(nil))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// TestSave_CreatesDirIfAbsent verifies that Save creates the config directory
// when it does not exist yet.
func TestSave_CreatesDirIfAbsent(t *testing.T) {
	tmp := t.TempDir()
	// Point HOME to the tmp dir so config dir is tmp/gitura (doesn't exist yet).
	overrideConfigDir(t, tmp)

	err := Save([]model.IgnoredCommenterDTO{{Login: "charlie", AddedAt: time.Now()}})
	require.NoError(t, err)

	got, err := Load()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "charlie", got[0].Login)
}

// overrideConfigDir redirects the OS config dir to the given base during the test.
// On Linux HOME controls os.UserConfigDir via XDG_CONFIG_HOME fallback.
func overrideConfigDir(t *testing.T, base string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", base)
}

// TestLoad_MalformedJSON_ReturnsError verifies that a file with invalid JSON
// returns a parse error.
func TestLoad_MalformedJSON_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	// Create the gitura directory and write invalid JSON.
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored_commenters.json"), []byte("{bad json"), 0o600))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings: parse")
}

// TestSave_NilSlice_WritesEmptySlice verifies that passing nil to Save results
// in an empty (non-nil) slice when loaded back.
func TestSave_NilSlice_WritesEmptySlice(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	require.NoError(t, Save(nil))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// TestLoad_ReadError_ReturnsError verifies that a file that exists but cannot be
// read returns a non-nil error (simulated by making it a directory).
func TestLoad_ReadError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	// Create the gitura config directory and a subdirectory named after the file
	// so os.ReadFile fails (it's a directory, not a file).
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	// Create a directory where the file should be.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ignored_commenters.json"), 0o700))

	_, err := Load()
	require.Error(t, err)
}

// TestConfigDir_ErrorWhenNoHome verifies ConfigDir returns an error when
// os.UserConfigDir() cannot determine the directory.
func TestConfigDir_ErrorWhenNoHome(t *testing.T) {
	// Unset all env vars that os.UserConfigDir() uses on Linux.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	_, err := ConfigDir()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings:")
}

// TestSave_WriteError_ReturnsError verifies that if the temp file cannot be
// written (e.g., config dir is read-only), an error is returned.
func TestSave_WriteError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	// Create the gitura directory as read-only so WriteFile fails.
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o500)) // r-x, no write
	overrideConfigDir(t, tmp)

	err := Save([]model.IgnoredCommenterDTO{{Login: "x", AddedAt: time.Now()}})
	require.Error(t, err)
}

// TestLoad_ConfigDirError_ReturnsError verifies Load propagates a ConfigDir error.
func TestLoad_ConfigDirError_ReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	_, err := Load()
	require.Error(t, err)
}

// TestLoad_JSONNull_ReturnsEmptySlice verifies that a file containing JSON null
// returns an empty (non-nil) slice rather than nil.
func TestLoad_JSONNull_ReturnsEmptySlice(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored_commenters.json"), []byte("null"), 0o600))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// not writable, Save returns a "settings: create config dir" error.
func TestSave_MkdirAllError_ReturnsError(t *testing.T) {
	// Make the XDG_CONFIG_HOME itself read-only so MkdirAll(dir/gitura) fails.
	tmp := t.TempDir()
	require.NoError(t, os.Chmod(tmp, 0o500))       // r-x, cannot create subdirs
	t.Cleanup(func() { _ = os.Chmod(tmp, 0o700) }) // restore so TempDir cleanup works
	t.Setenv("XDG_CONFIG_HOME", tmp)

	err := Save([]model.IgnoredCommenterDTO{{Login: "x", AddedAt: time.Now()}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings:")
}

// TestSave_RenameError_ReturnsError verifies that if the final rename fails
// (e.g., target path is a directory), an error is returned.
func TestSave_RenameError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	// Pre-create the gitura dir and the target path as a directory so rename fails.
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ignored_commenters.json"), 0o700))

	err := Save([]model.IgnoredCommenterDTO{{Login: "x", AddedAt: time.Now()}})
	require.Error(t, err)
}
