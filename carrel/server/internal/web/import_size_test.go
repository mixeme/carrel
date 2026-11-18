package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
)

// A record larger than its collection takes (max-resource-size, RFC 4791)
// is not sent: the preview says it is too large and Import does not count
// it, and the report has it under «Failed» with the sizes. Stalwart takes
// records up to 512 KB and answered a note of 573 KB with 412, which the
// import read as a name taken and tried again under another one
// (acceptance on our test servers, 18.11.2026).
func TestImportLeavesARecordTheCollectionCannotTake(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	small := davtest.CalendarHome + "small/"
	srv.AddCalendar(caldav.Calendar{Path: small, Name: "Small", SupportedComponentSet: []string{"VEVENT"}, MaxResourceSize: 600})
	f := connectFixture(t, srv, small, nil)

	event := func(uid, summary, description string) string {
		return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260810T060000Z\r\nDTSTART:20260810T100000Z\r\nDTEND:20260810T110000Z\r\n" +
			"SUMMARY:" + summary + "\r\nDESCRIPTION:" + description + "\r\nEND:VEVENT\r\n"
	}
	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//T//EN\r\n" +
		event("light", "Лёгкое", "коротко") + event("heavy", "Тяжёлое", strings.Repeat("слово ", 200)) + "END:VCALENDAR\r\n"

	page := decode[view.ImportPage](t, f.h.do(get(f.path("calendar")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "two.ics", []byte(file))))
	row := previewRow(t, page.Preview, "Тяжёлое")
	if !strings.HasPrefix(string(row[3]), "Too large: ") || !strings.HasSuffix(string(row[3]), " takes records up to 600 B") {
		t.Errorf("the heavy one in the preview = %q", row)
	}
	if run := actionByID(page.Actions, "import.run"); run == nil || run.Label != "Import 1 event" {
		t.Errorf("Import = %q, want the light one alone", run.Label)
	}
	puts := srv.CountRequests("PUT", small)
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	r := page.Report
	if r == nil || r.Created != 1 || r.Failed != 1 || len(r.Failures) != 1 || !strings.HasPrefix(string(r.Failures[0]), "Тяжёлое · Too large: ") {
		t.Errorf("report = %+v", r)
	}
	if n := srv.CountRequests("PUT", small) - puts; n != 1 {
		t.Errorf("%d PUT sent, want the light one alone", n)
	}
}
