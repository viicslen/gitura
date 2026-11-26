package ghapi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/aymanbagabas/go-udiff"
	"github.com/google/go-github/v93/github"

	"gitura/internal/gitlocal"
)

// compareFileCap is the most files GitHub's compare API returns; it drops the rest without saying so.
const compareFileCap = 300

var errNoAccess = errors.New("no access")

// ErrNotSubmodule is returned for a path that holds no submodule, such as a text file whose patch only reads
// like a pointer change.
var ErrNotSubmodule = errors.New("not a submodule")

// SubmoduleChanges is what a PR's bump of one submodule changes inside it.
type SubmoduleChanges struct {
	// Owner and Repo name the submodule's GitHub repository; empty when it isn't on GitHub.
	Owner, Repo string
	// Files are the changed files, old→new even for a backwards bump, renamed to sit under the submodule path.
	Files []*github.CommitFile
	// Backwards is set when the new commit is an ancestor of the old one.
	Backwards bool
	// Diverged is set when neither commit is an ancestor of the other.
	Diverged bool
	// Unavailable says why Files is empty or incomplete, e.g. "no access".
	Unavailable string
	// TreeRef is the commit whose whole tree Files lists, for an added or removed submodule: their
	// patches are left out, for FillWholeFile to load from there.
	TreeRef string
	// OldSHAs holds, by file name, the old blob SHA of each file without a patch; its SHA is the new blob's.
	OldSHAs map[string]string
	// BlobsTruncated is set when the tree OldSHAs come from was truncated, so some may be missing.
	BlobsTruncated bool
}

// SubmoduleBump reads the old and new commits from the "Subproject commit" patch GitHub gives a
// submodule pointer change. An added or removed submodule has an empty old or new.
func SubmoduleBump(f *github.CommitFile) (from, to string, ok bool) {
	for line := range strings.SplitSeq(f.GetPatch(), "\n") {
		if sha, found := strings.CutPrefix(line, "-Subproject commit "); found {
			from, ok = sha, true
		} else if sha, found := strings.CutPrefix(line, "+Subproject commit "); found {
			to, ok = sha, true
		} else if !strings.HasPrefix(line, "@@") {
			return "", "", false
		}
	}
	return from, to, ok
}

// FetchSubmoduleChanges returns the files changed inside the submodule at path between commits
// from and to; an added submodule (from "") lists to's files as added, a removed one (to "") from's
// as removed. ref is a commit of owner/repo where the submodule exists. Expected gaps (no access,
// not on GitHub, too many files) are reported in Unavailable, not as errors.
// Errors are prefixed "github:".
func FetchSubmoduleChanges(ctx context.Context, client *github.Client, owner, repo, ref, path, from, to string) (SubmoduleChanges, error) {
	var ch SubmoduleChanges
	var err error
	if ch.Owner, ch.Repo, err = submoduleRepo(ctx, client, owner, repo, ref, path); err != nil {
		return ch, err
	}
	if ch.Owner == "" {
		ch.Unavailable = "not on GitHub"
		return ch, nil
	}
	switch {
	case from == "":
		ch.TreeRef = to
		err = treeFiles(ctx, client, &ch, to, "added")
	case to == "":
		ch.TreeRef = from
		err = treeFiles(ctx, client, &ch, from, "removed")
	default:
		err = compareSubmodule(ctx, client, &ch, from, to)
	}
	if errors.Is(err, errNoAccess) {
		ch.Unavailable = "no access"
		return ch, nil
	}
	if err != nil {
		return ch, err
	}
	ch.MoveUnder(path)
	return ch, nil
}

// MoveUnder renames ch's files to sit under directory path.
func (ch *SubmoduleChanges) MoveUnder(path string) {
	for _, f := range ch.Files {
		f.Filename = new(path + "/" + f.GetFilename())
		if f.PreviousFilename != nil {
			f.PreviousFilename = new(path + "/" + f.GetPreviousFilename())
		}
	}
	old := ch.OldSHAs
	ch.OldSHAs = make(map[string]string, len(old))
	for name, sha := range old {
		ch.OldSHAs[path+"/"+name] = sha
	}
}

