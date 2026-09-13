package fanout

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

type fake struct {
	id  string
	run func(ctx context.Context) ([]Item, error)
}

func (f fake) ID() string                                       { return f.id }
func (f fake) Run(ctx context.Context, _ Query) ([]Item, error) { return f.run(ctx) }

func answer(items []Item, err error) func(context.Context) ([]Item, error) {
	return func(context.Context) ([]Item, error) { return items, err }
}

func files(src string, names ...string) []Item {
	var out []Item
	for _, n := range names {
		out = append(out, Item{Col: src, Path: src + "/" + n, Kind: view.KindFile, FileName: n})
	}
	return out
}

func options() Options {
	return Options{
		Parallelism:   4,
		SourceTimeout: 5 * time.Second,
		TotalTimeout:  10 * time.Second,
		Key:           func(it Item) string { return it.FileName },
	}
}

func wait(t *testing.T, task *Task) Snapshot {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		ch := task.Changed()
		s := task.Snapshot()
		if !s.Running {
			return s
		}
		select {
		case <-ch:
		case <-deadline:
			t.Fatalf("task still running: %+v", s.Sources)
		}
	}
}

func waitSource(t *testing.T, task *Task, id string, state view.SourceState) Snapshot {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		ch := task.Changed()
		s := task.Snapshot()
		for _, src := range s.Sources {
			if src.ID == id && src.State == state {
				return s
			}
		}
		select {
		case <-ch:
		case <-deadline:
			t.Fatalf("source %s never %s: %+v", id, state, s.Sources)
		}
	}
}

func names(items []Item) string {
	var out []string
	for _, it := range items {
		out = append(out, it.FileName+"@"+it.Col)
	}
	return strings.Join(out, ",")
}

func states(s Snapshot) map[string]view.SourceState {
	out := map[string]view.SourceState{}
	for _, src := range s.Sources {
		out[src.ID] = src.State
	}
	return out
}

// S-15.1: items go in where they sort, not at the end, and an item already
// shown stays ahead of an equal newcomer.
func TestItemsArriveInPlace(t *testing.T) {
	gate := make(chan struct{})
	task, err := Start("t-1", Query{Text: "x"}, []Source{
		fake{"a", answer(files("a", "d", "b", "x"), nil)},
		fake{"b", func(context.Context) ([]Item, error) {
			<-gate
			return files("b", "e", "a", "c", "x"), nil
		}},
	}, options())
	if err != nil {
		t.Fatal(err)
	}
	first := waitSource(t, task, "a", view.SourceDone)
	if got := names(first.Items); got != "b@a,d@a,x@a" {
		t.Fatalf("after a: %s", got)
	}
	if !first.Running {
		t.Fatal("b has not answered: the task must still run")
	}
	close(gate)
	s := wait(t, task)
	if got := names(s.Items); got != "a@b,b@a,c@b,d@a,e@b,x@a,x@b" {
		t.Fatalf("after b: %s", got)
	}
	if s.Finished.IsZero() || s.Finished.Before(s.Started) {
		t.Errorf("started %v, finished %v", s.Started, s.Finished)
	}
}

