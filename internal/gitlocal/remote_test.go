package gitlocal

import "testing"

func TestParseRemoteURL_CommonForms_ReturnsOwnerAndRepo(t *testing.T) {
	for _, url := range []string{
		"git@github.com:octo/hello.git",
		"https://github.com/octo/hello.git",
		"https://github.com/octo/hello/",
		"ssh://git@github.com/octo/hello",
	} {
		owner, repo, ok := ParseRemoteURL(url)
		if !ok || owner != "octo" || repo != "hello" {
			t.Errorf("ParseRemoteURL(%q) = %q, %q, %v", url, owner, repo, ok)
		}
	}
	for _, url := range []string{"", "hello"} {
		if _, _, ok := ParseRemoteURL(url); ok {
			t.Errorf("ParseRemoteURL(%q) ok, want not ok", url)
		}
	}
}
