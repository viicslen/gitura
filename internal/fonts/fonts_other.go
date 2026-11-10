//go:build !windows

package fonts

import (
	"context"
	"os/exec"
	"runtime"
)

func listCommand(ctx context.Context) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e",
			`ObjC.import("AppKit"); ObjC.deepUnwrap($.NSFontManager.sharedFontManager.availableFontFamilies).join("\n")`)
	}
	return exec.CommandContext(ctx, "fc-list", "--format", "%{family[0]}\n")
}
