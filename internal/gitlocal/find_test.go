package gitlocal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fakeClone(t *testing.T, dir, config string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ".git"), "config", config)
	return dir
}

func TestFindRepos_MixedLayouts_IndexesReachableClones(t *testing.T) {
	root := fakeClone(t, t.TempDir(), "[remote \"origin\"]\n\turl = https://github.com/me/projects\n")
	api := fakeClone(t, filepath.Join(root, "api"), "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = git@github.com:Acme/API.git\n")
	fork := fakeClone(t, filepath.Join(root, "me", "web"), "[remote \"origin\"]\n\turl = https://github.com/me/web\n[remote \"upstream\"]\n\turl = https://github.com/acme/web.git\n[submodule \"lib\"]\n\turl = https://github.com/acme/lib\n")
	deep := fakeClone(t, filepath.Join(root, "github.com", "acme", "cli"), "[remote \"origin\"]\n\turl = https://github.com/acme/cli\n")
	fakeClone(t, filepath.Join(root, "a", "b", "c", "too-deep"), "[remote \"origin\"]\n\turl = https://github.com/acme/deep\n")
	fakeClone(t, filepath.Join(root, "node_modules", "dep"), "[remote \"origin\"]\n\turl = https://github.com/acme/dep\n")
	fakeClone(t, filepath.Join(api, "nested"), "[remote \"origin\"]\n\turl = https://github.com/acme/nested\n")

	got := FindRepos([]string{root, filepath.Join(root, "missing")})

	want := map[string]string{"me/projects": root, "acme/api": api, "me/web": fork, "acme/web": fork, "acme/cli": deep}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestFindRepos_ForkAndDirectClone_PrefersOrigin(t *testing.T) {
	root := t.TempDir()
	fakeClone(t, filepath.Join(root, "a-fork"), "[remote \"origin\"]\n\turl = https://github.com/me/web\n[remote \"upstream\"]\n\turl = https://github.com/acme/web\n")
	direct := fakeClone(t, filepath.Join(root, "b-web"), "[remote \"origin\"]\n\turl = https://github.com/acme/web\n")

	if got := FindRepos([]string{root})["acme/web"]; got != direct {
		t.Fatalf("acme/web = %q, want %q", got, direct)
	}
}

func TestWorktreeFor_BranchCheckedOutInWorktree_ReturnsWorktree(t *testing.T) {
	ctx := context.Background()
	main := t.TempDir()
	run(t, main, "init", "-q", "-b", "main")
	write(t, main, "a.txt", "a\n")
	run(t, main, "add", ".")
	run(t, main, "commit", "-q", "-m", "init")
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, main, "worktree", "add", "-q", "-b", "feat/x", wt)
	main, _ = filepath.EvalSymlinks(main)
	wt, _ = filepath.EvalSymlinks(wt)

	if got := WorktreeFor(ctx, main, "feat/x"); got != wt {
		t.Errorf("feat/x: got %q, want %q", got, wt)
	}
	if got := WorktreeFor(ctx, wt, "other"); got != main {
		t.Errorf("other branch: got %q, want main %q", got, main)
	}
	if got := WorktreeFor(ctx, wt, ""); got != main {
		t.Errorf("no branch: got %q, want main %q", got, main)
	}
}
