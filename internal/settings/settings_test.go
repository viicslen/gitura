package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kirsle/configdir"
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
	overrideConfigDir(t, t.TempDir()) // redirect config dir to tmp
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

// TestSave_EmptySlice_WritesTOML verifies that saving nil or empty writes
// a valid TOML file and loads back as empty (not nil).
func TestSave_EmptySlice_WritesTOML(t *testing.T) {
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
// On Linux XDG_CONFIG_HOME controls the configdir path; Refresh re-evaluates it.
// The initial Refresh picks up the new env var immediately; the cleanup Refresh
// restores the original path after the test finishes (when t.Setenv reverts the var).
func overrideConfigDir(t *testing.T, base string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", base)
	configdir.Refresh()
	t.Cleanup(func() { configdir.Refresh() })
}

// TestLoad_MalformedTOML_ReturnsError verifies that a file with invalid TOML
// returns a parse error.
func TestLoad_MalformedTOML_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	// Create the gitura directory and write invalid TOML.
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.toml"), []byte("{bad toml"), 0o600))

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
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "settings.toml"), 0o700))

	_, err := Load()
	require.Error(t, err)
}

// TestLoad_EmptyTOML_ReturnsEmptySlice verifies that an empty TOML file
// returns an empty (non-nil) slice rather than nil.
func TestLoad_EmptyTOML_ReturnsEmptySlice(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	// Write a valid TOML file with no entries.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.toml"), []byte(""), 0o600))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
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

// TestSave_MkdirAllError_ReturnsError verifies that if the config dir cannot be
// created (e.g., parent is not writable), Save returns a "settings: create config dir" error.
func TestSave_MkdirAllError_ReturnsError(t *testing.T) {
	// Make the XDG_CONFIG_HOME itself read-only so MkdirAll(dir/gitura) fails.
	tmp := t.TempDir()
	require.NoError(t, os.Chmod(tmp, 0o500))       // r-x, cannot create subdirs
	t.Cleanup(func() { _ = os.Chmod(tmp, 0o700) }) // restore so TempDir cleanup works
	t.Setenv("XDG_CONFIG_HOME", tmp)
	configdir.Refresh()
	t.Cleanup(func() { configdir.Refresh() })

	err := Save([]model.IgnoredCommenterDTO{{Login: "x", AddedAt: time.Now()}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings:")
}

// TestSave_RenameError_ReturnsError verifies that if the final rename fails
// (e.g., target path is a non-empty directory), an error is returned.
func TestSave_RenameError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	// Pre-create the gitura dir and the target path as a non-empty directory so
	// both os.Remove and os.Rename fail.
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	settingsPath := filepath.Join(dir, "settings.toml")
	require.NoError(t, os.MkdirAll(settingsPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(settingsPath, "dummy"), nil, 0o600))

	err := Save([]model.IgnoredCommenterDTO{{Login: "x", AddedAt: time.Now()}})
	require.Error(t, err)
}
