package unidiff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseDiffHunkHeader_AllForms_ParsesStartsAndCounts verifies all four @@ header formats including
// the optional trailing function-name context.
func TestParseDiffHunkHeader_AllForms_ParsesStartsAndCounts(t *testing.T) {
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
