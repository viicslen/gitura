package gitlocal

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// subHeader is a "Submodule <path> ..." line (prefix stripped) and the index of the file it precedes.
// failed is set when git printed "(diff failed)" under it.
type subHeader struct {
	line   string
	at     int
	failed bool
}

type header struct {
	path, old, new, note string
	dirty, diverged      bool
}

type pointer struct {
	// in is the submodule holding it, "" for the top level; from is its path at the base, when it moved since.
	path, in, from, old, new string
	dirty                    bool
	backwards                bool
	diverged                 bool
	unavailable              string
	at                       int
}

func collectHeaders(text string, at int) []subHeader {
	var hs []subHeader
	for line := range strings.SplitSeq(text, "\n") {
		if rest, ok := strings.CutPrefix(line, "Submodule "); ok {
			hs = append(hs, subHeader{line: rest, at: at})
		} else if line == "(diff failed)" && len(hs) > 0 {
			hs[len(hs)-1].failed = true
		}
	}
	return hs
}

// parseHeader reads "<p> contains modified content", "<p> contains untracked content",
// "<p> old..new:", "<p> old...new (new submodule)", "<p> old..new (rewind):" and the like;
// note is the text in parentheses. Three dots without a note mean neither commit is an ancestor of the other.
func parseHeader(line string) (header, bool) {
	for _, s := range []string{" contains modified content", " contains untracked content"} {
		if p, found := strings.CutSuffix(line, s); found {
			return header{path: p, dirty: true}, true
		}
	}
	var h header
	line = strings.TrimSuffix(line, ":")
	if i := strings.LastIndex(line, " ("); i >= 0 && strings.HasSuffix(line, ")") {
		line, h.note = line[:i], line[i+2:len(line)-1]
	}
	i := strings.LastIndex(line, " ")
	if i < 0 {
		return header{}, false
	}
	old, new, ok := strings.Cut(line[i+1:], "..")
	h.path, h.old, h.new = line[:i], old, strings.TrimPrefix(new, ".")
	h.diverged = ok && h.note == "" && strings.HasPrefix(new, ".")
	return h, ok
}

func parsePointers(ctx context.Context, dir string, files []File, headers []subHeader) ([]*pointer, error) {
	var ps []*pointer
	byPath := map[string]*pointer{}
	r := resolver{ctx: ctx, dir: dir}
	for i, sh := range headers {
		h, ok := parseHeader(sh.line)
		if !ok {
			continue
		}
		path, err := r.resolve(ps, h, followingFile(files, headers, i))
		if err != nil {
			return nil, err
		}
		p := byPath[path]
		if p == nil {
			// git prints a nested header's path relative to the submodule holding it.
			p = &pointer{path: path, in: strings.TrimSuffix(strings.TrimSuffix(path, h.path), "/"), at: sh.at}
			byPath[path] = p
			ps = append(ps, p)
		}
		p.dirty = p.dirty || h.dirty
		if h.new != "" {
			p.old, p.new = h.old, h.new
			p.backwards, p.diverged = h.note == "rewind", h.diverged
		}
		switch {
		case sh.failed:
			p.unavailable = "not checked out"
		case h.note == "commits not present":
			p.unavailable = "commits not fetched"
		}
	}
	return ps, nil
}

// followingFile is the first file git printed under headers[i], or "" when the next header comes first.
func followingFile(files []File, headers []subHeader, i int) string {
	at := headers[i].at
	if at >= len(files) || (i+1 < len(headers) && headers[i+1].at == at) {
		return ""
	}
	return files[at].Diff.Filename
}

// resolver turns header paths into repository-relative ones: git prints a nested submodule's path
// relative to its parent, so "nest" may be top-level nest or lib/nest.
type resolver struct {
	ctx  context.Context
	dir  string
	subs map[string]string // submodule path -> checked-out (or recorded) commit; loaded on first need
}

