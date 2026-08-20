// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstanceLockRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	lock := InstanceLock{PID: os.Getpid(), Port: 51734, Mode: ModeLocal}
	if err := WriteLock(path, lock); err != nil {
		t.Fatalf("WriteLock() error: %v", err)
	}
	got, err := ReadLock(path)
	if err != nil {
		t.Fatalf("ReadLock() error: %v", err)
	}
	if got.PID != lock.PID || got.Port != lock.Port || got.Mode != lock.Mode {
		t.Errorf("got %+v, want %+v", got, lock)
	}
}

func TestAcquireLockCurrentProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	lock := InstanceLock{PID: os.Getpid(), Mode: ModeRemote}
	if err := AcquireLock(path, lock); err != nil {
		t.Fatalf("first AcquireLock() error: %v", err)
	}
	if err := AcquireLock(path, lock); err != nil {
		t.Fatalf("idempotent AcquireLock() error: %v", err)
	}
	other := InstanceLock{PID: os.Getpid(), Mode: ModeLocal, Port: 1}
	if err := AcquireLock(path, other); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("AcquireLock() = %v, want ErrAlreadyRunning", err)
	}
}

func TestAcquireLockStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	stale := InstanceLock{PID: deadPID(t), Mode: ModeLocal, Port: 8080}
	if err := WriteLock(path, stale); err != nil {
		t.Fatal(err)
	}
	lock := InstanceLock{PID: os.Getpid(), Mode: ModeLocal, Port: 9000}
	if err := AcquireLock(path, lock); err != nil {
		t.Fatalf("AcquireLock() over stale lock error: %v", err)
	}
	got, err := ReadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != lock.PID || got.Port != lock.Port {
		t.Errorf("got %+v, want %+v", got, lock)
	}
}

func TestHolderAlive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	alive, lock, err := HolderAlive(path)
	if err != nil {
		t.Fatal(err)
	}
	if alive || lock != nil {
		t.Fatalf("empty lock: alive=%v lock=%+v", alive, lock)
	}

	if err := WriteLock(path, InstanceLock{PID: os.Getpid(), Mode: ModeRemote}); err != nil {
		t.Fatal(err)
	}
	alive, lock, err = HolderAlive(path)
	if err != nil {
		t.Fatal(err)
	}
	if !alive || lock == nil {
		t.Fatalf("alive=%v lock=%+v", alive, lock)
	}
}

func TestRemoveLockPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	if err := WriteLock(path, InstanceLock{PID: 42, Mode: ModeRemote}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveLock(path, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLock(path); err != nil {
		t.Fatal(err)
	}
	if err := RemoveLock(path, 42); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("lock still present: %+v", got)
	}
}

func TestProcessAliveCurrent(t *testing.T) {
	alive, err := processAlive(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !alive {
		t.Fatal("current process not alive")
	}
}

func TestProcessAliveMissing(t *testing.T) {
	alive, err := processAlive(deadPID(t))
	if err != nil {
		t.Fatal(err)
	}
	if alive {
		t.Fatal("dead pid reported alive")
	}
}

// deadPID is a process ID that is not running. PID 2 is kthreadd on Linux
// and must not be used as a stand-in for a stale lock.
func deadPID(t *testing.T) int {
	t.Helper()
	const pid = 1_000_000_000
	alive, err := processAlive(pid)
	if err != nil {
		t.Fatalf("processAlive(%d): %v", pid, err)
	}
	if alive {
		t.Fatalf("pid %d is unexpectedly alive", pid)
	}
	return pid
}
