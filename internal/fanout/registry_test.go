// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package fanout

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// blockingQuery returns a query that waits until the returned channel is closed
// or the context ends, which is what a task cancelled from the outside looks
// like from the inside.
func blockingQuery() (Query, chan struct{}) {
	release := make(chan struct{})
	return func(ctx context.Context, _ Source) ([]Item, bool, error) {
		select {
		case <-release:
			return nil, false, nil
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}, release
}

func TestRegistryGetIsScopedToTheSession(t *testing.T) {
	r := NewRegistry(nil)
	defer r.Close()
	query, release := blockingQuery()
	defer close(release)

	task, err := r.Start("session-a", sources(1), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got, err := r.Get("session-a", task.ID); err != nil || got != task {
		t.Fatalf("Get from the owning session = %v, %v", got, err)
	}
	// A task identifier from somebody else's session must be as unknown as one
	// that never existed (§16).
	if _, err := r.Get("session-b", task.ID); !errors.Is(err, ErrNoTask) {
		t.Errorf("Get from another session = %v, want ErrNoTask", err)
	}
	if _, err := r.Get("session-a", "made-up"); !errors.Is(err, ErrNoTask) {
		t.Errorf("Get of an unknown task = %v, want ErrNoTask", err)
	}
}

func TestRegistryTrimsAndCancelsOldTasks(t *testing.T) {
	r := NewRegistry(nil)
	defer r.Close()
	query, release := blockingQuery()
	defer close(release)

	tasks := make([]*Task, 0, MaxPerSession+2)
	for i := 0; i < MaxPerSession+2; i++ {
		task, err := r.Start("session", sources(1), query, Options{})
		if err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}
		tasks = append(tasks, task)
	}
	if r.Len() != MaxPerSession {
		t.Errorf("Len() = %d, want %d", r.Len(), MaxPerSession)
	}
	// The dropped tasks must be cancelled, not merely forgotten: a forgotten
	// task is a goroutine on a socket nobody will read (§21).
	for i := 0; i < 2; i++ {
		tasks[i].Wait()
		if !tasks[i].Snapshot().Cancelled {
			t.Errorf("task %d was dropped without being cancelled", i)
		}
		if _, err := r.Get("session", tasks[i].ID); !errors.Is(err, ErrNoTask) {
			t.Errorf("dropped task %d is still reachable", i)
		}
	}
	if _, err := r.Get("session", tasks[len(tasks)-1].ID); err != nil {
		t.Errorf("the newest task is gone: %v", err)
	}
}

func TestRegistryCancelSessionEndsEverything(t *testing.T) {
	r := NewRegistry(nil)
	defer r.Close()
	query, release := blockingQuery()
	defer close(release)

	first, err := r.Start("session", sources(2), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	second, err := r.Start("session", sources(2), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	other, err := r.Start("other", sources(1), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	r.CancelSession("session")
	first.Wait()
	second.Wait()
	if !first.Snapshot().Cancelled || !second.Snapshot().Cancelled {
		t.Error("a session's tasks survived the session")
	}
	if _, err := r.Get("other", other.ID); err != nil {
		t.Errorf("another session's task was cancelled too: %v", err)
	}
}

func TestRegistrySweepDropsAbandonedTasks(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	r := NewRegistry(clock)
	defer r.Close()
	query, release := blockingQuery()
	defer close(release)

	task, err := r.Start("session", sources(1), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if n := r.Sweep(); n != 0 {
		t.Errorf("Sweep dropped %d fresh tasks", n)
	}
	// A tab closed mid-poll never asks again; TaskTTL later the task goes.
	now = now.Add(TaskTTL + time.Second)
	if n := r.Sweep(); n != 1 {
		t.Errorf("Sweep dropped %d tasks, want 1", n)
	}
	task.Wait()
	if !task.Snapshot().Cancelled {
		t.Error("a swept task was not cancelled")
	}
	if r.Len() != 0 {
		t.Errorf("Len() = %d after the sweep", r.Len())
	}
}

func TestRegistryCloseCancelsEverySession(t *testing.T) {
	r := NewRegistry(nil)
	query, release := blockingQuery()
	defer close(release)

	first, err := r.Start("a", sources(1), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	second, err := r.Start("b", sources(1), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Close()
	first.Wait()
	second.Wait()
	if !first.Snapshot().Cancelled || !second.Snapshot().Cancelled {
		t.Error("Close left a task running")
	}
	if r.Len() != 0 {
		t.Errorf("Len() = %d after Close", r.Len())
	}
}

// TestRegistryConcurrentUse covers several browsers doing the obvious things at
// once: starting polls, reading progress, cancelling, and signing out.
func TestRegistryConcurrentUse(t *testing.T) {
	r := NewRegistry(nil)
	defer r.Close()
	query := func(context.Context, Source) ([]Item, bool, error) {
		return []Item{{Key: "k"}}, false, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			session := "session-" + string(rune('a'+i%3))
			for n := 0; n < 10; n++ {
				task, err := r.Start(session, sources(2), query, Options{})
				if err != nil {
					t.Errorf("Start: %v", err)
					return
				}
				if got, err := r.Get(session, task.ID); err == nil {
					_ = got.Snapshot()
				}
				switch n % 3 {
				case 0:
					r.Cancel(session, task.ID)
				case 1:
					r.CancelSession(session)
				}
			}
		}(i)
	}
	wg.Wait()
}