func (r *resolver) resolve(seen []*pointer, h header, file string) (string, error) {
	cands := make([]string, 0, len(seen)+1)
	for i := len(seen) - 1; i >= 0; i-- {
		cands = append(cands, seen[i].path+"/"+h.path)
	}
	cands = append(cands, h.path)
	for _, c := range cands {
		if strings.HasPrefix(file, c+"/") {
			return c, nil
		}
	}
	if len(cands) == 1 {
		return h.path, nil
	}
	if h.new != "" && strings.Trim(h.new, "0") == "" {
		return r.removed(seen, h), nil
	}
	if r.subs == nil {
		subs, err := submoduleCommits(r.ctx, r.dir)
		if err != nil {
			return "", err
		}
		r.subs = subs
	}
	if c := pickCandidate(cands, r.subs, h); c != "" {
		return c, nil
	}
	return r.removed(seen, h), nil
}

// removed places the header of a submodule deleted or moved away, which has no files under it, the work tree
// no longer holding it: the innermost changed submodule that removed it, its old tree having it as a gitlink at the
// header's old commit and its new commit (its index when only its work tree changed) not; else the top level.
func (r *resolver) removed(seen []*pointer, h header) string {
	for i := len(seen) - 1; i >= 0; i-- {
		p := seen[i]
		// A failed read means the tree is unreadable (new or not checked out submodule), so it can't hold h.
		if sha := r.gitlink(p.path, cmp.Or(p.old, "HEAD"), h.path); sha != "" && strings.HasPrefix(sha, h.old) && r.gitlink(p.path, p.new, h.path) == "" {
			return p.path + "/" + h.path
		}
	}
	return h.path
}

// gitlink is the commit the submodule sub records for its submodule at path at rev, or in its index when
// rev is ""; "" when it records none or can't be read.
func (r *resolver) gitlink(sub, rev, path string) string {
	args := []string{"ls-tree", rev, "--", path}
	if rev == "" {
		args = []string{"ls-files", "--stage", "--", path}
	}
	out, err := git(r.ctx, filepath.Join(r.dir, sub), args...)
	f := strings.Fields(out)
	if err != nil || len(f) < 3 || f[0] != "160000" {
		return ""
	}
	if rev == "" {
		return f[1] // "<mode> <sha> <stage>\t<path>"
	}
	return f[2] // "<mode> commit <sha>\t<path>"
}

// pickCandidate places a header with no files under it: the candidate checked out at the header's new commit;
// "" when no candidate is a submodule.
// shortcut: a file-less dirty header (dirty only via its own nested submodule) takes the innermost existing candidate; track parent closure if that misplaces one.
func pickCandidate(cands []string, subs map[string]string, h header) string {
	for _, c := range cands {
		if sha, exists := subs[c]; exists && (h.new == "" || strings.HasPrefix(sha, h.new)) {
			return c
		}
	}
	for _, c := range cands {
		if _, exists := subs[c]; exists {
			return c
		}
	}
	return ""
}

// submoduleCommits lists every submodule, nested and not-checked-out ones included.
func submoduleCommits(ctx context.Context, dir string) (map[string]string, error) {
	out, err := git(ctx, dir, "submodule", "status", "--recursive")
	if err != nil {
		return nil, err
	}
	subs := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		// "<state><sha> <path>", then " (<describe>)" unless state is "-" (not checked out).
		if len(line) < 2 {
			continue
		}
		sha, path, ok := strings.Cut(line[1:], " ")
		if !ok {
			continue
		}
		if i := strings.LastIndex(path, " ("); line[0] != '-' && i >= 0 && strings.HasSuffix(path, ")") {
			path = path[:i]
		}
		subs[path] = sha
	}
	return subs, nil
}

