package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v67/github"

	"gitura/internal/model"
)

// CommitSuggestion applies the suggestion block in comment to the file at filePath
// on headBranch and creates a commit with commitMessage.
//
// The filePath and headBranch must come from the parent thread (CommentThreadDTO.Path
// and PullRequestSummary.HeadBranch) since CommentDTO does not carry these fields.
//
// Error prefixes:
//   - "validation:"     — comment is not a suggestion or suggestion block is missing
//   - "notfound:"       — file not found at filePath
//   - "github:conflict" — SHA mismatch (file changed since the suggestion was made)
//   - "github:"         — other API errors
func CommitSuggestion(
	ctx context.Context,
	client *github.Client,
	owner, repo, headBranch, filePath string,
	comment model.CommentDTO,
	commitMessage string,
) (model.SuggestionCommitResult, error) {
	if !comment.IsSuggestion {
		return model.SuggestionCommitResult{}, fmt.Errorf("validation:not-a-suggestion")
	}

	suggestionContent, ok := parseSuggestionBlock(comment.Body)
	if !ok {
		return model.SuggestionCommitResult{}, fmt.Errorf("validation:suggestion block not found in comment body")
	}

	startLine, endLine, err := parseHunkTargetRange(comment.DiffHunk)
	if err != nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("github:parse diff hunk: %w", err)
	}

	rawContent, sha, err := fetchFileContent(ctx, client, owner, repo, filePath, headBranch)
	if err != nil {
		return model.SuggestionCommitResult{}, err
	}

	newContent, err := applyPatch(rawContent, startLine, endLine, suggestionContent)
	if err != nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("github:apply patch: %w", err)
	}

	commitResp, _, err := client.Repositories.UpdateFile(ctx, owner, repo, filePath,
		&github.RepositoryContentFileOptions{
			Message: &commitMessage,
			Content: []byte(newContent),
			SHA:     &sha,
			Branch:  &headBranch,
		})
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusConflict {
			return model.SuggestionCommitResult{}, fmt.Errorf(
				"github:conflict — the file has changed since this suggestion was made; please re-review the latest version")
		}
		return model.SuggestionCommitResult{}, fmt.Errorf("github:update file: %w", err)
	}

	return model.SuggestionCommitResult{
		CommitSHA: commitResp.GetSHA(),
		HTMLURL:   commitResp.GetHTMLURL(),
	}, nil
}

// fetchFileContent retrieves the decoded text content and SHA of a file from
// a GitHub repository at the given ref.
// Returns "notfound:" if the file does not exist, "github:" for other errors.
func fetchFileContent(ctx context.Context, client *github.Client, owner, repo, filePath, ref string) (content, sha string, err error) {
	fileContent, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, filePath,
		&github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", "", fmt.Errorf("notfound:file %s", filePath)
		}
		return "", "", fmt.Errorf("github:get file contents: %w", err)
	}
	decoded, err := fileContent.GetContent()
	if err != nil {
		return "", "", fmt.Errorf("github:decode file content: %w", err)
	}
	return decoded, fileContent.GetSHA(), nil
}

// parseSuggestionBlock extracts the content inside a ```suggestion fenced block
// from a GitHub review comment body. Returns the content and true on success.
func parseSuggestionBlock(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	var inside bool
	var result []string
	for _, line := range lines {
		stripped := strings.TrimRight(line, "\r")
		if !inside && strings.HasPrefix(stripped, "```suggestion") {
			inside = true
			continue
		}
		if inside && stripped == "```" {
			return strings.Join(result, "\n"), true
		}
		if inside {
			result = append(result, stripped)
		}
	}
	return "", false
}

// parseHunkTargetRange returns the 1-indexed [startLine, endLine] range in the
// new (HEAD branch) file that the suggestion should replace. It parses the @@
// header for the new-file start position and walks the diff lines, tracking the
// new-file line counter (incremented for context and added lines, skipped for
// removed lines). The range spans all consecutive added lines; if none are found
// the last context line is used as a single-line target.
func parseHunkTargetRange(diffHunk string) (startLine, endLine int, err error) {
	lines := strings.Split(diffHunk, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "@@") {
		return 0, 0, fmt.Errorf("missing @@ header")
	}

	newStart, _, parseErr := parseHunkHeader(lines[0])
	if parseErr != nil {
		return 0, 0, parseErr
	}

	newLine := newStart - 1
	startLine, endLine = -1, -1

	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "-") {
			// Exists only in old file; does not advance the new-file counter.
			continue
		}
		newLine++
		if strings.HasPrefix(l, "+") {
			if startLine == -1 {
				startLine = newLine
			}
			endLine = newLine
		}
	}

	if startLine == -1 {
		// No added lines found; target the last visible context line.
		endLine = newLine
		startLine = endLine
	}

	if startLine <= 0 {
		return 0, 0, fmt.Errorf("could not determine target line from diff hunk")
	}

	return startLine, endLine, nil
}

// parseHunkHeader extracts the new-file start and count from a unified diff
// @@ header. Handles both "@@ -A,B +C,D @@" and the short forms without a count.
func parseHunkHeader(header string) (newStart, newCount int, err error) {
	var a, b, c, d int
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d,%d", &a, &b, &c, &d); n == 4 {
		return c, d, nil
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d", &a, &b, &c); n == 3 {
		return c, 1, nil
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d,%d", &a, &c, &d); n == 3 {
		return c, d, nil
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d", &a, &c); n == 2 {
		return c, 1, nil
	}
	return 0, 0, fmt.Errorf("cannot parse hunk header: %q", header)
}

// applyPatch replaces lines [startLine, endLine] (1-indexed, inclusive) in
// fileContent with suggestionContent. The file's trailing-newline convention
// is preserved.
func applyPatch(fileContent string, startLine, endLine int, suggestionContent string) (string, error) {
	hasTrailing := strings.HasSuffix(fileContent, "\n")
	all := strings.Split(fileContent, "\n")
	// strings.Split of a newline-terminated string produces a phantom trailing "".
	if hasTrailing && len(all) > 0 && all[len(all)-1] == "" {
		all = all[:len(all)-1]
	}

	if startLine < 1 || endLine > len(all) || startLine > endLine {
		return "", fmt.Errorf("line range %d-%d out of bounds (file has %d lines)", startLine, endLine, len(all))
	}

	suggLines := strings.Split(suggestionContent, "\n")

	out := make([]string, 0, len(all)-(endLine-startLine+1)+len(suggLines))
	out = append(out, all[:startLine-1]...)
	out = append(out, suggLines...)
	out = append(out, all[endLine:]...)

	result := strings.Join(out, "\n")
	if hasTrailing {
		result += "\n"
	}
	return result, nil
}
