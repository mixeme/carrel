package card

import (
	"context"
	"errors"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
)

var (
	// ErrNoReporter: the connection cannot send a CardDAV REPORT. The source
	// is an error on the screen (S-21), never a silent PROPFIND of everything.
	ErrNoReporter = errors.New("card: the connection offers no CardDAV REPORT")
	// ErrNotFound: no contact with this UID in the address book.
	ErrNotFound = errors.New("card: no contact with this UID")
)

type cardReporting interface {
	CardReporter() dav.CardReporter
}

// Query reads every contact of an address book with a single REPORT. The
// cards come back in that reply and are not fetched again (S-11).
func (p *Provider) Query(ctx context.Context, collection string) ([]*model.Object, error) {
	return p.query(ctx, collection, &carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{AllProp: true},
	})
}

// searchProps are the card properties a search looks in: what the list and
// the card show. N is not among them: Radicale before 3.7.7 answers 500 to a
// text-match on a structured property, and FN, which vCard 3.0 and 4.0
// require, carries the same name.
var searchProps = []string{
	vcard.FieldFormattedName, vcard.FieldNickname,
	vcard.FieldEmail, vcard.FieldTelephone, vcard.FieldOrganization,
}

// Search reads the contacts that hold text in a shown property, with one
// REPORT: a text-match per property, any of them (RFC 6352 §10.5). The match
// is the server's; CardDAV's default collation is i;unicode-casemap (RFC 6352
// §8.3), so letter case does not count in any script (S-15.2).
func (p *Provider) Search(ctx context.Context, collection, text string) ([]*model.Object, error) {
	q := &carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{AllProp: true},
		FilterTest:  carddav.FilterAnyOf,
	}
	for _, name := range searchProps {
		q.PropFilters = append(q.PropFilters, carddav.PropFilter{
			Name:        name,
			TextMatches: []carddav.TextMatch{{Text: text}},
		})
	}
	return p.query(ctx, collection, q)
}

// FindUID reads the contact with this UID; the exact UID is checked here as
// well, whatever the server made of text-match.
func (p *Provider) FindUID(ctx context.Context, collection, uid string) (*model.Object, error) {
	objs, err := p.query(ctx, collection, &carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{AllProp: true},
		PropFilters: []carddav.PropFilter{{
			Name:        vcard.FieldUID,
			TextMatches: []carddav.TextMatch{{Text: uid, MatchType: carddav.MatchEquals}},
		}},
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

func (p *Provider) query(ctx context.Context, collection string, q *carddav.AddressBookQuery) ([]*model.Object, error) {
	rs, ok := p.t.(cardReporting)
	if !ok {
		return nil, ErrNoReporter
	}
	aos, err := rs.CardReporter().QueryAddressBook(ctx, collection, q)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Object, 0, len(aos))
	for _, ao := range aos {
		obj := model.NewObject(ao.Path, strings.Trim(ao.ETag, `"`), cardToProps(ao.Card))
		obj.Modified = ao.ModTime
		out = append(out, obj)
	}
	return out, nil
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