// pointerFile is the "Subproject commit" row for p; its Patch changes whenever the range or dirtiness does.
func pointerFile(ctx context.Context, dir string, p *pointer) File {
	old, new := p.old, p.new
	if new == "" {
		// Only the work tree is dirty, so the recorded commit is still the checked-out one.
		if head, err := git(ctx, filepath.Join(dir, p.path), "rev-parse", "--short", "HEAD"); err == nil {
			old, new = strings.TrimSpace(head), strings.TrimSpace(head)
		}
	}
	oldN, newN, body := "1", "1", ""
	if strings.Trim(old, "0") == "" {
		oldN = "0,0"
	} else {
		body += "\n-Subproject commit " + old
	}
	if strings.Trim(new, "0") == "" {
		newN = "0,0"
	} else {
		if p.dirty {
			new += "-dirty"
		}
		body += "\n+Subproject commit " + new
	}
	from := cmp.Or(p.from, p.path)
	f := parseFile(fmt.Sprintf("diff --git a/%[1]s b/%[2]s\n--- a/%[1]s\n+++ b/%[2]s\n@@ -%s +%s @@%s", from, p.path, oldN, newN, body))
	f.Diff.Status, f.Diff.PreviousFilename = "submodule", p.from
	f.Submodule = p.path
	f.SubmoduleRange = old + ".." + new
	f.SubmoduleBackwards, f.SubmoduleDiverged = p.backwards, p.diverged
	f.SubmoduleUnavailable, f.SubmoduleIn = p.unavailable, p.in
	return f
}

// renamePointers moves the pointers of submodules renamed since the base to their new path: git prints a moved
// submodule's header under its old path, where it finds no checkout ("commits not present"). Each holder's
// renames come from its own diff, from its old commit.
func renamePointers(ctx context.Context, dir, rev string, ps []*pointer) error {
	var holders []string
	for _, p := range ps {
		if p.unavailable != "" && !slices.Contains(holders, p.in) {
			holders = append(holders, p.in)
		}
	}
	for _, in := range holders {
		base, ok, err := holderBase(ctx, dir, rev, ps, in)
		if err != nil {
			return err
		}
		if ok {
			if err := renameIn(ctx, dir, in, base, ps); err != nil {
				return err
			}
		}
	}
	return nil
}

// holderBase is the revision the submodule in ("" for dir) is diffed from; ok is false when it has no checkout.
func holderBase(ctx context.Context, dir, rev string, ps []*pointer, in string) (base string, ok bool, err error) {
	if in == "" {
		return rev, true, nil
	}
	i := slices.IndexFunc(ps, func(p *pointer) bool { return p.path == in })
	// Without its own .git, git -C would find the parent repository instead.
	if _, err := os.Stat(filepath.Join(dir, in, ".git")); i < 0 || err != nil {
		return "", false, nil
	}
	base, err = diffBase(ctx, filepath.Join(dir, in), ps[i].old)
	return base, err == nil, err
}

// renameIn moves the pointers held by submodule in ("" for dir) that it renamed since base.
func renameIn(ctx context.Context, dir, in, base string, ps []*pointer) error {
	out, err := git(ctx, filepath.Join(dir, in), "diff", "--raw", "-z", "-M", "--diff-filter=R", base, "--")
	if err != nil {
		return err
	}
	prefix := ""
	if in != "" {
		prefix = in + "/"
	}
	// ":<mode> <mode> <sha> <sha> R<score>", "<from>", "<to>"
	e := strings.Split(out, "\x00")
	for i := 0; i+2 < len(e); i += 3 {
		j := slices.IndexFunc(ps, func(p *pointer) bool { return p.in == in && p.path == prefix+e[i+1] })
		if j < 0 || !strings.HasPrefix(e[i], ":160000 160000 ") {
			continue
		}
		p := ps[j]
		p.from, p.path = p.path, prefix+e[i+2]
		p.unavailable = movedUnavailable(ctx, filepath.Join(dir, p.path), p.old)
	}
	return nil
}

// movedUnavailable says why the files of the submodule moved to subDir are missing, "" when they aren't.
func movedUnavailable(ctx context.Context, subDir, sha string) string {
	// Without its own .git, git -C would find the parent repository instead.
	if _, err := os.Stat(filepath.Join(subDir, ".git")); err != nil {
		return "not checked out"
	}
	if _, err := git(ctx, subDir, "cat-file", "-e", sha+"^{commit}"); err != nil {
		return "commits not fetched"
	}
	return ""
}

