package fonts

import (
	"context"
	"os/exec"
	"syscall"
)

func listCommand(ctx context.Context) *exec.Cmd {
	// UTF-8 output so non-ASCII family names survive the OEM code page.
	c := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; Add-Type -AssemblyName System.Drawing; (New-Object System.Drawing.Text.InstalledFontCollection).Families.Name")
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return c
}
