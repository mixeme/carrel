package renderer_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/renderer"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// running is a poll that runs: its stream, its fallback every 1.5 s.
func running() *view.PollBar {
	return &view.PollBar{TaskID: "t1", Running: true, Percent: 40, Stream: "/app/search/progress/t1?from=x",
		Poll: "/app/search/progress/t1/poll?from=x", Every: 1500, Summary: view.Runs{{Text: "Polled 2 of 5"}}}
}

// A poll's bar answered alone (its fallback, a retry, Stop) is the zone poll
// as the lists draw it: a running one follows its run — the stream with the
// events of C-7.4, the fallback every Every milliseconds —, one that is over
// follows nothing, and with none standing the zone is there, closed
// (Typecase's note 124).
func TestThePollIsItsZone(t *testing.T) {
	rd := renderer.New("")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	answer := func(body any, extra []render.OOB) string {
		t.Helper()
		rec := httptest.NewRecorder()
		if err := rd.Fragment(rec, req, http.StatusOK, render.TargetPoll, body, extra); err != nil {
			t.Fatal(err)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("the poll's answer is %q: %s", ct, rec.Body)
		}
		return rec.Body.String()
	}
	run := answer(running(), nil)
	for _, want := range []string{`data-tc-zone="poll"`, `data-tc-stream="/app/search/progress/t1?from=x"`, `data-tc-changed="poll"`,
		`data-tc-ended="poll-finished"`, `data-tc-poll="/app/search/progress/t1/poll?from=x"`, `data-tc-every="1500"`} {
		if !strings.Contains(run, want) {
			t.Errorf("a running bar has no %s: %s", want, run)
		}
	}
	over := running()
	over.Running = false
	over.Sources = []view.SourceProgress{{State: view.SourceError}}
	if got := answer(*over, nil); !strings.Contains(got, `data-tc-zone="poll"`) || strings.Contains(got, "data-tc-stream") || strings.Contains(got, "data-tc-poll") {
		t.Errorf("a bar of a run that is over = %s, want the zone following nothing", got)
	}
	done := &view.PollBar{TaskID: "t1"}
	if got := answer(done, nil); got != document.EmptyPoll.Render() {
		t.Errorf("no bar standing = %s, want the closed zone", got)
	}
	// The end of a run brings the rail of its page out of band, drawn by
	// the screen of its section.
	rail := view.Rail{Nav: []view.NavRow{{ID: "nav.contacts", Label: "Contacts", On: true}}}
	if got := answer(done, []render.OOB{{Target: render.TargetRail, Body: rail}}); !strings.HasPrefix(got, document.EmptyPoll.Render()+`<template data-tc-zone="rail">`) || !strings.Contains(got, "Contacts") {
		t.Errorf("the end with the rail = %s", got)
	}
	// What the screens do not draw alone stays JSON.
	rec := httptest.NewRecorder()
	if err := rd.Fragment(rec, req, http.StatusOK, render.TargetPoll, done, []render.OOB{{Target: render.TargetQueue, Body: view.TransferQueue{}}}); err != nil {
		t.Fatal(err)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("a queue out of band answers %q", ct)
	}
}

