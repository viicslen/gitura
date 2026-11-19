package runner

import (
	"reflect"
	"testing"
)

func TestSubstitutePlaceholders_UntrustedInput_StaysOneArgument(t *testing.T) {
	argv, err := parseArgv("mytool --flag {{instructions}}")
	if err != nil {
		t.Fatalf("parseArgv: %v", err)
	}

	cases := []struct {
		name  string
		input string
	}{
		{"apostrophe", "it's fine"},
		{"quote and flag", "x' --flag 'y"},
		{"quote and separator", "'; echo pwned; '"},
		{"command substitution", "$(id) `id`"},
		{"spaces", "please fix this line"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := substitutePlaceholders(argv, placeholderReplacer(tc.input, "", nil, true))
			want := []string{"mytool", "--flag", tc.input}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestSubstitutePlaceholders_RepoPath_StaysOneArgument(t *testing.T) {
	argv, err := parseArgv("mytool --dir={{repo_path}}")
	if err != nil {
		t.Fatalf("parseArgv: %v", err)
	}

	got := substitutePlaceholders(argv, placeholderReplacer("", "/home/me/my repo", nil, false))
	want := []string{"mytool", "--dir=/home/me/my repo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSubstitutePlaceholders_Vars_ReplacedOnceWithinArgument(t *testing.T) {
	argv, err := parseArgv(`t3 thread start --title "PR #{{pr_number}}" . "{{comment_author}} on {{head_branch}}: {{instructions}}"`)
	if err != nil {
		t.Fatalf("parseArgv: %v", err)
	}
	vars := map[string]string{"pr_number": "42", "comment_author": "octocat", "head_branch": "feat/x"}

	got := substitutePlaceholders(argv, placeholderReplacer("rename {{pr_number}}", "", vars, true))
	want := []string{"t3", "thread", "start", "--title", "PR #42", ".", "octocat on feat/x: rename {{pr_number}}"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
