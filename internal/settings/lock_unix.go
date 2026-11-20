//go:build !windows

package settings

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile blocks until it holds an exclusive lock on path, creating the file if needed.
func lockFile(path string) (func(), error) {
	// #nosec G304 -- path is always ConfigDir()/settings.lock, never user input.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
