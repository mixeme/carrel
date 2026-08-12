// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestParseVCardsMultipleAndCyrillic(t *testing.T) {
	body := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:one\r\nFN:Ada\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:two\r\nFN:Ада\r\nEND:VCARD\r\n")
	cards := ParseVCards(body)
	if len(cards) != 2 {
		t.Fatalf("len = %d", len(cards))
	}
	if cards[0].Object.UID() != "one" || cards[1].Object.UID() != "two" {
		t.Fatalf("uids = %q %q", cards[0].Object.UID(), cards[1].Object.UID())
	}
	c, err := cards[1].Object.Contact()
	if err != nil {
		t.Fatal(err)
	}
	if c.FormattedName != "Ада" {
		t.Fatalf("FN = %q", c.FormattedName)
	}
}

func TestParseVCardsSkipsBadCard(t *testing.T) {
	body := []byte("BEGIN:VCARD\r\nUID:no-version\r\nFN:Bad\r\nEND:VCARD\r\n" +
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ok\r\nFN:Good\r\nEND:VCARD\r\n")
	cards := ParseVCards(body)
	if len(cards) != 2 {
		t.Fatalf("len = %d", len(cards))
	}
	if cards[0].Error == "" || cards[0].Object != nil {
		t.Fatalf("first should be error: %+v", cards[0])
	}
	if cards[1].Object == nil || cards[1].Object.UID() != "ok" {
		t.Fatalf("second = %+v", cards[1])
	}
}

func TestAssignUID(t *testing.T) {
	obj, err := NewVCard("3.0", "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.AssignUID("new-id"); err != nil {
		t.Fatal(err)
	}
	if obj.UID() != "new-id" {
		t.Fatalf("UID = %q", obj.UID())
	}
}

func TestReadImportPayloadZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("book/alice.vcf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:alice\r\nFN:Alice\r\nEND:VCARD\r\n"))
	w, err = zw.Create("book/bob.vcf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:bob\r\nFN:Bob\r\nEND:VCARD\r\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	cards, err := ReadImportPayload("contacts.zip", buf.Bytes(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len = %d", len(cards))
	}
	names := map[string]bool{}
	for _, c := range cards {
		if c.Error != "" {
			t.Fatalf("parse error: %s", c.Error)
		}
		names[c.Object.UID()] = true
		if !strings.Contains(c.Source, ".vcf") {
			t.Fatalf("source = %q", c.Source)
		}
	}
	if !names["alice"] || !names["bob"] {
		t.Fatalf("uids = %v", names)
	}
}
