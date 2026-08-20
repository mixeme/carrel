// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMkColPropsCalendar(t *testing.T) {
	var method, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	g := NewGuard(GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := NewClient(g, srv.URL, "mix", "secret")
	if err != nil {
		t.Fatal(err)
	}
	err = client.MkColProps(context.Background(), "MKCALENDAR", "/calendars/mix/trips/", []ColProp{
		{Name: DisplayNameName, Value: "Trips"},
		{Name: CalendarColorName, Value: "#4A6B52"},
		{Name: SupportedCalendarComponentSetName, Value: `<C:comp name="VEVENT"/>`},
	})
	if err != nil {
		t.Fatalf("MkColProps: %v", err)
	}
	if method != "MKCALENDAR" {
		t.Fatalf("method = %q", method)
	}
	if !strings.Contains(body, "Trips") || !strings.Contains(body, "calendar-color") {
		t.Fatalf("body = %q", body)
	}
}

func TestPropPatchDisplayName(t *testing.T) {
	var method, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()

	g := NewGuard(GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := NewClient(g, srv.URL, "mix", "secret")
	if err != nil {
		t.Fatal(err)
	}
	err = client.PropPatch(context.Background(), "/calendars/mix/trips/", []ColProp{
		{Name: DisplayNameName, Value: "Birthdays"},
	}, nil)
	if err != nil {
		t.Fatalf("PropPatch: %v", err)
	}
	if method != "PROPPATCH" {
		t.Fatalf("method = %q", method)
	}
	if !strings.Contains(body, "Birthdays") {
		t.Fatalf("body = %q", body)
	}
}

func TestWrapRequestError403(t *testing.T) {
	err := WrapRequestError("MKCALENDAR", "/c/x/", &HTTPError{
		Code: http.StatusForbidden,
		Err:  fmtError(`<D:error><D:need-privileges/></D:error>`),
	})
	var reqErr *RequestError
	if !errors.As(err, &reqErr) {
		t.Fatal("expected RequestError")
	}
	if reqErr.Code != 403 {
		t.Fatalf("code = %d", reqErr.Code)
	}
}

func fmtError(s string) error { return &plainErr{s} }

type plainErr struct{ s string }

func (e *plainErr) Error() string { return e.s }
