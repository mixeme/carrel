// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"time"
)

func stopProcess(cmd *exec.Cmd, grace time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		return waitExit(<-done)
	}
	select {
	case err := <-done:
		return waitExit(err)
	case <-time.After(grace):
		_ = cmd.Process.Kill()
		return waitExit(<-done)
	}
}

func waitExit(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}
