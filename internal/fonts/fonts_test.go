package fonts

import (
	"context"
	"slices"
	"testing"
)

func TestParse_TrimsDedupesAndSorts(t *testing.T) {
	got := parse("Noto Sans\r\n\nFira Code\nNoto Sans\n  \n")
	want := []string{"Fira Code", "Noto Sans"}
	if !slices.Equal(got, want) {
		t.Fatalf("parse = %q, want %q", got, want)
	}
}

func TestList_ReturnsInstalledFamilies(t *testing.T) {
	got, err := List(context.Background())
	if err != nil {
		t.Skipf("no font listing on this machine: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one font family")
	}
}
