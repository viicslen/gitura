// Package unidiff parses unified diff patches into frontend diff DTOs.
package unidiff

import (
	"fmt"
	"strings"

	"gitura/internal/model"
)

// ParseHunks parses a raw unified diff patch string into a slice of
// DiffHunkDTOs. Each hunk starts at a @@ header. "No newline" marker lines are
// silently skipped.
func ParseHunks(patch string) []model.DiffHunkDTO {
	var hunks []model.DiffHunkDTO
	var cur *model.DiffHunkDTO
	oldLine, newLine := 0, 0

	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@") {
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			oldStart, oldLines, newStart, newLines := parseDiffHunkHeader(line)
			cur = &model.DiffHunkDTO{
				Header:   line,
				OldStart: oldStart,
				OldLines: oldLines,
				NewStart: newStart,
				NewLines: newLines,
			}
			oldLine = oldStart
			newLine = newStart
			continue
		}
		if cur == nil {
			continue
		}
		// Skip "\ No newline at end of file" markers.
		if strings.HasPrefix(line, `\ `) {
			continue
		}
		cur.Lines = append(cur.Lines, parseDiffLine(line, &oldLine, &newLine))
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	return hunks
}

// parseDiffLine parses one line from a diff hunk body, advances the old/new
// line counters, and returns the corresponding DiffLineDTO.
func parseDiffLine(line string, oldLine, newLine *int) model.DiffLineDTO {
	switch {
	case strings.HasPrefix(line, "+"):
		dl := model.DiffLineDTO{Type: model.DiffLineAdd, NewNo: *newLine, Content: line[1:]}
		*newLine++
		return dl
	case strings.HasPrefix(line, "-"):
		dl := model.DiffLineDTO{Type: model.DiffLineDelete, OldNo: *oldLine, Content: line[1:]}
		*oldLine++
		return dl
	default:
		// Context line: leading space stripped when present.
		content := line
		if len(line) > 0 && line[0] == ' ' {
			content = line[1:]
		}
		dl := model.DiffLineDTO{Type: model.DiffLineContext, OldNo: *oldLine, NewNo: *newLine, Content: content}
		*oldLine++
		*newLine++
		return dl
	}
}

// parseDiffHunkHeader extracts (oldStart, oldLines, newStart, newLines) from a
// unified diff @@ header line. Returns (0,0,0,0) when the header cannot be
// parsed. Handles all four short forms produced by Git.
func parseDiffHunkHeader(header string) (oldStart, oldLines, newStart, newLines int) {
	var a, b, c, d int
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d,%d", &a, &b, &c, &d); n == 4 {
		return a, b, c, d
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d,%d +%d", &a, &b, &c); n == 3 {
		return a, b, c, 1
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d,%d", &a, &c, &d); n == 3 {
		return a, 1, c, d
	}
	if n, _ := fmt.Sscanf(header, "@@ -%d +%d", &a, &c); n == 2 {
		return a, 1, c, 1
	}
	return 0, 0, 0, 0
}
