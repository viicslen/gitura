// Package gitlocal reads diffs from a local git checkout so its changes can be reviewed like a PR.
// Errors are prefixed with "git:".
package gitlocal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitura/internal/model"
	"gitura/internal/unidiff"
)

const (
	// BaseMergeBase compares the working tree with the merge-base of HEAD and the default branch.
	BaseMergeBase = "merge-base"
	// BaseHEAD compares the working tree with HEAD: uncommitted changes only.
	BaseHEAD = "HEAD"
)

// File is one changed file; Patch is its raw diff, used to tell whether it changed since it was viewed.
type File struct {
	Diff  model.ParsedDiffDTO
	Patch string
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	// #nosec G204 -- fixed binary, no shell; callers pass literal flags, a resolved revision, and paths after "--".
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	// Polling `git status` must not take index.lock, or the user's own git commands would fail.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return string(out), fmt.Errorf("git: %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// TopLevel returns the root of the repository containing dir.
func TopLevel(ctx context.Context, dir string) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	return strings.TrimSpace(out), err
}

// ResolveBase turns BaseMergeBase or BaseHEAD into the revision to diff against.
func ResolveBase(ctx context.Context, dir, base string) (string, error) {
	switch base {
	case BaseHEAD:
		return "HEAD", nil
	case BaseMergeBase:
	default:
		return "", fmt.Errorf("validation: unknown base %q", base)
	}
	for _, ref := range []string{"origin/HEAD", "origin/main", "origin/master", "main", "master"} {
		if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
			continue
		}
		out, err := git(ctx, dir, "merge-base", "HEAD", ref)
		return strings.TrimSpace(out), err
	}
	return "", fmt.Errorf("git: no default branch (origin/HEAD, main or master) to compare against")
}

// Diff returns every file that differs between base and the working tree, untracked files included.
// dir must be the repository root so untracked paths line up with diff paths.
func Diff(ctx context.Context, dir, base string) ([]File, error) {
	rev, err := ResolveBase(ctx, dir, base)
	if err != nil {
		return nil, err
	}
	out, err := git(ctx, dir, "diff", "--no-color", "--no-ext-diff", "-M", rev, "--")
	if err != nil {
		return nil, err
	}
	files := splitPatch(out)

	untracked, err := git(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(untracked, "\x00") {
		if path == "" {
			continue
		}
		out, err := git(ctx, dir, "diff", "--no-color", "--no-ext-diff", "--no-index", "--", os.DevNull, path)
		// --no-index exits 1 whenever the files differ, which they always do here.
		var exitErr *exec.ExitError
		if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
			return nil, err
		}
		files = append(files, splitPatch(out)...)
	}
	return files, nil
}

func splitPatch(out string) []File {
	var files []File
	// Hunk body lines start with a space, + or -, so this header can't appear inside one.
	for _, chunk := range strings.Split("\n"+out, "\ndiff --git ")[1:] {
		files = append(files, parseFile("diff --git "+strings.TrimSuffix(chunk, "\n")))
	}
	return files
}

func parseFile(patch string) File {
	d := model.ParsedDiffDTO{Status: "modified"}
	lines := strings.Split(patch, "\n")
	// "diff --git a/P b/P": without a rename both halves are equal, so P is the second half.
	// shortcut: paths git C-quotes (tabs, newlines, quotes) come out quoted; fine until someone has such a file.
	if ab := strings.TrimPrefix(lines[0], "diff --git "); len(ab) >= 5 {
		d.Filename = ab[(len(ab)-5)/2+5:]
	}
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "@@") {
			break
		}
		applyHeaderLine(&d, line)
	}
	d.Hunks = unidiff.ParseHunks(patch)
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			switch l.Type {
			case model.DiffLineAdd:
				d.TotalAdditions++
			case model.DiffLineDelete:
				d.TotalDeletions++
			}
		}
	}
	return File{Diff: d, Patch: patch}
}

func applyHeaderLine(d *model.ParsedDiffDTO, line string) {
	switch {
	case strings.HasPrefix(line, "new file mode"):
		d.Status = "added"
	case strings.HasPrefix(line, "deleted file mode"):
		d.Status = "removed"
	case strings.HasPrefix(line, "rename from "):
		d.Status = "renamed"
		d.PreviousFilename = strings.TrimPrefix(line, "rename from ")
	case strings.HasPrefix(line, "rename to "):
		d.Filename = strings.TrimPrefix(line, "rename to ")
	case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
		d.IsBinary = true
	case strings.HasPrefix(line, "+++ b/"):
		// Git appends a tab to names containing spaces.
		d.Filename = strings.TrimSuffix(strings.TrimPrefix(line, "+++ b/"), "\t")
	case strings.HasPrefix(line, "--- a/") && d.Status == "removed":
		d.Filename = strings.TrimSuffix(strings.TrimPrefix(line, "--- a/"), "\t")
	}
}

// Fingerprint changes whenever the diff against any base could have changed: a commit, checkout,
// stage, or edit to a changed or untracked file. Cheaper than recomputing the diff.
func Fingerprint(ctx context.Context, dir string) (string, error) {
	head, _ := git(ctx, dir, "rev-parse", "HEAD")
	status, err := git(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(status)
	for _, entry := range strings.Split(status, "\x00") {
		// Rename entries are followed by a bare old path; stat'ing its tail fails the same way every time.
		if len(entry) < 4 {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, entry[3:])); err == nil {
			fmt.Fprintf(&b, "\x00%d:%d", info.ModTime().UnixNano(), info.Size())
		}
	}
	return b.String(), nil
}

// OriginRepo returns the GitHub owner and repository of dir's origin remote.
func OriginRepo(ctx context.Context, dir string) (owner, repo string, err error) {
	out, err := git(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	owner, repo, ok := ParseRemoteURL(strings.TrimSpace(out))
	if !ok {
		return "", "", fmt.Errorf("git: origin %q is not an owner/repo remote", strings.TrimSpace(out))
	}
	return owner, repo, nil
}

// ParseRemoteURL reads owner and repository from an SSH (git@host:o/r.git) or URL-style remote.
func ParseRemoteURL(url string) (owner, repo string, ok bool) {
	s := strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	i := strings.LastIndexAny(s, "/:")
	if i < 0 {
		return "", "", false
	}
	repo, s = s[i+1:], s[:i]
	owner = s[strings.LastIndexAny(s, "/:")+1:]
	return owner, repo, owner != "" && repo != ""
}
