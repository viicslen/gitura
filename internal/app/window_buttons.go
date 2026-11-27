package app

import (
	"slices"
	"strings"
)

var allWindowButtons = []string{"minimize", "maximize", "close"}

// WindowButtons lists the title strip's window buttons the window manager wants shown,
// in order, from "minimize", "maximize" and "close".
func (a *App) WindowButtons() []string {
	return wmWindowButtons()
}

// windowButtons keeps the buttons named in a GTK decoration layout ("menu:minimize,maximize,close")
// that the compositor supports.
func windowButtons(layout string, canMinimize, canMaximize bool) []string {
	named := strings.FieldsFunc(layout, func(r rune) bool { return r == ',' || r == ':' })
	for i := range named {
		named[i] = strings.TrimSpace(named[i])
	}
	shown := []string{}
	for _, b := range allWindowButtons {
		if !slices.Contains(named, b) || (b == "minimize" && !canMinimize) || (b == "maximize" && !canMaximize) {
			continue
		}
		shown = append(shown, b)
	}
	return shown
}
