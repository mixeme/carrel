package cal

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
)

var (
	// ErrNoReporter: the connection cannot send a CalDAV REPORT. The source is
	// an error on the screen (S-21), never a silent PROPFIND of everything.
	ErrNoReporter = errors.New("cal: the connection offers no CalDAV REPORT")
	// ErrNotFound: no record with this UID in the collection.
	ErrNotFound = errors.New("cal: no record with this UID")
)

// Unreadable is a resource the REPORT returned that could not become a record.
// A collection with one broken object still lists the rest and names the one
// it skipped; it does not go blank.
type Unreadable struct {
	Path string
	UID  string
	Err  error
}

type calReporting interface {
	CalReporter() dav.CalReporter
}

// Query reads the records of one component with a single REPORT. The bodies
// come back in that reply and are not fetched again (S-11). A nil window reads
// the whole collection; the agenda always passes one.
func (p *Provider) Query(ctx context.Context, collection, component string, from, to *time.Time) ([]*model.Object, []Unreadable, error) {
	filter := caldav.CompFilter{Name: component}
	if from != nil {
		filter.Start = from.UTC()
	}
	if to != nil {
		filter.End = to.UTC()
	}
	return p.query(ctx, collection, component, filter)
}

// FindUID reads the record with this UID. CalDAV text-match is a substring
// test, so the exact UID is checked here.
func (p *Provider) FindUID(ctx context.Context, collection, component, uid string) (*model.Object, error) {
	filter := caldav.CompFilter{
		Name:  component,
		Props: []caldav.PropFilter{{Name: ical.PropUID, TextMatch: &caldav.TextMatch{Text: uid}}},
	}
	objs, bad, err := p.query(ctx, collection, component, filter)
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if UID(o) == uid {
			return o, nil
		}
	}
	for _, u := range bad {
		if u.UID == uid {
			return nil, u.Err
		}
	}
	return nil, ErrNotFound
}

func (p *Provider) query(ctx context.Context, collection, component string, filter caldav.CompFilter) ([]*model.Object, []Unreadable, error) {
	rs, ok := p.t.(calReporting)
	if !ok {
		return nil, nil, ErrNoReporter
	}
	q := &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{Name: ical.CompCalendar, Comps: []caldav.CompFilter{filter}},
	}
	cos, err := rs.CalReporter().QueryCalendar(ctx, collection, q)
	if err != nil {
		return nil, nil, err
	}
	var objs []*model.Object
	var bad []Unreadable
	for _, co := range cos {
		obj, err := fromReport(co)
		if err != nil {
			bad = append(bad, Unreadable{Path: co.Path, UID: reportUID(co), Err: err})
			continue
		}
		if obj.MainComponent() != component {
			continue
		}
		objs = append(objs, obj)
	}
	return objs, bad, nil
}

// fromReport turns a REPORT entry into a record. go-webdav hands the body over
// decoded, so it is encoded once to become the document the record keeps; the
// encoder refuses what RFC 5545 forbids (a VEVENT without DTSTAMP), and that
// refusal is what makes the entry unreadable.
func fromReport(co caldav.CalendarObject) (*model.Object, error) {
	if co.Data == nil {
		return nil, errors.New("the reply carried no calendar data")
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(co.Data); err != nil {
		return nil, err
	}
	return Parse(co.Path, strings.Trim(co.ETag, `"`), &buf)
}

func reportUID(co caldav.CalendarObject) string {
	if co.Data == nil {
		return ""
	}
	for _, child := range co.Data.Children {
		if p := child.Props.Get(ical.PropUID); p != nil {
			return p.Value
		}
	}
	return ""
}

// NewObject builds the skeleton of a new record: VCALENDAR with VERSION and
// PRODID, one main component with UID and DTSTAMP. Everything else arrives
// through a patch, the same way an edit does — a form never builds a record.
func NewObject(component, uid string, now time.Time) (*model.Object, error) {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Carrel//Carrel//EN")
	main := ical.NewComponent(component)
	main.Props.SetText(ical.PropUID, uid)
	main.Props.SetDateTime(ical.PropDateTimeStamp, now.UTC())
	cal.Children = append(cal.Children, main)
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	return Parse("", "", &buf)
}

// Delete removes a record with If-Match (S-8). A delete without a version is
// refused, like an update without one.
func (p *Provider) Delete(ctx context.Context, path, etag string) error {
	if etag == "" {
		return ErrEmptyETag
	}
	return p.t.Delete(ctx, path, `"`+strings.Trim(etag, `"`)+`"`)
}
