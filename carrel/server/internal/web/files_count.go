package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The owner's decision of 13.11.2026: «Objects» of a file collection is how
// many files it holds at every depth (s-profile-1: «Documents 9», «Public
// 2», «Files 1»). No server says it — Nextcloud counts one folder's members,
// the rest nothing (SV-57) —, so Carrel walks the tree: in the background,
// when Connections opens, storage after storage, a PROPFIND of depth 1 a
// folder, as Files reads them. While it goes, a row says how many so far and
// that it is counting; the count is kept in the session with the change tag
// of the root it was counted at, and walked again only when that tag moved,
// or on Update collection list. A refusal mid-way leaves a dash and says why.
// The decision of 03.10.2026 (ФБ11: a folder row of Files counts only what
// the same PROPFIND says) is the listing's and stands.

// filesCount is one walk of the file collections of a session: a task of the
// session, so that its end cancels the walk (session.Task).
type filesCount struct {
	id     string
	cancel context.CancelFunc

	mu       sync.Mutex
	running  bool
	counting map[string]int // by filesKey: files so far of a storage being walked
}

func (f *filesCount) ID() string { return f.id }

func (f *filesCount) Running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *filesCount) Cancel() { f.cancel() }

// so is how many files the walk has counted in a storage it walks now.
func (f *filesCount) so(key string) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.counting[key]
	return n, ok
}

func (f *filesCount) set(key string, n int, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if on {
		f.counting[key] = n
	} else {
		delete(f.counting, key)
	}
}

// filesKey names a storage among the walk's and in the session's cache.
func filesKey(accountID, path string) string { return accountID + "\x00" + path }

// filesCountKey is the cache entry of a storage's count: «n\x00tag\x00at» —
// n files at the root's change tag, found at at (Unix nanoseconds);
// «!\x00why\x00at» — the walk was refused; empty — not counted (Update
// collection list asks for a walk so).
func filesCountKey(path string) string { return "files\x00" + path }

type filesCounted struct {
	n    int
	tag  string
	note string
	at   time.Time // when the count or the refusal was last found
	ok   bool      // a number or a refusal is kept
}

func peekFilesCount(sess *session.Session, accountID, path string) filesCounted {
	data, ok := sess.Cache().PeekETagMap(accountID, filesCountKey(path))
	if !ok || len(data) == 0 {
		return filesCounted{}
	}
	parts := strings.SplitN(string(data), "\x00", 3)
	if len(parts) != 3 {
		return filesCounted{}
	}
	nanos, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return filesCounted{}
	}
	at := time.Unix(0, nanos)
	if parts[0] == "!" {
		return filesCounted{note: parts[1], at: at, ok: true}
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return filesCounted{}
	}
	return filesCounted{n: n, tag: parts[1], at: at, ok: true}
}

// putFilesCount keeps a count (head n, tail the root's change tag) or a
// refusal (head «!», tail why), found at at.
func putFilesCount(sess *session.Session, accountID, path, head, tail string, at time.Time) {
	value := head + "\x00" + tail + "\x00" + strconv.FormatInt(at.UnixNano(), 10)
	sess.Cache().PutETagMap(accountID, filesCountKey(path), []byte(value))
}

// freshFilesCount tells whether a storage's kept count was found so lately
// that its root is not asked again: as long as a whole read of a list is
// trusted (CARREL_CACHE_COLLECTION_TTL).
func (s *Server) freshFilesCount(kept filesCounted) bool {
	return kept.ok && s.now().Sub(kept.at) < s.cfg.CacheCollectionTTL
}

// forgetFilesCount asks for the storages of a connection to be counted
// again (Update collection list).
func forgetFilesCount(sess *session.Session, blob sealed.Blob, accountID string) {
	for _, col := range blob.CollectionsOf(accountID) {
		if col.Has(view.KindFile) {
			sess.Cache().PutETagMap(accountID, filesCountKey(col.Path), nil)
		}
	}
}

// latestFilesCount is the session's newest walk, if any.
func latestFilesCount(sess *session.Session) *filesCount {
	for _, t := range sess.Tasks() {
		if f, ok := t.(*filesCount); ok {
			return f
		}
	}
	return nil
}

