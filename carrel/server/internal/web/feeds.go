package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/view"
)

// S-25.2: a subscription reads its feed — one file at an address — on every
// visit, through the guard, and keeps nothing (C-2). To the lists, the cards,
// the combined views and search it is a read-only collection; what would
// write is not there at all.

// feedReader reads one feed.
type feedReader struct {
	client          *http.Client
	url             string
	login, password string
	max             int64
}

func (s *Server) newFeedReader(url, login, password string) *feedReader {
	return &feedReader{client: s.guard.HTTPClient(), url: url, login: login, password: password, max: s.cfg.DAVMaxResponseBytes}
}

// read fetches and parses the feed.
func (f *feedReader) read(ctx context.Context) (publish.Parsed, publish.Feed, error) {
	got, err := publish.Fetch(ctx, f.client, f.url, f.login, f.password, f.max)
	if err != nil {
		return publish.Parsed{}, got, err
	}
	p, err := publish.Parse(got.Doc, time.Now())
	return p, got, err
}

// records are the feed's records of one kind.
func (f *feedReader) records(ctx context.Context, k view.Kind) ([]*model.Object, error) {
	p, _, err := f.read(ctx)
	if err != nil {
		return nil, err
	}
	return p.Records[k], nil
}

// search is the feed's records of one kind that hold text: a feed has no
// server to search, so Carrel sifts what it read.
func (f *feedReader) search(ctx context.Context, k view.Kind, text string) ([]*model.Object, error) {
	objs, err := f.records(ctx, k)
	if err != nil {
		return nil, err
	}
	needle := dupes.Fold(text)
	var out []*model.Object
	for _, o := range objs {
		for _, vals := range o.Props() {
			hit := false
			for _, v := range vals {
				if strings.Contains(dupes.Fold(v.Text), needle) {
					hit = true
					break
				}
			}
			if hit {
				out = append(out, o)
				break
			}
		}
	}
	return out, nil
}

// find is one record of the feed by UID, or the providers' not-found.
func (f *feedReader) find(ctx context.Context, k view.Kind, uid string) (*model.Object, error) {
	objs, err := f.records(ctx, k)
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if (k == view.KindContact && card.UID(o) == uid) || (k != view.KindContact && cal.UID(o) == uid) {
			return o, nil
		}
	}
	if k == view.KindContact {
		return nil, card.ErrNotFound
	}
	return nil, cal.ErrNotFound
}

var componentKinds = map[string]view.Kind{"VEVENT": view.KindEvent, "VTODO": view.KindTask, "VJOURNAL": view.KindNote}

// contactsAll is every contact of the collection or the feed.
func (c *collectionCtx) contactsAll(ctx context.Context) ([]*model.Object, error) {
	if c.feed != nil {
		return c.feed.records(ctx, view.KindContact)
	}
	return cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, "", c.ttl, c.now(), cardCodec,
		func() ([]*model.Object, bool, error) {
			got, err := c.card.Query(ctx, c.col.Path)
			return got, true, err
		})
}

// calQuery is the records of one component of the collection or the feed.
// A feed is read whole: the window is the list's to apply.
func (c *collectionCtx) calQuery(ctx context.Context, component string, from, to *time.Time) ([]*model.Object, []cal.Unreadable, error) {
	if c.feed != nil {
		objs, err := c.feed.records(ctx, componentKinds[component])
		return objs, nil, err
	}
	if from == nil && to == nil {
		// A whole read, kept by the session (S-4.2, wholeread.go). What the
		// server could not parse is named by the list, so such a read is
		// answered as it came and not kept.
		var unreadable []cal.Unreadable
		objs, err := cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, component, c.ttl, c.now(), calCodec,
			func() ([]*model.Object, bool, error) {
				got, bad, err := c.cal.Query(ctx, c.col.Path, component, nil, nil)
				unreadable = bad
				if err == nil && component == "VEVENT" {
					putCount(c.sess, c.conn.ID, c.col.Path, component, len(got)+len(bad))
				}
				return got, len(bad) == 0, err
			})
		return objs, unreadable, err
	}
	// A read by window — an agenda's week — is kept the same way, under the
	// window: what falls in it is the server's answer, kept as it came, not
	// cut here from a whole read (S-4.2, «раскрытые повторы по диапазону»).
	var unreadable []cal.Unreadable
	objs, err := cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, c.col.Path, windowKey(component, from, to), c.ttl, c.now(), calCodec,
		func() ([]*model.Object, bool, error) {
			got, bad, err := c.cal.Query(ctx, c.col.Path, component, from, to)
			unreadable = bad
			if err == nil && component == "VEVENT" {
				countEvents(ctx, c.sess, c.cal, c.conn.ID, c.col.Path) // the rail's number (C-6.3)
			}
			return got, len(bad) == 0, err
		})
	return objs, unreadable, err
}

// feedRoute reports whether a request may open a feed: to read it — the list,
// a record, the export — or to copy a record out of it. Anything that would
// write is the same 404 as a foreign address.
func feedRoute(r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && (strings.HasSuffix(r.Pattern, "{col}") || strings.HasSuffix(r.Pattern, "{uid}") || strings.HasSuffix(r.Pattern, "/export")):
		return true
	case strings.HasSuffix(r.Pattern, "/move"):
		return r.FormValue("copy") != ""
	}
	return false
}
