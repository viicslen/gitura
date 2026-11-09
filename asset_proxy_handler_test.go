package main

import (
	"net/url"
	"reflect"
	"testing"
)

func TestRepoFileFromURL(t *testing.T) {
	cases := []struct {
		in          string
		owner, repo string
		want        []repoFileRef
	}{
		{"https://github.com/o/r/raw/assets/pr-screenshots/a.png", "o", "r",
			[]repoFileRef{{"assets", "pr-screenshots/a.png"}, {"assets/pr-screenshots", "a.png"}}},
		{"https://github.com/o/r/raw/refs/heads/main/a%20b.png", "o", "r", []repoFileRef{{"main", "a b.png"}}},
		{"https://github.com/o/r/blob/main/a.png?raw=true", "o", "r", []repoFileRef{{"main", "a.png"}}},
		{"https://raw.githubusercontent.com/o/r/abc123/a.png", "o", "r", []repoFileRef{{"abc123", "a.png"}}},
		{"https://github.com/o/r/blob/main/a.png", "", "", nil},
		{"https://github.com/user-attachments/assets/1234", "", "", nil},
		{"https://github.com/o/r/raw/main/../../x.png", "", "", nil},
		{"https://avatars.githubusercontent.com/u/1", "", "", nil},
	}
	for _, c := range cases {
		u, _ := url.Parse(c.in)
		owner, repo, got := repoFileFromURL(u)
		if owner != c.owner || repo != c.repo || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got (%q,%q,%v)", c.in, owner, repo, got)
		}
	}
}
