// Package fanout asks many sources one question at once (S-15). A task is an
// object the session keeps: its query, its sources and their states, and the
// items that have arrived, kept in order. Nothing is pushed from here (S-9.4):
// the request layer reads a snapshot, and waits on Changed when it streams.
package fanout

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

// Query is what every source of a task is asked.
type Query struct {
	Text     string     // empty: a list, not a search
	From, To *time.Time // agenda window; nil: no window
}

// Item is one answer. Records carry the object the REPORT returned; files
// carry the name and size the folder listing returned.
type Item struct {
	AccountID, Col, Path, UID, ETag string
	Kind                            view.Kind
	FromCache                       bool
	Object                          *model.Object // records; nil for files
	FileName                        string
	FileSize                        int64
}

// Source is one collection behind a task.
type Source interface {
	ID() string
	Run(ctx context.Context, q Query) ([]Item, error)
}

var (
	// ErrIncomplete wraps an answer that stopped short, such as a folder walk
	// that reached its entry ceiling. The items stay, the source counts as
	// answered, and the reason is its note.
	ErrIncomplete = errors.New("the answer is incomplete")
	// ErrBusy: the session already runs as many fanouts as it may. The task
	// does not start and is not queued.
	ErrBusy = errors.New("fanout: too many live fanouts")
	// ErrUnknownSource: a retry named a source the task does not have.
	ErrUnknownSource = errors.New("fanout: no such source")
)

// Options are the limits and hooks of one task.
type Options struct {
	Parallelism   int           // sources asked at the same time
	SourceTimeout time.Duration // one source
	TotalTimeout  time.Duration // one round: the start, or a retry after the end
	// Key orders items: ascending, and an item arriving later goes after an
	// equal one already shown.
	Key func(Item) string
	// Acquire reserves a live slot for a round and Release frees it; both nil
	// means no ceiling. A round holds its slot until every source settles.
	Acquire func() bool
	Release func()
	// Finished runs once per round, after the last source settles, outside
	// the task's lock.
	Finished func(Snapshot)
}

// SourceStatus is one source in a snapshot (C-8.1).
type SourceStatus struct {
	ID        string
	State     view.SourceState
	Count     int  // items it answered with; meaningful for done and empty
	FromCache bool // every item it answered with came from the session cache
	Err       error
	Note      string // why an answered source is incomplete
}

// Snapshot is a task as one reader sees it.
type Snapshot struct {
	ID       string
	Query    Query
	Running  bool
	Started  time.Time // start of the last round
	Finished time.Time // end of the last round; zero while running
	Sources  []SourceStatus
	Items    []Item
}

// Task is one fanout. Lock order: the task's lock may be held while Acquire
// and Release take the session's lock, never the other way round; Running
// does not lock, so the session can ask it under its own.
type Task struct {
	id    string
	query Query
	opts  Options
	sem   *semaphore.Weighted

	running atomic.Bool

	mu       sync.Mutex
	runs     []*run
	items    []entry
	ctx      context.Context
	cancel   context.CancelFunc
	pending  int
	round    int  // bumped by every round
	open     bool // the round has sources to settle
	started  time.Time
	finished time.Time
	changed  chan struct{}
}

type run struct {
	src   Source
	gen   int // bumped on every launch and expiry: a late answer of an older one is dropped
	state view.SourceState
	count int
	cache bool
	err   error
	note  string
}

type entry struct {
	key  string
	src  int
	item Item
}

// Start asks every source and returns at once; the sources settle in the
// background. With no slot free it returns ErrBusy and asks nobody.
func Start(id string, q Query, sources []Source, opts Options) (*Task, error) {
	if opts.Parallelism < 1 {
		opts.Parallelism = 1
	}
	if opts.Acquire != nil && !opts.Acquire() {
		return nil, ErrBusy
	}
	t := &Task{
		id:      id,
		query:   q,
		opts:    opts,
		sem:     semaphore.NewWeighted(int64(opts.Parallelism)),
		changed: make(chan struct{}),
	}
	t.mu.Lock()
	for _, src := range sources {
		t.runs = append(t.runs, &run{src: src, state: view.SourceWaiting})
	}
	t.beginRoundLocked()
	for i := range t.runs {
		t.launchLocked(i)
	}
	snap, round, done := t.endRoundIfIdleLocked()
	t.mu.Unlock()
	t.finish(snap, round, done)
	return t, nil
}

