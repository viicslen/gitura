package gitlocal

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// scanDepth reaches <root>/github.com/<owner>/<repo>, the deepest common layout.
const scanDepth = 3

type repoIndex struct {
	found     map[string]string
	viaOrigin map[string]bool
}

// FindRepos walks roots up to three levels deep and maps each "owner/repo" (lowercased) named by a
// clone's remotes to that clone's directory. A clone whose origin names the repo wins over one that
// only has it as another remote, such as a fork's upstream. It reads .git/config directly instead
// of running git, skips hidden directories, node_modules and vendor, and doesn't descend into repos.
func FindRepos(roots []string) map[string]string {
	idx := repoIndex{found: map[string]string{}, viaOrigin: map[string]bool{}}
	for _, root := range roots {
		idx.walk(root, 0)
	}
	return idx.found
}

func (idx repoIndex) walk(dir string, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			continue
		}
		// A .git file is a worktree or submodule; worktrees are found from their main clone.
		if e.IsDir() {
			idx.add(dir)
		}
		// A chosen folder is searched even when it is a repository itself.
		if depth > 0 {
			return
		}
	}
	if depth == scanDepth {
		return
	}
	for _, e := range entries {
		if e.IsDir() && !skipDir(e.Name()) {
			idx.walk(filepath.Join(dir, e.Name()), depth+1)
		}
	}
}

func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor"
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (idx repoIndex) add(dir string) {
	for remote, urls := range remoteURLs(filepath.Join(dir, ".git", "config")) {
		for _, url := range urls {
			owner, repo, ok := ParseRemoteURL(url)
			if !ok {
				continue
			}
			k := strings.ToLower(owner + "/" + repo)
			if _, seen := idx.found[k]; !seen || (remote == "origin" && !idx.viaOrigin[k]) {
				idx.found[k] = dir
				idx.viaOrigin[k] = remote == "origin"
			}
		}
	}
}

// remoteURLs reads the url keys of each [remote "name"] section of a git config file.
func remoteURLs(path string) map[string][]string {
	f, err := os.Open(path) // #nosec G304 -- .git/config under a directory the user chose to scan.
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	urls := map[string][]string{}
	remote := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			name, ok := strings.CutPrefix(line, "[remote \"")
			remote = ""
			if ok {
				remote = strings.TrimSuffix(name, "\"]")
			}
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && remote != "" && strings.TrimSpace(key) == "url" {
			urls[remote] = append(urls[remote], strings.TrimSpace(value))
		}
	}
	return urls
}

// WorktreeFor returns the worktree of dir's repository that has branch checked out, or the main
// worktree when none does; it returns dir itself when git can't list the worktrees.
func WorktreeFor(ctx context.Context, dir, branch string) string {
	out, err := git(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return dir
	}
	main := ""
	for _, entry := range strings.Split(strings.TrimSpace(out), "\n\n") {
		head, rest, _ := strings.Cut(entry, "\n")
		path := strings.TrimPrefix(head, "worktree ")
		attrs := strings.Split(rest, "\n")
		if slices.Contains(attrs, "bare") {
			continue
		}
		if main == "" {
			main = path
		}
		if branch != "" && slices.Contains(attrs, "branch refs/heads/"+branch) && isDir(path) {
			return path
		}
	}
	if main == "" {
		return dir
	}
	return main
}
