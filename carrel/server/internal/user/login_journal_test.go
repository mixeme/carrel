package user

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/store"
)

// The journal of S-10.6 shows sign-ins (frame s-admin-7): a successful one
// as login.ok, a wrong password on an account that exists as login.fail.
// An unknown login writes nothing — the journal never keeps what somebody
// typed — and neither does a disabled account: the answer to the client is
// one for all three (S-29.1), the journal names accounts, not attempts at
// names. By the owner's decision of 16.10.2026 (Ад38; s-admin-7) a refused
// sign-in has no actor («—»), its address goes in, and the attempts in a
// row on one account are one line with their count — a successful one
// keeps no address: where a person usually is is nobody's business.
func TestSignInIsJournaled(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	ctx := context.Background()
	last := func() store.AuditEntry {
		a := f.st.Snapshot().Audit
		return a[len(a)-1]
	}
	count := func() int { return len(f.st.Snapshot().Audit) }

	if _, _, _, ok := f.svc.Authenticate(ctx, "Admin ", "password123"); !ok {
		t.Fatal("sign-in failed")
	}
	if e := last(); e.What != "login.ok" || e.Who != "admin" || e.Whom != "admin" || e.Detail != "" {
		t.Errorf("after a sign-in: %+v", e)
	}
	n := count()
	if _, _, _, ok := f.svc.Authenticate(ctx, "admin", "wrong"); ok {
		t.Fatal("a wrong password signed in")
	}
	if e := last(); count() != n+1 || e.What != "login.fail" || e.Who != "—" || e.Whom != "admin" || e.Detail != "1st attempt" {
		t.Errorf("after a wrong password: %+v (entries %d → %d)", e, n, count())
	}
	// The next ones in a row, from an address: the same line, counted.
	from := WithClientIP(ctx, "203.0.113.9")
	f.svc.Authenticate(from, "admin", "wrong")
	f.svc.Authenticate(from, "admin", "wrong")
	if e := last(); count() != n+1 || e.Detail != "3rd attempt · ip 203.0.113.9" {
		t.Errorf("a series: %+v (entries %d → %d)", e, n, count())
	}
	f.svc.LoginLimited("admin", "203.0.113.9")
	if e := last(); count() != n+1 || e.Detail != "4th attempt · ip 203.0.113.9 · rate-limited" {
		t.Errorf("over the limit: %+v", e)
	}
	// A successful sign-in ends the series.
	f.svc.Authenticate(ctx, "admin", "password123")
	f.svc.Authenticate(ctx, "admin", "wrong")
	if e := last(); e.Detail != "1st attempt" {
		t.Errorf("after a sign-in, a new series: %+v", e)
	}
	n = count()
	f.svc.Authenticate(ctx, "nobody' OR 1=1", "wrong")
	if count() != n {
		t.Errorf("an unknown login was journaled: %+v", last())
	}
	_ = f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(id).Disabled = true
		return nil, nil
	})
	n = count()
	f.svc.Authenticate(ctx, "admin", "password123")
	if count() != n {
		t.Errorf("a disabled account's attempt was journaled: %+v", last())
	}
}
