package github

import (
	"context"
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

func newTestClientForDiff(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// TestParseCommitFileDiff_SingleHunk verifies that a single-hunk patch is parsed
// correctly into context/add/delete lines with proper line numbers.
func TestParseCommitFileDiff_SingleHunk_ParsesAllLineTypes(t *testing.T) {
	// @@ -1,3 +1,4 @@
	//  context        (old=1, new=1)
	// -old line       (old=2)
	// +new line       (new=2)
	// +extra line     (new=3)
	//  context2       (old=3, new=4)
	patch := "@@ -1,3 +1,4 @@\n context\n-old line\n+new line\n+extra line\n context2"
	f := &github.CommitFile{
		Filename:  github.String("main.go"),
		Status:    github.String("modified"),
		Additions: github.Int(2),
		Deletions: github.Int(1),
		Changes:   github.Int(3),
		Patch:     github.String(patch),
	}

	got := ParseCommitFileDiff(f)

	assert.Equal(t, "main.go", got.Filename)
	assert.Equal(t, "modified", got.Status)
	assert.Equal(t, 2, got.TotalAdditions)
	assert.Equal(t, 1, got.TotalDeletions)
	assert.False(t, got.IsBinary)
	require.Len(t, got.Hunks, 1)

	h := got.Hunks[0]
	assert.Equal(t, "@@ -1,3 +1,4 @@", h.Header)
	assert.Equal(t, 1, h.OldStart)
	assert.Equal(t, 3, h.OldLines)
	assert.Equal(t, 1, h.NewStart)
	assert.Equal(t, 4, h.NewLines)
	require.Len(t, h.Lines, 5)

	// context line
	assert.Equal(t, model.DiffLineContext, h.Lines[0].Type)
	assert.Equal(t, 1, h.Lines[0].OldNo)
	assert.Equal(t, 1, h.Lines[0].NewNo)
	assert.Equal(t, "context", h.Lines[0].Content)

	// delete line
	assert.Equal(t, model.DiffLineDelete, h.Lines[1].Type)
	assert.Equal(t, 2, h.Lines[1].OldNo)
	assert.Equal(t, 0, h.Lines[1].NewNo)
	assert.Equal(t, "old line", h.Lines[1].Content)

	// add line 1
	assert.Equal(t, model.DiffLineAdd, h.Lines[2].Type)
	assert.Equal(t, 0, h.Lines[2].OldNo)
	assert.Equal(t, 2, h.Lines[2].NewNo)
	assert.Equal(t, "new line", h.Lines[2].Content)

	// add line 2
	assert.Equal(t, model.DiffLineAdd, h.Lines[3].Type)
	assert.Equal(t, 0, h.Lines[3].OldNo)
	assert.Equal(t, 3, h.Lines[3].NewNo)
	assert.Equal(t, "extra line", h.Lines[3].Content)

	// context line 2
	assert.Equal(t, model.DiffLineContext, h.Lines[4].Type)
	assert.Equal(t, 3, h.Lines[4].OldNo)
	assert.Equal(t, 4, h.Lines[4].NewNo)
	assert.Equal(t, "context2", h.Lines[4].Content)
}

// TestParseCommitFileDiff_MultipleHunks verifies that a patch with two @@ blocks
// produces two separate hunks with correct start line numbers.
func TestParseCommitFileDiff_MultipleHunks_TwoHunksCreated(t *testing.T) {
	patch := "@@ -1,2 +1,2 @@\n line1\n-old1\n+new1\n@@ -10,2 +10,2 @@\n ctx\n-old2\n+new2"
	f := &github.CommitFile{
		Filename:  github.String("file.go"),
		Status:    github.String("modified"),
		Additions: github.Int(2),
		Deletions: github.Int(2),
		Changes:   github.Int(4),
		Patch:     github.String(patch),
	}

	got := ParseCommitFileDiff(f)

	require.Len(t, got.Hunks, 2)
	assert.Equal(t, 1, got.Hunks[0].OldStart)
	assert.Equal(t, 1, got.Hunks[0].NewStart)
	assert.Equal(t, 10, got.Hunks[1].OldStart)
	assert.Equal(t, 10, got.Hunks[1].NewStart)
}

// TestParseCommitFileDiff_BinaryFile verifies that a file with no patch but
// non-zero changes count is flagged as binary and has no hunks.
func TestParseCommitFileDiff_BinaryFile_FlaggedAsBinary(t *testing.T) {
	f := &github.CommitFile{
		Filename:  github.String("image.png"),
		Status:    github.String("modified"),
		Additions: github.Int(0),
		Deletions: github.Int(0),
		Changes:   github.Int(1),
		// Patch intentionally nil
	}

	got := ParseCommitFileDiff(f)

	assert.True(t, got.IsBinary)
	assert.Empty(t, got.Hunks)
}

// TestParseCommitFileDiff_EmptyFile verifies that an empty patch with zero
// changes is not flagged as binary and has no hunks (e.g. an empty new file).
func TestParseCommitFileDiff_EmptyFile_NotBinary(t *testing.T) {
	f := &github.CommitFile{
		Filename:  github.String("empty.go"),
		Status:    github.String("added"),
		Additions: github.Int(0),
		Deletions: github.Int(0),
		Changes:   github.Int(0),
	}

	got := ParseCommitFileDiff(f)

	assert.False(t, got.IsBinary)
	assert.Empty(t, got.Hunks)
}

// TestParseCommitFileDiff_NoNewlineMarker verifies that "\ No newline at end of
// file" markers are silently skipped and do not produce diff lines.
func TestParseCommitFileDiff_NoNewlineMarker_Skipped(t *testing.T) {
	patch := "@@ -1,1 +1,1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file"
	f := &github.CommitFile{
		Filename:  github.String("f.go"),
		Status:    github.String("modified"),
		Additions: github.Int(1),
		Deletions: github.Int(1),
		Changes:   github.Int(2),
		Patch:     github.String(patch),
	}

	got := ParseCommitFileDiff(f)

	require.Len(t, got.Hunks, 1)
	require.Len(t, got.Hunks[0].Lines, 2)
	assert.Equal(t, model.DiffLineDelete, got.Hunks[0].Lines[0].Type)
	assert.Equal(t, model.DiffLineAdd, got.Hunks[0].Lines[1].Type)
}

// TestParseCommitFileDiff_RenamedFile verifies that PreviousFilename is populated
// for renamed files.
func TestParseCommitFileDiff_RenamedFile_PreviousFilenameSet(t *testing.T) {
	f := &github.CommitFile{
		Filename:         github.String("new.go"),
		PreviousFilename: github.String("old.go"),
		Status:           github.String("renamed"),
		Additions:        github.Int(0),
		Deletions:        github.Int(0),
		Changes:          github.Int(0),
	}

	got := ParseCommitFileDiff(f)

	assert.Equal(t, "new.go", got.Filename)
	assert.Equal(t, "old.go", got.PreviousFilename)
	assert.Equal(t, "renamed", got.Status)
}

// TestParseDiffHunkHeader_AllForms verifies all four @@ header formats including
// the optional trailing function-name context.
func TestParseDiffHunkHeader_AllForms(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantOldSt int
		wantOldLn int
		wantNewSt int
		wantNewLn int
	}{
		{"full form -A,B +C,D", "@@ -3,7 +3,9 @@ func foo()", 3, 7, 3, 9},
		{"no old count -A +C,D", "@@ -3 +3,9 @@", 3, 1, 3, 9},
		{"no new count -A,B +C", "@@ -3,7 +3 @@", 3, 7, 3, 1},
		{"minimal -A +C", "@@ -3 +3 @@", 3, 1, 3, 1},
		{"unparseable", "not a header", 0, 0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os, ol, ns, nl := parseDiffHunkHeader(tc.header)
			assert.Equal(t, tc.wantOldSt, os)
			assert.Equal(t, tc.wantOldLn, ol)
			assert.Equal(t, tc.wantNewSt, ns)
			assert.Equal(t, tc.wantNewLn, nl)
		})
	}
}

