// +build solaris plan9 netbsd openbsd

// Copyright (c) 2013-2018 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

// getAvailableDiskSpace is simply a stub that returns
// 1 byte greater than the disk space that prevents writes.
// Unfortunately there is not a good way to get the current
// disk space on these platforms in Go at this time.
func getAvailableDiskSpace(path string) (uint64, error) {
	return minAvailableSpaceUpdate + 1, nil
}
