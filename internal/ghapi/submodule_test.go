package ghapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmoduleBump_Patches_ReadsOldAndNew(t *testing.T) {
	cases := []struct {
		patch, old, new string
		ok              bool
	}{
		{"@@ -1 +1 @@\n-Subproject commit a5\n+Subproject commit 29", "a5", "29", true},
		{"@@ -0,0 +1 @@\n+Subproject commit 29", "", "29", true},
		{"@@ -1 +1 @@\n-Subproject commit a5\n+x", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		from, to, ok := SubmoduleBump(&github.CommitFile{Patch: &c.patch})
		if from != c.old || to != c.new || ok != c.ok {
			t.Errorf("%q: got %q %q %v", c.patch, from, to, ok)
		}
	}
}

func TestInvertCommitFile_NewToOldDiff_ReadsOldToNew(t *testing.T) {
	f := &github.CommitFile{
		Filename: new("a.txt"), PreviousFilename: new("b.txt"), Status: new("renamed"),
		Additions: new(2), Deletions: new(1),
		Patch: new("@@ -10,3 +10,4 @@ func f()\n ctx\n-gone\n+one\n+two\n ctx\n@@ -20 +21,0 @@\n-x\n\\ No newline at end of file"),
	}

	InvertCommitFile(f)

	assert.Equal(t, "b.txt", f.GetFilename())
	assert.Equal(t, "a.txt", f.GetPreviousFilename())
	assert.Equal(t, 1, f.GetAdditions())
	assert.Equal(t, 2, f.GetDeletions())
	assert.Equal(t, "@@ -10,4 +10,3 @@ func f()\n ctx\n-one\n-two\n+gone\n ctx\n@@ -21,0 +20 @@\n+x\n\\ No newline at end of file", f.GetPatch())

	added := &github.CommitFile{Status: new("added")}
	InvertCommitFile(added)
	assert.Equal(t, "removed", added.GetStatus())
}

func serve(t *testing.T, h http.HandlerFunc) *github.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newTestClient(t, srv)
}

