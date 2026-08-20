// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package installcheck

import (
	"os"
	"syscall"
)

func diskFree(dir string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
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
