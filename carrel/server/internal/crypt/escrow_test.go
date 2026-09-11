package crypt

import (
	"bytes"
	"context"
	"testing"
)

func TestEscrowDepositRoundTrip(t *testing.T) {
	pub, priv, err := GenerateEscrowKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dataKey, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	login := "alice"
	deposited, err := DepositDataKey(pub, dataKey, login)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenDepositedKey(priv, deposited, login)
	if err != nil {
		t.Fatal(err)
	}
	if got != dataKey {
		t.Fatal("deposited key mismatch")
	}
	if _, err := OpenDepositedKey(priv, deposited, "bob"); err == nil {
		t.Fatal("wrong login AAD should fail")
	}
}

func TestEscrowMasterWrapRoundTrip(t *testing.T) {
	_, priv, err := GenerateEscrowKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	master := "sixteen-char-pass!"
	wrapped, salt, p, err := WrapEscrowPrivateKey(context.Background(), master, priv)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := UnwrapEscrowPrivateKey(context.Background(), master, salt, p, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, priv[:]) {
		t.Fatal("master wrap mismatch")
	}
	Wipe(opened)
	if _, err := UnwrapEscrowPrivateKey(context.Background(), "wrong-password-here", salt, p, wrapped); err == nil {
		t.Fatal("wrong master password should fail")
	}
}
