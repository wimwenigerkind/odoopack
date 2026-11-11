//go:build !windows

package store

import (
	"fmt"
	"io/fs"
	"syscall"
)

func linkCount(path string, info fs.FileInfo) (uint64, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("%s: no link count available", path)
	}
	return uint64(st.Nlink), nil
}
