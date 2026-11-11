package app

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestParseArgs_FlagsAndRef_ReturnsTarget(t *testing.T) {
	la, err := parseArgs([]string{"--new-window", "--repo", "o/r", "my-branch"}, "/w", io.Discard)
	require.NoError(t, err)
	assert.True(t, la.newWindow)
	assert.Equal(t, &launchTarget{dir: "/w", repo: "o/r", ref: "my-branch"}, la.target)
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
		opts, a := &options.App{}, New("")
		require.NoError(t, ConfigureLaunch(opts, a, tc.args, ".", io.Discard))
		assert.Equal(t, tc.lock, opts.SingleInstanceLock != nil, tc.args)
		assert.Equal(t, "12", a.launch.Load().ref)
	}
}

func TestConfigureLaunch_UnknownFlag_ReturnsError(t *testing.T) {
	assert.Error(t, ConfigureLaunch(&options.App{}, New(""), []string{"--bogus"}, ".", io.Discard))
}
