//go:build windows

package settings

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile blocks until it holds an exclusive lock on path, creating the file if needed.
func lockFile(path string) (func(), error) {
	// #nosec G304 -- path is always ConfigDir()/settings.lock, never user input.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
		_ = f.Close()
	}, nil
}
