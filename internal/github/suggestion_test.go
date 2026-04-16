package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

// newTestClientForSuggestion creates a *github.Client pointing at the given
// httptest server. Reuses the same approach as newTestClientForPR.
func newTestClientForSuggestion(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// makeFileContentJSON builds a minimal GitHub REST /contents response for a
// text file encoded as standard base64.
func makeFileContentJSON(path, sha, rawText string) map[string]interface{} {
	return map[string]interface{}{
		"type":     "file",
		"encoding": "base64",
		"name":     path,
		"path":     path,
		"sha":      sha,
		// GitHub returns base64 with embedded newlines; go-github strips them.
		"content": base64.StdEncoding.EncodeToString([]byte(rawText)),
	}
}

// makeUpdateFileRespJSON builds a minimal GitHub REST /contents PUT response.
func makeUpdateFileRespJSON(commitSHA, htmlURL string) map[string]interface{} {
	return map[string]interface{}{
		"commit": map[string]interface{}{
			"sha":      commitSHA,
			"html_url": htmlURL,
		},
		"content": map[string]interface{}{
			"name": "test.go",
			"sha":  "new-content-sha",
		},
	}
}

// A realistic diff hunk showing one changed line on the HEAD branch.
// The PR changed line 2 from "old" to "changed"; new file has "changed" at line 2.
const singleLineDiffHunk = "@@ -1,3 +1,3 @@\n aaa\n-old\n+changed\n ccc"

// TestCommitSuggestion_Success_ReturnsCommitSHA verifies the happy path: file is
// fetched, suggestion applied, committed, and the commit SHA/URL are returned.
func TestCommitSuggestion_Success_ReturnsCommitSHA(t *testing.T) {
	// HEAD branch file content: line 2 already says "changed" (the PR's change).
	headFile := "aaa\nchanged\nccc\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			b, _ := json.Marshal(makeFileContentJSON("test.go", "sha-abc", headFile))
			_, _ = fmt.Fprint(w, string(b))
		case http.MethodPut:
			b, _ := json.Marshal(makeUpdateFileRespJSON("commit-sha-123", "https://github.com/owner/repo/commit/commit-sha-123"))
			_, _ = fmt.Fprint(w, string(b))
		}
	}))
	defer srv.Close()

	comment := model.CommentDTO{
		ID:           1,
		IsSuggestion: true,
		Body:         "```suggestion\nbetter\n```",
		DiffHunk:     singleLineDiffHunk,
	}

	result, err := CommitSuggestion(
		context.Background(),
		newTestClientForSuggestion(srv),
		"owner", "repo", "main", "test.go",
		comment, "Apply suggestion",
	)
	require.NoError(t, err)
	assert.Equal(t, "commit-sha-123", result.CommitSHA)
	assert.Equal(t, "https://github.com/owner/repo/commit/commit-sha-123", result.HTMLURL)
}

