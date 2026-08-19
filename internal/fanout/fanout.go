// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package fanout polls several DAV collections at once for the unified view and
// the cross-source search (§14, §16).
//
// The shape of it is fixed by one requirement: the handler must not block until
// the last server answers. A task is therefore an object with a lifetime of its
// own — started by a request, outliving it, reporting progress as each source
// lands, and cancelled explicitly when the page is left. Nothing here derives
// its context from a request context; a request that ends must not cancel the
// task it started, and a task that is abandoned must not keep a goroutine
// waiting on a socket nobody is reading (§21).
package fanout

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// State is where one source has got to (§16).
type State string

const (
	// StateWaiting is a source whose turn has not come yet.
	StateWaiting State = "waiting"
	// StateQuerying is a source being polled right now.
	StateQuerying State = "querying"
	// StateDone is a source that answered with records.
	StateDone State = "done"
	// StateEmpty is a source that answered with nothing, which is not an
	// error and must not be shown as one.
	StateEmpty State = "empty"
	// StateError is a source that refused or failed.
	StateError State = "error"
	// StateTimeout is a source that had not answered when the ceiling was
	// reached. The person is never left with an indicator that never stops.
	StateTimeout State = "timeout"
	// StateCancelled is a source dropped because the task was cancelled.
	StateCancelled State = "cancelled"
)

// Terminal reports whether a state can still change on its own.
func (s State) Terminal() bool {
	switch s {
	case StateWaiting, StateQuerying:
		return false
	default:
		return true
	}
}

// Failed reports whether a source is one of the "unavailable" of the summary
// line.
func (s State) Failed() bool { return s == StateError || s == StateTimeout }

// Source is one collection to poll. Everything the progress panel prints about
// a source is here, so a snapshot needs nothing else to render.
type Source struct {
	// ID is stable across retries and is what the retry button names.
	ID              string
	Kind            string
	AccountID       string
	AccountLabel    string
	Collection      string
	CollectionLabel string
	Color           string
	// ReadOnly marks a collection that cannot be written to, which is what
	// makes it unusable as the target of a merge (§15).
	ReadOnly bool
}

// Item is one record a source produced. The package does not look inside Data;
// it merges by Key and hands Data back untouched.
type Item struct {
	SourceID string
	// Key orders the merged list. Callers build it from the natural key of
	// the type: the start time of an event, the display name of a contact
	// (§14).
	Key  string
	Data any
}

// Query polls one source. It returns the records found and whether the answer
// came out of the session cache without a request leaving the process (§12,
// §16). A Query must respect ctx: it is cancelled on the source timeout, on the
// overall ceiling and when the task is abandoned.
type Query func(ctx context.Context, src Source) (items []Item, fromCache bool, err error)

// Default timeouts of §16.
const (
	DefaultSourceTimeout = 10 * time.Second
	DefaultTotalTimeout  = 30 * time.Second
	// DefaultParallel caps how many sources are in flight at once, so ten
	// accounts do not open ten sockets in the same instant.
	DefaultParallel = 6
)

// Options configure a task. Zero values fall back to the defaults above.
type Options struct {
	SourceTimeout time.Duration
	TotalTimeout  time.Duration
	Parallel      int
	Now           func() time.Time
}

