package ghapi

import (
	"testing"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

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
		Filename:  new("main.go"),
		Status:    new("modified"),
		Additions: new(2),
		Deletions: new(1),
		Changes:   new(3),
		Patch:     new(patch),
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
		Filename:  new("file.go"),
		Status:    new("modified"),
		Additions: new(2),
		Deletions: new(2),
		Changes:   new(4),
		Patch:     new(patch),
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
		Filename:  new("image.png"),
		Status:    new("modified"),
		Additions: new(0),
		Deletions: new(0),
		Changes:   new(1),
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
		Filename:  new("empty.go"),
		Status:    new("added"),
		Additions: new(0),
		Deletions: new(0),
		Changes:   new(0),
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
		Filename:  new("f.go"),
		Status:    new("modified"),
		Additions: new(1),
		Deletions: new(1),
		Changes:   new(2),
		Patch:     new(patch),
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
		Filename:         new("new.go"),
		PreviousFilename: new("old.go"),
		Status:           new("renamed"),
		Additions:        new(0),
		Deletions:        new(0),
		Changes:          new(0),
	}

	got := ParseCommitFileDiff(f)

	assert.Equal(t, "new.go", got.Filename)
	assert.Equal(t, "old.go", got.PreviousFilename)
	assert.Equal(t, "renamed", got.Status)
}

// TestPRFileFromCommitFile_PatchAndBinary_MapsFieldsAndDetectsBinary verifies that
// all DTO fields are populated and a patch-less file with changes counts as binary.
func TestPRFileFromCommitFile_PatchAndBinary_MapsFieldsAndDetectsBinary(t *testing.T) {
	text := PRFileFromCommitFile(&github.CommitFile{
		Filename: new("main.go"), Status: new("modified"),
		Additions: new(5), Deletions: new(2), Changes: new(7),
		Patch: new("@@ -1 +1 @@\n-old\n+new"),
	})
	assert.Equal(t, "main.go", text.Filename)
	assert.Equal(t, "modified", text.Status)
	assert.Equal(t, 5, text.Additions)
	assert.Equal(t, 2, text.Deletions)
	assert.Equal(t, 7, text.Changes)
	assert.False(t, text.IsBinary)

	bin := PRFileFromCommitFile(&github.CommitFile{
		Filename: new("logo.png"), Status: new("added"), Changes: new(1),
	})
	assert.Equal(t, "logo.png", bin.Filename)
	assert.True(t, bin.IsBinary)
}
