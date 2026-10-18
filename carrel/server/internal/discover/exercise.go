package discover

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// Exercise is the second, mutating stage of the DAV validator (U-6.13
// /admin/dav): in the first writable collection of each kind it creates a
// test object, reads it back and deletes it, and says what each step got.
// Only by explicit consent; nothing else on the server is touched, and an
// object the server would not delete is named with its path.
func Exercise(ctx context.Context, g *dav.Guard, res *Result, username, password, certPin string) view.ExerciseReport {
	report := view.ExerciseReport{OK: true}
	client, err := dav.NewClient(g, res.BaseURL, username, password, certPin)
	if err != nil {
		report.OK = false
		report.Rows = append(report.Rows, view.ExerciseRow{Operation: "Connect", Result: "failed", Detail: view.Text(formatErr(err))})
		return report
	}
	tag := make([]byte, 6)
	_, _ = rand.Read(tag)
	id := "carrel-check-" + hex.EncodeToString(tag)

	type probe struct {
		kind  view.Kind
		noun  string
		ext   string
		body  string
		found string
		media string
	}
	probes := []probe{
		{view.KindEvent, "an event", ".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//check//EN\r\nBEGIN:VEVENT\r\nUID:" + id +
			"\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260101T100000Z\r\nDTEND:20260101T110000Z\r\nSUMMARY:Carrel check\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", id, "text/calendar; charset=utf-8"},
		{view.KindTask, "a task", ".ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//check//EN\r\nBEGIN:VTODO\r\nUID:" + id +
			"\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Carrel check\r\nEND:VTODO\r\nEND:VCALENDAR\r\n", id, "text/calendar; charset=utf-8"},
		{view.KindContact, "a contact", ".vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + id + "\r\nFN:Carrel check\r\nN:check;Carrel;;;\r\nEND:VCARD\r\n", id, "text/vcard; charset=utf-8"},
		{view.KindFile, "a file", ".txt", "Carrel check " + id + "\n", id, "text/plain; charset=utf-8"},
	}
	for _, p := range probes {
		op := view.Text("Create, read back, delete · " + string(p.kind))
		var col *Collection
		for i := range res.Collections {
			c := &res.Collections[i]
			if !c.ReadOnly && hasKind(c.Kinds, p.kind) {
				col = c
				break
			}
		}
		if col == nil {
			report.Rows = append(report.Rows, view.ExerciseRow{Operation: op, Result: "skipped",
				Detail: "No collection of this kind that can be written to."})
			continue
		}
		path := strings.TrimSuffix(col.Path, "/") + "/" + id + "-" + string(p.kind) + p.ext
		r := exercised{path: path}
		if _, ok := r.step(ctx, client, "PUT", http.Header{"If-None-Match": {"*"}, "Content-Type": {p.media}}, p.body, nil); ok {
			// The delete names the version read back; after a failed read
			// it is still tried, the object being there.
			etag, _ := r.step(ctx, client, "GET", nil, "", func(data string) bool { return strings.Contains(data, p.found) })
			var h http.Header
			if etag != "" {
				h = http.Header{"If-Match": {etag}}
			}
			r.step(ctx, client, "DELETE", h, "", nil)
		}
		if r.failed || r.stays {
			report.OK = false
		}
		report.Rows = append(report.Rows, view.ExerciseRow{Operation: op, Result: r.result(), Detail: view.Text(r.detail())})
	}
	return report
}

func hasKind(kinds []view.Kind, k view.Kind) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

// exercised is one kind's create, read back and delete (s-admin-29): the
// methods asked and the codes answered, as the frame writes them —
// «PUT · GET · DELETE /path → 201 · 200 · 204», a refusal with its words.
type exercised struct {
	path           string
	methods, codes []string
	failed         bool
	// stays: the object was written and not deleted yet.
	stays bool
	note  string
}

// step asks the server once; check, when given, judges the body read.
// It answers the ETag the server gave and whether the step went through.
func (e *exercised) step(ctx context.Context, client *dav.Client, method string, h http.Header, body string, check func(string) bool) (string, bool) {
	e.methods = append(e.methods, method)
	var rd io.Reader
	length := int64(-1)
	if body != "" {
		rd = strings.NewReader(body)
		length = int64(len(body))
	}
	resp, err := client.Forward(ctx, method, e.path, h, rd, length, false)
	if err != nil {
		e.codes = append(e.codes, formatErr(err))
		e.failed = true
		return "", false
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		e.codes = append(e.codes, fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
		e.failed = true
		return "", false
	}
	e.codes = append(e.codes, fmt.Sprint(resp.StatusCode))
	switch method {
	case "PUT":
		e.stays = true
	case "DELETE":
		e.stays = false
	}
	if check != nil && !check(string(data)) {
		e.failed = true
		e.note = "the server answered with something else"
		return resp.Header.Get("ETag"), false
	}
	return resp.Header.Get("ETag"), true
}

func (e *exercised) result() view.Text {
	if e.failed || e.stays {
		return "failed"
	}
	return "ok"
}

func (e *exercised) detail() string {
	d := strings.Join(e.methods, " · ") + " " + e.path + " → " + strings.Join(e.codes, " · ")
	if e.note != "" {
		d += ", " + e.note
	}
	if e.stays {
		d += " — the test object stays on the server"
	}
	return d
}
