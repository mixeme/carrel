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
	// ErrServerMissed: the server's search found nothing, and the book read
	// whole and sifted here found cards that hold the text (docs/servers.md,
	// SV-51). The records are good; the note says who found them.
	ErrServerMissed = errors.New("card: the server's search found nothing, and Carrel's own sifting of the book did")
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
	var partial *PartialRead
	if errors.As(err, &partial) {
		// A fallback read has no server-side filter, even when incomplete.
		if fetched {
			objs = siftCards(objs, text)
		}
		return objs, err
	}
	if err != nil {
		return objs, err
	}
	if !fetched {
		// Nothing is the answer a text-match cannot be told from an honest
		// miss by. DAViCal answers a search for Cyrillic in another case with
		// nothing as soon as EMAIL or TEL stands among the alternatives, even
		// for a card whose FN holds the text (SV-51). So an empty answer is
		// checked against the book itself, as a calendar's is (SV-47); the
		// read is the caller's to keep (ReadWhole), and a server that
		// searched well loses one read and says nothing more.
		if len(objs) == 0 {
			if all, err := p.readWhole(ctx, collection); err == nil {
				if kept := siftCards(all, text); len(kept) > 0 {
					return kept, ErrServerMissed
				}
			} else if errors.As(err, &partial) {
				return siftCards(all, text), err
			}
		}
		return objs, nil
	}
	// Read in the round-about way: no filter travelled with it, so the
	// searching is ours and the note says so beside the results (S-15).
	return siftCards(objs, text), ErrServerDoesNotQuery
}

// Sift keeps the cards a search for text finds: the properties Search asks
// the server about, compared the same way. It is the search of a book already
// read whole (S-4.2).
func Sift(objs []*model.Object, text string) []*model.Object { return siftCards(objs, text) }

// readWhole reads the book whole: through ReadWhole when the caller gave one,
// from the server otherwise.
func (p *Provider) readWhole(ctx context.Context, collection string) ([]*model.Object, error) {
	if p.ReadWhole != nil {
		return p.ReadWhole(ctx, collection)
	}
	return p.Query(ctx, collection)
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

// FindUID reads the contact with this UID. The filter asks for a substring —
// the default match, the one every server has — and the exact UID is checked
// here, whatever the server made of text-match. Asking for match-type=equals
// would be asking for something optional we do not need: Xandikos takes it and
// answers 207 with nothing at all, for a card it hands over to the same filter
// without it (docs/servers.md, SV-40).
func (p *Provider) FindUID(ctx context.Context, collection, uid string) (*model.Object, error) {
	objs, _, err := p.query(ctx, collection, dav.CardQuery{
		Matches: []dav.CardMatch{{Prop: vcard.FieldUID, Text: uid}},
	})
	var partial *PartialRead
	if err != nil && !errors.As(err, &partial) {
		return nil, err
	}
	for _, o := range objs {
		if UID(o) == uid {
			return o, nil
		}
	}
	if err != nil {
		return nil, err
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
		// A book larger than one answer may be (CARREL_DAV_MAX_RESPONSE_BYTES)
		// is read the same round-about way: its multigets come in parts.
		// A filter that breaks the server is the third reason. Xandikos
		// answers 500 to any text-match over a book holding one value with
		// a character outside ASCII — its own UnicodeEncodeError, whatever
		// the text asked for — while the same book without a filter hands
		// every card over (docs/servers.md, SV-42). Left as it came, a
		// search would simply die on any book with one accented name.
		f, can := rs.CardReporter().(dav.CardFetcher)
		if !can || !refusedVersion(err) && !errors.Is(err, dav.ErrResponseTooLarge) &&
			!brokeOnTheFilter(err, len(q.Matches) > 0) {
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
	var bad []Unreadable
	for _, ao := range aos {
		card := ao.Card
		if ao.DecodeError == nil && card != nil && ownRead(ao.Data) {
			// Read again with its bare parameters named and its colons
			// plain (readCard): a list reads a card as the record does.
			card, ao.DecodeError = readCard(ao.Data)
		}
		if ao.DecodeError == nil {
			ao.DecodeError = brokenLine(ao.Data)
		}
		if ao.DecodeError != nil || card == nil {
			err := ao.DecodeError
			if err == nil {
				err = errors.New("the resource contains no vCard")
			}
			bad = append(bad, Unreadable{Path: ao.Path, Err: err})
			continue
		}
		var lines []string
		if blocks := cardBlocks([]byte(ao.Data)); len(blocks) > 0 {
			lines = blocks[0]
		}
		obj := model.NewObject(ao.Path, strings.Trim(ao.ETag, `"`), cardToProps(card, lines))
		obj.Modified = ao.ModTime
		out = append(out, obj)
	}
	if len(bad) != 0 {
		return out, fetched, &PartialRead{Records: bad}
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

// brokeOnTheFilter reports a server that fell over on the filter rather than
// refusing it: 500 to a report that carried one. Without a filter there is
// nothing to have broken on and nothing to fall back from, so the refusal
// goes to whoever asked, as it did before.
func brokeOnTheFilter(err error, filtered bool) bool {
	return filtered && dav.IsStatus(err, 500)
}

// UID returns the UID of a contact.// UID returns the UID of a contact.
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