// C-8.1: an empty answer, a refusal, a server that is not there and one that
// is too slow are four different states.
func TestStatesTellEmptyFromRefusal(t *testing.T) {
	opts := options()
	opts.SourceTimeout = 50 * time.Millisecond
	refused := errors.New("403 Forbidden")
	task, err := Start("t-1", Query{Text: "x"}, []Source{
		fake{"empty", answer(nil, nil)},
		fake{"found", answer(files("found", "one", "two"), nil)},
		fake{"refused", answer(nil, refused)},
		fake{"down", answer(nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")})},
		fake{"slow", func(ctx context.Context) ([]Item, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}},
		fake{"cut", answer(files("cut", "three"), fmt.Errorf("%w: stopped after 2000 entries", ErrIncomplete))},
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	s := wait(t, task)
	want := map[string]view.SourceState{
		"empty": view.SourceEmpty, "found": view.SourceDone, "refused": view.SourceError,
		"down": view.SourceUnavailable, "slow": view.SourceTimeout, "cut": view.SourceDone,
	}
	for id, st := range want {
		if states(s)[id] != st {
			t.Errorf("%s = %s, want %s", id, states(s)[id], st)
		}
	}
	for _, src := range s.Sources {
		switch src.ID {
		case "found":
			if src.Count != 2 || src.Err != nil {
				t.Errorf("found = %+v", src)
			}
		case "refused":
			if !errors.Is(src.Err, refused) {
				t.Errorf("refused err = %v", src.Err)
			}
		case "cut":
			if src.Count != 1 || !strings.Contains(src.Note, "2000 entries") {
				t.Errorf("cut = %+v", src)
			}
		}
	}
	if len(s.Items) != 3 {
		t.Errorf("items = %s: a refusal keeps nothing, an incomplete answer keeps what it has", names(s.Items))
	}
}

// The per-source timeout applies even when the ceiling is far away.
func TestSourceTimeout(t *testing.T) {
	opts := options()
	opts.SourceTimeout = 30 * time.Millisecond
	task, _ := Start("t-1", Query{}, []Source{fake{"slow", func(ctx context.Context) ([]Item, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}}, opts)
	if s := wait(t, task); states(s)["slow"] != view.SourceTimeout {
		t.Fatalf("states = %v", states(s))
	}
}

func TestParallelismIsACeiling(t *testing.T) {
	opts := options()
	opts.Parallelism = 3
	var now, peak atomic.Int32
	var sources []Source
	for i := 0; i < 10; i++ {
		sources = append(sources, fake{fmt.Sprint(i), func(context.Context) ([]Item, error) {
			n := now.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			now.Add(-1)
			return nil, nil
		}})
	}
	task, _ := Start("t-1", Query{}, sources, opts)
	s := wait(t, task)
	if p := peak.Load(); p > 3 || p < 1 {
		t.Fatalf("peak = %d, ceiling 3", p)
	}
	for id, st := range states(s) {
		if st != view.SourceEmpty {
			t.Errorf("%s = %s", id, st)
		}
	}
}

// S-15.1: a stop keeps what arrived, marks the rest, and is written by the
// time Cancel returns.
func TestCancelKeepsWhatArrived(t *testing.T) {
	var finished atomic.Int32
	var released atomic.Int32
	opts := options()
	opts.Acquire = func() bool { return true }
	opts.Release = func() { released.Add(1) }
	opts.Finished = func(Snapshot) { finished.Add(1) }
	task, _ := Start("t-1", Query{}, []Source{
		fake{"fast", answer(files("fast", "kept"), nil)},
		fake{"stuck", func(ctx context.Context) ([]Item, error) {
			<-ctx.Done()
			return files("stuck", "late"), nil
		}},
	}, opts)
	waitSource(t, task, "fast", view.SourceDone)
	task.Cancel()
	s := task.Snapshot()
	if s.Running || task.Running() {
		t.Fatal("still running after Cancel")
	}
	if states(s)["stuck"] != view.SourceCancelled || names(s.Items) != "kept@fast" {
		t.Fatalf("states %v, items %s", states(s), names(s.Items))
	}
	time.Sleep(20 * time.Millisecond) // the stuck source returns; its answer is dropped
	if s := task.Snapshot(); names(s.Items) != "kept@fast" || states(s)["stuck"] != view.SourceCancelled {
		t.Fatalf("a late answer was written: %v %s", states(s), names(s.Items))
	}
	task.Cancel()
	if finished.Load() != 1 || released.Load() != 1 {
		t.Errorf("finished %d, released %d; want one each", finished.Load(), released.Load())
	}
}

// C-8.1: the ceiling ends the task even when a source ignores its context.
func TestCeilingEndsASourceThatNeverReturns(t *testing.T) {
	opts := options()
	opts.TotalTimeout = 40 * time.Millisecond
	opts.SourceTimeout = time.Hour
	hold := make(chan struct{})
	returned := make(chan struct{})
	task, _ := Start("t-1", Query{}, []Source{fake{"deaf", func(context.Context) ([]Item, error) {
		defer close(returned)
		<-hold
		return files("deaf", "late"), nil
	}}}, opts)
	s := wait(t, task)
	if states(s)["deaf"] != view.SourceTimeout {
		t.Fatalf("states = %v", states(s))
	}
	close(hold)
	<-returned
	time.Sleep(10 * time.Millisecond)
	if s := task.Snapshot(); len(s.Items) != 0 || states(s)["deaf"] != view.SourceTimeout {
		t.Fatalf("late answer written: %v %s", states(s), names(s.Items))
	}
}

// S-15.1: retry asks that source and nobody else; after the end it is a new
// round with its own slot.
func TestRetryAsksOnlyThatSource(t *testing.T) {
	var calmCalls, flakyCalls atomic.Int32
	var acquired, released, finished atomic.Int32
	slots := true
	var mu sync.Mutex
	opts := options()
	opts.Acquire = func() bool {
		mu.Lock()
		defer mu.Unlock()
		if slots {
			acquired.Add(1)
		}
		return slots
	}
	opts.Release = func() { released.Add(1) }
	opts.Finished = func(Snapshot) { finished.Add(1) }
	task, _ := Start("t-1", Query{Text: "x"}, []Source{
		fake{"calm", func(context.Context) ([]Item, error) {
			calmCalls.Add(1)
			return files("calm", "m"), nil
		}},
		fake{"flaky", func(context.Context) ([]Item, error) {
			if flakyCalls.Add(1) == 1 {
				return nil, errors.New("502 Bad Gateway")
			}
			return files("flaky", "a", "z"), nil
		}},
	}, opts)
	if s := wait(t, task); states(s)["flaky"] != view.SourceError || names(s.Items) != "m@calm" {
		t.Fatalf("first round: %v %s", states(s), names(s.Items))
	}
	if err := task.Retry("nobody"); !errors.Is(err, ErrUnknownSource) {
		t.Errorf("unknown source: %v", err)
	}
	if err := task.Retry("flaky"); err != nil {
		t.Fatal(err)
	}
	s := wait(t, task)
	if states(s)["flaky"] != view.SourceDone || names(s.Items) != "a@flaky,m@calm,z@flaky" {
		t.Fatalf("after retry: %v %s", states(s), names(s.Items))
	}
	if calmCalls.Load() != 1 || flakyCalls.Load() != 2 {
		t.Errorf("calls: calm %d, flaky %d", calmCalls.Load(), flakyCalls.Load())
	}
	if acquired.Load() != 2 || released.Load() != 2 || finished.Load() != 2 {
		t.Errorf("acquired %d, released %d, finished %d; want 2 each", acquired.Load(), released.Load(), finished.Load())
	}

	mu.Lock()
	slots = false
	mu.Unlock()
	if err := task.Retry("calm"); !errors.Is(err, ErrBusy) {
		t.Errorf("retry without a slot: %v", err)
	}
	if task.Running() {
		t.Error("a refused retry started a round")
	}
}

func TestBusyAsksNobody(t *testing.T) {
	var asked atomic.Bool
	opts := options()
	opts.Acquire = func() bool { return false }
	task, err := Start("t-1", Query{}, []Source{fake{"a", func(context.Context) ([]Item, error) {
		asked.Store(true)
		return nil, nil
	}}}, opts)
	if !errors.Is(err, ErrBusy) || task != nil {
		t.Fatalf("task %v, err %v", task, err)
	}
	time.Sleep(10 * time.Millisecond)
	if asked.Load() {
		t.Error("a source was asked without a slot")
	}
}

func TestNoSourcesEndsAtOnce(t *testing.T) {
	var released, finished atomic.Int32
	opts := options()
	opts.Acquire = func() bool { return true }
	opts.Release = func() { released.Add(1) }
	opts.Finished = func(Snapshot) { finished.Add(1) }
	task, err := Start("t-1", Query{}, nil, opts)
	if err != nil || task.Running() || released.Load() != 1 || finished.Load() != 1 {
		t.Fatalf("err %v, running %v, released %d, finished %d", err, task.Running(), released.Load(), finished.Load())
	}
}
