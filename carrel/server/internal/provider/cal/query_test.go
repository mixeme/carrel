package cal

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
)

const work = davtest.CalendarHome + "work/"

func davProvider(t *testing.T) (*Provider, *davtest.Server) {
	t.Helper()
	srv := davtest.New(t)
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	g := dav.NewGuard(&conf.Config{
		DAVConnectTimeout: 5 * time.Second, DAVRequestTimeout: 10 * time.Second,
		DAVMaxResponseBytes: 1 << 20, DAVMaxRedirects: 5, DAVSSRFAllowlist: "127.0.0.1",
	})
	client, err := dav.NewClient(g, srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	return p, srv
}

func vevent(uid, start, summary string) string {
	return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:" + uid +
		"\nDTSTAMP:20260101T000000Z\nDTSTART:" + start + "\nSUMMARY:" + summary + "\nEND:VEVENT\nEND:VCALENDAR\n"
}

// S-11: the list is one REPORT, the window is the server's filter, and the
// bodies the REPORT returned are not fetched again with GET.
func TestQueryReadsBodiesFromOneReport(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(work+"in.ics", vevent("in", "20260115T100000Z", "Inside"))
	srv.Seed(work+"out.ics", vevent("out", "20260301T100000Z", "Outside"))
	srv.Seed(work+"todo.ics", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nSUMMARY:Task\nEND:VTODO\nEND:VCALENDAR\n")

	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	objs, bad, err := p.Query(t.Context(), work, "VEVENT", &from, &to)
	if err != nil || len(bad) != 0 {
		t.Fatalf("err = %v, unreadable = %v", err, bad)
	}
	if len(objs) != 1 || Text(objs[0], "SUMMARY") != "Inside" || objs[0].ETag == "" || objs[0].Path != work+"in.ics" {
		t.Fatalf("objs = %+v", objs)
	}
	if n := srv.CountRequests("REPORT", work); n != 1 {
		t.Errorf("REPORT requests = %d, want 1", n)
	}
	if n := srv.CountRequests("GET", work); n != 0 {
		t.Errorf("GET requests = %d: the REPORT body was fetched again", n)
	}
}

type stubReports struct {
	noReports
	objs []caldav.CalendarObject
}

func (s stubReports) CalReporter() dav.CalReporter { return s }

func (s stubReports) QueryCalendar(context.Context, string, *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	return s.objs, nil
}

func (s stubReports) MultiGetCalendar(context.Context, string, *caldav.CalendarMultiGet) ([]caldav.CalendarObject, error) {
	return nil, nil
}

func (s stubReports) SyncCollection(context.Context, string, *caldav.SyncQuery) (*caldav.SyncResponse, error) {
	return nil, nil
}

func decodedICS(t *testing.T, ics string) *ical.Calendar {
	t.Helper()
	c, err := ical.NewDecoder(strings.NewReader(strings.ReplaceAll(ics, "\n", "\r\n"))).Decode()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// One object another client wrote badly does not blank the calendar: the
// rest is listed and the broken one is named. go-webdav's own server refuses
// to serve such an object at all, so the REPORT is stubbed with what a real
// server sends — the stored bytes, decoded by the client.
func TestQueryNamesUnreadableAndKeepsTheRest(t *testing.T) {
	p, err := New(stubReports{objs: []caldav.CalendarObject{
		{Path: work + "good.ics", ETag: `"g"`, Data: decodedICS(t, vevent("good", "20260115T100000Z", "Good"))},
		{Path: work + "bad.ics", ETag: `"b"`, Data: decodedICS(t, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:bad\nDTSTART:20260116T100000Z\nEND:VEVENT\nEND:VCALENDAR\n")},
	}})
	if err != nil {
		t.Fatal(err)
	}

	objs, bad, err := p.Query(t.Context(), work, "VEVENT", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || UID(objs[0]) != "good" || objs[0].ETag != "g" {
		t.Errorf("objs = %+v", objs)
	}
	if len(bad) != 1 || bad[0].Path != work+"bad.ics" || bad[0].UID != "bad" || bad[0].Err == nil {
		t.Errorf("unreadable = %+v", bad)
	}
	if _, err := p.FindUID(t.Context(), work, "VEVENT", "bad"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("FindUID of an unreadable record = %v, want its reason, not not-found", err)
	}
}

// S-15.2: CalDAV prop-filters are all-of, so any-of is one REPORT per
// property, and a record two of them find is returned once.
func TestSearchIsAnyOfShownProperties(t *testing.T) {
	p, srv := davProvider(t)
	event := func(uid, extra string) string {
		return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VEVENT\nUID:" + uid +
			"\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\n" + extra + "\nEND:VEVENT\nEND:VCALENDAR\n"
	}
	srv.Seed(work+"plan.ics", event("plan", "SUMMARY:Launch plan"))
	srv.Seed(work+"pad.ics", event("pad", "SUMMARY:Visit\nLOCATION:Launch pad"))
	srv.Seed(work+"day.ics", event("day", "SUMMARY:Launch\nDESCRIPTION:Launch day"))
	srv.Seed(work+"other.ics", event("other", "SUMMARY:Other\nCOMMENT:Launch"))
	srv.Seed(work+"todo.ics", "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nSUMMARY:Launch\nEND:VTODO\nEND:VCALENDAR\n")

	objs, bad, err := p.Search(t.Context(), work, "VEVENT", "Launch")
	if err != nil || len(bad) != 0 {
		t.Fatalf("err %v, unreadable %v", err, bad)
	}
	var uids []string
	for _, o := range objs {
		uids = append(uids, UID(o))
	}
	seen := map[string]int{}
	for _, u := range uids {
		seen[u]++
	}
	if seen["plan"] != 1 || seen["pad"] != 1 || seen["day"] != 1 || seen["other"] != 0 || seen["t"] != 0 {
		t.Fatalf("found %v", uids)
	}
	if n := srv.CountRequests("REPORT", work); n != 3 {
		t.Errorf("REPORT requests = %d, want one per property", n)
	}
}

func TestFindUIDIsExact(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(work+"a.ics", vevent("abc", "20260115T100000Z", "Short"))
	srv.Seed(work+"b.ics", vevent("abcd", "20260115T100000Z", "Long"))
	obj, err := p.FindUID(t.Context(), work, "VEVENT", "abc")
	if err != nil || Text(obj, "SUMMARY") != "Short" {
		t.Fatalf("obj = %+v, err = %v", obj, err)
	}
	if _, err := p.FindUID(t.Context(), work, "VEVENT", "ab"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindUID(ab) = %v, want ErrNotFound", err)
	}
}

// S-15.2: a search does not count letter case, in any script. The REPORT names
// i;unicode-casemap; CalDAV's default ignores case in Latin only.
func TestSearchIgnoresCaseInEveryScript(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(work+"meet.ics", vevent("meet", "20260115T100000Z", "Встреча с Ковалёвой"))
	srv.Seed(work+"plan.ics", vevent("plan", "20260116T100000Z", "Launch plan"))
	for _, q := range []string{"ковал", "КОВАЛ", "Ковал"} {
		objs, bad, err := p.Search(t.Context(), work, "VEVENT", q)
		if err != nil || len(bad) != 0 || len(objs) != 1 || UID(objs[0]) != "meet" {
			t.Errorf("%s: objs %v, unreadable %v, err %v", q, objs, bad, err)
		}
	}
	if objs, _, err := p.Search(t.Context(), work, "VEVENT", "LAUNCH"); err != nil || len(objs) != 1 || UID(objs[0]) != "plan" {
		t.Errorf("LAUNCH: %v, %v", objs, err)
	}
}

// A server that refuses the collation is asked again with its default. What it
// found comes with ErrLatinCaseOnly, and the other properties are not asked by
// name again.
func TestSearchFallsBackWhenCollationIsRefused(t *testing.T) {
	p, srv := davProvider(t)
	srv.RefuseCollation = true
	srv.Seed(work+"plan.ics", vevent("plan", "20260116T100000Z", "Launch plan"))
	objs, bad, err := p.Search(t.Context(), work, "VEVENT", "Launch")
	if !errors.Is(err, ErrLatinCaseOnly) || len(bad) != 0 || len(objs) != 1 || UID(objs[0]) != "plan" {
		t.Fatalf("objs %v, unreadable %v, err %v", objs, bad, err)
	}
	if n := srv.CountRequests("REPORT", work); n != 4 {
		t.Errorf("REPORT requests = %d, want 4: one refused, then one a property", n)
	}
}

type noReports struct {
	dav.Transport
}

func (noReports) PutCreate(context.Context, string, io.Reader) (string, error) { return "", nil }

func TestQueryWithoutReporterIsAnError(t *testing.T) {
	p, err := New(noReports{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Query(t.Context(), work, "VEVENT", nil, nil); !errors.Is(err, ErrNoReporter) {
		t.Fatalf("err = %v, want ErrNoReporter", err)
	}
}

func TestNewObjectIsASkeleton(t *testing.T) {
	obj, err := NewObject("VTODO", "0123456789abcdef0123456789abcdef", time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if obj.MainComponent() != "VTODO" || UID(obj) != "0123456789abcdef0123456789abcdef" || len(obj.PropText("DTSTAMP")) != 1 {
		t.Fatalf("skeleton = %+v", obj.Props())
	}
	if len(obj.Props()) != 2 {
		t.Errorf("props = %v, want UID and DTSTAMP only", obj.Props())
	}
	if !strings.Contains(string(obj.CalendarDoc()), "PRODID:") {
		t.Error("no PRODID on the calendar")
	}
}

func TestDeleteRequiresVersion(t *testing.T) {
	p, srv := davProvider(t)
	etag := srv.Seed(work+"x.ics", vevent("x", "20260115T100000Z", "X"))
	if err := p.Delete(t.Context(), work+"x.ics", ""); !errors.Is(err, ErrEmptyETag) {
		t.Fatalf("delete without version = %v", err)
	}
	if err := p.Delete(t.Context(), work+"x.ics", "stale"); !dav.IsStatus(err, 412) {
		t.Fatalf("stale version = %v, want 412", err)
	}
	if err := p.Delete(t.Context(), work+"x.ics", etag); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := srv.Object(work + "x.ics"); ok {
		t.Error("still there")
	}
}

// docs/servers.md, SV-47: a server whose text-match finds nothing — Xandikos
// matches only a whole value, and only one without a space — and says 207
// with nothing, the same as an honest miss. An empty answer is checked
// against the collection itself; what the sifting finds comes with the note
// that Carrel did the searching.
func TestAnEmptySearchIsCheckedAgainstTheCollection(t *testing.T) {
	p, srv := davProvider(t)
	srv.CalendarTextMatchFindsNothing = true
	srv.Seed(work+"plan.ics", vevent("plan", "20260115T100000Z", "Launch plan"))
	srv.Seed(work+"visit.ics", vevent("visit", "20260116T100000Z", "Visit"))

	objs, _, err := p.Search(t.Context(), work, "VEVENT", "launch")
	if !errors.Is(err, ErrServerDoesNotFilter) {
		t.Fatalf("err = %v, want the note that Carrel did the searching", err)
	}
	if len(objs) != 1 || UID(objs[0]) != "plan" {
		t.Fatalf("found %+v, want the one event whose summary holds the text", objs)
	}
}

// And on a server that searched well, a miss is a miss: one read more to be
// sure of it, and no note on the screen — the server was right.
func TestAnHonestMissSaysNothingMore(t *testing.T) {
	p, srv := davProvider(t)
	srv.Seed(work+"visit.ics", vevent("visit", "20260116T100000Z", "Visit"))

	objs, _, err := p.Search(t.Context(), work, "VEVENT", "launch")
	if err != nil || len(objs) != 0 {
		t.Fatalf("objs %+v, err %v; want nothing and no note", objs, err)
	}
}
