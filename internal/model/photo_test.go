// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
)

func TestPhotoValueKeepsVCard30(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	obj, err := NewVCard("3.0", "uid-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Apply((&Patch{}).Set("PHOTO", PhotoValue(obj.Version(), jpeg))); err != nil {
		t.Fatal(err)
	}
	if obj.Version() != "3.0" {
		t.Fatalf("version = %s", obj.Version())
	}
	out, err := obj.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "ENCODING=b") && !strings.Contains(body, "ENCODING=B") {
		t.Fatalf("expected vCard 3.0 PHOTO encoding:\n%s", body)
	}
	inline, ok, err := obj.ExtractPhoto()
	if err != nil || !ok {
		t.Fatalf("extract: ok=%v err=%v", ok, err)
	}
	if string(inline.Bytes) != string(jpeg) {
		t.Fatalf("bytes = %v", inline.Bytes)
	}
}