// movedFiles diffs moved submodule p's checkout from its old commit, since git prints no files under it.
func movedFiles(ctx context.Context, dir string, p *pointer) ([]File, error) {
	if p.from == "" || p.unavailable != "" {
		return nil, nil
	}
	subDir := filepath.Join(dir, p.path)
	out, err := git(ctx, subDir, "diff", "--no-color", "--no-ext-diff", "--submodule=diff", "-M", p.old, "--")
	if err != nil {
		return nil, err
	}
	files, headers := splitPatch(out)
	files, err = withSubmodules(ctx, subDir, p.old, files, headers)
	if err != nil {
		return nil, err
	}
	for i := range files {
		underSubmodule(&files[i], p.path)
	}
	return files, nil
}

// underSubmodule turns f, listed by the diff of submodule sub, into a row of dir's, settled so tagSubmoduleFiles skips it.
func underSubmodule(f *File, sub string) {
	f.Diff.Filename = sub + "/" + f.Diff.Filename
	if f.Diff.PreviousFilename != "" {
		f.Diff.PreviousFilename = sub + "/" + f.Diff.PreviousFilename
	}
	f.Submodule = strings.TrimSuffix(sub+"/"+f.Submodule, "/")
	if f.SubmoduleRange != "" {
		f.SubmoduleIn = strings.TrimSuffix(sub+"/"+f.SubmoduleIn, "/")
	}
	f.own = true
}

// withSubmodules inserts a pointer row before each submodule's files, adds untracked files inside
// dirty submodules, and tags every file inside a submodule with the innermost one's path.
func withSubmodules(ctx context.Context, dir, rev string, files []File, headers []subHeader) ([]File, error) {
	ps, err := parsePointers(ctx, dir, files, headers)
	if err != nil {
		return nil, err
	}
	if err := renamePointers(ctx, dir, rev, ps); err != nil {
		return nil, err
	}
	if ps, err = withUntracked(ctx, dir, ps, len(files)); err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return files, nil
	}
	if err := markOwn(ctx, dir, rev, files, ps); err != nil {
		return nil, err
	}
	out, err := insertPointers(ctx, dir, rev, files, ps)
	if err != nil {
		return nil, err
	}
	if out, err = appendUntracked(ctx, dir, out, ps); err != nil {
		return nil, err
	}
	tagSubmoduleFiles(out, ps)
	return out, nil
}

// appendUntracked adds the untracked files inside dirty submodules, which git diff never lists.
func appendUntracked(ctx context.Context, dir string, out []File, ps []*pointer) ([]File, error) {
	for _, p := range ps {
		// A header may name a checkout no longer there, such as a nested submodule's old path after a move.
		if _, err := os.Stat(filepath.Join(dir, p.path, ".git")); !p.dirty || err != nil {
			continue
		}
		untracked, err := untrackedFiles(ctx, dir, p.path)
		if err != nil {
			return nil, err
		}
		out = append(out, untracked...)
	}
	return out, nil
}

// withUntracked marks dirty the pointers of the submodules holding untracked files, adding one at index at
// for each git diff printed no header for: it never does for untracked content alone.
func withUntracked(ctx context.Context, dir string, ps []*pointer, at int) ([]*pointer, error) {
	if _, err := os.Stat(filepath.Join(dir, ".gitmodules")); err != nil {
		return ps, nil
	}
	us, err := untrackedSubmodules(ctx, dir, "")
	if err != nil {
		return nil, err
	}
	for _, u := range us {
		// movedFiles lists the submodules nested in a moved checkout, untracked files included.
		if slices.ContainsFunc(ps, func(p *pointer) bool {
			return p.from != "" && p.unavailable == "" && strings.HasPrefix(u.path, p.path+"/")
		}) {
			continue
		}
		if i := slices.IndexFunc(ps, func(p *pointer) bool { return p.path == u.path }); i >= 0 {
			ps[i].dirty = true
			continue
		}
		u.at = at
		ps = append(ps, u)
	}
	return ps, nil
}

