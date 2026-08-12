// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"strings"
	"testing"
)

const sampleMultiStatus = `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/dav.php/principals/mix/</d:href>
    <d:propstat>
      <d:prop>
        <d:current-user-principal><d:href>/dav.php/principals/mix/</d:href></d:current-user-principal>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`

func TestParseMultiStatus(t *testing.T) {
	ms, err := ParseMultiStatus(strings.NewReader(sampleMultiStatus))
	if err != nil {
		t.Fatalf("ParseMultiStatus: %v", err)
	}
	if len(ms.Responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(ms.Responses))
	}
	var principal CurrentUserPrincipal
	if err := ms.Responses[0].DecodeProp(&principal); err != nil {
		t.Fatalf("DecodeProp: %v", err)
	}
	if principal.Href.Path != "/dav.php/principals/mix/" {
		t.Fatalf("principal path = %q", principal.Href.Path)
	}
}