func TestFetchSubmoduleChanges_BackwardsBump_ComparesReversedAndInverts(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"behind","files":[]}`))
		case "/repos/so/sr/compare/new...old":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"added","additions":1,"patch":"@@ -0,0 +1 @@\n+a"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "old", "new")
	require.NoError(t, err)
	assert.True(t, ch.Backwards)
	assert.Equal(t, "so/sr", ch.Owner+"/"+ch.Repo)
	require.Len(t, ch.Files, 1)
	assert.Equal(t, "lib/a.txt", ch.Files[0].GetFilename())
	assert.Equal(t, "removed", ch.Files[0].GetStatus())
	assert.Equal(t, "@@ -1 +0,0 @@\n-a", ch.Files[0].GetPatch())
}

func TestFetchSubmoduleChanges_Gaps_ReportsUnavailable(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/gh":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/private/tree/b"}`))
		case "/repos/o/r/contents/off":
			_, _ = w.Write([]byte(`{"type":"submodule","submodule_git_url":"https://gitlab.com/x/y.git"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	for path, want := range map[string]string{"gh": "no access", "off": "not on GitHub"} {
		ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", path, "a", "b")
		require.NoError(t, err)
		assert.Equal(t, want, ch.Unavailable, path)
	}
	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "gh", "", "b")
	require.NoError(t, err)
	assert.Equal(t, "no access", ch.Unavailable, "an added submodule's tree is out of reach too")
}

func TestFetchSubmoduleChanges_AddedOrRemovedSubmodule_ListsWholeTree(t *testing.T) {
	truncated := "false"
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/x"}`))
		case "/repos/so/sr/git/trees/new", "/repos/so/sr/git/trees/old":
			assert.Equal(t, "1", r.URL.Query().Get("recursive"))
			_, _ = w.Write([]byte(`{"truncated":` + truncated + `,"tree":[
				{"path":"src","type":"tree"},
				{"path":"src/a.txt","type":"blob","sha":"b1"},
				{"path":"vendor","type":"commit","sha":"c1"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	for to, status := range map[string]string{"new": "added", "": "removed"} {
		from := map[string]string{"new": "", "": "old"}[to]
		ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", from, to)
		require.NoError(t, err)
		require.Len(t, ch.Files, 1, status)
		assert.Equal(t, "lib/src/a.txt", ch.Files[0].GetFilename())
		assert.Equal(t, status, ch.Files[0].GetStatus())
		assert.Equal(t, "b1", ch.Files[0].GetSHA())
		assert.Equal(t, from+to, ch.TreeRef)
		assert.Empty(t, ch.Unavailable)
	}

	truncated = "true"
	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "", "new")
	require.NoError(t, err)
	assert.Equal(t, "too many files", ch.Unavailable)
}

// serveBlobs answers GitHub's raw blob requests for so/sr from blobs, by SHA.
func serveBlobs(t *testing.T, blobs map[string]string) *github.Client {
	t.Helper()
	return serve(t, func(w http.ResponseWriter, r *http.Request) {
		b, ok := blobs[strings.TrimPrefix(r.URL.Path, "/repos/so/sr/git/blobs/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(b))
	})
}

func TestFillWholeFile_TextSymlinkBinaryAndRemoved_BuildsOneSidedPatchOrMarksBinary(t *testing.T) {
	client := serveBlobs(t, map[string]string{"a1": "x\ny\n", "l1": "../target", "big": strings.Repeat("x\n", maxBlobText/2+1)})
	ctx := context.Background()

	added := &github.CommitFile{Filename: new("lib/a.txt"), Status: new("added")}
	require.NoError(t, FillWholeFile(ctx, client, "so", "sr", "a1", added))
	assert.Equal(t, "@@ -0,0 +1,2 @@\n+x\n+y", added.GetPatch())
	assert.Equal(t, 2, added.GetAdditions())

	removed := &github.CommitFile{Filename: new("lib/a.txt"), Status: new("removed")}
	require.NoError(t, FillWholeFile(ctx, client, "so", "sr", "a1", removed))
	assert.Equal(t, "@@ -1,2 +0,0 @@\n-x\n-y", removed.GetPatch())

	link := &github.CommitFile{Filename: new("lib/link"), Status: new("added")}
	require.NoError(t, FillWholeFile(ctx, client, "so", "sr", "l1", link))
	assert.Equal(t, "@@ -0,0 +1 @@\n+../target\n\\ No newline at end of file", link.GetPatch(), "a symlink shows its target")

	big := &github.CommitFile{Filename: new("lib/big.txt"), Status: new("added")}
	require.NoError(t, FillWholeFile(ctx, client, "so", "sr", "big", big))
	assert.True(t, ParseCommitFileDiff(big).IsBinary)
}

func TestFindOpenPRForCommit_ClosedOrOtherHead_OnlyMatchesOpenPRAtSHA(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"number":1,"state":"closed","head":{"sha":"abc"}},
			{"number":2,"state":"open","head":{"sha":"later"}},
			{"number":3,"state":"open","head":{"sha":"abc"}}]`))
	})

	pr, err := FindOpenPRForCommit(context.Background(), client, "o", "r", "abc")
	require.NoError(t, err)
	assert.Equal(t, 3, pr.GetNumber())

	pr, err = FindOpenPRForCommit(context.Background(), client, "o", "r", "zzz")
	require.NoError(t, err)
	assert.Nil(t, pr)
}