func (o Options) withDefaults() Options {
	if o.SourceTimeout <= 0 {
		o.SourceTimeout = DefaultSourceTimeout
	}
	if o.TotalTimeout <= 0 {
		o.TotalTimeout = DefaultTotalTimeout
	}
	if o.Parallel <= 0 {
		o.Parallel = DefaultParallel
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// sourceState is the mutable half of a source, held under the task lock.
type sourceState struct {
	src       Source
	state     State
	fromCache bool
	reason    string
	items     []Item
	startedAt time.Time
	duration  time.Duration
	attempt   int
}

// Task is one fan-out in progress.
type Task struct {
	ID string

	opts    Options
	query   Query
	started time.Time

	// root is cancelled once, by Cancel, and every attempt hangs off it.
	root       context.Context
	rootCancel context.CancelFunc

	mu       sync.Mutex
	order    []string
	sources  map[string]*sourceState
	inFlight int
	// runCancel stops the sweep that is under the overall ceiling. A retry
	// after the ceiling gets its own context, so it is not born cancelled.
	runCancel context.CancelFunc
	expired   bool
	cancelled bool
	// settled records that the elapsed time has been taken. A task that
	// finished in less than the clock's resolution has a legitimate elapsed
	// time of zero, so the duration itself cannot be the sentinel.
	settled bool
	elapsed time.Duration
	subs    map[chan struct{}]struct{}

	wg   sync.WaitGroup
	stop sync.Once
}

// ErrNoSources is returned when a task is asked for with nothing to poll.
var ErrNoSources = errors.New("fanout: no sources selected")

// Start begins polling every source and returns at once. The caller keeps the
// task to render progress from and must Cancel it when the page it belongs to
// is left or replaced (§16).
func Start(id string, sources []Source, query Query, opts Options) (*Task, error) {
	if query == nil {
		return nil, errors.New("fanout: query is required")
	}
	if len(sources) == 0 {
		return nil, ErrNoSources
	}
	opts = opts.withDefaults()
	root, rootCancel := context.WithCancel(context.Background())
	runCtx, runCancel := context.WithTimeout(root, opts.TotalTimeout)

	t := &Task{
		ID:         id,
		opts:       opts,
		query:      query,
		started:    opts.Now(),
		root:       root,
		rootCancel: rootCancel,
		sources:    make(map[string]*sourceState, len(sources)),
		order:      make([]string, 0, len(sources)),
		runCancel:  runCancel,
		subs:       make(map[chan struct{}]struct{}),
	}
	for _, src := range sources {
		if _, seen := t.sources[src.ID]; seen || src.ID == "" {
			continue
		}
		t.sources[src.ID] = &sourceState{src: src, state: StateWaiting}
		t.order = append(t.order, src.ID)
	}
	if len(t.order) == 0 {
		rootCancel()
		return nil, ErrNoSources
	}

	gate := make(chan struct{}, opts.Parallel)
	t.mu.Lock()
	t.inFlight = len(t.order)
	t.mu.Unlock()
	for _, id := range t.order {
		t.wg.Add(1)
		go t.run(runCtx, id, gate)
	}
	// The ceiling has to be enforced even if every goroutine is stuck: mark
	// whatever has not answered and let the task read as finished.
	t.wg.Add(1)
	go t.watchCeiling(runCtx)
	return t, nil
}

// run polls one source. Every exit path goes through finish, so a source cannot
// be left in "querying" whatever the query does.
func (t *Task) run(ctx context.Context, id string, gate chan struct{}) {
	defer t.wg.Done()

	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		t.finish(id, nil, false, ctx.Err())
		return
	}
	src, ok := t.begin(id)
	if !ok {
		return
	}
	attemptCtx, cancel := context.WithTimeout(ctx, t.opts.SourceTimeout)
	defer cancel()
	items, fromCache, err := t.query(attemptCtx, src)
	t.finish(id, items, fromCache, err)
}

// begin moves a source to "querying" and reports whether it should be polled at
// all: a task cancelled before its turn came must not open a connection.
func (t *Task) begin(id string) (Source, bool) {
	t.mu.Lock()
	st := t.sources[id]
	if st == nil {
		t.mu.Unlock()
		return Source{}, false
	}
	if t.cancelled {
		st.state = StateCancelled
		t.settleLocked()
		t.mu.Unlock()
		t.notify()
		return Source{}, false
	}
	st.state = StateQuerying
	st.startedAt = t.opts.Now()
	st.attempt++
	src := st.src
	t.mu.Unlock()
	t.notify()
	return src, true
}

func (t *Task) finish(id string, items []Item, fromCache bool, err error) {
	t.mu.Lock()
	st := t.sources[id]
	if st == nil {
		t.mu.Unlock()
		return
	}
	if !st.startedAt.IsZero() {
		st.duration = t.opts.Now().Sub(st.startedAt)
	}
	st.fromCache = fromCache
	switch {
	case err == nil:
		st.items = items
		st.reason = ""
		if len(items) == 0 {
			st.state = StateEmpty
		} else {
			st.state = StateDone
		}
	case t.cancelled && isCancellation(err):
		st.state = StateCancelled
		st.reason = "poll interrupted"
	case isDeadline(err) || (t.expired && isCancellation(err)):
		st.state = StateTimeout
		st.reason = "timed out"
	case isCancellation(err):
		st.state = StateCancelled
		st.reason = "poll interrupted"
	default:
		st.state = StateError
		st.reason = reasonOf(err)
	}
	t.settleLocked()
	t.mu.Unlock()
	t.notify()
}

// settleLocked keeps the in-flight count and the elapsed time in step with the
// source states. It must be called with the lock held.
func (t *Task) settleLocked() {
	running := 0
	for _, st := range t.sources {
		if !st.state.Terminal() {
			running++
		}
	}
	t.inFlight = running
	if running == 0 {
		if !t.settled {
			t.settled = true
			t.elapsed = t.opts.Now().Sub(t.started)
		}
		if t.runCancel != nil {
			// Release the ceiling timer as soon as there is nothing left to
			// time out.
			t.runCancel()
		}
	}
}

// watchCeiling marks anything still outstanding when the overall ceiling passes.
func (t *Task) watchCeiling(ctx context.Context) {
	defer t.wg.Done()
	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return
	}
	t.mu.Lock()
	t.expired = true
	changed := false
	for _, st := range t.sources {
		if st.state.Terminal() {
			continue
		}
		st.state = StateTimeout
		st.reason = "timed out"
		if !st.startedAt.IsZero() {
			st.duration = t.opts.Now().Sub(st.startedAt)
		}
		changed = true
	}
	if changed {
		t.settleLocked()
	}
	t.mu.Unlock()
	if changed {
		t.notify()
	}
}

