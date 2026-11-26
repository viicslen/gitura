package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestLocalReview_TrackedDirBecomesSubmodule_DiffsViewedAndDraftsPerFile(t *testing.T) {
	root := t.TempDir()
	sub, dir := filepath.Join(root, "sub"), filepath.Join(root, "parent")
	gitRun(t, root, "init", "-q", "-b", "main", sub)
	gitRun(t, root, "init", "-q", "-b", "main", dir)
	require.NoError(t, os.WriteFile(filepath.Join(sub, "a.txt"), []byte("sub a\n"), 0o644))
	gitRun(t, sub, "add", ".")
	gitRun(t, sub, "commit", "-q", "-m", "sub")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "a.txt"), []byte("old a\n"), 0o644))
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	gitRun(t, dir, "rm", "-rq", "lib")
	gitRun(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "lib")

	app := &App{ctx: context.Background()}
	_, err := app.StartLocalReview(dir, "HEAD")
	require.NoError(t, err)
	t.Cleanup(app.StopLocalReview)

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Submodule)
	}
	assert.Equal(t, []string{".gitmodules ", "lib lib", "lib/a.txt lib", "lib/a.txt "}, rows)

	inner, err := app.GetFileDiff("lib", "lib/a.txt", false)
	require.NoError(t, err)
	own, err := app.GetFileDiff("", "lib/a.txt", false)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 0}, []int{inner.TotalAdditions, inner.TotalDeletions})
	assert.Equal(t, []int{0, 1}, []int{own.TotalAdditions, own.TotalDeletions})

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/a.txt", false, true)))
	viewed, err := app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Submodule: "lib", Path: "lib/a.txt"}}, viewed.Files)

	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"})
	require.NoError(t, err)
	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "nope", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"})
	require.ErrorContains(t, err, "validation:")
}

func TestLocalReview_ParentFileAndNestedPointerSharePath_SeparateRows(t *testing.T) {
	root := t.TempDir()
	nest, sub, dir := filepath.Join(root, "nest"), filepath.Join(root, "sub"), filepath.Join(root, "parent")
	for _, d := range []string{nest, sub, dir} {
		gitRun(t, root, "init", "-q", "-b", "main", d)
	}
	require.NoError(t, os.WriteFile(filepath.Join(nest, "n.txt"), []byte("n\n"), 0o644))
	gitRun(t, nest, "add", ".")
	gitRun(t, nest, "commit", "-q", "-m", "nest")
	gitRun(t, sub, "-c", "protocol.file.allow=always", "submodule", "add", "-q", nest, "nest")
	gitRun(t, sub, "commit", "-q", "-m", "sub")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "nest"), []byte("old\n"), 0o644))
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	gitRun(t, dir, "rm", "-rq", "lib")
	gitRun(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "lib")
	gitRun(t, dir, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init", "--recursive")

	app := &App{ctx: context.Background()}
	_, err := app.StartLocalReview(dir, "HEAD")
	require.NoError(t, err)
	t.Cleanup(app.StopLocalReview)

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Status+" "+f.Repo)
	}
	assert.Subset(t, rows, []string{"lib/nest removed ", "lib/nest submodule lib"})

	own, err := app.GetFileDiff("", "lib/nest", false)
	require.NoError(t, err)
	pointer, err := app.GetFileDiff("lib", "lib/nest", true)
	require.NoError(t, err)
	assert.Equal(t, "removed", own.Status)
	assert.Equal(t, "submodule", pointer.Status)

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/nest", true, true)))
	viewed, err := app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Submodule: "lib", Path: "lib/nest", Pointer: true}}, viewed.Files)

	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Path: "lib/nest", Pointer: true, Line: 1, Side: "RIGHT", Body: "pointer"})
	require.NoError(t, err)
	review, err := app.AddDraftComment(model.DraftCommentDTO{Path: "lib/nest", Line: 1, Side: "LEFT", Body: "own"})
	require.NoError(t, err)
	var drafts []string
	for _, d := range review.Comments {
		drafts = append(drafts, d.Submodule+":"+d.Body)
	}
	assert.Equal(t, []string{"lib:pointer", ":own"}, drafts)
}