func TestFetchSubmoduleChanges_DivergedBump_MergesBothSidesIntoOldToNewDiff(t *testing.T) {
	blobs := map[string]string{"b1": "a\nold\n", "b2": "a\nnew\n", "bigold": strings.Repeat("x", maxBlobText+1), "bignew": "\x00\x01", "e2": ""}
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		blob, isBlob := blobs[strings.TrimPrefix(r.URL.Path, "/repos/so/sr/git/blobs/")]
		switch {
		case r.URL.Path == "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case r.URL.Path == "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[
				{"filename":"new-only.txt","status":"added","additions":1,"changes":1,"patch":"@@ -0,0 +1 @@\n+n"},
				{"filename":"both.txt","status":"modified","patch":"@@ -1,2 +1,2 @@\n a\n-base\n+new"},
				{"filename":"same.txt","status":"modified","patch":"@@ -1 +1 @@\n-base\n+x"},
				{"filename":"big.bin","status":"modified","changes":1,"sha":"bignew"},
				{"filename":"empty.txt","status":"modified","sha":"e2"},
				{"filename":"run.sh","status":"modified","sha":"r"}]}`))
		case r.URL.Path == "/repos/so/sr/compare/new...old":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[
				{"filename":"old-only.txt","status":"added","additions":1,"changes":1,"patch":"@@ -0,0 +1 @@\n+o"},
				{"filename":"empty.txt","status":"removed","deletions":1,"patch":"@@ -1 +0,0 @@\n-base"},
				{"filename":"both.txt","status":"modified","patch":"@@ -1,2 +1,2 @@\n a\n-base\n+old"},
				{"filename":"same.txt","status":"modified","patch":"@@ -1 +1 @@\n-base\n+x"},
				{"filename":"big.bin","status":"modified","changes":1,"sha":"bigold"}]}`))
		case r.URL.Path == "/repos/so/sr/git/trees/old":
			_, _ = w.Write([]byte(`{"tree":[{"path":"both.txt","type":"blob","sha":"b1"},{"path":"same.txt","type":"blob","sha":"s"},
				{"path":"big.bin","type":"blob","sha":"bigold"},{"path":"old-only.txt","type":"blob","sha":"o"},
				{"path":"run.sh","type":"blob","mode":"100644","sha":"r"}]}`))
		case r.URL.Path == "/repos/so/sr/git/trees/new":
			_, _ = w.Write([]byte(`{"tree":[{"path":"both.txt","type":"blob","sha":"b2"},{"path":"same.txt","type":"blob","sha":"s"},
				{"path":"big.bin","type":"blob","sha":"bignew"},{"path":"new-only.txt","type":"blob","sha":"n"},{"path":"empty.txt","type":"blob","sha":"e2"},
				{"path":"run.sh","type":"blob","mode":"100755","sha":"r"}]}`))
		case isBlob:
			_, _ = w.Write([]byte(blob))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "old", "new")
	require.NoError(t, err)
	assert.True(t, ch.Diverged)
	got := map[string]*github.CommitFile{}
	var names []string
	for _, f := range ch.Files {
		got[f.GetFilename()], names = f, append(names, f.GetFilename())
	}
	assert.Equal(t, []string{"lib/big.bin", "lib/both.txt", "lib/empty.txt", "lib/new-only.txt", "lib/old-only.txt", "lib/run.sh"}, names, "same.txt ends where it started; run.sh only changed mode")
	assert.Equal(t, "added", got["lib/empty.txt"].GetStatus(), "absent at old, empty at new: still added")
	assert.Equal(t, "bigold", ch.OldSHAs["lib/big.bin"])
	assert.Equal(t, "bignew", got["lib/big.bin"].GetSHA())
	assert.Equal(t, "@@ -1,2 +1,2 @@\n a\n-old\n+new", got["lib/both.txt"].GetPatch())
	assert.Equal(t, [3]int{1, 1, 2}, [3]int{got["lib/both.txt"].GetAdditions(), got["lib/both.txt"].GetDeletions(), got["lib/both.txt"].GetChanges()})
	assert.Equal(t, "@@ -0,0 +1 @@\n+n", got["lib/new-only.txt"].GetPatch())
	assert.Equal(t, "removed", got["lib/old-only.txt"].GetStatus())
	assert.Equal(t, "@@ -1 +0,0 @@\n-o", got["lib/old-only.txt"].GetPatch())
	assert.True(t, ParseCommitFileDiff(got["lib/big.bin"]).IsBinary)
}