// treeFiles fills ch.Files with the files of commit sha, all given status and without patches.
// shortcut: a nested submodule in the tree is left out, not listed as added or removed; list its tree too if needed.
func treeFiles(ctx context.Context, client *github.Client, ch *SubmoduleChanges, sha, status string) error {
	t, err := tree(ctx, client, ch, sha)
	if err != nil {
		return err
	}
	var files []*github.CommitFile
	for _, e := range t.Entries {
		if e.GetType() != "blob" {
			continue
		}
		if len(files) == compareFileCap {
			t.Truncated = new(true)
			break
		}
		files = append(files, &github.CommitFile{Filename: e.Path, SHA: e.SHA, Status: new(status)})
	}
	if t.GetTruncated() {
		ch.Unavailable = "too many files"
	}
	ch.Files = files
	return nil
}

// tree returns commit sha's recursive tree.
func tree(ctx context.Context, client *github.Client, ch *SubmoduleChanges, sha string) (*github.Tree, error) {
	t, resp, err := client.Git.GetTree(ctx, ch.Owner, ch.Repo, sha, true)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, errNoAccess
	}
	if err != nil {
		return nil, fmt.Errorf("github: tree %s/%s@%s: %w", ch.Owner, ch.Repo, sha, err)
	}
	return t, nil
}

// blobsAt maps every non-directory path at commit sha to its tree entry; complete is false when
// GitHub truncated the tree.
func blobsAt(ctx context.Context, client *github.Client, ch *SubmoduleChanges, sha string) (blobs map[string]*github.TreeEntry, complete bool, err error) {
	t, err := tree(ctx, client, ch, sha)
	if err != nil {
		return nil, false, err
	}
	blobs = make(map[string]*github.TreeEntry, len(t.Entries))
	for _, e := range t.Entries {
		if e.GetType() != "tree" {
			blobs[e.GetPath()] = e
		}
	}
	return blobs, !t.GetTruncated(), nil
}

// FillWholeFile gives f, a file of an added or removed submodule listed without its patch, the
// all-added or all-deleted patch of blob sha of owner/repo. A binary file, or one over 1 MB, gets a
// change count instead, so it reads as binary.
// Errors are prefixed "github:".
func FillWholeFile(ctx context.Context, client *github.Client, owner, repo, sha string, f *github.CommitFile) error {
	text, ok, err := blobText(ctx, client, owner, repo, sha)
	switch {
	case err != nil:
		return err
	case !ok:
		f.Changes = new(1)
		return nil
	case f.GetStatus() == "removed":
		return setPatch(f, text, "")
	}
	return setPatch(f, "", text)
}

// compareSubmodule fills ch.Files, ch.Backwards and ch.Diverged from the comparison of from with to.
func compareSubmodule(ctx context.Context, client *github.Client, ch *SubmoduleChanges, from, to string) error {
	c, err := compare(ctx, client, ch.Owner, ch.Repo, from, to)
	if err != nil {
		return err
	}
	files := capFiles(ch, c.Files)
	switch c.GetStatus() {
	case "behind":
		// compare(old...new) lists nothing for a rewind; new...old is the same change, reversed.
		ch.Backwards = true
		files, err = reversedFiles(ctx, client, ch, from, to)
	case "diverged":
		ch.Diverged = true
		files, err = divergedFiles(ctx, client, ch, files, from, to)
	}
	if err == nil && !ch.Diverged {
		err = recordBlobs(ctx, client, ch, files, from, to)
	}
	if err != nil {
		return err
	}
	ch.Files = files
	return nil
}

// recordBlobs fills ch.OldSHAs for the files of a one-way bump that have no patch, from one tree:
// from's, or to's for a backwards bump, whose inverted files carry the old blob and get the new one.
// shortcut: a path missing from a truncated tree gets an empty blob, so a change on that side goes unseen; fetch the blob by path if that matters.
func recordBlobs(ctx context.Context, client *github.Client, ch *SubmoduleChanges, files []*github.CommitFile, from, to string) error {
	if !slices.ContainsFunc(files, func(f *github.CommitFile) bool { return f.GetPatch() == "" }) {
		return nil
	}
	ref := from
	if ch.Backwards {
		ref = to
	}
	blobs, complete, err := blobsAt(ctx, client, ch, ref)
	if err != nil {
		return err
	}
	ch.BlobsTruncated = !complete
	ch.OldSHAs = map[string]string{}
	for _, f := range files {
		switch {
		case f.GetPatch() != "":
		case !ch.Backwards:
			ch.OldSHAs[f.GetFilename()] = blobs[cmp.Or(f.GetPreviousFilename(), f.GetFilename())].GetSHA()
		default:
			if f.GetStatus() != "added" {
				ch.OldSHAs[f.GetFilename()] = f.GetSHA()
			}
			f.SHA = new(blobs[f.GetFilename()].GetSHA())
		}
	}
	return nil
}

