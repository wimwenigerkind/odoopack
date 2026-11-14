package store

import (
	"os"
	"syscall"
	"unsafe"
)

// The syscall package does not export LockFileEx and UnlockFileEx, and
// golang.org/x/sys is not worth a dependency for two calls. kernel32.dll is
// one of the system DLLs syscall loads from System32 only.
var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x2

// The lock covers the whole file range, so every holder locks the same
// region whatever the file's size.
const allBytes = ^uint32(0)

func lockFile(f *os.File, exclusive bool) error {
	var flags uint32
	if exclusive {
		flags = lockfileExclusiveLock
	}
	var ol syscall.Overlapped
	r, _, err := procLockFileEx.Call(f.Fd(), uintptr(flags), 0, uintptr(allBytes), uintptr(allBytes), uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockFileEx.Call(f.Fd(), 0, uintptr(allBytes), uintptr(allBytes), uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}