// typeChangeRepo is a repository whose committed lib and work-tree lib differ in type: a submodule replaced by
// a regular file, or with fileFirst the reverse.
func typeChangeRepo(t *testing.T, fileFirst bool) string {
	t.Helper()
	root := t.TempDir()
	sub, dir := filepath.Join(root, "sub"), filepath.Join(root, "parent")
	gitRun(t, root, "init", "-q", "-b", "main", sub)
	gitRun(t, root, "init", "-q", "-b", "main", dir)
	require.NoError(t, os.WriteFile(filepath.Join(sub, "a.txt"), []byte("sub a\n"), 0o644))
	gitRun(t, sub, "add", ".")
	gitRun(t, sub, "commit", "-q", "-m", "sub")
	addFile := func() {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "lib"), []byte("one\ntwo\n"), 0o644))
		gitRun(t, dir, "add", "lib")
	}
	addSubmodule := func() {
		gitRun(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "lib")
	}
	first, second := addSubmodule, addFile
	if fileFirst {
		first, second = addFile, addSubmodule
	}
	first()
	gitRun(t, dir, "commit", "-q", "-m", "init")
	gitRun(t, dir, "rm", "-rqf", "lib")
	require.NoError(t, os.RemoveAll(filepath.Join(dir, "lib")))
	second()
	return dir
}

func TestLocalReview_SubmoduleAndFileSwapAtOnePath_EachRowHasOwnDiffViewedAndDrafts(t *testing.T) {
	for name, fileFirst := range map[string]bool{"submodule replaced by file": false, "file replaced by submodule": true} {
		t.Run(name, func(t *testing.T) {
			app := &App{ctx: context.Background()}
			_, err := app.StartLocalReview(typeChangeRepo(t, fileFirst), "HEAD")
			require.NoError(t, err)
			t.Cleanup(app.StopLocalReview)

			files, err := app.GetPRFiles()
			require.NoError(t, err)
			var rows []string
			for _, f := range files {
				if f.Filename == "lib" {
					rows = append(rows, f.Status+" "+f.Repo)
				}
			}
			assert.ElementsMatch(t, []string{"submodule ", map[bool]string{false: "added ", true: "removed "}[fileFirst]}, rows)

			pointer, err := app.GetFileDiff("", "lib", true)
			require.NoError(t, err)
			file, err := app.GetFileDiff("", "lib", false)
			require.NoError(t, err)
			assert.Equal(t, "submodule", pointer.Status)
			assert.Equal(t, 2, file.TotalAdditions+file.TotalDeletions)

			require.NoError(t, viewedErr(app.SetFileViewed("", "lib", true, true)))
			viewed, err := app.GetViewedFiles()
			require.NoError(t, err)
			assert.Equal(t, []model.FileKeyDTO{{Path: "lib", Pointer: true}}, viewed.Files)
			require.NoError(t, viewedErr(app.SetFileViewed("", "lib", false, true)))
			require.NoError(t, viewedErr(app.SetFileViewed("", "lib", true, false)))
			viewed, err = app.GetViewedFiles()
			require.NoError(t, err)
			assert.Equal(t, []model.FileKeyDTO{{Path: "lib"}}, viewed.Files)

			_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "lib", Pointer: true, Line: 1, Side: "RIGHT", Body: "pointer"})
			require.NoError(t, err)
			review, err := app.AddDraftComment(model.DraftCommentDTO{Path: "lib", Line: 1, Side: "RIGHT", Body: "file"})
			require.NoError(t, err)
			var drafts []string
			for _, d := range review.Comments {
				drafts = append(drafts, fmt.Sprint(d.Pointer, ":", d.Body))
			}
			assert.Equal(t, []string{"true:pointer", "false:file"}, drafts)
		})
	}
}

func TestLocalReview_RefreshDuringDraftsViewedAndRestarts_NoRace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	gitRun(t, root, "init", "-q", "-b", "main", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644))
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0o644))

	app := &App{ctx: context.Background()}
	_, err := app.StartLocalReview(dir, "HEAD")
	require.NoError(t, err)
	t.Cleanup(app.StopLocalReview)
	_, err = app.GetPRFiles()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 5 {
			_, _ = app.GetPRFiles()
		}
	}()
	for i := range 5 {
		_, err := app.AddDraftComment(model.DraftCommentDTO{Path: "a.txt", Line: 1, Side: "RIGHT", Body: "x"})
		require.NoError(t, err)
		_, _ = app.GetFileDiff("", "a.txt", false)
		_, _ = app.SetFileViewed("", "a.txt", false, true)
		_, _ = app.GetViewedFiles()
		if i%2 == 0 {
			app.StopLocalReview()
		}
		_, err = app.StartLocalReview(dir, "HEAD")
		require.NoError(t, err)
	}
	<-done
}