// untrackedSubmodules returns a dirty pointer, parents first, for each submodule under sub ("" for dir) whose
// work tree holds untracked files, its own or a nested submodule's.
func untrackedSubmodules(ctx context.Context, dir, sub string) ([]*pointer, error) {
	// Overrides a status.showUntrackedFiles=no that would hide untracked content inside submodules; -c, unlike
	// --untracked-files, also reaches the status git runs inside each one.
	out, err := git(ctx, filepath.Join(dir, sub), "-c", "status.showUntrackedFiles=normal", "status", "--porcelain=v2", "-z")
	if err != nil {
		return nil, err
	}
	var ps []*pointer
	for _, p := range flaggedUntracked(out) {
		if sub != "" {
			p = sub + "/" + p
		}
		inner, err := untrackedSubmodules(ctx, dir, p)
		if err != nil {
			return nil, err
		}
		ps = append(append(ps, &pointer{path: p, in: sub, dirty: true}), inner...)
	}
	return ps, nil
}

// flaggedUntracked lists the submodules `git status --porcelain=v2 -z` flags "S..U", holding untracked content.
func flaggedUntracked(status string) []string {
	var paths []string
	entries := strings.Split(status, "\x00")
	for i := 0; i < len(entries); i++ {
		// "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"; "2" adds "<X><score>" before the path, its original path following.
		f := strings.SplitN(entries[i], " ", 9)
		if f[0] == "2" && len(f) == 9 {
			_, f[8], _ = strings.Cut(f[8], " ")
			i++
		}
		if (f[0] == "1" || f[0] == "2") && len(f) == 9 && strings.HasPrefix(f[2], "S") && strings.HasSuffix(f[2], "U") {
			paths = append(paths, f[8])
		}
	}
	return paths
}

func tagSubmoduleFiles(files []File, ps []*pointer) {
	for i := range files {
		if files[i].SubmoduleRange == "" && !files[i].own {
			files[i].Submodule = innermost(ps, files[i].Diff.Filename)
		}
	}
}

// markOwn sets own on the files of dir's own diff, then tags those of each submodule holding another one,
// whose files a nested submodule's may share a path with, as when it turned tracked directory x into submodule x.
func markOwn(ctx context.Context, dir, rev string, files []File, ps []*pointer) error {
	if err := markRepo(ctx, dir, "", rev, files); err != nil {
		return err
	}
	for _, p := range ps {
		gone := p.new != "" && strings.Trim(p.new, "0") == ""
		if gone || p.unavailable != "" || !slices.ContainsFunc(ps, func(q *pointer) bool { return strings.HasPrefix(q.path, p.path+"/") }) {
			continue
		}
		subDir := filepath.Join(dir, p.path)
		from, err := diffBase(ctx, subDir, p.old)
		if err != nil {
			return err
		}
		if err := markRepo(ctx, subDir, p.path, from, files); err != nil {
			return err
		}
	}
	return nil
}

// diffBase is the revision git diffs a submodule's work tree from: old, the empty tree for a new submodule,
// or HEAD when only its work tree changed.
func diffBase(ctx context.Context, subDir, old string) (string, error) {
	switch {
	case old == "":
		return "HEAD", nil
	case strings.Trim(old, "0") != "":
		return old, nil
	}
	empty, err := git(ctx, subDir, "hash-object", "-t", "tree", os.DevNull)
	return strings.TrimSpace(empty), err
}

