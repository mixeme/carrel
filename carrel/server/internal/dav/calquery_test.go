package dav

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
)

func eventText(uid, extra string) string {
	return strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:x", "BEGIN:VEVENT", "UID:" + uid,
		"DTSTAMP:20260101T000000Z", "DTSTART:20260810T100000Z", extra, "END:VEVENT", "END:VCALENDAR", ""}, "\r\n")
}

// One record go-ical cannot read — here a parameter with no colon after it,
// on which go-ical panics — is that record's trouble: the others of the answer
// arrive, the broken one comes back without data, and nothing panics. Through
// go-webdav the whole calendar failed, or the process went down with it.
func TestCalendarQueryKeepsTheReadableRecords(t *testing.T) {
	reply := `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
<d:response><d:href>/cal/a.ics</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><c:calendar-data><![CDATA[` +
		eventText("a", "SUMMARY:fine") + `]]></c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>/cal/b.ics</d:href><d:propstat><d:prop><d:getetag>"v2"</d:getetag><c:calendar-data><![CDATA[` +
		eventText("b", "X-BROKEN;A=b") + `]]></c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>/cal/c.ics</d:href><d:propstat><d:prop><d:getetag>"v3"</d:getetag><c:calendar-data><![CDATA[` +
		eventText("c", `X-BROKEN;A="x"y:v`) + `]]></c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)

	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	objs, err := testClient(t, srv.URL).CalReporter().QueryCalendar(t.Context(), "/cal/", &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{
			Name: "VEVENT", Start: from, End: from.AddDate(0, 0, 7),
			Props: []caldav.PropFilter{{Name: "SUMMARY", TextMatch: &caldav.TextMatch{Text: "fi"}}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 3 || objs[0].Data == nil || objs[1].Data != nil || objs[2].Data != nil {
		t.Fatalf("objects = %+v", objs)
	}
	paths := xmlPaths(t, body)
	for key, text := range map[string]string{
		"calendar-query/prop/calendar-data": "",
		"calendar-query/filter/comp-filter{name=VCALENDAR}/comp-filter{name=VEVENT}/time-range{start=20260810T000000Z}{end=20260817T000000Z}": "",
		"calendar-query/filter/comp-filter{name=VCALENDAR}/comp-filter{name=VEVENT}/prop-filter{name=SUMMARY}/text-match":                     "fi",
	} {
		if got, ok := paths[key]; !ok || got != text {
			t.Errorf("%s: %q, %v; body %s", key, got, ok, body)
		}
	}
}
