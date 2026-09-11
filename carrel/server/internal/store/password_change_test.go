package store

import (
	"context"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
)

func TestChangePasswordPreservesDataKey(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	st := &State{}
	u := &User{ID: "u-test000001", Login: "alice", CreatedAt: now}
	if err := ApplyPassword(ctx, u, "alice", "oldpass123", st, now); err != nil {
		t.Fatal(err)
	}
	plain := sealed.Blob{Connections: []sealed.Connection{{ID: "a-test000001", Server: "https://dav.example.com"}}}
	dataKey, ok := DataKeyForLogin(ctx, u, "oldpass123")
	if !ok {
		t.Fatal("data key")
	}
	blob, err := sealed.Seal(dataKey, plain, u.Login)
	if err != nil {
		t.Fatal(err)
	}
	u.SealedBlob = blob

	if err := ChangePassword(ctx, u, u.Login, "oldpass123", "newpass1234"); err != nil {
		t.Fatal(err)
	}
	if u.MustChangePassword {
		t.Fatal("must-change should be cleared")
	}
	if _, ok := DataKeyForLogin(ctx, u, "oldpass123"); ok {
		t.Fatal("old password should not work")
	}
	newKey, ok := DataKeyForLogin(ctx, u, "newpass1234")
	if !ok {
		t.Fatal("new password should work")
	}
	if newKey != dataKey {
		t.Fatal("data key changed")
	}
	got, err := sealed.Open(newKey, u.SealedBlob, u.Login)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Connections) != 1 || got.Connections[0].ID != "a-test000001" {
		t.Fatal("sealed blob unreadable after password change")
	}
}
