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
	"path"
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
	// Submodule is the submodule whose diff holds the file: the innermost one holding it, unless that one
	// replaced a directory its parent tracked, whose files stay the parent's. The fields below are set only on its
	// pointer row (Diff.Status "submodule"): SubmoduleRange is "old..new", new suffixed "-dirty" for a
	// dirty work tree; SubmoduleUnavailable says why the files inside are missing; SubmoduleIn is the
	// submodule holding it, "" for the top level.
	Submodule, SubmoduleRange, SubmoduleUnavailable, SubmoduleIn string
	SubmoduleBackwards, SubmoduleDiverged                        bool
	// own is set on a file a repository's own diff listing placed, which may share its path with a nested submodule's.
	own bool
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
	// Nested submodules are recursed into; one whose commits are missing prints "(diff failed)" and exits 0.
	out, err := git(ctx, dir, "diff", "--no-color", "--no-ext-diff", "--submodule=diff", "-M", rev, "--")
	if err != nil {
		return nil, err
	}
	files, headers := splitPatch(out)
	if files, err = withSubmodules(ctx, dir, rev, files, headers); err != nil {
		return nil, err
	}
	untracked, err := untrackedFiles(ctx, dir, "")
	return append(files, untracked...), err
}

// untrackedFiles diffs the untracked files in the repository at sub ("" for dir itself) against nothing.
func untrackedFiles(ctx context.Context, dir, sub string) ([]File, error) {
	untracked, err := git(ctx, filepath.Join(dir, sub), "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []File
	for p := range strings.SplitSeq(untracked, "\x00") {
		if p == "" {
			continue
		}
		out, err := git(ctx, dir, "diff", "--no-color", "--no-ext-diff", "--no-index", "--", os.DevNull, path.Join(sub, p))
		// --no-index exits 1 whenever the files differ, which they always do here.
		var exitErr *exec.ExitError
		if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
			return nil, err
		}
		f, _ := splitPatch(out)
		files = append(files, f...)
	}
	return files, nil
}

// splitPatch also returns the "Submodule ..." lines --submodule=diff puts before each submodule's
// files, so they don't trail onto the previous file's patch.
func splitPatch(out string) ([]File, []subHeader) {
	var files []File
	// Hunk body lines start with a space, + or -, so neither header can appear inside one.
	chunks := strings.Split("\n"+out, "\ndiff --git ")
	headers := collectHeaders(chunks[0], 0)
	for _, chunk := range chunks[1:] {
		patch, tail, _ := strings.Cut("diff --git "+strings.TrimSuffix(chunk, "\n"), "\nSubmodule ")
		files = append(files, parseFile(patch))
		headers = append(headers, collectHeaders("Submodule "+tail, len(files))...)
	}
	return files, headers
}

func parseFile(patch string) File {
	d := model.ParsedDiffDTO{Status: "modified"}
	lines := strings.Split(patch, "\n")
	// "diff --git a/P b/P": without a rename both halves are equal, so P is the second half.
	// shortcut: paths git C-quotes (tabs, newlines, quotes) come out quoted; fine until someone has such a file.
	ab := strings.TrimPrefix(lines[0], "diff --git ")
	if len(ab) >= 5 {
		d.Filename = ab[(len(ab)-5)/2+5:]
	}
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "@@") {
			break
		}
		applyHeaderLine(&d, line)
	}
	if d.Status == "renamed" {
		prefixRename(&d, ab)
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

// prefixRename restores the submodule path that --submodule=diff puts on "diff --git a/<pre><from> b/<pre><to>"
// but leaves off the "rename from"/"rename to" lines.
func prefixRename(d *model.ParsedDiffDTO, ab string) {
	rest := strings.TrimPrefix(ab, "a/")
	pre, to, ok := strings.Cut(rest, d.PreviousFilename+" b/")
	if ok && (pre == "" || strings.HasSuffix(pre, "/")) && strings.HasPrefix(to, pre) {
		d.Filename, d.PreviousFilename = to, pre+d.PreviousFilename
	}
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
	// -c, unlike --untracked-files, also reaches the status git runs inside each submodule.
	status, err := git(ctx, dir, "-c", "status.showUntrackedFiles=all", "status", "--porcelain=v1", "-z")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(status)
	for entry := range strings.SplitSeq(status, "\x00") {
		// Rename entries are followed by a bare old path; stat'ing its tail fails the same way every time.
		if len(entry) < 4 {
			continue
		}
		p := filepath.Join(dir, entry[3:])
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\x00%d:%d", info.ModTime().UnixNano(), info.Size())
		// With status.showUntrackedFiles=all a directory entry is a submodule (or embedded repo), whose status
		// line stays " M" however much more changes inside it.
		if info.IsDir() {
			if sub, err := Fingerprint(ctx, p); err == nil {
				b.WriteString(sub)
			}
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

// PushedBranch returns the origin branch HEAD tracks when the working tree has no changes
// (untracked files included) and no commits that aren't on that branch; ok is false otherwise.
func PushedBranch(ctx context.Context, dir string) (branch string, ok bool) {
	upstream, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "@{upstream}")
	branch, ok = strings.CutPrefix(strings.TrimSpace(upstream), "origin/")
	if err != nil || !ok {
		return "", false
	}
	status, err := git(ctx, dir, "status", "--porcelain")
	if err != nil || status != "" {
		return "", false
	}
	ahead, err := git(ctx, dir, "rev-list", "--count", "@{upstream}..HEAD")
	if err != nil || strings.TrimSpace(ahead) != "0" {
		return "", false
	}
	return branch, true
}
