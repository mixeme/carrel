package web

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
)

// S-4.2: a collection read whole is kept, and read again only when it changed.
//
// Before a read the collection is asked for its change tag — one depth-0
// PROPFIND for getctag or sync-token, the property discovery already reads.
// The same tag as the read this session kept means nothing changed: the
// records are rebuilt from the bodies kept by path and version, and no REPORT
// goes out. A different tag, no kept read, or a body the cache let go means
// the read goes to the server as before, and what it answers is kept.
//
// A server without a tag gets the soft limit instead: a kept read younger than
// CARREL_CACHE_COLLECTION_TTL is used without asking. A write Carrel makes
// drops the kept reads of the collection it wrote to (noteWrite), so this
// session never shows itself a list older than its own writes, tag or no tag.
//
// A search of a collection whose whole read is kept and still stands is
// sifted from it, with the properties and the comparison the server is asked
// for (card.Sift, cal.Sift): no REPORT. Without one the search goes to the
// server, and a calendar search that finds nothing is checked against a whole
// read (SV-47) that is then kept — so a second search of the same unchanged
// calendar asks nothing but the tag.
//
// What is not kept: a read by window (an agenda's week) and a filtered search.
// Both still go to the server. A read that came back with records it could not
// parse is not kept either: the list names them, and a kept read would hide
// them.
//
// The cache never stands in for the server. When the tag cannot be read, the
// read goes to the server; when the server does not answer, the refusal is
// shown, not what was kept (S-4.2, «Чего кэш не делает»).
//
// Until 24.09.2026 none of this existed. The tag was collected and never
// compared, and every list, agenda and search read its collections whole,
// every time. The owner pointed out that a search which finds nothing and
// reads the collection to be sure (SV-47) pays for that read only once if the
// read is kept, and S-4.2 already asked for exactly this.

var (
	propGetCTagWhole   = xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
	propSyncTokenWhole = xml.Name{Space: "DAV:", Local: "sync-token"}
)

// wholeRead is a kept read: the tag it was made under, when, and the records
// it found by path and version. The bodies are kept apart, by version.
type wholeRead struct {
	Tag   string      `json:"tag"`
	At    time.Time   `json:"at"`
	Items []wholeItem `json:"items"`
}

type wholeItem struct {
	Path string `json:"path"`
	ETag string `json:"etag"`
	// Modified is what the server said of the record's last change in the
	// answer to the read, not in the body: without it a list rebuilt from
	// the cache would lose the source's «changed» line (S-22).
	Modified time.Time `json:"modified,omitzero"`
}

// wholeKeyPrefix is the start of every kept read of one collection; with the
// component it is the key of one (the empty component is an address book's).
func wholeKeyPrefix(collection string) string { return wholeKeyRoot + collection + "\x00" }

// wholeKeyRoot starts the key of every kept read of every collection.
const wholeKeyRoot = "whole\x00"

// wholeCodec turns a record into the bytes kept for it and back.
type wholeCodec struct {
	encode func(*model.Object) ([]byte, error)
	decode func(path, etag string, body []byte) (*model.Object, error)
}

// cachedWholeRead reads a collection's component whole through the session
// cache. read is the read that goes to the server; kept reports whether its
// answer may be kept (false for one with unreadable records).
func cachedWholeRead(ctx context.Context, sess *session.Session, t dav.Transport, connID, collection, component string,
	ttl time.Duration, now time.Time, codec wholeCodec,
	read func() (objs []*model.Object, keep bool, err error)) ([]*model.Object, error) {
	if sess == nil || t == nil {
		objs, _, err := read()
		return objs, err
	}
	cache := sess.Cache()
	key := wholeKeyPrefix(collection) + component
	tag, tagErr := changeTag(ctx, t, collection)
	if tagErr == nil {
		if objs, ok := keptFor(cache, connID, key, tag, ttl, now, codec); ok {
			return objs, nil
		}
	}
	objs, keep, err := read()
	if err != nil || !keep || tagErr != nil {
		return objs, err
	}
	w := wholeRead{Tag: tag, At: now, Items: make([]wholeItem, 0, len(objs))}
	for _, o := range objs {
		body, err := codec.encode(o)
		if err != nil {
			return objs, nil // not kept; the answer stands
		}
		cache.PutBody(connID, o.Path, o.ETag, body)
		w.Items = append(w.Items, wholeItem{Path: o.Path, ETag: o.ETag, Modified: o.Modified})
	}
	if data, err := json.Marshal(w); err == nil {
		cache.PutETagMap(connID, key, data)
	}
	return objs, nil
}

