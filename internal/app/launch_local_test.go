package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func TestTakeLaunch_Local_OpensPROnlyWhenPushedAndOpen(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		untracked   bool
		wantPR      bool
	}{
		{"pushed with open PR", "open", false, true},
		{"untracked file", "open", true, false},
		{"closed PR", "closed", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			remote, dir := filepath.Join(tmp, "o", "r.git"), filepath.Join(tmp, "work")
			require.NoError(t, os.MkdirAll(remote, 0o755))
			require.NoError(t, os.MkdirAll(dir, 0o755))
			gitRun(t, remote, "init", "-q", "--bare", "-b", "main")
			gitRun(t, dir, "init", "-q", "-b", "feat")
			gitRun(t, dir, "remote", "add", "origin", remote)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644))
			gitRun(t, dir, "add", ".")
			gitRun(t, dir, "commit", "-q", "-m", "a")
			gitRun(t, dir, "push", "-q", "-u", "origin", "feat")
			if tc.untracked {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644))
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/o/r/pulls":
					assert.Equal(t, "o:feat", r.URL.Query().Get("head"))
					_, _ = w.Write([]byte(`[{"number": 7}]`))
				case "/repos/o/r/pulls/7":
					_, _ = fmt.Fprintf(w, `{"number": 7, "state": %q}`, tc.state)
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			base := srv.URL + "/"
			client, err := github.NewClient(github.WithURLs(&base, nil))
			require.NoError(t, err)

			a := New("")
			a.ctx, a.ghClient = context.Background(), client
			a.launch.Store(&launchTarget{dir: dir, local: true})
			launch, err := a.TakeLaunch()
			require.NoError(t, err)
			if tc.wantPR {
				require.NotNil(t, launch.PR)
				assert.Equal(t, 7, launch.PR.Number)
				assert.Empty(t, launch.LocalPath)
			} else {
				assert.Nil(t, launch.PR)
				assert.NotEmpty(t, launch.LocalPath)
			}
		})
	}
}