func TestFetchSubmoduleChanges_DivergedNestedGitlinkMovedOnBothSides_PointerChange(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[{"filename":"vendor","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit base\n+Subproject commit v2"}]}`))
		case "/repos/so/sr/compare/new...old":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[{"filename":"vendor","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit base\n+Subproject commit v1"}]}`))
		case "/repos/so/sr/git/trees/old":
			_, _ = w.Write([]byte(`{"tree":[{"path":"vendor","type":"commit","mode":"160000","sha":"v1"}]}`))
		case "/repos/so/sr/git/trees/new":
			_, _ = w.Write([]byte(`{"tree":[{"path":"vendor","type":"commit","mode":"160000","sha":"v2"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "old", "new")
	require.NoError(t, err)
	require.Len(t, ch.Files, 1)
	from, to, ok := SubmoduleBump(ch.Files[0])
	assert.Equal(t, []any{"lib/vendor", "modified", "v1", "v2", true}, []any{ch.Files[0].GetFilename(), ch.Files[0].GetStatus(), from, to, ok})
}

func TestFetchSubmoduleChanges_DivergedRenameAgainstIndependentAdd_StatusFromExistence(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[{"filename":"b","status":"added","patch":"@@ -0,0 +1 @@\n+B"}]}`))
		case "/repos/so/sr/compare/new...old":
			_, _ = w.Write([]byte(`{"status":"diverged","files":[{"filename":"b","previous_filename":"a","status":"renamed","changes":0}]}`))
		case "/repos/so/sr/git/trees/old":
			_, _ = w.Write([]byte(`{"tree":[{"path":"b","type":"blob","sha":"a1"}]}`))
		case "/repos/so/sr/git/trees/new":
			_, _ = w.Write([]byte(`{"tree":[{"path":"a","type":"blob","sha":"a1"},{"path":"b","type":"blob","sha":"b1"}]}`))
		case "/repos/so/sr/git/blobs/a1":
			_, _ = w.Write([]byte("A\n"))
		case "/repos/so/sr/git/blobs/b1":
			_, _ = w.Write([]byte("B\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "old", "new")
	require.NoError(t, err)
	require.Len(t, ch.Files, 2)
	assert.Equal(t, []string{"lib/a", "added", "", "@@ -0,0 +1 @@\n+A"},
		[]string{ch.Files[0].GetFilename(), ch.Files[0].GetStatus(), ch.Files[0].GetPreviousFilename(), ch.Files[0].GetPatch()})
	assert.Equal(t, []string{"lib/b", "modified", "", "@@ -1 +1 @@\n-A\n+B"},
		[]string{ch.Files[1].GetFilename(), ch.Files[1].GetStatus(), ch.Files[1].GetPreviousFilename(), ch.Files[1].GetPatch()})
}

func TestFetchSubmoduleChanges_DivergedGitlinkSwappedWithFile_ListsPointerAndFileApart(t *testing.T) {
	link := `{"tree":[{"path":"vendor","type":"commit","mode":"160000","sha":"v1"}]}`
	blob := `{"tree":[{"path":"vendor","type":"blob","sha":"t1"}]}`
	for name, tc := range map[string]struct {
		oldTree, newTree string
		ptr, file        []string
	}{
		"gitlink replaced by file": {link, blob, []string{"removed", "v1", ""}, []string{"added", "@@ -0,0 +1 @@\n+text", "t1"}},
		"file replaced by gitlink": {blob, link, []string{"added", "", "v1"}, []string{"removed", "@@ -1 +0,0 @@\n-text", ""}},
	} {
		t.Run(name, func(t *testing.T) {
			client := serve(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/o/r/contents/lib":
					_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
				case "/repos/so/sr/compare/old...new", "/repos/so/sr/compare/new...old":
					_, _ = w.Write([]byte(`{"status":"diverged","files":[{"filename":"vendor","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"}]}`))
				case "/repos/so/sr/git/trees/old":
					_, _ = w.Write([]byte(tc.oldTree))
				case "/repos/so/sr/git/trees/new":
					_, _ = w.Write([]byte(tc.newTree))
				case "/repos/so/sr/git/blobs/t1":
					_, _ = w.Write([]byte("text\n"))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			})

			ch, err := FetchSubmoduleChanges(context.Background(), client, "o", "r", "head", "lib", "old", "new")
			require.NoError(t, err)
			require.Len(t, ch.Files, 2)
			ptr, file := ch.Files[0], ch.Files[1]
			from, to, isBump := SubmoduleBump(ptr)
			assert.True(t, isBump)
			assert.Equal(t, append([]string{"lib/vendor"}, tc.ptr...), []string{ptr.GetFilename(), ptr.GetStatus(), from, to})
			_, _, isBump = SubmoduleBump(file)
			assert.False(t, isBump)
			assert.Equal(t, append([]string{"lib/vendor"}, tc.file...), []string{file.GetFilename(), file.GetStatus(), file.GetPatch(), file.GetSHA()})
		})
	}
}
