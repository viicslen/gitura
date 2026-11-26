package gitlocal

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// subRepo builds parent/ with submodule lib/ (a.txt) that itself holds submodule nest/ (n.txt).
func subRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	nested, inner, dir := filepath.Join(root, "nested"), filepath.Join(root, "inner"), filepath.Join(root, "parent")
	for _, d := range []string{nested, inner, dir} {
		run(t, root, "init", "-q", "-b", "main", d)
	}
	write(t, nested, "n.txt", "n\n")
	run(t, nested, "add", ".")
	run(t, nested, "commit", "-q", "-m", "n")
	write(t, inner, "a.txt", "a\n")
	run(t, inner, "add", ".")
	run(t, inner, "-c", "protocol.file.allow=always", "submodule", "add", "-q", nested, "nest")
	run(t, inner, "commit", "-q", "-m", "a")
	write(t, dir, "r.txt", "r\n")
	run(t, dir, "add", ".")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", inner, "lib")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init", "--recursive")
	run(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestDiff_SubmoduleChanges_ListsInnerFilesUnderPointerRow(t *testing.T) {
	ctx := context.Background()
	dir := subRepo(t)
	lib := filepath.Join(dir, "lib")
	write(t, lib, "a.txt", "a\nb\n")
	run(t, lib, "commit", "-q", "-am", "b")
	write(t, lib, "a.txt", "a\nb\nc\n")
	write(t, lib, "u.txt", "u\n")
	write(t, filepath.Join(lib, "nest"), "n.txt", "n\nn\n")
	write(t, dir, "r.txt", "r\nr\n")

	files, err := Diff(ctx, dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ name, status, sub string }
	want := []row{
		{"lib", "submodule", "lib"},
		{"lib/a.txt", "modified", "lib"},
		{"lib/nest", "submodule", "lib/nest"},
		{"lib/nest/n.txt", "modified", "lib/nest"},
		{"r.txt", "modified", ""},
		{"lib/u.txt", "added", "lib"},
	}
	var got []row
	for _, f := range files {
		got = append(got, row{f.Diff.Filename, f.Diff.Status, f.Submodule})
		if strings.Contains(f.Patch, "\nSubmodule ") {
			t.Errorf("%s: patch kept a Submodule header:\n%s", f.Diff.Filename, f.Patch)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %v, want %v", i, got[i], want[i])
		}
	}
	if r := files[0].SubmoduleRange; !strings.Contains(r, "..") || !strings.HasSuffix(r, "-dirty") {
		t.Errorf("lib range %q: want old..new-dirty", r)
	}
	if d := files[1].Diff; d.TotalAdditions != 2 {
		t.Errorf("lib/a.txt should combine committed and uncommitted lines; got +%d", d.TotalAdditions)
	}
}

func TestParseHeader_HeaderForms_ReadsPathAndRange(t *testing.T) {
	cases := []struct {
		line string
		want header
	}{
		{"lib contains modified content", header{path: "lib", dirty: true}},
		{"my lib contains untracked content", header{path: "my lib", dirty: true}},
		{"lib 543c5ae..9982d5e:", header{path: "lib", old: "543c5ae", new: "9982d5e"}},
		{"lib 7f24658..6a68966 (rewind):", header{path: "lib", old: "7f24658", new: "6a68966", note: "rewind"}},
		{"lib 7f24658...651c7a2:", header{path: "lib", old: "7f24658", new: "651c7a2", diverged: true}},
		{"newsub 0000000...29505ab (new submodule)", header{path: "newsub", old: "0000000", new: "29505ab", note: "new submodule"}},
		{"newsub 29505ab...0000000 (submodule deleted)", header{path: "newsub", old: "29505ab", new: "0000000", note: "submodule deleted"}},
		{"lib d371592...1111111 (commits not present)", header{path: "lib", old: "d371592", new: "1111111", note: "commits not present"}},
	}
	for _, c := range cases {
		got, ok := parseHeader(c.line)
		if !ok || got != c.want {
			t.Errorf("%q: got %+v %v, want %+v", c.line, got, ok, c.want)
		}
	}
}

func TestFingerprint_EditInsideDirtySubmodule_Changes(t *testing.T) {
	ctx := context.Background()
	dir := subRepo(t)
	write(t, filepath.Join(dir, "lib"), "a.txt", "a\nb\n")
	before, err := Fingerprint(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "lib"), "a.txt", "a\nb\nc\n")
	if after, _ := Fingerprint(ctx, dir); before == after {
		t.Error("fingerprint did not change after a second edit inside a dirty submodule")
	}
}

