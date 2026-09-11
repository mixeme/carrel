package crypt

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKDFIndependent(t *testing.T) {
	password := "hunter2"
	loginSalt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	wrapSalt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(loginSalt, wrapSalt) {
		t.Fatal("salts should differ in this test")
	}

	loginHash, _, err := HashLogin(context.Background(), password, loginSalt)
	if err != nil {
		t.Fatal(err)
	}
	wrapKey, _, err := DeriveWrapKey(context.Background(), password, wrapSalt)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(loginHash, wrapKey) {
		t.Fatal("login hash and wrap key must not be equal")
	}
	alt, err := DeriveKey(context.Background(), password, loginSalt, WrapKDF)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(alt, wrapKey) {
		t.Fatal("wrap salt must be required for wrap key")
	}
}

func TestPasswordChangePreservesDataKey(t *testing.T) {
	login := "alice"
	oldPass := "old-secret"
	newPass := "new-secret"

	wrapSalt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	oldWrapBytes, _, err := DeriveWrapKey(context.Background(), oldPass, wrapSalt)
	if err != nil {
		t.Fatal(err)
	}
	var oldWrap, newWrap [KeyLen]byte
	copy(oldWrap[:], oldWrapBytes)
	newWrapBytes, _, err := DeriveWrapKey(context.Background(), newPass, wrapSalt)
	if err != nil {
		t.Fatal(err)
	}
	copy(newWrap[:], newWrapBytes)

	dataKey, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("connections and duplicate decisions")

	wrapped, err := WrapDataKey(oldWrap, dataKey, login)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := EncryptBlob(dataKey, plaintext, login)
	if err != nil {
		t.Fatal(err)
	}

	rewrapped, err := RewrapDataKey(oldWrap, newWrap, wrapped, login)
	if err != nil {
		t.Fatal(err)
	}
	gotKey, err := UnwrapDataKey(newWrap, rewrapped, login)
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != dataKey {
		t.Fatal("data key changed after password change")
	}
	gotPlain, err := DecryptBlob(gotKey, blob, login)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPlain, plaintext) {
		t.Fatal("blob unreadable after password change")
	}
}

func TestWipeMakesBlobUnreadable(t *testing.T) {
	login := "bob"
	wrapSalt, _ := NewSalt()
	wrapBytes, _, err := DeriveWrapKey(context.Background(), "pass", wrapSalt)
	if err != nil {
		t.Fatal(err)
	}
	var wrap [KeyLen]byte
	copy(wrap[:], wrapBytes)

	dataKey, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := EncryptBlob(dataKey, []byte("secret state"), login)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := WrapDataKey(wrap, dataKey, login)
	if err != nil {
		t.Fatal(err)
	}

	Wipe(wrapped)
	if _, err := UnwrapDataKey(wrap, wrapped, login); err == nil {
		t.Fatal("wiped wrap should not unwrap")
	}
	if _, err := DecryptBlob(dataKey, blob, login); err != nil {
		t.Fatal("blob still readable with data key after wipe of wrap only")
	}

	Wipe(blob)
	if _, err := DecryptBlob(dataKey, blob, login); err == nil {
		t.Fatal("wiped blob should not decrypt")
	}
}

func TestAADPreventsSubstitution(t *testing.T) {
	key, _ := NewDataKey()
	plain := []byte("payload")

	wrapCT, err := WrapDataKey(key, key, "alice")
	if err != nil {
		t.Fatal(err)
	}
	blobCT, err := EncryptBlob(key, plain, "alice")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := UnwrapDataKey(key, blobCT, "alice"); err == nil {
		t.Fatal("blob ciphertext must not unwrap as wrap key")
	}
	if _, err := DecryptBlob(key, wrapCT, "alice"); err == nil {
		t.Fatal("wrap ciphertext must not decrypt as blob")
	}
	if _, err := DecryptBlob(key, blobCT, "bob"); err == nil {
		t.Fatal("blob ciphertext must not decrypt under another login AAD")
	}

	stateCT, err := EncryptState(key, plain, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptState(key, stateCT, 2); err == nil {
		t.Fatal("state ciphertext must not decrypt under another version AAD")
	}
}

func TestServerKeyOEXCL(t *testing.T) {
	dir := t.TempDir()
	k1, err := CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreateServerKey(dir)
	if !errors.Is(err, ErrServerKeyExists) {
		t.Fatalf("second create: got %v, want ErrServerKeyExists", err)
	}
	k2, err := LoadServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatal("loaded key differs from created key")
	}
	info, err := os.Stat(filepath.Join(dir, ServerKey))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
		}
	}
}

func TestServerKeyShortRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ServerKey), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServerKey(dir)
	if !errors.Is(err, ErrServerKeyShort) {
		t.Fatalf("got %v, want ErrServerKeyShort", err)
	}
}

func TestLoadOrCreateServerKey(t *testing.T) {
	dir := t.TempDir()
	k1, err := LoadOrCreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := LoadOrCreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatal("second load should return same key")
	}
}

func TestLoadServerKeyRejectsPermissiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode check is skipped on Windows")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ServerKey), bytes.Repeat([]byte{0xab}, KeyLen), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServerKey(dir)
	if !errors.Is(err, ErrServerKeyPermissive) {
		t.Fatalf("got %v, want ErrServerKeyPermissive", err)
	}
}

func TestValidateKDFParams(t *testing.T) {
	if err := ValidateKDFParams(KDFParams{}, false); err != nil {
		t.Fatalf("zero params without secrets: %v", err)
	}
	if err := ValidateKDFParams(KDFParams{}, true); err == nil {
		t.Fatal("zero params with secrets should fail")
	}
	if err := ValidateKDFParams(LoginKDF, true); err != nil {
		t.Fatalf("login defaults: %v", err)
	}
	bad := LoginKDF
	bad.Time = 0
	if err := ValidateKDFParams(bad, true); err == nil {
		t.Fatal("Time=0 should fail")
	}
}
