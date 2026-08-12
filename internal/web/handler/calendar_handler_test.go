// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"bytes"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/store"
	"gitea.mixdep.ru/mix/carrel/internal/web"
)

func TestCalendarHomeEmptyState(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "", testPassword)
	rec := a.get("/app/calendar")
	if rec.Code != http.StatusOK {
		t.Fatalf("calendar home = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Connect a CalDAV account") {
		t.Fatalf("missing calendar empty state:\n%s", rec.Body.String())
	}
}

func TestAgendaTemplatePrintChrome(t *testing.T) {
	body, err := fs.ReadFile(web.TemplateFS, "template/agenda.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agenda-print-root", "print-footer", "data-print", "agenda-day"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("agenda template missing %q", want)
		}
	}
}

func TestCalendarImportPreviewUIDCollision(t *testing.T) {
	davSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/">
 <d:response><d:href>/calendars/user/default/</d:href><d:propstat><d:prop><cs:getctag>1</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
 <d:response><d:href>/calendars/user/default/existing.ics</d:href><d:propstat><d:prop><d:getetag>"v1"</d:getetag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`)
	}))
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.Import = config.Import{MaxBytes: 1 << 20, MaxCards: 100}
	a.setupAdmin("root", "", testPassword)
	sess := a.session()
	acc := account.Account{
		ID: "cal-test", Label: "Test", BaseURL: davSrv.URL + "/", Username: "mix", Password: "secret", Enabled: true,
		Collections: []discovery.Collection{{Path: "/calendars/user/default/", DisplayName: "Personal", Kind: discovery.KindCalendar}},
	}
	if err := a.Store.PutDAVAccount(store.Actor{ID: sess.UserID, Login: sess.Login}, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
	colEnc := EncodeCollectionPath(acc.Collections[0].Path)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField(CSRFField, a.token())
	part, err := mw.CreateFormFile("file", "events.ics")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:existing\r\nDTSTAMP:20260812T090000Z\r\nDTSTART:20260812T100000Z\r\nDTEND:20260812T110000Z\r\nSUMMARY:Collision\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	rec := a.postRaw("/app/calendar/"+acc.ID+"/"+colEnc+"/import", mw.FormDataContentType(), &body)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "New UID will be assigned") {
		t.Fatalf("collision warning missing:\n%s", rec.Body.String())
	}
}

func TestEventPatchPreservesUnknownProperty(t *testing.T) {
	obj, err := model.ParseICal("/cal/event.ics", `"v1"`, []byte(
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n"+
			"UID:event\r\nDTSTART:20260812T100000Z\r\nDTEND:20260812T110000Z\r\n"+
			"SUMMARY:Before\r\nX-CARREL-TEST:kept\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	form := eventForm{
		Summary: "After", StartDate: "2026-08-12", StartTime: "10:00",
		EndDate: "2026-08-12", EndTime: "11:00",
		RRuleFreq: "NONE", RRuleInterval: "1", RRuleByDay: map[string]bool{},
	}
	patch, err := form.toPatch(time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Apply(patch); err != nil {
		t.Fatal(err)
	}
	if !obj.Has("X-CARREL-TEST") {
		t.Fatal("event patch removed unknown property")
	}
}

func TestStructuredRRuleRoundTrip(t *testing.T) {
	ev := model.Event{RRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;COUNT=5"}
	form := formFromEvent(ev)
	if form.RRule != "" || form.RRuleFreq != "WEEKLY" || !form.RRuleByDay["MO"] {
		t.Fatalf("form = %+v", form)
	}
	got, err := form.buildRRule()
	if err != nil {
		t.Fatal(err)
	}
	if got != ev.RRule {
		t.Fatalf("RRULE = %q, want %q", got, ev.RRule)
	}
}