func TestDiff_SubmoduleWithOnlyUntrackedFiles_ListsThemUnderDirtyPointerRows(t *testing.T) {
	dir := subRepo(t)
	write(t, filepath.Join(dir, "lib"), "new.txt", "x\n")

	want := []subRow{{"lib", "lib"}, {"lib/new.txt", "lib"}}
	if got := subRows(t, dir); !slices.Equal(got, want) {
		t.Fatalf("top level: got %v\nwant %v", got, want)
	}
	run(t, dir, "config", "status.showUntrackedFiles", "no")
	run(t, filepath.Join(dir, "lib"), "config", "status.showUntrackedFiles", "no")
	if got := subRows(t, dir); !slices.Equal(got, want) {
		t.Fatalf("top level, status.showUntrackedFiles=no: got %v\nwant %v", got, want)
	}

	if err := os.Remove(filepath.Join(dir, "lib", "new.txt")); err != nil {
		t.Fatal(err)
	}
	nest := filepath.Join(dir, "lib", "nest")
	write(t, nest, "new.txt", "y\n")
	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ name, sub, in string }
	var got []row
	for _, f := range files {
		got = append(got, row{f.Diff.Filename, f.Submodule, f.SubmoduleIn})
	}
	wantNested := []row{{"lib", "lib", ""}, {"lib/nest", "lib/nest", "lib"}, {"lib/nest/new.txt", "lib/nest", ""}}
	if !slices.Equal(got, wantNested) {
		t.Fatalf("nested: got %v\nwant %v", got, wantNested)
	}
	if r := files[1].SubmoduleRange; !strings.HasSuffix(r, "-dirty") {
		t.Errorf("lib/nest range %q: want a -dirty suffix", r)
	}

	before, err := Fingerprint(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, nest, "new2.txt", "z\n")
	if after, _ := Fingerprint(context.Background(), dir); before == after {
		t.Error("fingerprint did not change after another untracked file in a nested submodule")
	}
}