// ID is the task identifier the routes carry.
func (t *Task) ID() string { return t.id }

// Query is what the task asks.
func (t *Task) Query() Query { return t.query }

// Running reports whether a round is still going, its Finished hook included.
// It does not lock.
func (t *Task) Running() bool { return t.running.Load() }

// Changed is closed on the next change of the task. Take it before the
// snapshot, so a change between the two is not missed.
func (t *Task) Changed() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.changed
}

// Snapshot copies the task.
func (t *Task) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshotLocked()
}

// Cancel stops the round: sources not yet answered become cancelled, what has
// arrived stays. It returns after the states are written.
func (t *Task) Cancel() {
	t.mu.Lock()
	ctx, cancel := t.ctx, t.cancel
	t.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	t.expire(ctx)
}

// Retry asks one settled source again, and nobody else. Its earlier items go.
// After the end of the task this is a new round, which needs a slot.
func (t *Task) Retry(sourceID string) error {
	t.mu.Lock()
	i := -1
	for n, r := range t.runs {
		if r.src.ID() == sourceID {
			i = n
			break
		}
	}
	if i < 0 {
		t.mu.Unlock()
		return ErrUnknownSource
	}
	if s := t.runs[i].state; s == view.SourceWaiting || s == view.SourceQuerying {
		t.mu.Unlock()
		return nil
	}
	if t.pending == 0 {
		if t.opts.Acquire != nil && !t.opts.Acquire() {
			t.mu.Unlock()
			return ErrBusy
		}
		t.beginRoundLocked()
	}
	t.launchLocked(i)
	t.notifyLocked()
	t.mu.Unlock()
	return nil
}

func (t *Task) beginRoundLocked() {
	t.ctx, t.cancel = context.WithTimeout(context.Background(), t.opts.TotalTimeout)
	t.round++
	t.open = true
	t.started = time.Now()
	t.finished = time.Time{}
	t.running.Store(true)
	ctx := t.ctx
	go func() {
		<-ctx.Done()
		t.expire(ctx)
	}()
}

func (t *Task) launchLocked(i int) {
	r := t.runs[i]
	r.gen++
	r.state, r.count, r.cache, r.err, r.note = view.SourceWaiting, 0, false, nil, ""
	t.dropItemsLocked(i)
	t.pending++
	go t.work(t.ctx, i, r.gen, r.src)
}

func (t *Task) work(ctx context.Context, i, gen int, src Source) {
	if err := t.sem.Acquire(ctx, 1); err != nil {
		t.settle(ctx, nil, i, gen, nil, err)
		return
	}
	defer t.sem.Release(1)
	if !t.markQuerying(i, gen) {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, t.opts.SourceTimeout)
	defer cancel()
	items, err := src.Run(sctx, t.query)
	t.settle(ctx, sctx, i, gen, items, err)
}

func (t *Task) markQuerying(i, gen int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	r := t.runs[i]
	if r.gen != gen {
		return false
	}
	r.state = view.SourceQuerying
	t.notifyLocked()
	return true
}

// settle writes one answer. Keys are computed before the lock: a sort key
// may read the whole object.
func (t *Task) settle(roundCtx, source context.Context, i, gen int, items []Item, err error) {
	state, note := classify(roundCtx, source, items, err)
	answered := state == view.SourceDone || state == view.SourceEmpty
	var fresh []entry
	if answered {
		fresh = make([]entry, len(items))
		for n, it := range items {
			fresh[n] = entry{key: t.key(it), src: i, item: it}
		}
		sort.SliceStable(fresh, func(a, b int) bool { return fresh[a].key < fresh[b].key })
	}

	t.mu.Lock()
	r := t.runs[i]
	if r.gen != gen {
		t.mu.Unlock()
		return
	}
	r.state, r.note = state, note
	if answered {
		r.count = len(items)
		r.cache = len(items) > 0
		for _, it := range items {
			r.cache = r.cache && it.FromCache
		}
		t.items = merge(t.items, fresh)
	} else {
		r.err = err
		if r.err == nil {
			r.err = roundCtx.Err()
		}
	}
	t.pending--
	t.notifyLocked()
	snap, round, done := t.endRoundIfIdleLocked()
	t.mu.Unlock()
	t.finish(snap, round, done)
}