// An event of the run's stream is the poll's answer, its lines the event's
// data lines; what the screens do not draw is the JSON renderer's event.
func TestAnEventIsThePollsAnswer(t *testing.T) {
	rd := renderer.New("")
	var got bytes.Buffer
	if err := rd.Event(&got, render.EventPoll, render.TargetPoll, running(), nil); err != nil {
		t.Fatal(err)
	}
	s := got.String()
	if !strings.HasPrefix(s, "event: poll\ndata: <div class=\"tc-runbar\"") || !strings.HasSuffix(s, "\n\n") {
		t.Errorf("the event = %q", s)
	}
	for _, line := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(s, "event: poll\n"), "\n\n"), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			t.Errorf("a line of the event is no data line: %q", line)
		}
	}
	var mine, theirs bytes.Buffer
	if err := rd.Event(&mine, render.EventPoll, render.TargetMain, view.Rail{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := (render.JSONRenderer{}).Event(&theirs, render.EventPoll, render.TargetMain, view.Rail{}, nil); err != nil {
		t.Fatal(err)
	}
	if mine.String() != theirs.String() {
		t.Errorf("an event the screens do not draw = %q, want %q", mine.String(), theirs.String())
	}
}

// An upload's answer is the queue as the folder draws it, the zone queue —
// waiting for the next upload, its rows moving in place —, and the folder
// out of band, its main: the file sent stands in its list at once.
func TestTheQueueIsItsZone(t *testing.T) {
	rd := renderer.New("")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	q := view.TransferQueue{Title: "Uploading 1 file"}
	if err := rd.Fragment(rec, req, http.StatusOK, render.TargetQueue, q, []render.OOB{{Target: render.TargetMain, Body: view.FilesPage{}}}); err != nil {
		t.Fatal(err)
	}
	got := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("the queue's answer is %q: %s", ct, got)
	}
	for _, want := range []string{`data-tc-zone="queue"`, `data-tc-progress="`, `<template data-tc-zone="main"><div class="tc-main" data-tc-zone="main"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the queue's answer has no %s: %.400s", want, got)
		}
	}
}

// A tick of the rail asks the rail's zone (Typecase's note 186,
// components-43): the answer is the rail as its section draws it, the forms
// it sends, and out of band the poll and the list after it — each in its
// template, which the live layer puts in place, the page kept.
func TestTheRailIsItsZone(t *testing.T) {
	rd := renderer.New("")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rail := view.Rail{Nav: []view.NavRow{{ID: "nav.calendar", Label: "Calendar", On: true}},
		Ticks: &view.Form{Action: "/app/sources/calendar", Method: "POST", Target: render.TargetRail}}
	list := view.AgendaPage{Shell: view.Shell{Title: "Combined calendar", Rail: rail}, Days: []view.AgendaDay{}}
	rec := httptest.NewRecorder()
	if err := rd.Fragment(rec, req, http.StatusOK, render.TargetRail, rail,
		[]render.OOB{{Target: render.TargetPoll, Body: view.PollBar{TaskID: "t1"}}, {Target: render.TargetMain, Body: list}}); err != nil {
		t.Fatal(err)
	}
	got := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("the rail's answer is %q: %s", ct, got)
	}
	poll, main := strings.Index(got, `<template data-tc-zone="poll">`), strings.Index(got, `<template data-tc-zone="main">`)
	form := strings.Index(got, `action="/app/sources/calendar"`)
	if !strings.Contains(got, "Calendar") || form < 0 || poll < form || main < poll || !strings.Contains(got[main:], "Combined calendar") {
		t.Errorf("the rail's answer = %s", got)
	}
	if !strings.Contains(got[:poll], `data-tc-target="rail"`) {
		t.Errorf("the rail's form does not ask the rail's zone: %s", got[:poll])
	}
}

// The end of a poll brings the page it stands on (C-7.3; U-15.1, found on
// the acceptance stand): the bar as it ended, and out of band the rail and
// the list as it reads after the poll — its rows are what the poll brought.
// All of it HTML, the data lines of one event.
func TestTheEndOfAPollIsItsPage(t *testing.T) {
	rd := renderer.New("")
	rail := view.Rail{Nav: []view.NavRow{{ID: "nav.contacts", Label: "Contacts", On: true}}}
	list := view.ContactsListPage{Shell: view.Shell{Title: "Combined address book", Rail: rail},
		Groups: []view.ContactGroup{{Rubric: "A", Count: 1, Rows: []view.ContactRow{{Name: "Ada Lovelace", Href: "/app/contacts/a/b/ada"}}}}}
	var buf bytes.Buffer
	if err := rd.Event(&buf, render.EventPollFinished, render.TargetPoll, view.PollBar{TaskID: "t1"},
		[]render.OOB{{Target: render.TargetRail, Body: rail}, {Target: render.TargetMain, Body: list}}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "event: poll-finished\n") || strings.Contains(got, `"target"`) {
		t.Fatalf("the end of a poll is not drawn: %.400s", got)
	}
	rails, main := strings.Index(got, `<template data-tc-zone="rail">`), strings.Index(got, `<template data-tc-zone="main">`)
	if rails < 0 || main < rails || !strings.Contains(got[main:], "Ada Lovelace") {
		t.Errorf("the end of a poll = %.1200s", got)
	}
	// The top bar with it, drawn as the page draws it: Refresh on (the owner,
	// 24.11.2026: it stayed off until the page was loaded again).
	list.Top = view.TopBar{Refresh: view.Refresh{Action: view.Action{ID: "poll.again", Label: "Refresh", Icon: "i-refresh", Href: "/app/contacts?again=1", Method: "GET", Enabled: true}, Answered: "just now"}}
	buf.Reset()
	if err := rd.Event(&buf, render.EventPollFinished, render.TargetPoll, view.PollBar{TaskID: "t1"},
		[]render.OOB{{Target: render.TargetRail, Body: rail}, {Target: render.TargetMain, Body: list}, {Target: render.TargetTopbar, Body: list}}); err != nil {
		t.Fatal(err)
	}
	got = buf.String()
	top := strings.Index(got, `<template data-tc-zone="topbar">`)
	if strings.Contains(got, `"target"`) || top < 0 || !strings.Contains(got[top:], "just now") {
		t.Errorf("the end of a poll with its top bar = %.1500s", got)
	}
}
