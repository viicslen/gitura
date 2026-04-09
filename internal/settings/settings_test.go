package settings

import (
	"os"
	"path/filepath"
	"testing"

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

// TestLoad_MissingFile_ReturnsEmptyConfig verifies that a missing file produces
// a zero-value Config (empty ignored-commenters slice) and no error.
func TestLoad_MissingFile_ReturnsEmptyConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg.IgnoredCommenters)
	assert.Empty(t, cfg.IgnoredCommenters)
}

// TestSave_RoundTrip_PreservesAllFields verifies that saving and loading
// produces an identical Config.
func TestSave_RoundTrip_PreservesAllFields(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	input := Config{
		IgnoredCommenters: []model.IgnoredCommenterDTO{
			{Login: "alice"},
			{Login: "bob"},
		},
	}

	require.NoError(t, Save(input))

	got, err := Load()
	require.NoError(t, err)
	require.Len(t, got.IgnoredCommenters, 2)
	assert.Equal(t, "alice", got.IgnoredCommenters[0].Login)
	assert.Equal(t, "bob", got.IgnoredCommenters[1].Login)
}

// TestSave_EmptySlice_WritesEmptyConfig verifies that saving a Config with nil
// ignored commenters loads back as empty (not nil).
func TestSave_EmptySlice_WritesEmptyConfig(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	require.NoError(t, Save(Config{}))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got.IgnoredCommenters)
	assert.Empty(t, got.IgnoredCommenters)
}

// TestSave_CreatesDirIfAbsent verifies that Save creates the config directory
// when it does not exist yet.
func TestSave_CreatesDirIfAbsent(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	input := Config{
		IgnoredCommenters: []model.IgnoredCommenterDTO{
			{Login: "charlie"},
		},
	}
	require.NoError(t, Save(input))

	got, err := Load()
	require.NoError(t, err)
	require.Len(t, got.IgnoredCommenters, 1)
	assert.Equal(t, "charlie", got.IgnoredCommenters[0].Login)
}

// overrideConfigDir redirects the OS config dir to the given base during the test.
func overrideConfigDir(t *testing.T, base string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", base)
}

// TestLoad_MalformedTOML_ReturnsError verifies that a file with invalid TOML
// returns a parse error.
func TestLoad_MalformedTOML_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.toml"), []byte("[[invalid toml"), 0o600))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings: parse")
}

// TestSave_NilCommenters_WritesEmptySlice verifies that passing Config with nil
// IgnoredCommenters results in an empty (non-nil) slice when loaded back.
func TestSave_NilCommenters_WritesEmptySlice(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	require.NoError(t, Save(Config{IgnoredCommenters: nil}))

	got, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, got.IgnoredCommenters)
	assert.Empty(t, got.IgnoredCommenters)
}

// TestLoad_ReadError_ReturnsError verifies that a file that exists but cannot be
// read returns a non-nil error (simulated by making it a directory).
func TestLoad_ReadError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	// Create a directory where the file should be.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "settings.toml"), 0o700))

	_, err := Load()
	require.Error(t, err)
}

// TestConfigDir_ErrorWhenNoHome verifies ConfigDir returns an error when
// os.UserConfigDir() cannot determine the directory.
func TestConfigDir_ErrorWhenNoHome(t *testing.T) {
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
	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o500)) // r-x, no write
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	overrideConfigDir(t, tmp)

	err := Save(Config{IgnoredCommenters: []model.IgnoredCommenterDTO{{Login: "x"}}})
	require.Error(t, err)
}

// TestLoad_ConfigDirError_ReturnsError verifies Load propagates a ConfigDir error.
func TestLoad_ConfigDirError_ReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	_, err := Load()
	require.Error(t, err)
}

// TestSave_MkdirAllError_ReturnsError verifies that if the config dir cannot be
// created, an error is returned.
func TestSave_MkdirAllError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.Chmod(tmp, 0o500))
	t.Cleanup(func() { _ = os.Chmod(tmp, 0o700) })
	t.Setenv("XDG_CONFIG_HOME", tmp)

	err := Save(Config{IgnoredCommenters: []model.IgnoredCommenterDTO{{Login: "x"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settings:")
}

// TestSave_RenameError_ReturnsError verifies that if the final rename fails
// (e.g., target path is a directory), an error is returned.
func TestSave_RenameError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	overrideConfigDir(t, tmp)

	dir := filepath.Join(tmp, "gitura")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	// Create a directory where the file should be so rename fails.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "settings.toml"), 0o700))

	err := Save(Config{IgnoredCommenters: []model.IgnoredCommenterDTO{{Login: "x"}}})
	require.Error(t, err)
}
