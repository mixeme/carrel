package user

import (
	"context"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// S-25: a link lives until its expiry and not a moment longer; one made
// without an expiry lives on; a disabled owner's links die with the account;
// an envelope is bound to its own link and opens for no other.
func TestPublishedLinkLifetime(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	_, key, _, _ := f.svc.Authenticate(context.Background(), "admin", "password123")
	conn := sealed.Connection{ID: "a1", Server: "https://dav.example.org/", Login: "ann", Password: "pw"}
	target := PublishTarget{AccountID: "a1", Kind: "contact", Col: "/book/", Title: "Book"}

	until := f.now.Add(publish.DefaultTTL)
	dated, err := f.svc.Publish(id, key, conn, target, &until)
	if err != nil {
		t.Fatal(err)
	}
	forever, _ := f.svc.Publish(id, key, conn, target, nil)
	if l, ok := f.svc.OpenPublished(dated); !ok || l.Conn.Password != "pw" || l.Target.Col != "/book/" {
		t.Fatalf("a live link: %+v %v", l, ok)
	}

	f.advance(publish.DefaultTTL)
	if _, ok := f.svc.OpenPublished(dated); ok {
		t.Error("an expired link opened")
	}
	if _, ok := f.svc.OpenPublished(forever); !ok {
		t.Error("a link without an expiry died")
	}
	if n := len(f.svc.PublishedLinks(id)); n != 1 {
		t.Errorf("listed links = %d, want the one without an expiry", n)
	}

	// Swap the two envelopes: each is bound to its own hash.
	another, _ := f.svc.Publish(id, key, sealed.Connection{Server: "https://other.example/", Login: "x", Password: "other"}, target, nil)
	_ = f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		n := len(st.Published)
		st.Published[n-1].Envelope, st.Published[n-2].Envelope = st.Published[n-2].Envelope, st.Published[n-1].Envelope
		return nil, nil
	})
	if _, ok := f.svc.OpenPublished(another); ok {
		t.Error("an envelope opened under another link")
	}

	_ = f.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(id).Disabled = true
		return nil, nil
	})
	f.advance(time.Minute)
	if _, ok := f.svc.OpenPublished(forever); ok {
		t.Error("a disabled account's link opened")
	}
}

// s-publish-1: the list counts every open of a link — the written ones and
// those since, in memory — and a check that a link lives is not an open.
func TestPublishedLinkCountsOpens(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	_, key, _, _ := f.svc.Authenticate(context.Background(), "admin", "password123")
	conn := sealed.Connection{ID: "a1", Server: "https://dav.example.org/", Login: "ann", Password: "pw"}
	token, err := f.svc.Publish(id, key, conn, PublishTarget{AccountID: "a1", Kind: "contact", Col: "/book/", Title: "Book"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	opens := func() (listed, stored int) {
		ls := f.svc.PublishedLinks(id)
		if len(ls) != 1 {
			t.Fatalf("links = %d", len(ls))
		}
		return ls[0].Opens, f.st.Snapshot().Published[0].Opens
	}
	if n, _ := opens(); n != 0 {
		t.Errorf("a new link shows %d opens", n)
	}
	for i := 0; i < 3; i++ {
		f.svc.OpenPublished(token)
	}
	f.svc.PublicLinkValid(context.Background(), token)
	if n, stored := opens(); n != 3 || stored != 1 {
		t.Errorf("after three opens and a check: listed %d, stored %d; want 3 and the first", n, stored)
	}
	f.advance(lastSeenEvery)
	f.svc.OpenPublished(token)
	if n, stored := opens(); n != 4 || stored != 4 {
		t.Errorf("after the hour: listed %d, stored %d; want 4 and 4", n, stored)
	}
}
