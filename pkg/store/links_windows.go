package store

import (
	"io/fs"
	"os"
	"syscall"
)

func linkCount(path string, _ fs.FileInfo) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &data); err != nil {
		return 0, err
	}
	return uint64(data.NumberOfLinks), nil
}
