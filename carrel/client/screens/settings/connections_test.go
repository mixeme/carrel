package settings_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/client/screens/settings"
	"carrel.dev/carrel/server/view"
)

// Objects of a storage while its tree is walked says how many files so far
// and that it goes on, «1240…»; a walk refused mid-way leaves a dash with
// why in its hint (the owner's decision of 13.11.2026).
func TestObjectsOfAStorageWhileCountedAndRefused(t *testing.T) {
	n := 1240
	page := &view.ConnectionsPage{Accounts: []view.ConnectionCard{{
		Title: "sftpgo · vps",
		Collections: []view.CollectionRow{
			{Label: "Documents", Kinds: []view.Kind{view.KindFile}, Objects: &n, Counting: true},
			{Label: "Public", Kinds: []view.Kind{view.KindFile},
				ObjectsNote: "Not counted: PROPFIND /dav/Public/x/ → 503 Service Unavailable."},
		},
	}}}
	html := settings.Connections(page).Render()
	if !strings.Contains(html, ">1240…<") {
		t.Errorf("the walked storage does not say «1240…»:\n%s", html)
	}
	if !strings.Contains(html, `title="Not counted: PROPFIND /dav/Public/x/ → 503 Service Unavailable."`) {
		t.Errorf("the refused storage does not say why:\n%s", html)
	}
}
