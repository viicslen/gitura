package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowButtons_Layouts_KeepsNamedSupportedButtons(t *testing.T) {
	cases := []struct {
		name, layout             string
		canMinimize, canMaximize bool
		want                     []string
	}{
		{"default", "menu:minimize,maximize,close", true, true, []string{"minimize", "maximize", "close"}},
		{"gnome close only", "appmenu:close", true, true, []string{"close"}},
		{"empty layout", "", true, true, []string{}},
		{"colon only", ":", true, true, []string{}},
		{"left side, spaces", "close, maximize , minimize:", true, true, []string{"minimize", "maximize", "close"}},
		{"compositor lacks minimize", "menu:minimize,maximize,close", false, true, []string{"maximize", "close"}},
		{"compositor lacks both", "menu:minimize,maximize,close", false, false, []string{"close"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, windowButtons(tc.layout, tc.canMinimize, tc.canMaximize))
		})
	}
}
