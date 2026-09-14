package dav

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func calendarText(uid, summary string) string {
	return strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//T//EN", "BEGIN:VTODO",
		"UID:" + uid, "DTSTAMP:20260101T000000Z", "SUMMARY:" + summary, "END:VTODO", "END:VCALENDAR", ""}, "\r\n")
}

// S-15.2: the search REPORT names its collation inside the filter RFC 4791
// §9.7 nests, and reads what servers send: a body escaped or in CDATA, an href
// percent-encoded or absolute, and a body that does not parse, which does not
// take the rest with it.
func TestSearchCalendarNamesItsCollation(t *testing.T) {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(calendarText("a", "Встреча & Ковалёва")))
	reply := `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
<d:response><d:href>/cal/a%20b.ics</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag><c:calendar-data>` +
		escaped.String() + `</c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>http://example.test/cal/c.ics</d:href><d:propstat><d:prop><d:getetag>"v2"</d:getetag><c:calendar-data><![CDATA[` +
		calendarText("c", "Second") + `]]></c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>/cal/broken.ics</d:href><d:propstat><d:prop><d:getetag>"v3"</d:getetag><c:calendar-data>not a calendar</c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`

	var got []byte
	var depth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		depth = r.Header.Get("Depth")
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv.URL)

	searcher, ok := c.CalReporter().(CalSearcher)
	if !ok {
		t.Fatal("the client's CalReporter is no CalSearcher")
	}
	objs, err := searcher.SearchCalendar(t.Context(), "/cal/", CalSearch{
		Component: "VTODO", Prop: "SUMMARY", Text: "ковал & co", Collation: UnicodeCaseMap,
	})
	if err != nil {
		t.Fatal(err)
	}

	const want = "calendar-query/filter/comp-filter[VCALENDAR]/comp-filter[VTODO]/prop-filter[SUMMARY]/text-match{i;unicode-casemap}"
	var stack []string
	var matched bool
	dec := xml.NewDecoder(bytes.NewReader(got))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("body %s: %v", got, err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			label := el.Name.Local
			for _, a := range el.Attr {
				switch a.Name.Local {
				case "name":
					label += "[" + a.Value + "]"
				case "collation":
					label += "{" + a.Value + "}"
				}
			}
			if el.Name.Local == "text-match" && el.Name.Space != calDAVNS {
				t.Errorf("text-match in namespace %q", el.Name.Space)
			}
			stack = append(stack, label)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.Join(stack, "/") == want && string(el) == "ковал & co" {
				matched = true
			}
		}
	}
	if !matched || depth != "1" || !bytes.Contains(got, []byte("calendar-data")) || !bytes.Contains(got, []byte("getetag")) {
		t.Errorf("Depth %q, body %s", depth, got)
	}

	if len(objs) != 3 {
		t.Fatalf("objects = %+v", objs)
	}
	if objs[0].Path != "/cal/a b.ics" || objs[0].ETag != `"v1"` || objs[0].Data == nil ||
		objs[0].Data.Children[0].Props.Get("SUMMARY").Value != "Встреча & Ковалёва" {
		t.Errorf("escaped body: %+v", objs[0])
	}
	if objs[1].Path != "/cal/c.ics" || objs[1].Data == nil {
		t.Errorf("CDATA body with an absolute href: %+v", objs[1])
	}
	if objs[2].Path != "/cal/broken.ics" || objs[2].Data != nil {
		t.Errorf("unparsable body: %+v", objs[2])
	}
}