// keptWholeRead is the kept read of a collection's component when it still
// stands, and false otherwise — with nothing read from the server but the
// tag, and not even that when nothing is kept. A search sifts it here
// instead of asking (S-4.2); without it, the search goes to the server.
func keptWholeRead(ctx context.Context, sess *session.Session, t dav.Transport, connID, collection, component string,
	ttl time.Duration, now time.Time, codec wholeCodec) ([]*model.Object, bool) {
	if sess == nil || t == nil {
		return nil, false
	}
	cache := sess.Cache()
	key := wholeKeyPrefix(collection) + component
	if _, ok := cache.PeekETagMap(connID, key); !ok {
		return nil, false
	}
	tag, err := changeTag(ctx, t, collection)
	if err != nil {
		return nil, false
	}
	return keptFor(cache, connID, key, tag, ttl, now, codec)
}

// keptFor rebuilds the read kept under key if it stands under tag.
func keptFor(cache *session.Cache, connID, key, tag string, ttl time.Duration, now time.Time, codec wholeCodec) ([]*model.Object, bool) {
	data, ok := cache.PeekETagMap(connID, key)
	if !ok {
		return nil, false
	}
	var kept wholeRead
	if json.Unmarshal(data, &kept) != nil || !fresh(kept, tag, ttl, now) {
		return nil, false
	}
	return rebuild(cache, connID, kept, codec)
}

// fresh reports whether a kept read still stands: the same tag, or — where
// the server gives none — younger than the soft limit.
func fresh(kept wholeRead, tag string, ttl time.Duration, now time.Time) bool {
	if tag != "" {
		return kept.Tag == tag
	}
	return kept.Tag == "" && ttl > 0 && now.Sub(kept.At) < ttl
}

// rebuild makes the records of a kept read from the kept bodies, or reports
// that one of them is gone: the cache lets bodies go before maps, and a read
// with a hole in it is read again whole.
func rebuild(cache *session.Cache, connID string, kept wholeRead, codec wholeCodec) ([]*model.Object, bool) {
	objs := make([]*model.Object, 0, len(kept.Items))
	for _, it := range kept.Items {
		body, ok := cache.GetBody(connID, it.Path, it.ETag)
		if !ok {
			return nil, false
		}
		obj, err := codec.decode(it.Path, it.ETag, body)
		if err != nil {
			return nil, false
		}
		obj.Modified = it.Modified
		objs = append(objs, obj)
	}
	return objs, true
}

// changeTag is the collection's change tag, or "" when the server has none.
// An error means the tag could not be asked at all: the read then goes to the
// server and nothing is kept.
func changeTag(ctx context.Context, t dav.Transport, collection string) (string, error) {
	ms, err := t.PropFind(ctx, collection, dav.DepthZero, []xml.Name{propGetCTagWhole, propSyncTokenWhole})
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		if tag := strings.TrimSpace(string(r.Props[propGetCTagWhole])); tag != "" {
			return tag, nil
		}
		if tag := strings.TrimSpace(string(r.Props[propSyncTokenWhole])); tag != "" {
			return tag, nil
		}
	}
	return "", nil
}

// dropWholeReads forgets every kept read of a collection: Carrel wrote to it.
func dropWholeReads(sess *session.Session, connID, collection string) {
	if sess == nil {
		return
	}
	sess.Cache().InvalidateMapsUnder(connID, wholeKeyPrefix(collection))
}

// dropAllWholeReads forgets every kept read of the session: a person asked
// again — Refresh on a combined view, a search or a timeline run again — and
// that means what the servers hold now, even within the soft limit of a
// server without a change tag.
func dropAllWholeReads(sess *session.Session, blob sealed.Blob) {
	if sess == nil {
		return
	}
	for _, conn := range blob.Connections {
		sess.Cache().InvalidateMapsUnder(conn.ID, wholeKeyRoot)
	}
}

// calCodec and cardCodec keep a record as the text it was read in: a calendar
// object's whole document, a card as it encodes — the same round trip the
// conflict flow already keeps bodies by (rememberBody).
var (
	calCodec = wholeCodec{
		encode: func(o *model.Object) ([]byte, error) { return o.CalendarDoc(), nil },
		decode: func(path, etag string, body []byte) (*model.Object, error) {
			return cal.Parse(path, etag, bytes.NewReader(body))
		},
	}
	cardCodec = wholeCodec{
		encode: func(o *model.Object) ([]byte, error) {
			var buf bytes.Buffer
			err := card.Encode(&buf, o)
			return buf.Bytes(), err
		},
		decode: func(path, etag string, body []byte) (*model.Object, error) {
			return card.Parse(path, etag, bytes.NewReader(body))
		},
	}
)