// capFiles returns files, noting in ch when the compare API cut the list short.
func capFiles(ch *SubmoduleChanges, files []*github.CommitFile) []*github.CommitFile {
	if len(files) >= compareFileCap {
		ch.Unavailable = "too many files"
	}
	return files
}

// reversedFiles returns compare(to...from)'s files turned to read from→to.
func reversedFiles(ctx context.Context, client *github.Client, ch *SubmoduleChanges, from, to string) ([]*github.CommitFile, error) {
	c, err := compare(ctx, client, ch.Owner, ch.Repo, to, from)
	if err != nil {
		return nil, err
	}
	files := capFiles(ch, c.Files)
	for _, f := range files {
		InvertCommitFile(f)
	}
	return files, nil
}

// divergedFiles returns the from→to diff of a bump between commits on two branches. compare(from...to)
// gives newSide (merge-base→to) and to...from inverted gives from→merge-base; each path either touches
// is added, removed or modified by whether it exists at from and at to, without rename detection.
// shortcut: a truncated tree of either commit leaves the files out as "too many files"; check existence per path if that bites.
func divergedFiles(ctx context.Context, client *github.Client, ch *SubmoduleChanges, newSide []*github.CommitFile, from, to string) ([]*github.CommitFile, error) {
	oldSide, err := reversedFiles(ctx, client, ch, from, to)
	if err != nil {
		return nil, err
	}
	oldBlobs, oldComplete, err := blobsAt(ctx, client, ch, from)
	if err != nil {
		return nil, err
	}
	newBlobs, newComplete, err := blobsAt(ctx, client, ch, to)
	if err != nil {
		return nil, err
	}
	if !oldComplete || !newComplete {
		ch.Unavailable = "too many files"
		return nil, nil
	}
	b := divergedBump{ch: ch, old: oldBlobs, new: newBlobs, touched: map[string]*touch{}}
	b.add(oldSide, 1)
	b.add(newSide, 2)
	paths := slices.Sorted(maps.Keys(b.touched))
	ch.OldSHAs = map[string]string{}
	files := make([]*github.CommitFile, 0, len(paths))
	for _, p := range paths {
		fs, err := b.entries(ctx, client, p)
		if err != nil {
			return nil, err
		}
		files = append(files, fs...)
	}
	return files, nil
}

// divergedBump holds what divergedFiles builds each path's entry from.
type divergedBump struct {
	ch *SubmoduleChanges
	// old and new map the paths at from and to to their tree entries.
	old, new map[string]*github.TreeEntry
	touched  map[string]*touch
}

// touch is which sides (1 old, 2 new) changed a path, whether by a rename, and the last entry naming it.
type touch struct {
	sides   int
	renamed bool
	f       *github.CommitFile
}

func (b *divergedBump) add(files []*github.CommitFile, side int) {
	for _, f := range files {
		for _, p := range []string{f.GetFilename(), f.GetPreviousFilename()} {
			if p == "" {
				continue
			}
			t := b.touched[p]
			if t == nil {
				t = &touch{}
				b.touched[p] = t
			}
			t.sides, t.renamed, t.f = t.sides|side, t.renamed || f.GetStatus() == "renamed", f
		}
	}
}

// entries are path's from→to entries: a gitlink at one end and a file at the other lists as a pointer entry and a
// file entry, as git lists a type change.
func (b *divergedBump) entries(ctx context.Context, client *github.Client, path string) ([]*github.CommitFile, error) {
	if oldE, newE := b.old[path], b.new[path]; oldE != nil && newE != nil && isGitlink(oldE) != isGitlink(newE) {
		return b.typeChange(ctx, client, path)
	}
	f, err := b.file(ctx, client, path)
	if f == nil || err != nil {
		return nil, err
	}
	return []*github.CommitFile{f}, nil
}

