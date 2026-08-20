// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package installcheck

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func diskFree(dir string) (uint64, error) {
	root := filepath.VolumeName(dir)
	if root == "" {
		root = dir
	}
	if len(root) == 2 && root[1] == ':' {
		root += `\`
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel.NewProc("GetDiskFreeSpaceExW")
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}
	var free, total, avail uint64
	r, _, e := proc.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&avail)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if r == 0 {
		if e != syscall.Errno(0) {
			return 0, e
		}
		return 0, syscall.EINVAL
	}
	return avail, nil
}

func dirWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}
