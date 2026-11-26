package app

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v93/github"

	"gitura/internal/ghapi"
	"gitura/internal/model"
)

// GetPRFiles returns the list of files changed in the currently loaded PR.
// Results are cached; the cache is cleared when LoadPR is called.
// Error prefix: "notfound:" if no PR has been loaded.
func (a *App) GetPRFiles() ([]model.PRFileDTO, error) {
	if l := a.local.Load(); l != nil {
		return l.rows(a.ctx)
	}
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return nil, err
	}
	a.retryFailedSubmodules()
	result := make([]model.PRFileDTO, 0, len(a.prFilesCache))
	for _, f := range a.prFilesCache {
		result = append(result, a.prFileDTO(f))
	}
	return result, nil
}

// fileID identifies a file in the Files list by its PRFileDTO.Repo and path, and pointer for a submodule's
// pointer row, which a regular file can share both with when one replaces the other.
type fileID struct {
	repo, path string
	pointer    bool
}

// GetViewedFiles returns the files the user has marked as viewed on GitHub for the loaded PR,
// marks on submodule files included, and the submodules whose PR's marks couldn't be read.
// It is separate from GetPRFiles so the file list still loads when this lookup fails.
func (a *App) GetViewedFiles() (model.ViewedFilesDTO, error) {
	if l := a.local.Load(); l != nil {
		return model.ViewedFilesDTO{Files: l.viewedKeys()}, nil
	}
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return model.ViewedFilesDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ViewedFilesDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return model.ViewedFilesDTO{}, err
	}
	if err := a.loadViewed(); err != nil {
		return model.ViewedFilesDTO{}, err
	}
	out := model.ViewedFilesDTO{Files: []model.FileKeyDTO{}}
	for _, f := range a.prFilesCache {
		if dto := a.prFileDTO(f); dto.Viewed {
			out.Files = append(out.Files, model.FileKeyDTO{Submodule: dto.Repo, Path: dto.Filename, Pointer: dto.Status == "submodule"})
		}
	}
	for p, s := range a.prSubmodules {
		if s.viewedUnknown {
			out.UnknownSubmodules = append(out.UnknownSubmodules, p)
		}
	}
	return out, nil
}

// loadViewed fills viewedFiles on first use, and otherwise reads again the marks of the submodule PRs
// whose read failed, once their backoff has passed.
func (a *App) loadViewed() error {
	if a.viewedFiles != nil {
		for _, s := range a.prSubmodules {
			if s.viewedUnknown && s.due() {
				a.fetchViewedOf(s)
			}
		}
		return nil
	}
	viewed, err := ghapi.FetchViewedFiles(a.ctx, ghapi.NewHTTPClient(a.authToken), a.prOwner, a.prRepo, a.prNumber)
	if err != nil {
		return err
	}
	a.viewedFiles = make(map[fileID]bool, len(viewed))
	for p, v := range viewed {
		a.viewedFiles[fileID{path: p}] = v
	}
	for _, s := range a.prSubmodules {
		a.fetchViewedOf(s)
	}
	return nil
}

// SetFileViewed marks or unmarks a file as viewed on GitHub, like the "Viewed" checkbox on github.com.
// submodule is the file's PRFileDTO.Repo and pointer is set for a pointer row; GitHub keeps one mark per path,
// so on the loaded PR's own files a pointer row and a regular file at its path share it. rerouted reports that the submodule's PR, checked again,
// changed whether it takes the mark or gitura keeps it, so that submodule's rows in GetPRFiles changed.
// Error prefix: "notfound:" — no such file in the loaded change.
func (a *App) SetFileViewed(submodule, path string, pointer, viewed bool) (rerouted bool, err error) {
	if l := a.local.Load(); l != nil {
		return false, l.setViewed(fileID{submodule, path, pointer}, viewed)
	}
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return false, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return false, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return false, err
	}
	if submodule != "" {
		s, f, err := a.submoduleFile(fileID{submodule, path, pointer})
		if err != nil {
			return false, err
		}
		return a.setSubmoduleFileViewed(s, f, viewed)
	}
	if err := ghapi.SetFileViewed(a.ctx, ghapi.NewHTTPClient(a.authToken), a.prCache.NodeID, path, viewed); err != nil {
		return false, err
	}
	if a.viewedFiles != nil {
		a.viewedFiles[fileID{path: path}] = viewed
	}
	return false, nil
}