func TestDiff_SubmoduleRewoundOrNotCheckedOut_FlagsPointerRow(t *testing.T) {
	ctx := context.Background()
	dir := subRepo(t)
	lib := filepath.Join(dir, "lib")
	write(t, lib, "a.txt", "a\nb\n")
	run(t, lib, "commit", "-q", "-am", "b")
	run(t, dir, "commit", "-q", "-am", "bump")
	run(t, lib, "checkout", "-q", "HEAD~1")

	files, err := Diff(ctx, dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || !files[0].SubmoduleBackwards || files[1].Diff.Filename != "lib/a.txt" || files[1].Diff.TotalDeletions != 1 {
		t.Fatalf("rewind: got %+v", files)
	}

	run(t, dir, "commit", "-q", "-am", "rewind")
	run(t, dir, "submodule", "deinit", "-q", "-f", "lib")
	run(t, dir, "reset", "-q", "--soft", "HEAD~1")
	files, err = Diff(ctx, dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].SubmoduleUnavailable != "not checked out" {
		t.Fatalf("not checked out: got %+v", files)
	}
}

func TestDiff_SubmoduleDiverged_FlagsPointerRow(t *testing.T) {
	ctx := context.Background()
	dir := subRepo(t)
	lib := filepath.Join(dir, "lib")
	write(t, lib, "a.txt", "a\nb\n")
	run(t, lib, "commit", "-q", "-am", "b")
	run(t, dir, "commit", "-q", "-am", "bump")
	run(t, lib, "checkout", "-q", "HEAD~1")
	write(t, lib, "a.txt", "a\nc\n")
	run(t, lib, "commit", "-q", "-am", "c")

	files, err := Diff(ctx, dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 1 || !files[0].SubmoduleDiverged || files[0].SubmoduleBackwards {
		t.Fatalf("diverged: got %+v", files)
	}
}

// subRepoWithTopNest is subRepo plus a top-level submodule nest/ (t.txt), so "nest" names two submodules.
func subRepoWithTopNest(t *testing.T) string {
	t.Helper()
	dir := subRepo(t)
	top := filepath.Join(filepath.Dir(dir), "top")
	run(t, dir, "init", "-q", "-b", "main", top)
	write(t, top, "t.txt", "t\n")
	run(t, top, "add", ".")
	run(t, top, "commit", "-q", "-m", "t")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", top, "nest")
	run(t, dir, "commit", "-q", "-m", "top nest")
	return dir
}

type subRow struct{ name, sub string }

func subRows(t *testing.T, dir string) []subRow {
	t.Helper()
	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []subRow
	for _, f := range files {
		got = append(got, subRow{f.Diff.Filename, f.Submodule})
	}
	return got
}

func TestDiff_NestedAndTopLevelSubmoduleShareName_TagsEachByFullPath(t *testing.T) {
	dir := subRepoWithTopNest(t)
	lib := filepath.Join(dir, "lib")
	write(t, filepath.Join(lib, "nest"), "n.txt", "n\nn\n")
	run(t, filepath.Join(lib, "nest"), "commit", "-q", "-am", "n2")
	write(t, lib, "a.txt", "a\nb\n")
	write(t, filepath.Join(dir, "nest"), "t.txt", "t\nt\n")

	want := []subRow{
		{"lib", "lib"},
		{"lib/a.txt", "lib"},
		{"lib/nest", "lib/nest"},
		{"lib/nest/n.txt", "lib/nest"},
		{"nest", "nest"},
		{"nest/t.txt", "nest"},
	}
	if got := subRows(t, dir); !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestDiff_NestedSubmoduleDeletedBesideTopLevelNamesake_KeepsItUnderParent(t *testing.T) {
	dir := subRepoWithTopNest(t)
	lib := filepath.Join(dir, "lib")
	run(t, lib, "rm", "-q", "nest")
	run(t, lib, "commit", "-q", "-m", "rm nest")
	write(t, filepath.Join(dir, "nest"), "t.txt", "t\nt\n")

	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []subRow
	for _, f := range files {
		got = append(got, subRow{f.Diff.Filename, f.Submodule})
	}
	want := []subRow{
		{"lib", "lib"},
		{"lib/.gitmodules", "lib"},
		{"lib/nest", "lib/nest"},
		{"lib/nest/n.txt", "lib/nest"},
		{"nest", "nest"},
		{"nest/t.txt", "nest"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if d := files[3].Diff; d.Status != "removed" || d.TotalDeletions != 1 || d.TotalAdditions != 0 || !strings.Contains(files[3].Patch, "\n-n") {
		t.Errorf("lib/nest/n.txt: got %+v\n%s", d, files[3].Patch)
	}
}

func TestDiff_DeletedSubmoduleCommitGone_ListsOnlyPointerRow(t *testing.T) {
	dir := subRepo(t)
	run(t, filepath.Join(dir, "lib"), "rm", "-q", "nest")
	if err := os.RemoveAll(filepath.Join(dir, ".git", "modules", "lib", "modules", "nest")); err != nil {
		t.Fatal(err)
	}

	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []subRow
	for _, f := range files {
		got = append(got, subRow{f.Diff.Filename, f.Submodule})
	}
	want := []subRow{{"lib", "lib"}, {"lib/.gitmodules", "lib"}, {"lib/nest", "lib/nest"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if files[2].SubmoduleUnavailable != "" {
		t.Errorf("unavailable = %q, want the deleted pointer row's own (none)", files[2].SubmoduleUnavailable)
	}
}

// dirToSubRepo builds parent/ tracking lib/.keep and lib/a.txt, then replaces directory lib with a
// submodule holding a.txt and b.txt, uncommitted.
func dirToSubRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	sub, dir := filepath.Join(root, "sub"), filepath.Join(root, "parent")
	for _, d := range []string{sub, dir} {
		run(t, root, "init", "-q", "-b", "main", d)
	}
	write(t, sub, "a.txt", "sub a\n")
	write(t, sub, "b.txt", "sub b\n")
	run(t, sub, "add", ".")
	run(t, sub, "commit", "-q", "-m", "sub")
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "lib/.keep", "")
	write(t, dir, "lib/a.txt", "old a\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "init")
	run(t, dir, "rm", "-rq", "lib")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "lib")
	return dir
}

func TestDiff_TrackedDirBecomesSubmodule_TagsOnlyTheSubmoduleFiles(t *testing.T) {
	dir := dirToSubRepo(t)

	got := subRows(t, dir)

	// Unique path lib/.keep, and shared path lib/a.txt, both stay with the parent.
	want := []subRow{{".gitmodules", ""}, {"lib", "lib"}, {"lib/a.txt", "lib"}, {"lib/b.txt", "lib"}, {"lib/.keep", ""}, {"lib/a.txt", ""}}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestDiff_RenameOnlyInsideSubmodule_KeepsSubmodulePaths(t *testing.T) {
	dir := subRepo(t)
	lib := filepath.Join(dir, "lib")
	nest := filepath.Join(lib, "nest")
	run(t, nest, "mv", "n.txt", "m.txt")
	run(t, nest, "commit", "-q", "-m", "mv n")
	run(t, lib, "mv", "a.txt", "b.txt")
	run(t, lib, "commit", "-q", "-am", "mv a")

	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ name, prev, status, sub string }
	var got []row
	for _, f := range files {
		got = append(got, row{f.Diff.Filename, f.Diff.PreviousFilename, f.Diff.Status, f.Submodule})
	}
	want := []row{
		{"lib", "", "submodule", "lib"},
		{"lib/b.txt", "lib/a.txt", "renamed", "lib"},
		{"lib/nest", "", "submodule", "lib/nest"},
		{"lib/nest/m.txt", "lib/nest/n.txt", "renamed", "lib/nest"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestDiff_TrackedDirInsideSubmoduleBecomesNestedSubmodule_TagsEachFileByItsRepo(t *testing.T) {
	root := t.TempDir()
	x, lib, dir := filepath.Join(root, "x"), filepath.Join(root, "lib"), filepath.Join(root, "parent")
	for _, d := range []string{x, lib, dir} {
		run(t, root, "init", "-q", "-b", "main", d)
	}
	write(t, x, "a.txt", "x a\n")
	write(t, x, "b.txt", "x b\n")
	run(t, x, "add", ".")
	run(t, x, "commit", "-q", "-m", "x")
	if err := os.Mkdir(filepath.Join(lib, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, lib, "x/.keep", "")
	write(t, lib, "x/a.txt", "old a\n")
	run(t, lib, "add", ".")
	run(t, lib, "commit", "-q", "-m", "lib")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", lib, "lib")
	run(t, dir, "commit", "-q", "-m", "init")
	sub := filepath.Join(dir, "lib")
	run(t, sub, "rm", "-rq", "x")
	run(t, sub, "-c", "protocol.file.allow=always", "submodule", "add", "-q", x, "x")
	run(t, sub, "commit", "-q", "-m", "x becomes a submodule")

	got := subRows(t, dir)

	want := []subRow{
		{"lib", "lib"}, {"lib/.gitmodules", "lib"},
		{"lib/x", "lib/x"}, {"lib/x/a.txt", "lib/x"}, {"lib/x/b.txt", "lib/x"},
		{"lib/x/.keep", "lib"}, {"lib/x/a.txt", "lib"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestDiff_TopLevelSubmoduleRemovedBesideChangedOne_ListsItsFilesAsDeleted(t *testing.T) {
	dir := subRepo(t)
	tools := filepath.Join(filepath.Dir(dir), "tools")
	run(t, dir, "init", "-q", "-b", "main", tools)
	write(t, tools, "x.txt", "x\n")
	run(t, tools, "add", ".")
	run(t, tools, "commit", "-q", "-m", "x")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", tools, "tools")
	run(t, dir, "commit", "-q", "-m", "tools")
	lib := filepath.Join(dir, "lib")
	write(t, lib, "a.txt", "a\nb\n")
	run(t, lib, "commit", "-q", "-am", "b")
	run(t, dir, "rm", "-q", "tools")

	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []subRow
	for _, f := range files {
		got = append(got, subRow{f.Diff.Filename, f.Submodule})
	}
	want := []subRow{{".gitmodules", ""}, {"lib", "lib"}, {"lib/a.txt", "lib"}, {"tools", "tools"}, {"tools/x.txt", "tools"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if d := files[4].Diff; d.Status != "removed" || d.TotalDeletions != 1 {
		t.Errorf("tools/x.txt: got %+v", d)
	}
}

func TestDiff_TopLevelSubmoduleRemovedWhileNestedNamesakeAtSameCommitStays_ResolvesTopLevel(t *testing.T) {
	dir := subRepo(t)
	tools := filepath.Join(filepath.Dir(dir), "tools")
	run(t, dir, "init", "-q", "-b", "main", tools)
	write(t, tools, "x.txt", "x\n")
	run(t, tools, "add", ".")
	run(t, tools, "commit", "-q", "-m", "x")
	lib := filepath.Join(dir, "lib")
	run(t, lib, "-c", "protocol.file.allow=always", "submodule", "add", "-q", tools, "tools")
	run(t, lib, "commit", "-q", "-m", "tools")
	run(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", tools, "tools")
	run(t, dir, "commit", "-q", "-am", "tools")
	write(t, lib, "a.txt", "a\nb\n")
	run(t, lib, "commit", "-q", "-am", "b")
	run(t, dir, "rm", "-q", "tools")

	want := []subRow{{".gitmodules", ""}, {"lib", "lib"}, {"lib/a.txt", "lib"}, {"tools", "tools"}, {"tools/x.txt", "tools"}}
	if got := subRows(t, dir); !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

type movedRow struct{ name, prev, status, sub, rng, unavailable string }

// movedRows moves subRepo's lib to deps, lets edit change deps, and returns Diff's rows.
func movedRows(t *testing.T, edit func(deps string)) []movedRow {
	t.Helper()
	dir := subRepo(t)
	run(t, dir, "mv", "lib", "deps")
	edit(filepath.Join(dir, "deps"))
	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []movedRow
	for _, f := range files {
		got = append(got, movedRow{f.Diff.Filename, f.Diff.PreviousFilename, f.Diff.Status, f.Submodule, f.SubmoduleRange, f.SubmoduleUnavailable})
	}
	return got
}

// pointerRange is lib's gitlink range, its commit unchanged by the move.
func pointerRange(t *testing.T, rows []movedRow, suffix string) string {
	t.Helper()
	if len(rows) < 2 {
		t.Fatalf("rows = %v", rows)
	}
	sha, _, _ := strings.Cut(rows[1].rng, "..")
	return sha + ".." + sha + suffix
}

func TestDiff_DirtySubmoduleMoved_ListsItsEditsUnderRenamedPointerRow(t *testing.T) {
	got := movedRows(t, func(deps string) { write(t, deps, "a.txt", "a\nb\n") })

	want := []movedRow{
		{".gitmodules", "", "modified", "", "", ""},
		{"deps", "lib", "submodule", "deps", pointerRange(t, got, "-dirty"), ""},
		{"deps/a.txt", "", "modified", "deps", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestDiff_MovedSubmoduleWithOnlyUntrackedFile_ListsIt(t *testing.T) {
	got := movedRows(t, func(deps string) { write(t, deps, "u.txt", "u\n") })

	want := []movedRow{
		{".gitmodules", "", "modified", "", "", ""},
		{"deps", "lib", "submodule", "deps", pointerRange(t, got, "-dirty"), ""},
		{"deps/u.txt", "", "added", "deps", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestDiff_CleanSubmoduleMoved_ListsOnlyRenamedPointerRow(t *testing.T) {
	got := movedRows(t, func(string) {})

	want := []movedRow{
		{".gitmodules", "", "modified", "", "", ""},
		{"deps", "lib", "submodule", "deps", pointerRange(t, got, ""), ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestFingerprint_EditInsideMovedSubmodule_Changes(t *testing.T) {
	ctx := context.Background()
	dir := subRepo(t)
	run(t, dir, "mv", "lib", "deps")
	deps := filepath.Join(dir, "deps")
	write(t, deps, "a.txt", "a\nb\n")
	before, err := Fingerprint(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, deps, "a.txt", "a\nb\nc\n")
	if after, _ := Fingerprint(ctx, dir); before == after {
		t.Error("fingerprint did not change after a second edit inside a moved submodule")
	}
}

func TestDiff_NestedSubmoduleMovedAndEdited_ListsItsEditsUnderRenamedPointerRow(t *testing.T) {
	dir := subRepo(t)
	lib := filepath.Join(dir, "lib")
	run(t, lib, "mv", "nest", "nest2")
	write(t, filepath.Join(lib, "nest2"), "n.txt", "n\nn\n")
	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	var got []movedRow
	for _, f := range files {
		got = append(got, movedRow{f.Diff.Filename, f.Diff.PreviousFilename, f.Diff.Status, f.Submodule, "", f.SubmoduleUnavailable})
	}

	want := []movedRow{
		{"lib", "", "submodule", "lib", "", ""},
		{"lib/.gitmodules", "", "modified", "lib", "", ""},
		{"lib/nest2", "lib/nest", "submodule", "lib/nest2", "", ""},
		{"lib/nest2/n.txt", "", "modified", "lib/nest2", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if files[2].SubmoduleIn != "lib" {
		t.Errorf("SubmoduleIn = %q, want lib", files[2].SubmoduleIn)
	}
}

func TestDiff_MovedSubmoduleNotCheckedOut_FlagsNotCheckedOut(t *testing.T) {
	dir := subRepo(t)
	run(t, dir, "submodule", "deinit", "-q", "-f", "lib")
	run(t, dir, "mv", "lib", "deps")
	files, err := Diff(context.Background(), dir, BaseHEAD)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[1].Diff.Filename != "deps" || files[1].SubmoduleUnavailable != "not checked out" {
		t.Fatalf("got %+v", files)
	}
}

func TestDiff_SubmoduleMovedWithEditInNestedSubmodule_ListsNestedEdit(t *testing.T) {
	got := movedRows(t, func(deps string) { write(t, filepath.Join(deps, "nest"), "n.txt", "n\nn\n") })

	want := []movedRow{
		{".gitmodules", "", "modified", "", "", ""},
		{"deps", "lib", "submodule", "deps", pointerRange(t, got, "-dirty"), ""},
		{"deps/nest", "", "submodule", "deps/nest", got[2].rng, ""},
		{"deps/nest/n.txt", "", "modified", "deps/nest", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestDiff_SubmoduleMovedWithUntrackedFilesAndNestedEdit_ListsEachRowOnce(t *testing.T) {
	got := movedRows(t, func(deps string) {
		write(t, filepath.Join(deps, "nest"), "n.txt", "n\nn\n")
		write(t, filepath.Join(deps, "nest"), "u.txt", "u\n")
		write(t, deps, "v.txt", "v\n")
	})

	want := []movedRow{
		{".gitmodules", "", "modified", "", "", ""},
		{"deps", "lib", "submodule", "deps", pointerRange(t, got, "-dirty"), ""},
		{"deps/nest", "", "submodule", "deps/nest", got[2].rng, ""},
		{"deps/nest/n.txt", "", "modified", "deps/nest", "", ""},
		{"deps/nest/u.txt", "", "added", "deps/nest", "", ""},
		{"deps/v.txt", "", "added", "deps", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
