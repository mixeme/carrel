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
// names. No address goes in: the journal is kept for good, and the sessions
// list already shows where a live session came from (audit №3, Ф3-7).
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
	if e := last(); count() != n+1 || e.What != "login.fail" || e.Who != "admin" || e.Whom != "admin" || e.Detail != "" {
		t.Errorf("after a wrong password: %+v (entries %d → %d)", e, n, count())
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
