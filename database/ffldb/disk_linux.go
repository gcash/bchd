//go:build linux

// Copyright (c) 2013-2018 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"syscall"
)

// getAvailableDiskSpace returns the number of bytes of available disk space.
func getAvailableDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
