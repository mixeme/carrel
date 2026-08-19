// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// InstanceLock records the running desktop instance for single-instance behaviour.
// Local mode also stores the sidecar port; see docs/plans/desktop-wrapper.md §7.
type InstanceLock struct {
	PID  int  `json:"pid"`
	Port int  `json:"port,omitempty"`
	Mode Mode `json:"mode"`
}

var ErrAlreadyRunning = errors.New("desktop: another instance is running")

// ReadLock returns the lock file contents, or nil when the file is absent.
func ReadLock(path string) (*InstanceLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("desktop: read lock: %w", err)
	}
	var lock InstanceLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("desktop: parse lock: %w", err)
	}
	if lock.PID <= 0 {
		return nil, fmt.Errorf("desktop: lock pid %d", lock.PID)
	}
	if lock.Mode != ModeRemote && lock.Mode != ModeLocal {
		return nil, fmt.Errorf("desktop: lock mode %q", lock.Mode)
	}
	return &lock, nil
}

// WriteLock replaces the lock file.
func WriteLock(path string, lock InstanceLock) error {
	if lock.PID <= 0 {
		return fmt.Errorf("desktop: lock pid %d", lock.PID)
	}
	if lock.Mode != ModeRemote && lock.Mode != ModeLocal {
		return fmt.Errorf("desktop: lock mode %q", lock.Mode)
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("desktop: marshal lock: %w", err)
	}
	data = append(data, '\n')
	return writeAtomic(path, data)
}

// RemoveLock deletes the lock file. When expectPID is non-zero the file is
// removed only when it still names that process.
func RemoveLock(path string, expectPID int) error {
	if expectPID > 0 {
		lock, err := ReadLock(path)
		if err != nil {
			return err
		}
		if lock == nil {
			return nil
		}
		if lock.PID != expectPID {
			return nil
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("desktop: remove lock: %w", err)
	}
	return nil
}

// HolderAlive reports whether another live instance owns the lock.
func HolderAlive(path string) (bool, *InstanceLock, error) {
	lock, err := ReadLock(path)
	if err != nil {
		return false, nil, err
	}
	if lock == nil {
		return false, nil, nil
	}
	alive, err := processAlive(lock.PID)
	if err != nil {
		return false, lock, err
	}
	return alive, lock, nil
}

// AcquireLock writes the lock when no other live instance holds it.
func AcquireLock(path string, lock InstanceLock) error {
	alive, existing, err := HolderAlive(path)
	if err != nil {
		return err
	}
	if alive {
		if existing.PID == lock.PID && existing.Mode == lock.Mode && existing.Port == lock.Port {
			return nil
		}
		return fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, existing.PID)
	}
	return WriteLock(path, lock)
}
