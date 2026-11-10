package app

import (
	"fmt"

	"gitura/internal/ghapi"
	"gitura/internal/model"
)

// GetPRFiles returns the list of files changed in the currently loaded PR.
// Results are cached; the cache is cleared when LoadPR is called.
// Error prefix: "notfound:" if no PR has been loaded.
func (a *App) GetPRFiles() ([]model.PRFileDTO, error) {
	if a.local != nil {
		return a.localFiles()
	}
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if a.prFilesCache == nil {
		rawFiles, err := ghapi.FetchPRFilesRaw(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
		if err != nil {
			return nil, err
		}
		a.prFilesCache = rawFiles
	}
	result := make([]model.PRFileDTO, 0, len(a.prFilesCache))
	for _, f := range a.prFilesCache {
		dto := ghapi.PRFileFromCommitFile(f)
		dto.Viewed = a.viewedFiles[dto.Filename]
		result = append(result, dto)
	}
	return result, nil
}

// GetViewedFiles returns the paths the user has marked as viewed on GitHub for the loaded PR.
// It is separate from GetPRFiles so the file list still loads when this lookup fails.
func (a *App) GetViewedFiles() ([]string, error) {
	if a.local != nil {
		var paths []string
		for _, f := range a.local.files {
			if a.local.isViewed(f) {
				paths = append(paths, f.Diff.Filename)
			}
		}
		return paths, nil
	}
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if a.viewedFiles == nil {
		viewed, err := ghapi.FetchViewedFiles(a.ctx, ghapi.NewHTTPClient(a.authToken), a.prOwner, a.prRepo, a.prNumber)
		if err != nil {
			return nil, err
		}
		a.viewedFiles = viewed
	}
	paths := make([]string, 0, len(a.viewedFiles))
	for path, viewed := range a.viewedFiles {
		if viewed {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// SetFileViewed marks or unmarks a file as viewed on GitHub, like the "Viewed" checkbox on github.com.
func (a *App) SetFileViewed(path string, viewed bool) error {
	if a.local != nil {
		f, err := a.local.file(path)
		if err != nil {
			return err
		}
		if viewed {
			a.local.viewed[path] = f.Patch
		} else {
			delete(a.local.viewed, path)
		}
		return nil
	}
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := ghapi.SetFileViewed(a.ctx, ghapi.NewHTTPClient(a.authToken), a.prCache.NodeID, path, viewed); err != nil {
		return err
	}
	if a.viewedFiles != nil {
		a.viewedFiles[path] = viewed
	}
	return nil
}

// GetFileDiff fetches and parses the diff for a single file in the loaded PR.
// Binary files return ParsedDiffDTO{IsBinary: true} with no hunks, no error.
// Error prefix: "notfound:" if no PR is loaded or the file is not in the diff.
func (a *App) GetFileDiff(path string) (model.ParsedDiffDTO, error) {
	if a.local != nil {
		f, err := a.local.file(path)
		return f.Diff, err
	}
	if a.ghClient == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ParsedDiffDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if a.prFilesCache == nil {
		rawFiles, err := ghapi.FetchPRFilesRaw(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
		if err != nil {
			return model.ParsedDiffDTO{}, err
		}
		a.prFilesCache = rawFiles
	}
	for _, f := range a.prFilesCache {
		if f.GetFilename() == path {
			return ghapi.ParseCommitFileDiff(f), nil
		}
	}
	return model.ParsedDiffDTO{}, fmt.Errorf("notfound: file %q not in PR diff", path)
}
