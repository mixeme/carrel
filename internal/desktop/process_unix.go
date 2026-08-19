// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package desktop

import (
	"os"
	"syscall"
)

func processAlive(pid int) (bool, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err == os.ErrProcessDone {
		return false, nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == syscall.ESRCH {
		return false, nil
	}
	return false, err
}