// countFiles starts a walk of the session's file collections unless one is
// running: each storage's root is asked its change tag, and a storage whose
// tag is the one its count was kept at is not walked. A count found lately
// (freshFilesCount) is not asked about at all; when every count is so, no
// walk starts.
func (s *Server) countFiles(sess *session.Session, blob sealed.Blob) {
	if last := latestFilesCount(sess); last != nil && last.Running() {
		return
	}
	type storage struct {
		conn sealed.Connection
		col  sealed.Collection
	}
	var all []storage
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Has(view.KindFile) && !col.Gone && !s.freshFilesCount(peekFilesCount(sess, conn.ID, col.Path)) {
				all = append(all, storage{conn, col})
			}
		}
	}
	if len(all) == 0 {
		return
	}
	id, err := user.NewTaskID()
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &filesCount{id: id, cancel: cancel, running: true, counting: map[string]int{}}
	sess.KeepTask(f)
	go func() {
		defer func() {
			f.mu.Lock()
			f.running = false
			f.mu.Unlock()
			cancel()
		}()
		for _, st := range all {
			if ctx.Err() != nil {
				return
			}
			s.countStorage(ctx, sess, f, st.conn, st.col)
		}
	}()
}

// countStorage walks one storage, unless its root's change tag is the one
// its kept count stands at.
func (s *Server) countStorage(ctx context.Context, sess *session.Session, f *filesCount, conn sealed.Connection, col sealed.Collection) {
	key := filesKey(conn.ID, col.Path)
	p, err := s.fileProvider(conn)
	if err != nil {
		putFilesCount(sess, conn.ID, col.Path, "!", "Not counted: "+sentence(err.Error()), s.now())
		return
	}
	root := strings.TrimSuffix(fileRoot(conn, col), "/") + "/"
	top, err := p.Stat(ctx, root)
	if err != nil {
		if ctx.Err() == nil {
			putFilesCount(sess, conn.ID, col.Path, "!", notCounted(root, err), s.now())
		}
		return
	}
	// A server that gives a folder no change tag gives none that moves: its
	// count stands until Update collection list.
	if kept := peekFilesCount(sess, conn.ID, col.Path); kept.ok && kept.note == "" && kept.tag == top.ETag {
		putFilesCount(sess, conn.ID, col.Path, strconv.Itoa(kept.n), kept.tag, s.now())
		return
	}
	files := 0
	f.set(key, files, true)
	defer f.set(key, 0, false)
	queue := []string{root}
	for len(queue) > 0 {
		folder := queue[0]
		queue = queue[1:]
		entries, err := p.List(ctx, folder)
		if err != nil {
			if ctx.Err() == nil {
				putFilesCount(sess, conn.ID, col.Path, "!", notCounted(folder, err), s.now())
			}
			return
		}
		for _, e := range entries {
			if e.Dir {
				queue = append(queue, strings.TrimSuffix(e.Path, "/")+"/")
				continue
			}
			files++
		}
		f.set(key, files, true)
	}
	putFilesCount(sess, conn.ID, col.Path, strconv.Itoa(files), top.ETag, s.now())
}

// notCounted is why a storage has no count: the request refused, as the
// trace of a connection says it (s-profile-1: «PROPFIND /dav.php/ → 401
// Unauthorized»).
func notCounted(folder string, err error) string {
	var se *dav.StatusError
	if errors.As(err, &se) {
		return fmt.Sprintf("Not counted: PROPFIND %s → %d %s.", folder, se.Code, http.StatusText(se.Code))
	}
	return "Not counted: PROPFIND " + folder + " → " + strings.TrimSuffix(sentence(err.Error()), ".") + "."
}

// fileObjects is what a row of a file collection says in Objects: the files
// so far and that it is counting while the walk goes over it, else the kept
// count — or none, and why, when the walk was refused.
func fileObjects(sess *session.Session, accountID string, col sealed.Collection) (*int, bool, view.Text) {
	if f := latestFilesCount(sess); f != nil {
		if n, ok := f.so(filesKey(accountID, col.Path)); ok {
			return &n, true, ""
		}
	}
	kept := peekFilesCount(sess, accountID, col.Path)
	switch {
	case !kept.ok:
		return nil, false, ""
	case kept.note != "":
		return nil, false, text(kept.note)
	}
	n := kept.n
	return &n, false, ""
}
