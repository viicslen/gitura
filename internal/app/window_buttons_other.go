//go:build !linux || !cgo || gtk3 || purego

package app

func wmWindowButtons() []string {
	return allWindowButtons
}