// Retry polls one source again, leaving every other source alone (§16).
func (t *Task) Retry(id string) bool {
	t.mu.Lock()
	st := t.sources[id]
	if st == nil || t.cancelled || !st.state.Terminal() {
		t.mu.Unlock()
		return false
	}
	st.state = StateWaiting
	st.reason = ""
	st.items = nil
	st.duration = 0
	st.startedAt = time.Time{}
	// The retry is a sweep of its own, so the elapsed time is timed from here
	// rather than from the original poll, which would count the idle time in
	// between.
	t.settled = false
	t.elapsed = 0
	t.started = t.opts.Now()
	t.settleLocked()
	t.mu.Unlock()
	t.notify()

	// The retry hangs off the task root rather than the finished sweep, and
	// carries the source timeout on its own.
	gate := make(chan struct{}, 1)
	t.wg.Add(1)
	go t.run(t.root, id, gate)
	return true
}

// Cancel ends the task: every attempt's context is cancelled, so the sockets
// close, and partial results stay readable with their sources marked (§16).
func (t *Task) Cancel() {
	t.mu.Lock()
	t.cancelled = true
	for _, st := range t.sources {
		if !st.state.Terminal() {
			st.state = StateCancelled
			st.reason = "poll interrupted"
		}
	}
	t.settleLocked()
	t.mu.Unlock()
	t.stop.Do(func() { t.rootCancel() })
	t.notify()
}

// Wait blocks until nothing is in flight. It exists for tests and for shutdown;
// no request handler waits on a task.
func (t *Task) Wait() { t.wg.Wait() }

// Subscribe returns a channel that receives a token whenever the task changes,
// and the function that releases it. Sends are coalesced and never block, so a
// slow SSE reader cannot hold up a source goroutine.
func (t *Task) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	t.mu.Lock()
	t.subs[ch] = struct{}{}
	t.mu.Unlock()
	return ch, func() {
		t.mu.Lock()
		delete(t.subs, ch)
		t.mu.Unlock()
	}
}

