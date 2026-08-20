// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package exercise runs the optional mutating DAV validation pass of §6: after
// discovery succeeds it creates short-lived objects, exercises the operations
// Carrel actually uses, reads them back, and deletes them.
package exercise

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
)

const marker = "carrel-exercise"

// Run exercises writable collections from a successful discovery result. Steps
// are appended to trace in the same form as discovery diagnostics.
func Run(ctx context.Context, client *dav.Client, result *discovery.Result, trace *discovery.Trace) error {
	if client == nil || result == nil {
		return errors.New("exercise: client and discovery result are required")
	}
	if trace == nil {
		trace = &discovery.Trace{}
	}

	var firstErr error
	testedCalendar := false
	testedAddressBook := false
	testedFiles := false

	for _, col := range result.Collections {
		if col.ReadOnly {
			trace.Add(skipStep(col.Kind), "read-only collection skipped", 0, col.Path)
			continue
		}
		var err error
		switch col.Kind {
		case discovery.KindCalendar:
			if testedCalendar {
				continue
			}
			err = exerciseCalendar(ctx, client, col, trace)
			testedCalendar = true
		case discovery.KindAddressBook:
			if testedAddressBook {
				continue
			}
			err = exerciseAddressBook(ctx, client, col, trace)
			testedAddressBook = true
		case discovery.KindFiles:
			if testedFiles {
				continue
			}
			err = exerciseFiles(ctx, client, col, trace)
			testedFiles = true
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if !testedCalendar && !testedAddressBook && !testedFiles {
		trace.Add("exercise", "no writable collection to exercise", 0, result.BaseURL)
	}
	return firstErr
}

func skipStep(kind discovery.Kind) string {
	switch kind {
	case discovery.KindCalendar:
		return "calendar_skip"
	case discovery.KindAddressBook:
		return "addressbook_skip"
	case discovery.KindFiles:
		return "files_skip"
	default:
		return "skip"
	}
}

func exerciseCalendar(ctx context.Context, client *dav.Client, col discovery.Collection, trace *discovery.Trace) error {
	component := pickCalendarComponent(col.SupportedComponents)
	if component == "" {
		trace.Add("calendar_skip", "no supported component to write", 0, col.Path)
		return nil
	}

	uid := testUID()
	path := col.Path + url.PathEscape(uid) + ".ics"
	body := calendarBody(component, uid)
	cleanup := func() { _ = client.Delete(ctx, path, "") }
	defer cleanup()

	etag, err := client.PutOpts(ctx, path, strings.NewReader(body), dav.PutOptions{
		ContentType: dav.MediaTypeCalendar,
		IfNoneMatch: true,
	})
	if err != nil {
		return stepFail(trace, "calendar_create", path, err)
	}
	stepOK(trace, "calendar_create", path, http.StatusCreated)

	if err := expectPreconditionFailed(ctx, client, path, body, dav.PutOptions{
		ContentType: dav.MediaTypeCalendar,
		IfNoneMatch: true,
	}); err != nil {
		return stepFail(trace, "calendar_create_guard", path, err)
	}
	stepExpected412(trace, "calendar_create_guard", path)

	if err := expectPreconditionFailed(ctx, client, path, body, dav.PutOptions{
		ContentType: dav.MediaTypeCalendar,
		IfMatch:     `"stale"`,
	}); err != nil {
		return stepFail(trace, "calendar_conflict", path, err)
	}
	stepExpected412(trace, "calendar_conflict", path)

	var queryBody any
	var from, to time.Time
	if component == ical.CompEvent {
		from = time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		to = from.Add(48 * time.Hour)
		queryBody = dav.NewCalendarQuery(from, to)
	} else {
		queryBody = dav.NewCalendarComponentQuery(component, time.Time{}, time.Time{})
	}
	ms, err := client.Report(ctx, col.Path, dav.DepthOne, queryBody)
	if err != nil {
		return stepFail(trace, "calendar_query", col.Path, err)
	}
	if !reportContains(ms, path, uid) {
		return stepFail(trace, "calendar_query", col.Path, fmt.Errorf("object %s not in calendar-query answer", path))
	}
	stepOK(trace, "calendar_query", col.Path, http.StatusMultiStatus)

	ms, err = client.Report(ctx, col.Path, dav.DepthZero, dav.NewCalendarMultiget([]string{path}))
	if err != nil {
		return stepFail(trace, "calendar_multiget", col.Path, err)
	}
	if !reportContains(ms, path, uid) {
		return stepFail(trace, "calendar_multiget", col.Path, fmt.Errorf("object %s not in multiget answer", path))
	}
	stepOK(trace, "calendar_multiget", col.Path, http.StatusMultiStatus)

	rc, _, err := client.Get(ctx, path, nil)
	if err != nil {
		return stepFail(trace, "calendar_get", path, err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return stepFail(trace, "calendar_get", path, err)
	}
	if !bytes.Contains(got, []byte(uid)) {
		return stepFail(trace, "calendar_get", path, fmt.Errorf("GET body does not contain UID %s", uid))
	}
	stepOK(trace, "calendar_get", path, http.StatusOK)

	if err := client.Delete(ctx, path, etag); err != nil {
		return stepFail(trace, "calendar_delete", path, err)
	}
	stepOK(trace, "calendar_delete", path, http.StatusNoContent)
	cleanup = func() {}
	return nil
}

func exerciseAddressBook(ctx context.Context, client *dav.Client, col discovery.Collection, trace *discovery.Trace) error {
	uid := testUID()
	label := marker + " " + uid
	path := col.Path + url.PathEscape(uid) + ".vcf"
	body := vcardBody(uid, label)
	cleanup := func() { _ = client.Delete(ctx, path, "") }
	defer cleanup()

	etag, err := client.PutOpts(ctx, path, strings.NewReader(body), dav.PutOptions{
		ContentType: dav.MediaTypeVCard,
		IfNoneMatch: true,
	})
	if err != nil {
		return stepFail(trace, "addressbook_create", path, err)
	}
	stepOK(trace, "addressbook_create", path, http.StatusCreated)

	if err := expectPreconditionFailed(ctx, client, path, body, dav.PutOptions{
		ContentType: dav.MediaTypeVCard,
		IfNoneMatch: true,
	}); err != nil {
		return stepFail(trace, "addressbook_create_guard", path, err)
	}
	stepExpected412(trace, "addressbook_create_guard", path)

	if err := expectPreconditionFailed(ctx, client, path, body, dav.PutOptions{
		ContentType: dav.MediaTypeVCard,
		IfMatch:     `"stale"`,
	}); err != nil {
		return stepFail(trace, "addressbook_conflict", path, err)
	}
	stepExpected412(trace, "addressbook_conflict", path)

	ms, err := client.Report(ctx, col.Path, dav.DepthOne, dav.NewAddressBookQuery(label))
	if err != nil {
		return stepFail(trace, "addressbook_query", col.Path, err)
	}
	if !reportContainsVCard(ms, path, uid) {
		return stepFail(trace, "addressbook_query", col.Path, fmt.Errorf("object %s not in addressbook-query answer", path))
	}
	stepOK(trace, "addressbook_query", col.Path, http.StatusMultiStatus)

	ms, err = client.Report(ctx, col.Path, dav.DepthZero, dav.NewAddressBookMultiget([]string{path}))
	if err != nil {
		return stepFail(trace, "addressbook_multiget", col.Path, err)
	}
	if !reportContainsVCard(ms, path, uid) {
		return stepFail(trace, "addressbook_multiget", col.Path, fmt.Errorf("object %s not in multiget answer", path))
	}
	stepOK(trace, "addressbook_multiget", col.Path, http.StatusMultiStatus)

	rc, _, err := client.Get(ctx, path, nil)
	if err != nil {
		return stepFail(trace, "addressbook_get", path, err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return stepFail(trace, "addressbook_get", path, err)
	}
	if !bytes.Contains(got, []byte(uid)) {
		return stepFail(trace, "addressbook_get", path, fmt.Errorf("GET body does not contain UID %s", uid))
	}
	stepOK(trace, "addressbook_get", path, http.StatusOK)

	if err := client.Delete(ctx, path, etag); err != nil {
		return stepFail(trace, "addressbook_delete", path, err)
	}
	stepOK(trace, "addressbook_delete", path, http.StatusNoContent)
	cleanup = func() {}
	return nil
}

func exerciseFiles(ctx context.Context, client *dav.Client, col discovery.Collection, trace *discovery.Trace) error {
	name := testUID() + ".txt"
	path := strings.TrimSuffix(col.Path, "/") + "/" + name
	body := marker + "\n"
	cleanup := func() { _ = client.Delete(ctx, path, "") }
	defer cleanup()

	etag, err := client.PutOpts(ctx, path, strings.NewReader(body), dav.PutOptions{
		ContentType: "text/plain",
		IfNoneMatch: true,
	})
	if err != nil {
		return stepFail(trace, "files_create", path, err)
	}
	stepOK(trace, "files_create", path, http.StatusCreated)

	if err := expectPreconditionFailed(ctx, client, path, body, dav.PutOptions{
		ContentType: "text/plain",
		IfNoneMatch: true,
	}); err != nil {
		return stepFail(trace, "files_create_guard", path, err)
	}
	stepExpected412(trace, "files_create_guard", path)

	rc, _, err := client.Get(ctx, path, nil)
	if err != nil {
		return stepFail(trace, "files_get", path, err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return stepFail(trace, "files_get", path, err)
	}
	if string(got) != body {
		return stepFail(trace, "files_get", path, fmt.Errorf("GET body = %q, want %q", got, body))
	}
	stepOK(trace, "files_get", path, http.StatusOK)

	if err := client.Delete(ctx, path, etag); err != nil {
		return stepFail(trace, "files_delete", path, err)
	}
	stepOK(trace, "files_delete", path, http.StatusNoContent)
	cleanup = func() {}
	return nil
}

func pickCalendarComponent(supported []string) string {
	set := make(map[string]bool, len(supported))
	for _, name := range supported {
		set[strings.ToUpper(strings.TrimSpace(name))] = true
	}
	for _, want := range []string{ical.CompEvent, ical.CompToDo, ical.CompJournal} {
		if len(set) == 0 || set[want] {
			return want
		}
	}
	return ical.CompEvent
}

func calendarBody(component, uid string) string {
	now := time.Now().UTC()
	stamp := now.Format("20060102T150405Z")
	var extra string
	if component == ical.CompEvent {
		start := now.Truncate(time.Minute).Format("20060102T150405Z")
		end := now.Add(time.Hour).Truncate(time.Minute).Format("20060102T150405Z")
		extra = "DTSTART:" + start + "\r\nDTEND:" + end + "\r\n"
	}
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//Exercise//EN\r\n" +
		"BEGIN:" + component + "\r\nUID:" + uid + "\r\nDTSTAMP:" + stamp + "\r\n" +
		extra + "SUMMARY:" + marker + "\r\nEND:" + component + "\r\nEND:VCALENDAR\r\n"
}

func vcardBody(uid, fn string) string {
	return "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + uid + "\r\nFN:" + fn + "\r\nEND:VCARD\r\n"
}

func testUID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%s", marker, hex.EncodeToString(b[:]))
}

func expectPreconditionFailed(ctx context.Context, client *dav.Client, path, body string, opts dav.PutOptions) error {
	_, err := client.PutOpts(ctx, path, strings.NewReader(body), opts)
	if err == nil {
		return errors.New("expected HTTP 412 Precondition Failed")
	}
	if !dav.IsPreconditionFailed(err) {
		return err
	}
	return nil
}

func reportContains(ms *dav.MultiStatus, path, uid string) bool {
	if ms == nil {
		return false
	}
	for _, resp := range ms.Responses {
		gotPath, err := resp.Path()
		if err != nil || gotPath != path {
			continue
		}
		var data dav.CalendarData
		if err := resp.DecodeProp(&data); err != nil {
			continue
		}
		if strings.Contains(data.Data, uid) {
			return true
		}
	}
	return false
}

func reportContainsVCard(ms *dav.MultiStatus, path, uid string) bool {
	if ms == nil {
		return false
	}
	for _, resp := range ms.Responses {
		gotPath, err := resp.Path()
		if err != nil || gotPath != path {
			continue
		}
		var data dav.AddressData
		if err := resp.DecodeProp(&data); err != nil {
			continue
		}
		if strings.Contains(data.Data, uid) {
			return true
		}
	}
	return false
}

func stepOK(trace *discovery.Trace, name, target string, code int) {
	trace.Add(name, "ok", code, target)
}

func stepExpected412(trace *discovery.Trace, name, target string) {
	trace.Add(name, "412 as expected", 0, target)
}

func stepFail(trace *discovery.Trace, name, target string, err error) error {
	code := dav.StatusCode(err)
	detail := err.Error()
	if code == 0 {
		detail = err.Error()
	}
	trace.Add(name, detail, code, target)
	return fmt.Errorf("%s: %w", name, err)
}