// TestListPRFiles_Success verifies that files are fetched, binary detection
// works, and all DTO fields are populated correctly.
func TestListPRFiles_Success_MapsFieldsAndDetectsBinary(t *testing.T) {
	files := []map[string]interface{}{
		{
			"filename": "main.go", "status": "modified",
			"additions": 5, "deletions": 2, "changes": 7,
			"patch": "@@ -1 +1 @@\n-old\n+new",
		},
		{
			// Binary file: has changes but no patch.
			"filename": "logo.png", "status": "added",
			"additions": 0, "deletions": 0, "changes": 1,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(files)
		_, _ = fmt.Fprint(w, string(b))
	}))
	defer srv.Close()

	result, err := ListPRFiles(context.Background(), newTestClientForDiff(srv), "owner", "repo", 1)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "main.go", result[0].Filename)
	assert.Equal(t, "modified", result[0].Status)
	assert.Equal(t, 5, result[0].Additions)
	assert.Equal(t, 2, result[0].Deletions)
	assert.Equal(t, 7, result[0].Changes)
	assert.False(t, result[0].IsBinary)

	assert.Equal(t, "logo.png", result[1].Filename)
	assert.True(t, result[1].IsBinary)
}

// TestListPRFiles_APIError verifies that an API error is wrapped with "github:".
func TestListPRFiles_APIError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer srv.Close()

	_, err := ListPRFiles(context.Background(), newTestClientForDiff(srv), "owner", "repo", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestGetFileDiff_Success finds the target file among multiple files and returns
// a parsed diff.
func TestGetFileDiff_Success_ReturnsTargetFileDiff(t *testing.T) {
	files := []map[string]interface{}{
		{
			"filename": "other.go", "status": "modified",
			"additions": 1, "deletions": 0, "changes": 1,
			"patch": "@@ -1 +1 @@\n+added",
		},
		{
			"filename": "target.go", "status": "modified",
			"additions": 2, "deletions": 1, "changes": 3,
			"patch": "@@ -1,2 +1,3 @@\n ctx\n-del\n+add1\n+add2",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(files)
		_, _ = fmt.Fprint(w, string(b))
	}))
	defer srv.Close()

	got, err := GetFileDiff(context.Background(), newTestClientForDiff(srv), "owner", "repo", 1, "target.go")
	require.NoError(t, err)
	assert.Equal(t, "target.go", got.Filename)
	assert.Equal(t, 2, got.TotalAdditions)
	assert.Equal(t, 1, got.TotalDeletions)
	require.Len(t, got.Hunks, 1)
}

// TestGetFileDiff_NotFound verifies that a missing filename returns a "notfound:" error.
func TestGetFileDiff_NotFound_ReturnsNotFoundPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := GetFileDiff(context.Background(), newTestClientForDiff(srv), "owner", "repo", 1, "missing.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notfound:")
}

// TestGetFileDiff_APIError verifies that an API error is propagated with "github:".
func TestGetFileDiff_APIError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"message":"Bad credentials"}`)
	}))
	defer srv.Close()

	_, err := GetFileDiff(context.Background(), newTestClientForDiff(srv), "owner", "repo", 1, "f.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}