func (t *Task) notify() {
	t.mu.Lock()
	subs := make([]chan struct{}, 0, len(t.subs))
	for ch := range t.subs {
		subs = append(subs, ch)
	}
	t.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SourceStatus is one row of the sources panel.
type SourceStatus struct {
	Source
	State     State
	Count     int
	Attempt   int
	FromCache bool
	Reason    string
	Duration  time.Duration
}

// Snapshot is a consistent view of a task, safe to render.
type Snapshot struct {
	ID        string
	Sources   []SourceStatus
	Items     []Item
	Total     int
	Queried   int
	Records   int
	Failed    int
	Cached    int
	Running   bool
	Cancelled bool
	Elapsed   time.Duration
}

// Snapshot copies the current state. Items come back merged and sorted, so a
// slow source lands in its place in the list rather than at the end (§16).
func (t *Task) Snapshot() Snapshot {
	t.mu.Lock()
	snap := Snapshot{
		ID:        t.ID,
		Total:     len(t.order),
		Cancelled: t.cancelled,
		Sources:   make([]SourceStatus, 0, len(t.order)),
		Elapsed:   t.elapsed,
	}
	for _, id := range t.order {
		st := t.sources[id]
		if st == nil {
			continue
		}
		snap.Sources = append(snap.Sources, SourceStatus{
			Source: st.src, State: st.state, Count: len(st.items),
			Attempt: st.attempt, FromCache: st.fromCache,
			Reason: st.reason, Duration: st.duration.Round(time.Millisecond),
		})
		if st.state.Terminal() {
			snap.Queried++
		}
		if st.state.Failed() {
			snap.Failed++
		}
		if st.fromCache && st.state.Terminal() && !st.state.Failed() {
			snap.Cached++
		}
		snap.Items = append(snap.Items, st.items...)
		snap.Records += len(st.items)
	}
	snap.Running = t.inFlight > 0
	if snap.Running {
		snap.Elapsed = t.opts.Now().Sub(t.started)
	}
	t.mu.Unlock()

	sort.SliceStable(snap.Items, func(i, j int) bool { return snap.Items[i].Key < snap.Items[j].Key })
	return snap
}

// Summary is the line above the results (§16). While the poll runs it says how
// far it has got; when it ends it says how long it took, so the person is never
// left guessing whether anything is still coming.
func (s Snapshot) Summary() string {
	parts := []string{
		fmt.Sprintf("Queried %d of %d %s", s.Queried, s.Total, noun(s.Total, "source", "sources")),
		fmt.Sprintf("%d %s", s.Records, noun(s.Records, "record", "records")),
	}
	if s.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d unavailable", s.Failed))
	}
	if s.Cached > 0 {
		parts = append(parts, fmt.Sprintf("%d from cache", s.Cached))
	}
	if !s.Running {
		if s.Cancelled {
			parts = append(parts, "poll interrupted")
		}
		parts = append(parts, s.Elapsed.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}

// Percent is how far the poll bar has filled: queried sources over the total,
// 100 when nothing is left to wait for.
func (s Snapshot) Percent() int {
	if s.Total <= 0 {
		if s.Running {
			return 0
		}
		return 100
	}
	n := s.Queried
	if n > s.Total {
		n = s.Total
	}
	return 100 * n / s.Total
}

func noun(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func isCancellation(err error) bool { return errors.Is(err, context.Canceled) }

func isDeadline(err error) bool { return errors.Is(err, context.DeadlineExceeded) }

// reasonOf keeps the panel's error column short: a person scanning seven rows
// wants "connection refused", not a wrapped chain naming every layer.
func reasonOf(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		msg = msg[i+2:]
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "failed"
	}
	const max = 120
	if runes := []rune(msg); len(runes) > max {
		msg = string(runes[:max]) + "…"
	}
	return msg
}
