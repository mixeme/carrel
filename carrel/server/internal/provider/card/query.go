package card

import (
	"context"
	"errors"
	"strings"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
)

var (
	// ErrNoReporter: the connection cannot send a CardDAV REPORT. The source
	// is an error on the screen (S-21), never a silent PROPFIND of everything.
	ErrNoReporter = errors.New("card: the connection offers no CardDAV REPORT")
	// ErrNotFound: no contact with this UID in the address book.
	ErrNotFound = errors.New("card: no contact with this UID")
	// ErrServerDoesNotQuery: the server has no addressbook-query at all, so
	// the book was listed and fetched instead and the search happened here
	// (docs/servers.md, SV-23). The records are good; the note travels with
	// them so the screen can say who did the searching.
	ErrServerDoesNotQuery = errors.New("card: the server has no addressbook-query, so Carrel searched its answer")
)

type cardReporting interface {
	CardReporter() dav.CardReporter
}

// Query reads every contact of an address book with a single REPORT. The
// cards come back in that reply and are not fetched again (S-11).
func (p *Provider) Query(ctx context.Context, collection string) ([]*model.Object, error) {
	objs, _, err := p.query(ctx, collection, dav.CardQuery{})
	return objs, err
}

// searchProps are the card properties a search looks in: what the list and
// the card show. N is not among them: Radicale before 3.7.7 answers 500 to a
// text-match on a structured property, and FN, which vCard 3.0 and 4.0
// require, carries the same name (docs/servers.md, SV-3).
var searchProps = []string{
	vcard.FieldFormattedName, vcard.FieldNickname,
	vcard.FieldEmail, vcard.FieldTelephone, vcard.FieldOrganization,
}

// Search reads the contacts that hold text in a shown property, with one
// REPORT: a text-match per property, any of them (RFC 6352 §10.5). The match
// is the server's; CardDAV's default collation is i;unicode-casemap (RFC 6352
// §8.3), so letter case does not count in any script (S-15.2).
func (p *Provider) Search(ctx context.Context, collection, text string) ([]*model.Object, error) {
	q := dav.CardQuery{AnyOf: true}
	for _, name := range searchProps {
		q.Matches = append(q.Matches, dav.CardMatch{Prop: name, Text: text})
	}
	objs, fetched, err := p.query(ctx, collection, q)
	if err != nil || !fetched {
		return objs, err
	}
	// Read in the round-about way: no filter travelled with it, so the
	// searching is ours and the note says so beside the results (S-15).
	return siftCards(objs, text), ErrServerDoesNotQuery
}

// siftCards keeps the cards that hold text in a property a search looks in.
// The comparison is the one asked of the server: letter case does not count,
// in any script.
func siftCards(objs []*model.Object, text string) []*model.Object {
	needle := strings.ToLower(text)
	kept := make([]*model.Object, 0, len(objs))
	for _, o := range objs {
		for _, name := range searchProps {
			if hit := o.PropText(name); len(hit) > 0 && strings.Contains(strings.ToLower(strings.Join(hit, " ")), needle) {
				kept = append(kept, o)
				break
			}
		}
	}
	return kept
}

// FindUID reads the contact with this UID; the exact UID is checked here as
// well, whatever the server made of text-match.
func (p *Provider) FindUID(ctx context.Context, collection, uid string) (*model.Object, error) {
	objs, _, err := p.query(ctx, collection, dav.CardQuery{
		Matches: []dav.CardMatch{{Prop: vcard.FieldUID, Text: uid, Equals: true}},
	})
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if UID(o) == uid {
			return o, nil
		}
	}
	return nil, ErrNotFound
}