// markRepo sets own, and Submodule to sub, on the files not yet owned of the diff of the repository at repoDir
// (submodule sub, "" for dir itself) from rev. They come in the same order as in the diff listing submodule
// files too, where each sits after the files of any submodule at a path before its own, such as lib/a.txt
// deleted after lib/a.txt in the submodule that replaced directory lib: matching from the end picks the right
// one of two files sharing a path.
func markRepo(ctx context.Context, repoDir, sub, rev string, files []File) error {
	out, err := git(ctx, repoDir, "diff", "--name-only", "-z", "--no-ext-diff", "-M", rev, "--")
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, f := range files {
		listed[f.Diff.Filename] = true
	}
	prefix := ""
	if sub != "" {
		prefix = sub + "/"
	}
	var names []string
	for n := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		// Gitlinks are listed too, yet have no file of their own.
		if listed[prefix+n] {
			names = append(names, prefix+n)
		}
	}
	j := len(names) - 1
	for i := len(files) - 1; i >= 0 && j >= 0; i-- {
		if !files[i].own && files[i].Diff.Filename == names[j] {
			files[i].own, files[i].Submodule = true, sub
			j--
		}
	}
	return nil
}

// insertPointers puts each pointer row before its submodule's files, and a deleted or moved submodule's files after it.
func insertPointers(ctx context.Context, dir, rev string, files []File, ps []*pointer) ([]File, error) {
	out := make([]File, 0, len(files)+len(ps))
	next := 0
	for i := 0; i <= len(files); i++ {
		for ; next < len(ps) && ps[next].at == i; next++ {
			gone, err := deletedFiles(ctx, dir, rev, ps, ps[next])
			if err != nil {
				return nil, err
			}
			moved, err := movedFiles(ctx, dir, ps[next])
			if err != nil {
				return nil, err
			}
			gone = append(gone, moved...)
			out = append(out, pointerFile(ctx, dir, ps[next]))
			out = append(out, gone...)
		}
		if i < len(files) {
			out = append(out, files[i])
		}
	}
	return out, nil
}

// deletedFiles lists deleted submodule p's files from its old commit, since git prints none under a deleted
// submodule; none when p wasn't deleted or its old commit is gone.
func deletedFiles(ctx context.Context, dir, rev string, ps []*pointer, p *pointer) ([]File, error) {
	if p.new == "" || strings.Trim(p.new, "0") != "" || strings.Trim(p.old, "0") == "" {
		return nil, nil
	}
	gitDir := oldCommitGitDir(ctx, dir, rev, ps, p)
	if gitDir == "" {
		return nil, nil
	}
	return goneFiles(ctx, gitDir, p)
}

// goneFiles diffs deleted submodule p's old commit, held in gitDir, against the empty tree. Each submodule nested
// in it becomes a pointer row followed by its own files.
func goneFiles(ctx context.Context, gitDir string, p *pointer) ([]File, error) {
	// --bare: a modules/<name> dir's core.worktree points at the deleted work tree, which git would chdir to.
	empty, err := git(ctx, gitDir, "--bare", "hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		return nil, err
	}
	// --submodule=short overrides a diff.submodule setting, so a nested submodule reads as its gitlink.
	out, err := git(ctx, gitDir, "--bare", "diff", "--no-color", "--no-ext-diff", "--submodule=short",
		"--src-prefix=a/"+p.path+"/", "--dst-prefix=b/"+p.path+"/", p.old, strings.TrimSpace(empty), "--")
	if err != nil {
		return nil, err
	}
	gone, _ := splitPatch(out)
	var files []File
	for _, f := range gone {
		old, ok := goneGitlink(f)
		if !ok {
			files = append(files, f)
			continue
		}
		nested, err := goneNested(ctx, gitDir, p, f.Diff.Filename, old)
		if err != nil {
			return nil, err
		}
		files = append(files, nested...)
	}
	return files, nil
}

// goneGitlink is the commit f, a row of a deleted submodule's diff, recorded when it is a nested submodule's gitlink.
func goneGitlink(f File) (string, bool) {
	if !strings.Contains(f.Patch, "\ndeleted file mode 160000\n") {
		return "", false
	}
	_, sha, ok := strings.Cut(f.Patch, "\n-Subproject commit ")
	sha, _, _ = strings.Cut(sha, "\n")
	return sha, ok
}

