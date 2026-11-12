package ghapi

import (
	"context"
	"fmt"

	"github.com/google/go-github/v93/github"

	"gitura/internal/model"
	"gitura/internal/unidiff"
)

// FetchPRFilesRaw fetches the complete list of raw CommitFile objects for a
// pull request. All pages are collected before returning. The full Patch field
// is preserved so callers can parse diffs from the cache.
// Errors are prefixed with "github:".
func FetchPRFilesRaw(ctx context.Context, client *github.Client, owner, repo string, number int) ([]*github.CommitFile, error) {
	var all []*github.CommitFile
	opts := &github.ListOptions{PerPage: 100}
	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("github: list PR files %s/%s#%d: %w", owner, repo, number, err)
		}
		all = append(all, files...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// ParseCommitFileDiff converts a GitHub CommitFile into a structured
// ParsedDiffDTO by parsing the raw unified diff patch.
// Binary files (non-zero changes but empty patch) have IsBinary=true and no hunks.
func ParseCommitFileDiff(f *github.CommitFile) model.ParsedDiffDTO {
	isBinary := f.GetPatch() == "" && f.GetChanges() > 0
	dto := model.ParsedDiffDTO{
		Filename:         f.GetFilename(),
		PreviousFilename: f.GetPreviousFilename(),
		Status:           f.GetStatus(),
		IsBinary:         isBinary,
		TotalAdditions:   f.GetAdditions(),
		TotalDeletions:   f.GetDeletions(),
	}
	if patch := f.GetPatch(); patch != "" {
		dto.Hunks = unidiff.ParseHunks(patch)
	}
	return dto
}

// PRFileFromCommitFile converts a GitHub CommitFile to a PRFileDTO.
// A file is considered binary when its patch is empty but changes is non-zero.
func PRFileFromCommitFile(f *github.CommitFile) model.PRFileDTO {
	isBinary := f.GetPatch() == "" && f.GetChanges() > 0
	return model.PRFileDTO{
		Filename:         f.GetFilename(),
		Status:           f.GetStatus(),
		Additions:        f.GetAdditions(),
		Deletions:        f.GetDeletions(),
		Changes:          f.GetChanges(),
		PreviousFilename: f.GetPreviousFilename(),
		IsBinary:         isBinary,
	}
}
