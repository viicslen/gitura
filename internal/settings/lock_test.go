package settings

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockFile_SecondLocker_WaitsForRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	unlock, err := lockFile(path)
	require.NoError(t, err)

	acquired := make(chan func())
	go func() {
		second, err := lockFile(path)
		if err == nil {
			acquired <- second
		}
	}()

	select {
	case <-acquired:
		t.Fatal("second lock acquired while the first was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case second := <-acquired:
		second()
	case <-time.After(2 * time.Second):
		t.Fatal("second lock never acquired after release")
	}
}