// goneNested is the pointer row of the submodule at path, at commit old, nested in deleted submodule p whose git
// dir is gitDir, followed by its files when its own git dir there still holds old.
func goneNested(ctx context.Context, gitDir string, p *pointer, path, old string) ([]File, error) {
	n := &pointer{path: path, in: p.path, old: old[:min(7, len(old))], new: "0000000"}
	nestedDir := filepath.Join(gitDir, "modules", submoduleName(ctx, gitDir, p.old, strings.TrimPrefix(path, p.path+"/"), "--bare"))
	switch {
	case !isDir(nestedDir):
		n.unavailable = "not checked out"
	case !hasCommit(ctx, nestedDir, old):
		n.unavailable = "commits not fetched"
	}
	files := []File{pointerFile(ctx, "", n)}
	if n.unavailable != "" {
		return files, nil
	}
	inner, err := goneFiles(ctx, nestedDir, &pointer{path: path, old: old})
	if err != nil {
		return nil, err
	}
	for i := range inner {
		if inner[i].SubmoduleRange == "" {
			inner[i].Submodule, inner[i].own = path, true
		}
	}
	return append(files, inner...), nil
}

func hasCommit(ctx context.Context, gitDir, sha string) bool {
	_, err := git(ctx, gitDir, "--bare", "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// oldCommitGitDir is the git dir of p's work tree, while it still has one, or else its parent's
// modules/<name>, whichever holds p's old commit; "" when neither does.
func oldCommitGitDir(ctx context.Context, dir, rev string, ps []*pointer, p *pointer) string {
	var cands []string
	// Without its own .git, git -C would find the parent repository instead.
	if _, err := os.Stat(filepath.Join(dir, p.path, ".git")); err == nil {
		if out, err := git(ctx, filepath.Join(dir, p.path), "rev-parse", "--absolute-git-dir"); err == nil {
			cands = append(cands, strings.TrimSpace(out))
		}
	}
	parent, parentRev := parentOf(ps, p, rev)
	parentDir := filepath.Join(dir, parent)
	name := submoduleName(ctx, parentDir, parentRev, strings.TrimPrefix(p.path, parent+"/"))
	if out, err := git(ctx, parentDir, "rev-parse", "--path-format=absolute", "--git-path", "modules/"+name); err == nil {
		cands = append(cands, strings.TrimSpace(out))
	}
	for _, c := range cands {
		if hasCommit(ctx, c, p.old) {
			return c
		}
	}
	return ""
}

// parentOf returns the submodule holding p ("" for dir itself) and the commit it was at before, when p still existed.
func parentOf(ps []*pointer, p *pointer, rev string) (path, old string) {
	path = innermost(ps, p.path)
	for _, q := range ps {
		if q.path == path && path != "" {
			if q.old == "" {
				return path, "HEAD"
			}
			return path, q.old
		}
	}
	return path, rev
}

// submoduleName looks rel up in the .gitmodules of the repository at parentDir as of rev, falling back
// to git's default name, the path. flags go before the git command, such as --bare for a git dir.
func submoduleName(ctx context.Context, parentDir, rev, rel string, flags ...string) string {
	out, err := git(ctx, parentDir, append(flags, "config", "--blob", rev+":.gitmodules", "-z", "--get-regexp", `^submodule\..*\.path$`)...)
	if err != nil {
		return rel
	}
	for entry := range strings.SplitSeq(out, "\x00") {
		if key, val, ok := strings.Cut(entry, "\n"); ok && val == rel {
			return strings.TrimSuffix(strings.TrimPrefix(key, "submodule."), ".path")
		}
	}
	return rel
}

func innermost(ps []*pointer, filename string) string {
	sub := ""
	for _, p := range ps {
		if strings.HasPrefix(filename, p.path+"/") && len(p.path) > len(sub) {
			sub = p.path
		}
	}
	return sub
}
