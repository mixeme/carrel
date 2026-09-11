package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/web"
)

func testServer(t *testing.T) (*web.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	srv := web.New(cfg, st, mgr, web.NewStubUsers(st), web.JSONRenderer{}, "test", "test")
	return srv, st
}

func TestHealthz(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSetupRedirectWhenFirstRun(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want redirect to setup", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/setup") {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
}

func TestForeignAccountNotFound(t *testing.T) {
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	err = st.Mutate(func(s *store.State) ([]store.AuditEntry, error) {
		s.Users = append(s.Users, store.User{
			ID: "a-testuser01", Login: "alice", Role: store.RoleUser,
		})
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	dataKey, _ := crypt.NewDataKey()
	s, err := mgr.Create("a-testuser01", "alice", dataKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := web.New(cfg, st, mgr, web.NewStubUsers(st), web.JSONRenderer{}, "t", "c")
	req := httptest.NewRequest(http.MethodGet, "/app/contacts/a-other/col", nil)
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: s.ID()})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign account: status = %d, want 404", rec.Code)
	}
}

func mustConfig(t *testing.T) *conf.Config {
	t.Helper()
	c, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCSRFRequiredForMutatingSessionRequest(t *testing.T) {
	srv, st := testServer(t)
	dataKey, _ := crypt.NewDataKey()
	mgr := session.NewManager(mustConfig(t))
	sess, err := mgr.Create("a-testuser01", "alice", dataKey)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Mutate(func(s *store.State) ([]store.AuditEntry, error) {
		s.Users = append(s.Users, store.User{ID: "a-testuser01", Login: "alice", Role: store.RoleAdmin})
		return nil, nil
	})
	cfg, _ := conf.Load()
	srv = web.New(cfg, st, mgr, web.NewStubUsers(st), web.JSONRenderer{}, "t", "c")
	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: status = %d, want 403", rec.Code)
	}
}

func TestFragmentHeader(t *testing.T) {
	srv, st := testServer(t)
	dataKey, _ := crypt.NewDataKey()
	mgr := session.NewManager(mustConfig(t))
	_ = st.Mutate(func(s *store.State) ([]store.AuditEntry, error) {
		s.Users = append(s.Users, store.User{ID: "a-testuser01", Login: "alice", Role: store.RoleUser})
		return nil, nil
	})
	sess, _ := mgr.Create("a-testuser01", "alice", dataKey)
	cfg, _ := conf.Load()
	srv = web.New(cfg, st, mgr, web.NewStubUsers(st), web.JSONRenderer{}, "t", "c")
	req := httptest.NewRequest(http.MethodGet, "/app/contacts", nil)
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "main")
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"target":"main"`) {
		t.Fatalf("expected fragment envelope, got %s", rec.Body.String())
	}
}

func TestUnknownRouteNotFound(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
