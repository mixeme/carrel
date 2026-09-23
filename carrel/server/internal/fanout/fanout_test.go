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

// The ceiling counts far requests in flight, and a source takes a slot
// around each one (Slot). Ten sources asking twice apiece never put more
// than three requests on the wire at once.
//
// Until 22.09.2026 the ceiling counted sources instead, and the fanout
// held a slot around the whole of a source's work. That bounded any
// source, cooperating or not — a stronger promise than this one — but it
// bounded the wrong thing: a collection of three kinds waited three times
// as long as one of a single kind while the permitted concurrency stood
// idle, and a slow source kept its slot and blocked others. The promise
// now is narrower and true of what matters: **a source that asks must
// take a slot for each ask.** A source that asks without one escapes the
// ceiling, and only its author can prevent that; both of Carrel's own
// sources go through one code path that takes it
// (web.collectionSource.askKind).
func TestTheCeilingCountsRequestsInFlight(t *testing.T) {
	opts := options()
	opts.Parallelism = 3
	var now, peak atomic.Int32
	var sources []Source
	for i := 0; i < 10; i++ {
		sources = append(sources, fake{fmt.Sprint(i), func(ctx context.Context) ([]Item, error) {
			for ask := 0; ask < 2; ask++ {
				release, err := Slot(ctx)
				if err != nil {
					return nil, err
				}
				n := now.Add(1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				now.Add(-1)
				release()
			}
			return nil, nil
		}})
	}
	task, _ := Start("t-1", Query{}, sources, opts)
	s := wait(t, task)
	if p := peak.Load(); p > 3 || p < 1 {
		t.Fatalf("peak = %d requests at once, ceiling 3", p)
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

// A source that panics is a source that failed: its state says so, the others
// answer, and the process lives. Without this, one malformed record in one
// collection ended the server for everyone (S-21).
func TestPanickingSourceIsAnError(t *testing.T) {
	task, err := Start("t-1", Query{}, []Source{
		fake{"boom", func(context.Context) ([]Item, error) { panic("malformed") }},
		fake{"found", answer(files("found", "one"), nil)},
	}, options())
	if err != nil {
		t.Fatal(err)
	}
	s := wait(t, task)
	if states(s)["boom"] != view.SourceError || states(s)["found"] != view.SourceDone {
		t.Errorf("states = %v", states(s))
	}
	for _, src := range s.Sources {
		if src.ID == "boom" && (src.Err == nil || !strings.Contains(src.Err.Error(), "malformed")) {
			t.Errorf("boom err = %v", src.Err)
		}
	}
}

// A source that must ask three times gets three times the budget. Until
// 22.09.2026 it got the budget of one, so on a server that answers every
// request in three seconds a calendar of events, tasks and notes could
// not be searched while an address book beside it could — and the screen
// blamed the server for a shortage that was ours.
type askingSource struct {
	id    string
	asks  int
	each  time.Duration
	done  chan struct{}
	calls int
}

func (s *askingSource) ID() string { return s.id }
func (s *askingSource) Asks() int  { return s.asks }

func (s *askingSource) Run(ctx context.Context, q Query) ([]Item, error) {
	for i := 0; i < s.asks; i++ {
		select {
		case <-time.After(s.each):
			s.calls++
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	close(s.done)
	return []Item{{Path: "/p"}}, nil
}

func TestASourceIsGivenItsShareForEveryAsk(t *testing.T) {
	// Four asks of 20ms each against a 50ms-per-ask budget: 80ms of work
	// under 200ms. With the old whole-source budget of 50ms it would be cut
	// off after the second, so the two rules still tell each other apart —
	// but the slack is 120ms rather than the 30ms this test used to leave.
	// Thirty was not enough: a 40ms sleep on a loaded machine overruns, the
	// budget ends the run, Run returns before closing the channel, and the
	// test waits three seconds for something that already gave up
	// (a full run on 23.09.2026 caught exactly that).
	src := &askingSource{id: "s", asks: 4, each: 20 * time.Millisecond, done: make(chan struct{})}
	task, err := Start("t", Query{}, []Source{src}, Options{
		Parallelism:   2,
		SourceTimeout: 50 * time.Millisecond,
		TotalTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-src.done:
	case <-time.After(3 * time.Second):
		t.Fatal("the source never finished")
	}
	for task.Running() {
		time.Sleep(5 * time.Millisecond)
	}
	snap := task.Snapshot()
	if len(snap.Sources) != 1 || snap.Sources[0].State != view.SourceDone {
		t.Fatalf("source = %+v, want done", snap.Sources)
	}
	if src.calls != 4 {
		t.Errorf("%d asks got through, want 4", src.calls)
	}
}

// A source that says nothing keeps the plain budget.
func TestASilentSourceKeepsTheOneBudget(t *testing.T) {
	if got := budgetFor(&askingSource{asks: 1}, 10*time.Second); got != 10*time.Second {
		t.Errorf("one ask = %v", got)
	}
	if got := budgetFor(&askingSource{asks: 3}, 10*time.Second); got != 30*time.Second {
		t.Errorf("three asks = %v", got)
	}
}