// expire settles every source still open when the round's context ends:
// cancelled when it was stopped, timeout when the ceiling passed. A source
// that ignores its context cannot keep the indicator alive.
func (t *Task) expire(ctx context.Context) {
	t.mu.Lock()
	if t.ctx != ctx || t.pending == 0 {
		t.mu.Unlock()
		return
	}
	state := view.SourceTimeout
	if errors.Is(ctx.Err(), context.Canceled) {
		state = view.SourceCancelled
	}
	for _, r := range t.runs {
		if r.state == view.SourceWaiting || r.state == view.SourceQuerying {
			r.gen++
			r.state, r.err = state, ctx.Err()
		}
	}
	t.pending = 0
	t.notifyLocked()
	snap, round, done := t.endRoundIfIdleLocked()
	t.mu.Unlock()
	t.finish(snap, round, done)
}

// endRoundIfIdleLocked closes a round whose sources have all settled. The
// task keeps reading as running until finish has run the hook, so a reader
// that sees the end also sees what the hook wrote.
func (t *Task) endRoundIfIdleLocked() (Snapshot, int, bool) {
	if t.pending > 0 || !t.open {
		return Snapshot{}, 0, false
	}
	t.open = false
	t.finished = time.Now()
	t.cancel()
	if t.opts.Release != nil {
		t.opts.Release()
	}
	snap := t.snapshotLocked()
	snap.Running = false
	return snap, t.round, true
}

func (t *Task) finish(snap Snapshot, round int, done bool) {
	if !done {
		return
	}
	if t.opts.Finished != nil {
		t.opts.Finished(snap)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.round == round && !t.open {
		t.running.Store(false)
		t.notifyLocked()
	}
}

func (t *Task) notifyLocked() {
	close(t.changed)
	t.changed = make(chan struct{})
}

func (t *Task) dropItemsLocked(i int) {
	kept := t.items[:0]
	for _, e := range t.items {
		if e.src != i {
			kept = append(kept, e)
		}
	}
	for n := len(kept); n < len(t.items); n++ {
		t.items[n] = entry{}
	}
	t.items = kept
}

func (t *Task) snapshotLocked() Snapshot {
	s := Snapshot{
		ID:       t.id,
		Query:    t.query,
		Running:  t.running.Load(),
		Started:  t.started,
		Finished: t.finished,
		Sources:  make([]SourceStatus, len(t.runs)),
		Items:    make([]Item, len(t.items)),
	}
	for n, r := range t.runs {
		s.Sources[n] = SourceStatus{ID: r.src.ID(), State: r.state, Count: r.count, FromCache: r.cache, Err: r.err, Note: r.note}
	}
	for n, e := range t.items {
		s.Items[n] = e.item
	}
	return s
}

func (t *Task) key(it Item) string {
	if t.opts.Key == nil {
		return ""
	}
	return t.opts.Key(it)
}

// merge inserts sorted fresh entries into sorted items; an entry already
// shown stays ahead of an equal newcomer, so rows do not jump.
func merge(items, fresh []entry) []entry {
	if len(fresh) == 0 {
		return items
	}
	out := make([]entry, 0, len(items)+len(fresh))
	a, b := 0, 0
	for a < len(items) && b < len(fresh) {
		if fresh[b].key < items[a].key {
			out = append(out, fresh[b])
			b++
		} else {
			out = append(out, items[a])
			a++
		}
	}
	out = append(out, items[a:]...)
	return append(out, fresh[b:]...)
}

// classify is C-8.1. An empty answer and a refusal differ; a server that
// could not be reached is unavailable, not a refusal.
func classify(round, source context.Context, items []Item, err error) (view.SourceState, string) {
	switch {
	case err == nil || errors.Is(err, ErrIncomplete):
		note := ""
		if err != nil {
			note = strings.TrimPrefix(err.Error(), ErrIncomplete.Error()+": ")
		}
		if len(items) == 0 {
			return view.SourceEmpty, note
		}
		return view.SourceDone, note
	case errors.Is(round.Err(), context.Canceled):
		return view.SourceCancelled, ""
	case round.Err() != nil,
		source != nil && errors.Is(source.Err(), context.DeadlineExceeded),
		errors.Is(err, context.DeadlineExceeded):
		return view.SourceTimeout, ""
	}
	var op *net.OpError
	var dns *net.DNSError
	if errors.As(err, &op) || errors.As(err, &dns) {
		if op != nil && op.Timeout() {
			return view.SourceTimeout, ""
		}
		return view.SourceUnavailable, ""
	}
	return view.SourceError, ""
}