// query asks for the cards in vCard 4.0 (S-7). A server that converts cards
// (sabre/dav: Baikal, Nextcloud) answers a REPORT without a version in 3.0 and
// drops what 3.0 cannot hold before any patch, and the edit writes 3.0 back. A
// server that keeps cards as sent answers as kept either way. A server that
// refuses 4.0 is asked again without a version; a 3.0 card read as 4.0 is
// written back as 4.0, a change of format and not of data (docs/servers.md,
// SV-1).
// The bool says the book was read the round-about way: listed and fetched,
// with no filter on the wire. Whoever asked for a search must then do the
// searching; whoever asked for everything loses nothing.
func (p *Provider) query(ctx context.Context, collection string, q dav.CardQuery) ([]*model.Object, bool, error) {
	rs, ok := p.t.(cardReporting)
	if !ok {
		return nil, false, ErrNoReporter
	}
	cq, ok := rs.CardReporter().(dav.CardQuerier)
	if !ok {
		return nil, false, ErrNoReporter
	}
	q.Version = dav.VCard4
	aos, err := cq.QueryCards(ctx, collection, q)
	if refusedVersion(err) {
		q.Version = ""
		aos, err = cq.QueryCards(ctx, collection, q)
	}
	fetched := false
	if err != nil {
		// Refused twice, and the second one carried no version: the report
		// itself is what the server does not have, not the format we asked
		// the cards in (docs/servers.md, SV-23). Yandex answers 400, and 403
		// with the supported-report precondition when a filter travels.
		f, can := rs.CardReporter().(dav.CardFetcher)
		if !can || !refusedVersion(err) {
			return nil, false, err
		}
		if aos, err = f.FetchCards(ctx, collection); err != nil {
			return nil, false, err
		}
		fetched = true
	}
	if len(aos) == 0 && !fetched {
		if listed, hollow := confirmEmpty(ctx, rs, collection, len(q.Matches) > 0); hollow {
			aos, fetched = listed, true
		}
	}
	out := make([]*model.Object, 0, len(aos))
	for _, ao := range aos {
		var lines []string
		if blocks := cardBlocks([]byte(ao.Data)); len(blocks) > 0 {
			lines = blocks[0]
		}
		obj := model.NewObject(ao.Path, strings.Trim(ao.ETag, `"`), cardToProps(ao.Card, lines))
		obj.Modified = ao.ModTime
		out = append(out, obj)
	}
	return out, fetched, nil
}

// refusedVersion reports a refusal of the address-data version: RFC 6352 §8.6
// names the precondition supported-address-data, and go-webdav's server
// answers it with 409.
func refusedVersion(err error) bool {
	for _, code := range []int{400, 403, 409, 412, 415, 422, 501} {
		if dav.IsStatus(err, code) {
			return true
		}
	}
	return false
}

// UID returns the UID of a contact.
func UID(obj *model.Object) string {
	if obj == nil {
		return ""
	}
	if v := obj.PropText(vcard.FieldUID); len(v) > 0 {
		return v[0]
	}
	return ""
}

// NewObject is the skeleton of a new contact: VERSION and UID. The rest
// arrives through a patch, like an edit.
func NewObject(uid string) *model.Object {
	return model.NewObject("", "", map[string][]model.RawValue{
		vcard.FieldVersion: {{Text: "4.0"}},
		vcard.FieldUID:     {{Text: uid}},
	})
}

// Delete removes a contact with If-Match (S-8).
func (p *Provider) Delete(ctx context.Context, path, etag string) error {
	if etag == "" {
		return ErrEmptyETag
	}
	return p.t.Delete(ctx, path, `"`+strings.Trim(etag, `"`)+`"`)
}

// confirmEmpty asks whether an empty answer was the truth. A hollow
// addressbook-query answers nothing about a book that holds cards: Google
// answers 207 with no responses, filter or none, while the same book lists
// its cards and hands them over to a multiget (docs/servers.md, SV-26).
// Telling the two apart costs no card bodies — an unfiltered query for
// versions alone, and the book's own listing when that comes back empty too.
// Proven hollow, the book is read the round-about way, and whoever asked for a
// search does the searching.
func confirmEmpty(ctx context.Context, rs cardReporting, collection string, filtered bool) ([]dav.CardObject, bool) {
	f, can := rs.CardReporter().(dav.CardFetcher)
	if !can {
		return nil, false
	}
	if filtered {
		// An unfiltered answer of its own means the query works: nothing
		// matched, and that is the answer.
		if etags, err := f.QueryEtags(ctx, collection); err != nil || len(etags) > 0 {
			return nil, false
		}
	}
	paths, err := f.ListCards(ctx, collection)
	if err != nil || len(paths) == 0 {
		return nil, false
	}
	aos, err := f.MultiGetCards(ctx, collection, paths)
	if err != nil {
		return nil, false
	}
	return aos, len(aos) > 0
}
