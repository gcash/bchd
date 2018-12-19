// +build windows

// Copyright (c) 2013-2018 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"syscall"
	"unsafe"
)

// getAvailableDiskSpace returns the number of bytes of available disk space.
func getAvailableDiskSpace(path string) (uint64, error) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytes, totalBytes, availBytes int64
	_, _, err := c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&freeBytes)), uintptr(unsafe.Pointer(&totalBytes)), uintptr(unsafe.Pointer(&availBytes)))
	if err != nil && err.(syscall.Errno) != 0 {
		return 0, err
	}

	return uint64(freeBytes), nil
}
