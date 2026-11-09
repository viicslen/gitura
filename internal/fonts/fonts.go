// Package fonts lists the font families installed on the system.
package fonts

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// List returns the installed font family names, sorted and deduplicated.
func List(ctx context.Context) ([]string, error) {
	out, err := listCommand(ctx).Output()
	if err != nil {
		return nil, fmt.Errorf("fonts: list: %w", err)
	}
	return parse(string(out)), nil
}

func parse(out string) []string {
	families := []string{}
	for _, line := range strings.Split(out, "\n") {
		if f := strings.TrimSpace(line); f != "" {
			families = append(families, f)
		}
	}
	slices.Sort(families)
	return slices.Compact(families)
}
