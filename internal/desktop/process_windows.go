// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package desktop

import (
	"golang.org/x/sys/windows"
)

func processAlive(pid int) (bool, error) {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			return false, nil
		}
		return false, err
	}
	_ = windows.CloseHandle(handle)
	return true, nil
}
