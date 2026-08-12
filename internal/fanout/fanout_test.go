// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package fanout

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sources(n int) []Source {
	out := make([]Source, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Source{
			ID: fmt.Sprintf("src-%d", i), Kind: "calendar",
			AccountID: "acc", AccountLabel: "Work",
			Collection: fmt.Sprintf("/cal/%d/", i), CollectionLabel: fmt.Sprintf("Calendar %d", i),
		})
	}
	return out
}

// waitFor polls a condition with a deadline. Every test here is about a task
// that finishes on its own schedule, and sleeping a fixed time instead would
// either be slow or flaky.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// tick is a clock that advances a millisecond a reading. Windows' wall clock is
// coarse enough that a task finishing in the same instant it started reports no
// duration at all, which says nothing about the code under test.
func tick() func() time.Time {
	var mu sync.Mutex
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(time.Millisecond)
		return now
	}
}

func TestStartMergesAndSortsResults(t *testing.T) {
	query := func(_ context.Context, src Source) ([]Item, bool, error) {
		switch src.ID {
		case "src-0":
			return []Item{{SourceID: src.ID, Key: "b", Data: "b"}}, false, nil
		case "src-1":
			return []Item{{SourceID: src.ID, Key: "a", Data: "a"}}, true, nil
		default:
			return nil, false, nil
		}
	}
	task, err := Start("t1", sources(3), query, Options{Now: tick()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()
	task.Wait()

	snap := task.Snapshot()
	if snap.Running {
		t.Error("the task reads as running after Wait")
	}
	if snap.Total != 3 || snap.Queried != 3 || snap.Records != 2 {
		t.Errorf("snapshot = total %d, queried %d, records %d", snap.Total, snap.Queried, snap.Records)
	}
	if snap.Cached != 1 {
		t.Errorf("Cached = %d, want the one source that answered from cache", snap.Cached)
	}
	// A slow source must land in its place in the list, not at the end.
	if len(snap.Items) != 2 || snap.Items[0].Key != "a" || snap.Items[1].Key != "b" {
		t.Errorf("items are not merged in key order: %+v", snap.Items)
	}
	states := map[string]State{}
	for _, s := range snap.Sources {
		states[s.ID] = s.State
	}
	if states["src-0"] != StateDone || states["src-1"] != StateDone {
		t.Errorf("states = %v", states)
	}
	// A source with nothing to say is empty, which is not an error (§16).
	if states["src-2"] != StateEmpty {
		t.Errorf("src-2 state = %q, want empty", states["src-2"])
	}
	if snap.Failed != 0 {
		t.Errorf("Failed = %d, want 0", snap.Failed)
	}
	if snap.Elapsed <= 0 {
		t.Error("a finished task reports no elapsed time")
	}
	// The elapsed time is taken once, when the last source lands, and does not
	// grow while the results sit on screen.
	if again := task.Snapshot().Elapsed; again != snap.Elapsed {
		t.Errorf("Elapsed changed after the task finished: %v then %v", snap.Elapsed, again)
	}
	for _, s := range snap.Sources {
		if s.State == StateDone && s.Duration <= 0 {
			t.Errorf("source %s finished without a duration", s.ID)
		}
	}
}

func TestSourceTimeoutDoesNotStopTheRest(t *testing.T) {
	query := func(ctx context.Context, src Source) ([]Item, bool, error) {
		if src.ID == "src-0" {
			<-ctx.Done()
			return nil, false, ctx.Err()
		}
		return []Item{{SourceID: src.ID, Key: src.ID}}, false, nil
	}
	task, err := Start("t2", sources(3), query, Options{
		SourceTimeout: 30 * time.Millisecond, TotalTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()
	task.Wait()

	snap := task.Snapshot()
	if snap.Running {
		t.Error("the task is still running")
	}
	if snap.Records != 2 {
		t.Errorf("Records = %d, want the two sources that answered", snap.Records)
	}
	if snap.Failed != 1 {
		t.Errorf("Failed = %d, want 1", snap.Failed)
	}
	for _, s := range snap.Sources {
		if s.ID != "src-0" {
			continue
		}
		if s.State != StateTimeout {
			t.Errorf("the slow source is %q, want timeout", s.State)
		}
		if s.Reason == "" {
			t.Error("a timed-out source gives no reason")
		}
	}
}

func TestTotalTimeoutMarksWhateverIsOutstanding(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	query := func(ctx context.Context, _ Source) ([]Item, bool, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, false, ctx.Err()
	}
	task, err := Start("t3", sources(2), query, Options{
		SourceTimeout: time.Minute, TotalTimeout: 30 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()

	// The ceiling has to be enforced even though both goroutines are stuck, so
	// the person is never left with an indicator that never stops (§16).
	waitFor(t, 2*time.Second, "the ceiling to mark the outstanding sources", func() bool {
		return !task.Snapshot().Running
	})
	snap := task.Snapshot()
	if snap.Failed != 2 {
		t.Errorf("Failed = %d, want both sources marked", snap.Failed)
	}
	if snap.Cancelled {
		t.Error("a task that ran out of time reads as cancelled")
	}
}

func TestRetryPollsOneSourceAgain(t *testing.T) {
	var attempts int32
	query := func(_ context.Context, src Source) ([]Item, bool, error) {
		if src.ID != "src-0" {
			return []Item{{SourceID: src.ID, Key: src.ID}}, false, nil
		}
		if atomic.AddInt32(&attempts, 1) == 1 {
			return nil, false, errors.New("connection refused")
		}
		return []Item{{SourceID: src.ID, Key: "recovered"}}, false, nil
	}
	task, err := Start("t4", sources(2), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()
	task.Wait()

	before := task.Snapshot()
	if before.Failed != 1 {
		t.Fatalf("Failed = %d, want the first attempt to have failed", before.Failed)
	}
	if !task.Retry("src-0") {
		t.Fatal("Retry refused a failed source")
	}
	waitFor(t, 2*time.Second, "the retry to finish", func() bool { return !task.Snapshot().Running })

	after := task.Snapshot()
	if after.Failed != 0 {
		t.Errorf("Failed = %d after a successful retry", after.Failed)
	}
	if after.Records != 2 {
		t.Errorf("Records = %d, want both sources", after.Records)
	}
	for _, s := range after.Sources {
		if s.ID == "src-0" {
			if s.Attempt != 2 {
				t.Errorf("Attempt = %d, want 2", s.Attempt)
			}
			if s.Reason != "" {
				t.Errorf("Reason = %q, want it cleared", s.Reason)
			}
		}
	}
	// Retrying a source of a cancelled task must not open a connection.
	task.Cancel()
	if task.Retry("src-0") {
		t.Error("Retry restarted a source of a cancelled task")
	}
}

func TestRetryUnknownSource(t *testing.T) {
	task, err := Start("t5", sources(1), func(context.Context, Source) ([]Item, bool, error) {
		return nil, false, nil
	}, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()
	task.Wait()
	if task.Retry("nope") {
		t.Error("Retry accepted a source that is not in the task")
	}
}

func TestCancelKeepsPartialResults(t *testing.T) {
	started := make(chan struct{}, 1)
	query := func(ctx context.Context, src Source) ([]Item, bool, error) {
		if src.ID == "src-0" {
			return []Item{{SourceID: src.ID, Key: "kept"}}, false, nil
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, false, ctx.Err()
	}
	task, err := Start("t6", sources(2), query, Options{SourceTimeout: time.Minute, TotalTimeout: time.Minute})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started
	task.Cancel()
	task.Wait()

	snap := task.Snapshot()
	if !snap.Cancelled {
		t.Error("the snapshot does not say the poll was interrupted")
	}
	if snap.Running {
		t.Error("a cancelled task still reads as running")
	}
	// §16: what did arrive stays on screen, with the interrupted source marked.
	if snap.Records != 1 || len(snap.Items) != 1 || snap.Items[0].Key != "kept" {
		t.Errorf("partial results were dropped: %+v", snap.Items)
	}
	for _, s := range snap.Sources {
		if s.ID == "src-1" && s.State != StateCancelled {
			t.Errorf("the interrupted source is %q, want cancelled", s.State)
		}
	}
	if !strings.Contains(snap.Summary(), "interrupted") {
		t.Errorf("Summary() = %q, want it to mention the interruption", snap.Summary())
	}
	// Cancelling twice must be harmless: the page may be left while the button
	// is being pressed.
	task.Cancel()
}

func TestCancelBeforeATurnComesOpensNothing(t *testing.T) {
	var polled int32
	release := make(chan struct{})
	query := func(ctx context.Context, _ Source) ([]Item, bool, error) {
		atomic.AddInt32(&polled, 1)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, false, ctx.Err()
	}
	// One source at a time, so the rest are still waiting when Cancel lands.
	task, err := Start("t7", sources(4), query, Options{Parallel: 1, SourceTimeout: time.Minute, TotalTimeout: time.Minute})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, time.Second, "the first source to be polled", func() bool { return atomic.LoadInt32(&polled) > 0 })
	task.Cancel()
	close(release)
	task.Wait()

	if got := atomic.LoadInt32(&polled); got > 1 {
		t.Errorf("%d sources were polled after the task was cancelled", got-1)
	}
	for _, s := range task.Snapshot().Sources {
		if !s.State.Terminal() {
			t.Errorf("source %s is left in %q", s.ID, s.State)
		}
	}
}

func TestStartRejectsEmptyInput(t *testing.T) {
	ok := func(context.Context, Source) ([]Item, bool, error) { return nil, false, nil }
	if _, err := Start("t8", nil, ok, Options{}); !errors.Is(err, ErrNoSources) {
		t.Errorf("Start with no sources = %v, want ErrNoSources", err)
	}
	if _, err := Start("t8", []Source{{ID: ""}}, ok, Options{}); !errors.Is(err, ErrNoSources) {
		t.Errorf("Start with an unnamed source = %v, want ErrNoSources", err)
	}
	if _, err := Start("t8", sources(1), nil, Options{}); err == nil {
		t.Error("Start accepted a task with no query")
	}
}

// TestConcurrentSnapshotsAndRetries is the race test §21 asks for: progress is
// read while sources land and are retried, so every field of a task has to be
// under its lock.
func TestConcurrentSnapshotsAndRetries(t *testing.T) {
	query := func(_ context.Context, src Source) ([]Item, bool, error) {
		if src.ID == "src-1" {
			return nil, false, errors.New("refused")
		}
		return []Item{{SourceID: src.ID, Key: src.ID}}, false, nil
	}
	task, err := Start("t9", sources(6), query, Options{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer task.Cancel()

	var readers, writers sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = task.Snapshot().Summary()
			}
		}()
	}
	writers.Add(1)
	go func() {
		defer writers.Done()
		for i := 0; i < 20; i++ {
			task.Retry("src-1")
			time.Sleep(time.Millisecond)
		}
	}()
	// Subscribers come and go as tabs open and close.
	writers.Add(1)
	go func() {
		defer writers.Done()
		for i := 0; i < 20; i++ {
			updates, release := task.Subscribe()
			select {
			case <-updates:
			case <-time.After(2 * time.Millisecond):
			}
			release()
		}
	}()
	writers.Wait()
	close(stop)
	readers.Wait()
	task.Cancel()
	task.Wait()
}