// TestCommitSuggestion_Conflict_ReturnsConflictError verifies that an HTTP 409
// from UpdateFile is reported as a "github:conflict" prefixed error.
func TestCommitSuggestion_Conflict_ReturnsConflictError(t *testing.T) {
	headFile := "aaa\nchanged\nccc\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			b, _ := json.Marshal(makeFileContentJSON("test.go", "sha-abc", headFile))
			_, _ = fmt.Fprint(w, string(b))
		case http.MethodPut:
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"message":"409 Conflict"}`)
		}
	}))
	defer srv.Close()

	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "```suggestion\nbetter\n```",
		DiffHunk:     singleLineDiffHunk,
	}

	_, err := CommitSuggestion(
		context.Background(),
		newTestClientForSuggestion(srv),
		"owner", "repo", "main", "test.go",
		comment, "Apply suggestion",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:conflict")
}

// TestCommitSuggestion_NotASuggestion_ReturnsValidationError verifies that a
// comment with IsSuggestion=false immediately returns a "validation:" error
// without making any HTTP calls.
func TestCommitSuggestion_NotASuggestion_ReturnsValidationError(t *testing.T) {
	comment := model.CommentDTO{IsSuggestion: false}

	_, err := CommitSuggestion(context.Background(), nil, "o", "r", "main", "f.go", comment, "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation:")
	assert.Contains(t, err.Error(), "not-a-suggestion")
}

// TestCommitSuggestion_MissingHunkHeader_ReturnsGithubError verifies that a
// malformed diff_hunk (no @@ header) is reported as a "github:" error.
func TestCommitSuggestion_MissingHunkHeader_ReturnsGithubError(t *testing.T) {
	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "```suggestion\nfixed\n```",
		DiffHunk:     "no @@ header here",
	}

	_, err := CommitSuggestion(context.Background(), nil, "o", "r", "main", "f.go", comment, "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestCommitSuggestion_MultiLineSuggestion_AppliesCorrectRange verifies that a
// suggestion spanning multiple added lines replaces the correct range in the file.
func TestCommitSuggestion_MultiLineSuggestion_AppliesCorrectRange(t *testing.T) {
	// HEAD branch file: PR changed lines 2-3 from "old2/old3" to "new2/new3".
	headFile := "line1\nnew2\nnew3\nline4\nline5\n"

	// Diff hunk: old file had old2/old3; PR replaced with new2/new3.
	multiHunk := "@@ -1,5 +1,5 @@\n line1\n-old2\n-old3\n+new2\n+new3\n line4\n line5"

	var capturedContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			b, _ := json.Marshal(makeFileContentJSON("file.go", "sha-abc", headFile))
			_, _ = fmt.Fprint(w, string(b))
		case http.MethodPut:
			var putBody struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err == nil {
				decoded, _ := base64.StdEncoding.DecodeString(putBody.Content)
				capturedContent = string(decoded)
			}
			b, _ := json.Marshal(makeUpdateFileRespJSON("multi-commit-sha", "https://github.com/o/r/commit/multi-commit-sha"))
			_, _ = fmt.Fprint(w, string(b))
		}
	}))
	defer srv.Close()

	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "```suggestion\nbetter2\nbetter3\n```",
		DiffHunk:     multiHunk,
	}

	result, err := CommitSuggestion(
		context.Background(),
		newTestClientForSuggestion(srv),
		"o", "r", "main", "file.go",
		comment, "Apply multi-line suggestion",
	)
	require.NoError(t, err)
	assert.Equal(t, "multi-commit-sha", result.CommitSHA)
	// Verify lines 2-3 were replaced with the suggestion content.
	assert.Equal(t, "line1\nbetter2\nbetter3\nline4\nline5\n", capturedContent)
}

// TestCommitSuggestion_NoSuggestionBlock_ReturnsValidationError verifies that
// IsSuggestion=true but missing fenced block returns a "validation:" error.
func TestCommitSuggestion_NoSuggestionBlock_ReturnsValidationError(t *testing.T) {
	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "just a regular comment without a suggestion block",
		DiffHunk:     singleLineDiffHunk,
	}

	_, err := CommitSuggestion(context.Background(), nil, "o", "r", "main", "f.go", comment, "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation:")
}

// TestCommitSuggestion_FileNotFound_ReturnsNotFoundError verifies that a 404
// from the file contents endpoint is reported as a "notfound:" error.
func TestCommitSuggestion_FileNotFound_ReturnsNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
		}
	}))
	defer srv.Close()

	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "```suggestion\nbetter\n```",
		DiffHunk:     singleLineDiffHunk,
	}

	_, err := CommitSuggestion(
		context.Background(),
		newTestClientForSuggestion(srv),
		"owner", "repo", "main", "missing.go",
		comment, "Apply suggestion",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notfound:")
}

// TestCommitSuggestion_UpdateFileError_ReturnsGithubError verifies that a
// non-conflict server error on UpdateFile is wrapped with a "github:" prefix.
func TestCommitSuggestion_UpdateFileError_ReturnsGithubError(t *testing.T) {
	headFile := "aaa\nchanged\nccc\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			b, _ := json.Marshal(makeFileContentJSON("test.go", "sha-abc", headFile))
			_, _ = fmt.Fprint(w, string(b))
		case http.MethodPut:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"message":"Internal Server Error"}`)
		}
	}))
	defer srv.Close()

	comment := model.CommentDTO{
		IsSuggestion: true,
		Body:         "```suggestion\nbetter\n```",
		DiffHunk:     singleLineDiffHunk,
	}

	_, err := CommitSuggestion(
		context.Background(),
		newTestClientForSuggestion(srv),
		"owner", "repo", "main", "test.go",
		comment, "Apply suggestion",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestParseHunkHeader_ShortForms verifies all four supported @@ header formats.
func TestParseHunkHeader_ShortForms(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantStart  int
		wantCount  int
		wantErrStr string
	}{
		{
			name:      "full form -A,B +C,D",
			header:    "@@ -1,3 +5,4 @@",
			wantStart: 5,
			wantCount: 4,
		},
		{
			name:      "no old count -A +C,D",
			header:    "@@ -1 +5,4 @@",
			wantStart: 5,
			wantCount: 4,
		},
		{
			name:      "no new count -A,B +C",
			header:    "@@ -1,3 +5 @@",
			wantStart: 5,
			wantCount: 1,
		},
		{
			name:      "minimal -A +C",
			header:    "@@ -1 +5 @@",
			wantStart: 5,
			wantCount: 1,
		},
		{
			name:       "unparseable",
			header:     "not a hunk header",
			wantErrStr: "cannot parse hunk header",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, count, err := parseHunkHeader(tc.header)
			if tc.wantErrStr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrStr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantCount, count)
		})
	}
}
