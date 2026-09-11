package store

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/crypt"
)

func TestRewrapLoginChangesAAD(t *testing.T) {
	ctx := context.Background()
	login := "alice"
	newLogin := "alicia"
	password := "password123"

	u := &User{Login: login}
	if err := ApplyPassword(ctx, u, login, password); err != nil {
		t.Fatal(err)
	}
	dataKey, ok := DataKeyForLogin(ctx, u, password)
	if !ok {
		t.Fatal("data key")
	}
	plain := []byte(`{"connections":[{"id":"c1","server":"https://x/"}]}`)
	blob, err := crypt.EncryptBlob(dataKey, plain, login)
	if err != nil {
		t.Fatal(err)
	}
	u.SealedBlob = blob

	wrapKey, _, err := crypt.DeriveWrapKey(ctx, password, u.WrapSalt)
	if err != nil {
		t.Fatal(err)
	}
	var wrap [crypt.KeyLen]byte
	copy(wrap[:], wrapKey)
	crypt.Wipe(wrapKey)

	if err := RewrapLogin(u, newLogin, wrap, dataKey); err != nil {
		t.Fatal(err)
	}
	if u.Login != newLogin {
		t.Fatalf("login = %q", u.Login)
	}
	if _, err := crypt.DecryptBlob(dataKey, u.SealedBlob, login); err == nil {
		t.Fatal("old login AAD should not decrypt blob")
	}
	got, err := crypt.DecryptBlob(dataKey, u.SealedBlob, newLogin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("blob = %q", got)
	}
	if _, err := crypt.UnwrapDataKey(wrap, u.WrappedKey, login); err == nil {
		t.Fatal("old login AAD should not unwrap key")
	}
	if _, err := crypt.UnwrapDataKey(wrap, u.WrappedKey, newLogin); err != nil {
		t.Fatal(err)
	}
}
