package gitlocal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "keep.txt", "a\nb\nc\n")
	write(t, dir, "gone.txt", "x\n")
	write(t, dir, "old name.txt", "one\ntwo\nthree\nfour\nfive\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "init")

	run(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "committed.txt", "c\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "feature")

	write(t, dir, "keep.txt", "a\nB\nc\n")
	write(t, dir, "new file.txt", "1\n2\n")
	run(t, dir, "rm", "-q", "gone.txt")
	run(t, dir, "mv", "old name.txt", "new name.txt")

	files, err := Diff(ctx, dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]File{}
	for _, f := range files {
		got[f.Diff.Filename] = f
	}
	cases := []struct {
		name, status, prev string
		adds, dels         int
	}{
		{"keep.txt", "modified", "", 1, 1},
		{"gone.txt", "removed", "", 0, 1},
		{"new name.txt", "renamed", "old name.txt", 0, 0},
		{"new file.txt", "added", "", 2, 0},
	}
	for _, c := range cases {
		f, ok := got[c.name]
		if !ok {
			t.Errorf("%s missing; got %v", c.name, got)
			continue
		}
		d := f.Diff
		if d.Status != c.status || d.PreviousFilename != c.prev || d.TotalAdditions != c.adds || d.TotalDeletions != c.dels {
			t.Errorf("%s: got status=%s prev=%q +%d -%d", c.name, d.Status, d.PreviousFilename, d.TotalAdditions, d.TotalDeletions)
		}
	}
	if len(files) != len(cases) {
		t.Errorf("got %d files, want %d", len(files), len(cases))
	}
	if h := got["keep.txt"].Diff.Hunks; len(h) != 1 || h[0].Lines[len(h[0].Lines)-1].Content != "c" {
		t.Errorf("keep.txt hunks: %+v", h)
	}

	branch, err := Diff(ctx, dir, BaseMergeBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(branch) != len(cases)+1 {
		t.Errorf("merge-base diff should add committed.txt; got %d files", len(branch))
	}
}

func TestFingerprintChangesOnEdit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	write(t, dir, "f.txt", "a\n")
	before, err := Fingerprint(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "f.txt", "a\nb\n")
	after, _ := Fingerprint(ctx, dir)
	if before == after {
		t.Error("fingerprint did not change after editing an untracked file")
	}
}
