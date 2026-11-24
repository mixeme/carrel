//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// hasProp reports whether the record itself in doc holds prop.
func hasProp(doc, prop string) bool {
	for _, l := range matrixLines(doc) {
		if u := strings.ToUpper(l); strings.HasPrefix(u, prop+":") || strings.HasPrefix(u, prop+";") {
			return true
		}
	}
	return false
}

// liveMatrix runs the matrix on the collections cols names, by kind (a
// server may keep events and tasks apart, SV-52, or answer for cards and
// calendars at two addresses), in an app and a session of its own: its two
// hundred requests in a few seconds are past the bucket of one session (the
// steps after it on Baikal answered 429), which a person's session never
// reaches. A kind whose collection does not hold it is left out.
func liveMatrix(ctx context.Context, t *testing.T, client *dav.Client, endpoint, user, pass string, cols map[view.Kind]string, suffix string) {
	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"label": {"matrix " + suffix}, "server": {endpoint}, "login": {user}, "password": {pass}, "noPassword": {askedNone(pass)},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	connID := blob.Connections[0].ID
	colOf := map[view.Kind]string{}
	for kind, path := range cols {
		if path == "" {
			continue
		}
		if c := catalogEntry(t, blob, path); c.Has(kind) {
			colOf[kind] = c.Path
		} else {
			t.Logf("%s holds no %s: the %s part of the matrix is left out", path, kind, kind)
		}
	}
	personal := personalAccount(endpoint)
	for i, r := range matrixRecords() {
		col, ok := colOf[r.kind]
		if !ok {
			continue
		}
		if personal && strings.Contains(r.doc(""), "\nATTENDEE") {
			t.Logf("%s: a record with attendees is not seeded in a person's own account — the server would send the invitations", r.name)
			continue
		}
		for j, c := range r.cases {
			t.Run(r.name+"/"+c.name, func(t *testing.T) {
				app.freshSession(t)
				matrixCaseRun(ctx, t, app, client, connID, col, suffix, i, j, r, c)
			})
		}
	}
	// A new record holds what was typed and no line for a field left empty.
	for _, n := range []struct {
		kind view.Kind
		set  map[string]string
		says string
	}{
		{view.KindContact, map[string]string{"fn": "Grace Hopper", "email.0": "grace@example.com"}, "grace@example.com"},
		{view.KindEvent, map[string]string{"summary": "Launch", "start.date": "2026-03-02", "start.time": "10:00", "end.date": "2026-03-02", "end.time": "11:00"}, "SUMMARY:Launch"},
		{view.KindTask, map[string]string{"summary": "Ship"}, "SUMMARY:Ship"},
	} {
		col, ok := colOf[n.kind]
		if !ok {
			continue
		}
		t.Run("new "+string(n.kind), func(t *testing.T) {
			app.freshSession(t)
			list := matrixList(n.kind, connID, col)
			var v url.Values
			switch n.kind {
			case view.KindContact:
				v = liveForm(liveDecode[view.ContactEditPage](t, "new", app.get(list+"/new")).Form)
			case view.KindEvent:
				v = liveForm(liveDecode[view.EventEditPage](t, "new", app.get(list+"/new")).Form)
			default:
				v = liveForm(liveDecode[view.TaskEditPage](t, "new", app.get(list+"/new")).Form)
			}
			for k, val := range n.set {
				v.Set(k, val)
			}
			rr := app.post(list+"/new", v)
			var file string
			if rr.Code == http.StatusOK {
				// Saved, and the server did not keep all of it: the answer
				// is the record with the report (C-9), at the address the
				// server holds it.
				var body struct {
					Loss   *view.LossReport `json:"loss"`
					Source *struct {
						Path string `json:"path"`
					} `json:"source"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Loss == nil || body.Source == nil {
					t.Fatalf("new: 200 without a loss report:\n%s", rr.Body.String())
				}
				t.Logf("new: the server kept everything but %v", body.Loss.Properties)
				file = body.Source.Path
			} else {
				loc := redirected(t, "new", rr)
				uid, _ := url.PathUnescape(loc[strings.LastIndex(loc, "/")+1:])
				ext, _ := matrixExt(n.kind)
				file = col + uid + ext
			}
			removeLater(t, client, file)
			got := bodyIfThere(ctx, t, client, file)
			if got == "" {
				t.Skipf("the new record is not at %s: the server named it itself", file)
			}
			mustHave(t, "new "+string(n.kind), got, n.says)
			// A record written straight to the server, without the form,
			// shows what the server itself serves empty (Google:
			// DESCRIPTION and LOCATION on every event): those lines are its.
			own := serverEmpty(ctx, t, client, col, n.kind, suffix)
			var empty []string
			for _, l := range matrixLines(got) {
				if strings.HasSuffix(l, ":") && !own[strings.ToUpper(l)] {
					empty = append(empty, l)
				}
			}
			if len(empty) > 0 {
				t.Errorf("a new %s holds empty lines %v:\n%s", n.kind, empty, got)
			}
		})
	}
}

// serverEmpty are the empty lines («DESCRIPTION:») the server serves on a
// record of kind written straight to it with none: the server's own, not
// Carrel's.
func serverEmpty(ctx context.Context, t *testing.T, client *dav.Client, col string, kind view.Kind, suffix string) map[string]bool {
	t.Helper()
	ext, media := matrixExt(kind)
	uid := "matrix-plain-" + suffix + "-" + string(kind)
	doc := map[view.Kind]string{
		view.KindContact: "BEGIN:VCARD\nVERSION:3.0\nUID:" + uid + "\nFN:Plain\nN:Plain;;;;\nEND:VCARD\n",
		view.KindEvent:   matrixCal("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART:20260302T070000Z\nDTEND:20260302T080000Z\nSUMMARY:Plain\nEND:VEVENT"),
		view.KindTask:    matrixCal("BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:Plain\nEND:VTODO"),
	}[kind]
	out := map[string]bool{}
	if !trySeed(ctx, t, client, col+uid+ext, media, doc) {
		return out
	}
	removeLater(t, client, col+uid+ext)
	got := bodyIfThere(ctx, t, client, col+uid+ext)
	if kept := uidOf(got); kept != "" && kept != uid {
		removeLater(t, client, col+kept)
	}
	for _, l := range matrixLines(strings.ReplaceAll(got, "\r\n", "\n")) {
		if strings.HasSuffix(l, ":") {
			out[strings.ToUpper(l)] = true
			t.Logf("the server serves %q on a record written without it", l)
		}
	}
	return out
}

// personalAccount reports a server whose account is the owner's own mail —
// Yandex, Google, iCloud, Mail.ru —, where an event with attendees sends
// invitations from it (TestGoogleEventInTheExistingCalendar: «no attendee
// is ever added»). The test accounts (Baikal, Nextcloud, SoGo's test box)
// may.
func personalAccount(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range []string{"yandex.", "google", "icloud.com", "mail.ru"} {
		if strings.Contains(host, h) {
			return true
		}
	}
	return false
}

// matrixCaseRun is one case of the matrix: seed, prime, read, save, read,
// compare.
func matrixCaseRun(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, col, suffix string, i, j int, r matrixRecord, c matrixCase) {
	ext, media := matrixExt(r.kind)
	uid := "matrix-" + suffix + "-" + string(rune('a'+i)) + string(rune('a'+j))
	file := col + uid + ext
	if !trySeed(ctx, t, client, file, media, r.doc(uid)) {
		t.Skip("the server refused the record")
	}
	removeLater(t, client, file)
	seeded := bodyIfThere(ctx, t, client, file)
	if seeded == "" {
		t.Skip("the server kept nothing at the address")
	}
	// A server may name the record itself and write its UID over the one
	// sent (SV-25, Google): it answers GET at the address it was sent to and
	// lives under the name it gave, as liveContactFields reads it.
	if kept := uidOf(seeded); kept != "" && kept != uid {
		t.Logf("the server named the record %q, not %q", kept, uid)
		uid = kept
		removeLater(t, client, col+kept)
	}
	rec := matrixList(r.kind, connID, col) + "/" + url.PathEscape(uid)
	primeLost := matrixSave(t, app, r.kind, rec, r.prime)
	before, _ := rawBody(ctx, t, client, file)
	lost := matrixSave(t, app, r.kind, rec, c.set)
	after, _ := rawBody(ctx, t, client, file)
	var changed []string
	for _, n := range c.changed {
		changed = append(changed, bareName(n))
	}
	for i, n := range lost {
		lost[i] = bareName(n)
	}
	for i, n := range primeLost {
		primeLost[i] = bareName(n)
	}
	// A server that keeps a length as an end (Yandex) holds DTEND where the
	// record had DURATION: the end is then the line an end or a move
	// changes.
	if strings.Contains(r.doc(uid), "\nDURATION:") && !hasProp(before, "DURATION") && hasProp(before, "DTEND") {
		t.Log("the server keeps the length as an end: DTEND stands for DURATION")
		changed = append(append([]string{}, changed...), "DTEND")
	}
	reported := false
	for _, n := range changed {
		reported = reported || contains(lost, n)
	}
	var unasked []string
	for _, n := range matrixChanged(before, after) {
		// A server that schedules makes the account the organizer of an
		// event it sends invitations for (RFC 6638; docs/servers.md, SV-60):
		// its line, not the edit's.
		if n == "ORGANIZER" && contains(changed, "ATTENDEE") && !hasProp(before, "ORGANIZER") {
			t.Logf("the server made the account the organizer: %q", lineOf(after, "ORGANIZER"))
			continue
		}
		if !contains(changed, n) && !contains(lost, n) && !contains(primeLost, n) {
			unasked = append(unasked, n)
		}
	}
	if len(unasked) > 0 {
		t.Errorf("lines changed beyond %v: %v\nbefore:\n%s\nafter:\n%s", changed, unasked, before, after)
	}
	// What the record held before and what was typed is demanded after;
	// not of a property the server said it did not keep — the person was
	// told.
	typed := strings.ToLower(strings.Join(slices.Collect(maps.Values(c.set)), " "))
	// Google keeps a colon escaped (SV-61): «https\://» is «https://».
	before = strings.ReplaceAll(before, `\:`, ":")
	after = strings.ReplaceAll(after, `\:`, ":")
	for _, s := range c.says {
		low := strings.ToLower(s)
		prop := ""
		if i := strings.Index(s, ":"); i > 0 && !strings.Contains(s[:i], "/") {
			prop = bareName(strings.SplitN(s[:i], ";", 2)[0])
		}
		switch {
		case prop != "" && contains(changed, prop) && count(before, prop) < 2 && strings.Contains(strings.ToLower(before), low):
			// A second value the server did not keep (Google keeps one
			// TITLE, one ORG): the one the form showed is the one changed.
			t.Logf("not demanded, the server held one %s: %q", prop, s)
		case reported:
			t.Logf("not demanded, the server said it did not keep %v: %q", lost, s)
		case !strings.Contains(strings.ToLower(before), low) && !strings.Contains(typed, low):
			t.Logf("not demanded, the record did not hold it before: %q", s)
		case !strings.Contains(strings.ToLower(after), low):
			t.Errorf("missing %q after the save:\n%s", s, after)
		}
	}
	for _, g := range c.gone {
		if strings.Contains(after, g) && !reported {
			t.Errorf("%q stayed:\n%s", g, after)
		}
	}
}
