//go:build windows

package filelock

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	lockfileExclusiveLock = 0x00000002
)

func lockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		lockfileExclusiveLock,
		0,          // reserved
		1,          // lock 1 byte
		0,          // high word
		ol,
	)
}

func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0, // reserved
		1, // unlock 1 byte
		0, // high word
		ol,
	)
}