// typeChange is the pointer entry and the file entry of path, a gitlink at one end and a file at the other.
func (b *divergedBump) typeChange(ctx context.Context, client *github.Client, path string) ([]*github.CommitFile, error) {
	oldE, newE := b.old[path], b.new[path]
	ptr := &github.CommitFile{Filename: new(path), Status: new("removed")}
	file := &github.CommitFile{Filename: new(path), Status: new("added"), SHA: newE.SHA}
	oldPtr, newPtr, blob, oldBlob := pointerText(oldE.GetSHA()), "", newE.GetSHA(), ""
	if isGitlink(newE) {
		ptr.Status, ptr.SHA, file.Status, file.SHA = new("added"), newE.SHA, new("removed"), nil
		oldPtr, newPtr, blob, oldBlob = "", pointerText(newE.GetSHA()), oldE.GetSHA(), oldE.GetSHA()
	}
	if err := setPatch(ptr, oldPtr, newPtr); err != nil {
		return nil, err
	}
	if err := FillWholeFile(ctx, client, b.ch.Owner, b.ch.Repo, blob, file); err != nil {
		return nil, err
	}
	if file.GetPatch() == "" {
		b.ch.OldSHAs[path] = oldBlob
	}
	return []*github.CommitFile{ptr, file}, nil
}

// file is path's from→to entry, nil when its blob and mode are the same at both ends. A path one side alone changed,
// not by a rename, keeps that side's patch; any other is diffed from its contents.
func (b *divergedBump) file(ctx context.Context, client *github.Client, path string) (*github.CommitFile, error) {
	oldE, atOld := b.old[path]
	newE, atNew := b.new[path]
	oldBlob, newBlob := oldE.GetSHA(), newE.GetSHA()
	if oldBlob == newBlob && oldE.GetMode() == newE.GetMode() {
		return nil, nil
	}
	status := "modified"
	switch {
	case !atOld:
		status = "added"
	case !atNew:
		status = "removed"
	}
	f := &github.CommitFile{Filename: &path}
	if t := b.touched[path]; t.sides != 3 && !t.renamed {
		f = t.f
	} else if err := b.diff(ctx, client, f); err != nil {
		return nil, err
	}
	f.Status, f.SHA = &status, &newBlob
	if f.GetPatch() == "" {
		b.ch.OldSHAs[path] = oldBlob
	}
	return f, nil
}

// isGitlink reports whether tree entry e is a submodule pointer, or absent (nil).
func isGitlink(e *github.TreeEntry) bool {
	return e == nil || e.GetType() == "commit"
}

// pointerText is a gitlink's content as git diffs it, "" for none.
func pointerText(sha string) string {
	if sha == "" {
		return ""
	}
	return "Subproject commit " + sha + "\n"
}

// diff gives f the patch between its contents at from and at to, or a change count when either is binary.
// A gitlink diffs as its pointer text, so a bump inside expands.
func (b *divergedBump) diff(ctx context.Context, client *github.Client, f *github.CommitFile) error {
	oldText, oldOK, err := b.text(ctx, client, b.old[f.GetFilename()])
	if err != nil {
		return err
	}
	newText, newOK, err := b.text(ctx, client, b.new[f.GetFilename()])
	if err != nil {
		return err
	}
	if !oldOK || !newOK {
		// No patch with a non-zero change count reads as binary.
		f.Changes = new(1)
		return nil
	}
	return setPatch(f, oldText, newText)
}

// text is tree entry e's content: a gitlink's pointer text, "" when e is nil. ok is false when it is binary
// or too large.
func (b *divergedBump) text(ctx context.Context, client *github.Client, e *github.TreeEntry) (string, bool, error) {
	if isGitlink(e) {
		return pointerText(e.GetSHA()), true, nil
	}
	return blobText(ctx, client, b.ch.Owner, b.ch.Repo, e.GetSHA())
}

// maxBlobText is the largest file diffed by content; a larger one reads as binary.
const maxBlobText = 1 << 20

// blobText returns the content of blob sha of owner/repo: for a symlink its target, as git diffs it. ok is false
// when it is binary or over maxBlobText.
func blobText(ctx context.Context, client *github.Client, owner, repo, sha string) (text string, ok bool, err error) {
	raw, _, err := client.Git.GetBlobRaw(ctx, owner, repo, sha)
	if err != nil {
		return "", false, fmt.Errorf("github: get blob %s/%s %s: %w", owner, repo, sha, err)
	}
	text = string(raw)
	return text, len(raw) <= maxBlobText && utf8.ValidString(text) && !strings.ContainsRune(text, 0), nil
}