// GetFileAtHead returns the text of a file at the loaded PR's head commit; for a file inside a
// bumped submodule (submodule set), at the commit the submodule is bumped to.
// Error prefix: "notfound:" if no PR is loaded or the file is not at the head commit.
func (a *App) GetFileAtHead(submodule, path string) (string, error) {
	owner, repo, ref, path, err := a.headOf(submodule, path)
	if err != nil {
		return "", err
	}
	content, _, err := ghapi.FetchFileContent(a.ctx, a.ghClient, owner, repo, path, ref)
	return content, err
}

// headOf returns the repository, commit and path that GetFileAtHead reads path from.
func (a *App) headOf(submodule, path string) (owner, repo, ref, rel string, err error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return "", "", "", "", fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return "", "", "", "", fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return "", "", "", "", err
	}
	if submodule == "" {
		return a.prOwner, a.prRepo, a.prCache.HeadSHA, path, nil
	}
	s := a.prSubmodules[submodule]
	switch {
	case s == nil || !strings.HasPrefix(path, submodule+"/"):
		return "", "", "", "", fmt.Errorf("notfound: file %q is not in a submodule %s this pull request changes", path, submodule)
	case s.to == "":
		return "", "", "", "", fmt.Errorf("notfound: file %q: its submodule is removed", path)
	}
	return s.Owner, s.Repo, s.to, s.rel(path), nil
}

// GetFileDiff fetches and parses the diff for a single file in the loaded PR; submodule is the file's
// PRFileDTO.Repo and pointer is set for a submodule's pointer row.
// Binary files return ParsedDiffDTO{IsBinary: true} with no hunks, no error.
// Error prefix: "notfound:" if no PR is loaded or the file is not in the diff.
func (a *App) GetFileDiff(submodule, path string, pointer bool) (model.ParsedDiffDTO, error) {
	if l := a.local.Load(); l != nil {
		f, err := l.file(fileID{submodule, path, pointer})
		return f.Diff, err
	}
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return model.ParsedDiffDTO{}, err
	}
	f, err := a.prFile(fileID{submodule, path, pointer})
	if err != nil {
		return model.ParsedDiffDTO{}, err
	}
	return a.prFileDiff(f)
}

// prFile returns the listed file id names. Callers hold prMu.
func (a *App) prFile(id fileID) (*github.CommitFile, error) {
	if id.repo != "" {
		_, f, err := a.submoduleFile(id)
		return f, err
	}
	for _, f := range a.prFilesCache {
		if a.prSubmoduleOf[f] == nil && a.idOf(nil, f) == id {
			return f, nil
		}
	}
	return nil, fmt.Errorf("notfound: file %q not in PR diff", id.path)
}

// submoduleFile returns the bump of submodule id.repo and its listed file id names. Callers hold prMu.
func (a *App) submoduleFile(id fileID) (*prSubmodule, *github.CommitFile, error) {
	s := a.prSubmodules[id.repo]
	if f := a.bumpFile(s, id.path, id.pointer); f != nil {
		return s, f, nil
	}
	return nil, nil, fmt.Errorf("notfound: file %q is not among the changes of a submodule %s this pull request bumps", id.path, id.repo)
}

// prFileDiff parses f's diff, first loading the content of a whole-tree submodule file still without it.
func (a *App) prFileDiff(f *github.CommitFile) (model.ParsedDiffDTO, error) {
	if s := a.prSubmoduleOf[f]; s != nil && s.TreeRef != "" && f.Patch == nil && f.GetChanges() == 0 {
		if err := ghapi.FillWholeFile(a.ctx, a.ghClient, s.Owner, s.Repo, f.GetSHA(), f); err != nil {
			return model.ParsedDiffDTO{}, err
		}
	}
	return ghapi.ParseCommitFileDiff(f), nil
}
