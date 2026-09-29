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
			got := substitutePlaceholders(argv, tc.input, "", true)
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

	got := substitutePlaceholders(argv, "", "/home/me/my repo", false)
	want := []string{"mytool", "--dir=/home/me/my repo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
