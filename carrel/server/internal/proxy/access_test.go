package proxy_test

import (
	"strings"
	"testing"
)

// Owner's decision 24.09.2026 (revisiting S-27): a device is read only or
// read and write. A read-only device reads as before; a write is refused
// here, 403 with need-privileges, and never reaches the collection's server;
// the home lists its collections with the read privilege alone, so a client
// shows them read-only. The right changes without a new password: the same
// password writes again once the device is read and write.
func TestAReadOnlyDeviceWritesNothing(t *testing.T) {
	f := newFixture(t)
	if err := f.users.SetDeviceAccess(f.userID, f.deviceID, true); err != nil {
		t.Fatal(err)
	}
	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\nBEGIN:VEVENT\r\nUID:e1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261001T100000Z\r\nSUMMARY:Refused\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	puts := f.far.CountRequests("PUT", work)
	for _, m := range []string{"PUT", "DELETE", "PROPPATCH", "MKCOL", "MOVE", "COPY", "LOCK"} {
		resp, body := f.do(m, f.col(work)+"e1.ics", event, map[string]string{"Destination": f.srv.URL + f.col(work) + "e2.ics"})
		if resp.StatusCode != 403 || !strings.Contains(body, "need-privileges") {
			t.Errorf("%s: %d %s", m, resp.StatusCode, body)
		}
	}
	if n := f.far.CountRequests("PUT", work); n != puts {
		t.Errorf("a write reached the far server: %d PUTs", n-puts)
	}
	if resp, body := f.do("GET", f.col(docs)+"readme.txt", "", nil); resp.StatusCode != 200 || body != "hello from the files" {
		t.Errorf("read: %d %q", resp.StatusCode, body)
	}
	_, home := f.do("PROPFIND", "/cal/", `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-privilege-set/></d:prop></d:propfind>`, map[string]string{"Depth": "1"})
	if !strings.Contains(home, "<d:read/>") || strings.Contains(home, "<d:write/>") {
		t.Errorf("home of a read-only device:\n%s", home)
	}

	if err := f.users.SetDeviceAccess(f.userID, f.deviceID, false); err != nil {
		t.Fatal(err)
	}
	if resp, body := f.do("PUT", f.col(work)+"e1.ics", event, map[string]string{"Content-Type": "text/calendar; charset=utf-8", "If-None-Match": "*"}); resp.StatusCode/100 != 2 {
		t.Errorf("read and write again: %d %s", resp.StatusCode, body)
	}
	if _, home := f.do("PROPFIND", "/cal/", `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-privilege-set/></d:prop></d:propfind>`, map[string]string{"Depth": "1"}); !strings.Contains(home, "<d:write/>") {
		t.Errorf("home of a read-and-write device:\n%s", home)
	}
}