// setPatch gives f the unified diff from oldText to newText in GitHub's patch form, with its counts.
func setPatch(f *github.CommitFile, oldText, newText string) error {
	u, err := udiff.ToUnifiedDiff("a", "b", oldText, udiff.Strings(oldText, newText), udiff.DefaultContextLines)
	if err != nil {
		return fmt.Errorf("diff %s: %w", f.GetFilename(), err)
	}
	adds, dels := 0, 0
	for _, h := range u.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case udiff.Insert:
				adds++
			case udiff.Delete:
				dels++
			}
		}
	}
	// GitHub's patch starts at the first hunk and has no trailing newline.
	patch := strings.TrimSuffix(strings.TrimPrefix(u.String(), "--- a\n+++ b\n"), "\n")
	f.Patch, f.Additions, f.Deletions, f.Changes = &patch, new(adds), new(dels), new(adds+dels)
	return nil
}

func compare(ctx context.Context, client *github.Client, owner, repo, base, head string) (*github.CommitsComparison, error) {
	c, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, nil)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, errNoAccess
	}
	if err != nil {
		return nil, fmt.Errorf("github: compare %s/%s %s...%s: %w", owner, repo, base, head, err)
	}
	return c, nil
}

// submoduleRepo returns the GitHub repository of the submodule at path, or empty names when it isn't on GitHub.
func submoduleRepo(ctx context.Context, client *github.Client, owner, repo, ref, path string) (subOwner, subRepo string, err error) {
	entry, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", "", fmt.Errorf("github: get submodule %s: %w", path, err)
	}
	if entry.GetType() != "submodule" {
		return "", "", fmt.Errorf("github: %s at %s is a %s: %w", path, ref, entry.GetType(), ErrNotSubmodule)
	}
	// GitHub resolves relative .gitmodules URLs (../lib.git) into html_url, and leaves it empty off GitHub.
	subOwner, subRepo, ok := gitlocal.ParseRemoteURL(strings.Split(entry.GetHTMLURL(), "/tree/")[0])
	if !ok {
		return "", "", nil
	}
	return subOwner, subRepo, nil
}

var hunkHeader = regexp.MustCompile(`^@@ -(\S+) \+(\S+) @@`)

// InvertCommitFile turns f, a diff from new to old, into the diff from old to new: lines, line
// numbers, counts, status and rename direction swap, and removals stay ahead of additions.
func InvertCommitFile(f *github.CommitFile) {
	f.Additions, f.Deletions = f.Deletions, f.Additions
	switch f.GetStatus() {
	case "added":
		f.Status = new("removed")
	case "removed":
		f.Status = new("added")
	case "renamed":
		f.Filename, f.PreviousFilename = f.PreviousFilename, f.Filename
	}
	if f.Patch != nil {
		f.Patch = new(invertPatch(f.GetPatch()))
	}
}

func invertPatch(patch string) string {
	lines := strings.Split(patch, "\n")
	out := make([]string, 0, len(lines))
	var dels, adds []string
	last := &dels
	flush := func() {
		out = append(append(out, dels...), adds...)
		dels, adds = nil, nil
	}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "+"):
			dels, last = append(dels, "-"+l[1:]), &dels
		case strings.HasPrefix(l, "-"):
			adds, last = append(adds, "+"+l[1:]), &adds
		case strings.HasPrefix(l, `\`) && len(*last) > 0:
			// "\ No newline at end of file" belongs to the line before it.
			*last = append(*last, l)
		default:
			flush()
			out = append(out, hunkHeader.ReplaceAllString(l, "@@ -$2 +$1 @@"))
		}
	}
	flush()
	return strings.Join(out, "\n")
}

// FindOpenPRForCommit returns the open PR in owner/repo whose head is exactly sha, or nil if none.
// Errors are prefixed "github:".
func FindOpenPRForCommit(ctx context.Context, client *github.Client, owner, repo, sha string) (*github.PullRequest, error) {
	prs, _, err := client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, fmt.Errorf("github: list PRs for %s/%s@%s: %w", owner, repo, sha, err)
	}
	for _, pr := range prs {
		if pr.GetState() == "open" && pr.GetHead().GetSHA() == sha {
			return pr, nil
		}
	}
	return nil, nil
}
