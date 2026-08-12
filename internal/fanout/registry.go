// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package fanout

import (
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// A task belongs to the session that started it (§16), and nothing about it
// reaches the volume. The registry is the owner: it keeps at most a handful per
// session, cancels whatever it drops, and cancels everything a session leaves
// behind when that session ends. Without an owner, a person clicking through
// searches would accumulate live polls until the process ran out of sockets.
const (
	// MaxPerSession bounds how many tasks one browser can hold open.
	MaxPerSession = 4
	// TaskTTL drops a task nobody has asked about. A page left open keeps
	// polling status, so a task that goes quiet for this long is gone.
	TaskTTL = 10 * time.Minute
)

// ErrNoTask is returned for an unknown, finished-and-swept or cancelled task.
var ErrNoTask = errors.New("fanout: no such task")

type entry struct {
	task      *Task
	sessionID string
	touched   time.Time
}

// Registry holds the live tasks of every session.
type Registry struct {
	now func() time.Time

	mu     sync.Mutex
	byID   map[string]*entry
	bySess map[string][]string
}

// NewRegistry returns an empty registry. A nil now means time.Now.
func NewRegistry(now func() time.Time) *Registry {
	if now == nil {
		now = time.Now
	}
	return &Registry{now: now, byID: make(map[string]*entry), bySess: make(map[string][]string)}
}

// Start creates a task for one session and registers it.
func (r *Registry) Start(sessionID string, sources []Source, query Query, opts Options) (*Task, error) {
	id, err := newTaskID()
	if err != nil {
		return nil, err
	}
	task, err := Start(id, sources, query, opts)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.sweepLocked()
	r.byID[id] = &entry{task: task, sessionID: sessionID, touched: r.now()}
	r.bySess[sessionID] = append(r.bySess[sessionID], id)
	stale := r.trimLocked(sessionID)
	r.mu.Unlock()
	cancelAll(stale)
	return task, nil
}

// Get returns a session's task and marks it as still wanted.
func (r *Registry) Get(sessionID, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[taskID]
	if !ok || e.sessionID != sessionID {
		return nil, ErrNoTask
	}
	e.touched = r.now()
	return e.task, nil
}

// Cancel ends one task of one session.
func (r *Registry) Cancel(sessionID, taskID string) {
	r.mu.Lock()
	e, ok := r.byID[taskID]
	if !ok || e.sessionID != sessionID {
		r.mu.Unlock()
		return
	}
	r.dropLocked(taskID)
	r.mu.Unlock()
	e.task.Cancel()
}

// CancelSession ends every task of a session. Logout and expiry call it, so a
// closed session leaves no poll running (§12, §16).
func (r *Registry) CancelSession(sessionID string) {
	r.mu.Lock()
	ids := append([]string(nil), r.bySess[sessionID]...)
	tasks := make([]*Task, 0, len(ids))
	for _, id := range ids {
		if e, ok := r.byID[id]; ok {
			tasks = append(tasks, e.task)
		}
		delete(r.byID, id)
	}
	delete(r.bySess, sessionID)
	r.mu.Unlock()
	cancelAll(tasks)
}

// Close ends every task in the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	tasks := make([]*Task, 0, len(r.byID))
	for _, e := range r.byID {
		tasks = append(tasks, e.task)
	}
	r.byID = make(map[string]*entry)
	r.bySess = make(map[string][]string)
	r.mu.Unlock()
	cancelAll(tasks)
}

// Len reports how many tasks are registered, for tests.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byID)
}

// Sweep drops tasks nobody has asked about within TaskTTL and cancels them.
func (r *Registry) Sweep() int {
	r.mu.Lock()
	stale := r.sweepLocked()
	r.mu.Unlock()
	cancelAll(stale)
	return len(stale)
}

// sweepLocked must be called with the lock held.
func (r *Registry) sweepLocked() []*Task {
	cutoff := r.now().Add(-TaskTTL)
	var stale []*Task
	for id, e := range r.byID {
		if e.touched.Before(cutoff) {
			stale = append(stale, e.task)
			r.dropLocked(id)
		}
	}
	return stale
}

// trimLocked keeps a session inside MaxPerSession, oldest first.
func (r *Registry) trimLocked(sessionID string) []*Task {
	ids := r.bySess[sessionID]
	if len(ids) <= MaxPerSession {
		return nil
	}
	var stale []*Task
	for _, id := range ids[:len(ids)-MaxPerSession] {
		if e, ok := r.byID[id]; ok {
			stale = append(stale, e.task)
			delete(r.byID, id)
		}
	}
	r.bySess[sessionID] = append([]string(nil), ids[len(ids)-MaxPerSession:]...)
	return stale
}

// dropLocked unlinks a task without cancelling it.
func (r *Registry) dropLocked(taskID string) {
	e, ok := r.byID[taskID]
	if !ok {
		return
	}
	delete(r.byID, taskID)
	ids := r.bySess[e.sessionID]
	for i, id := range ids {
		if id == taskID {
			ids = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(ids) == 0 {
		delete(r.bySess, e.sessionID)
	} else {
		r.bySess[e.sessionID] = ids
	}
}

func cancelAll(tasks []*Task) {
	for _, task := range tasks {
		task.Cancel()
	}
}

func newTaskID() (string, error) {
	b, err := crypto.Random(16)
	if err != nil {
		return "", err
	}
	defer crypto.Zero(b)
	return base64.RawURLEncoding.EncodeToString(b), nil
}
