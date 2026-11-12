package app

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestParseArgs_FlagsAndRef_ReturnsTarget(t *testing.T) {
	la, err := parseArgs([]string{"--new-window", "--repo", "o/r", "my-branch"}, "/w", io.Discard)
	require.NoError(t, err)
	assert.True(t, la.newWindow)
	assert.Equal(t, &launchTarget{dir: "/w", repo: "o/r", ref: "my-branch"}, la.target)
}

func TestParseArgs_Local_ReturnsLocalTarget(t *testing.T) {
	for _, tc := range []struct {
		args []string
		ref  string
	}{{[]string{"--local"}, ""}, {[]string{"--local", "../repo"}, "../repo"}} {
		la, err := parseArgs(tc.args, "/w", io.Discard)
		require.NoError(t, err)
		assert.Equal(t, &launchTarget{dir: "/w", ref: tc.ref, local: true}, la.target, tc.args)
	}
}

func TestParseArgs_LocalWithRepo_ReturnsError(t *testing.T) {
	_, err := parseArgs([]string{"--local", "--repo", "o/r"}, "/w", io.Discard)
	assert.Error(t, err)
}

func TestParseArgs_NoArgs_ReturnsNoTarget(t *testing.T) {
	la, err := parseArgs(nil, "/w", io.Discard)
	require.NoError(t, err)
	assert.Nil(t, la.target)
}

func TestConfigureLaunch_NewWindow_SkipsSingleInstanceLock(t *testing.T) {
	for _, tc := range []struct {
		args []string
		lock bool
	}{{[]string{"12"}, true}, {[]string{"--new-window", "12"}, false}} {
		opts, a := &application.Options{}, New("")
		require.NoError(t, ConfigureLaunch(opts, a, tc.args, ".", io.Discard))
		assert.Equal(t, tc.lock, opts.SingleInstance != nil, tc.args)
		assert.Equal(t, "12", a.launch.Load().ref)
	}
}

func TestSecondInstance_FullOSArgs_SkipsExecutable(t *testing.T) {
	a := New("")
	a.secondInstance(application.SecondInstanceData{
		Args:       []string{"/usr/bin/gitura", "--repo", "o/r", "12"},
		WorkingDir: "/w",
	})
	assert.Equal(t, &launchTarget{dir: "/w", repo: "o/r", ref: "12"}, a.launch.Load())
}

func TestConfigureLaunch_UnknownFlag_ReturnsError(t *testing.T) {
	assert.Error(t, ConfigureLaunch(&application.Options{}, New(""), []string{"--bogus"}, ".", io.Discard))
}
