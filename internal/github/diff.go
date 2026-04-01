package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v67/github"

	"gitura/internal/model"
)

// FetchPRFilesRaw fetches the complete list of raw CommitFile objects for a
// pull request. All pages are collected before returning. Unlike ListPRFiles,
// this preserves the full Patch field so callers can parse diffs from the cache.
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

// CommitFileToPRFileDTO converts a GitHub CommitFile to a PRFileDTO.
// Exported for use from app.go without duplicating the conversion logic.
func CommitFileToPRFileDTO(f *github.CommitFile) model.PRFileDTO {
	return commitFileToDTO(f)
}

// ListPRFiles fetches the complete list of files changed in a pull request.
// All pages are collected before returning.
// Errors are prefixed with "github:".
func ListPRFiles(ctx context.Context, client *github.Client, owner, repo string, number int) ([]model.PRFileDTO, error) {
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
	result := make([]model.PRFileDTO, 0, len(all))
	for _, f := range all {
		result = append(result, commitFileToDTO(f))
	}
	return result, nil
}

// GetFileDiff fetches and parses the unified diff for a specific file in a
// pull request. Returns "notfound:" if the filename is not in the changed
// file list; all other errors are prefixed with "github:".
func GetFileDiff(ctx context.Context, client *github.Client, owner, repo string, number int, filename string) (model.ParsedDiffDTO, error) {
	opts := &github.ListOptions{PerPage: 100}
	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			return model.ParsedDiffDTO{}, fmt.Errorf("github: list PR files %s/%s#%d: %w", owner, repo, number, err)
		}
		for _, f := range files {
			if f.GetFilename() == filename {
				return ParseCommitFileDiff(f), nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return model.ParsedDiffDTO{}, fmt.Errorf("notfound: file %q not in PR %s/%s#%d diff", filename, owner, repo, number)
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
		dto.Hunks = parseUnifiedDiffHunks(patch)
	}
	return dto
}

// parseUnifiedDiffHunks parses a raw unified diff patch string into a slice of
// DiffHunkDTOs. Each hunk starts at a @@ header. "No newline" marker lines are
// silently skipped.
func parseUnifiedDiffHunks(patch string) []model.DiffHunkDTO {
	var hunks []model.DiffHunkDTO
	var cur *model.DiffHunkDTO
	oldLine, newLine := 0, 0

	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@") {
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			oldStart, oldLines, newStart, newLines := parseDiffHunkHeader(line)
			cur = &model.DiffHunkDTO{
				Header:   line,
				OldStart: oldStart,
				OldLines: oldLines,
				NewStart: newStart,
				NewLines: newLines,
			}
			oldLine = oldStart
			newLine = newStart
			continue
		}
		if cur == nil {
			continue
		}
		// Skip "\ No newline at end of file" markers.
		if strings.HasPrefix(line, `\ `) {
			continue
		}
		cur.Lines = append(cur.Lines, parseDiffLine(line, &oldLine, &newLine))
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	return hunks
}

// parseDiffLine parses one line from a diff hunk body, advances the old/new
// line counters, and returns the corresponding DiffLineDTO.
func parseDiffLine(line string, oldLine, newLine *int) model.DiffLineDTO {
	switch {
	case strings.HasPrefix(line, "+"):
		dl := model.DiffLineDTO{Type: model.DiffLineAdd, NewNo: *newLine, Content: line[1:]}
		*newLine++
		return dl
	case strings.HasPrefix(line, "-"):
		dl := model.DiffLineDTO{Type: model.DiffLineDelete, OldNo: *oldLine, Content: line[1:]}
		*oldLine++
		return dl
	default:
		// Context line: leading space stripped when present.
		content := line
		if len(line) > 0 && line[0] == ' ' {
			content = line[1:]
		}
		dl := model.DiffLineDTO{Type: model.DiffLineContext, OldNo: *oldLine, NewNo: *newLine, Content: content}
		*oldLine++
		*newLine++
		return dl
	}
}

// parseDiffHunkHeader extracts (oldStart, oldLines, newStart, newLines) from a
// unified diff @@ header line. Returns (0,0,0,0) when the header cannot be
// parsed. Handles all four short forms produced by Git.
func parseDiffHunkHeader(header string) (oldStart, oldLines, newStart, newLines int) {
	var a, b, c, d int
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d,%d", &a, &b, &c, &d); n == 4 {
		return a, b, c, d
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d", &a, &b, &c); n == 3 {
		return a, b, c, 1
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d,%d", &a, &c, &d); n == 3 {
		return a, 1, c, d
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d", &a, &c); n == 2 {
		return a, 1, c, 1
	}
	return 0, 0, 0, 0
}

// commitFileToDTO converts a GitHub CommitFile to a PRFileDTO.
// A file is considered binary when its patch is empty but changes is non-zero.
func commitFileToDTO(f *github.CommitFile) model.PRFileDTO {
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
