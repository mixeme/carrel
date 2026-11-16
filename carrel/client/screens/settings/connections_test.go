package settings_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/client/screens/settings"
	"carrel.dev/carrel/server/view"
)

// Objects of a storage while its tree is walked is how many files so far in
// the badge of work under way, the words in its hint (s-profile-37, the
// owner's decision of 16.11.2026); a walk refused mid-way leaves a dash in
// the failure's colour with why in its hint (the owner's decision of
// 13.11.2026).
func TestObjectsOfAStorageWhileCountedAndRefused(t *testing.T) {
	n := 1240
	page := &view.ConnectionsPage{Accounts: []view.ConnectionCard{{
		Title: "sftpgo · vps",
		Collections: []view.CollectionRow{
			{Label: "Documents", Kinds: []view.Kind{view.KindFile}, Objects: &n, Counting: true},
			{Label: "Public", Kinds: []view.Kind{view.KindFile},
				ObjectsNote: "Counting stopped: the server refused to list a folder in it — PROPFIND /dav/Public/x/ → 503 Service Unavailable."},
			{Label: "Archive", Kinds: []view.Kind{view.KindFile}, Counting: true},
		},
	}}}
	html := settings.Connections(page).Render()
	if !strings.Contains(html, `<span class="tc-badge is-progress" title="Counting: 1240 so far.">1240</span>`) {
		t.Errorf("the walked storage does not say «1240» in the badge of work under way:\n%s", html)
	}
	if !strings.Contains(html, `<span class="tc-badge is-progress" title="Counting: 0 so far.">0</span>`) {
		t.Errorf("the storage waiting for the walk does not say «0» in the badge of work under way:\n%s", html)
	}
	if !strings.Contains(html, `<span class="tc-dim--danger" title="Counting stopped: the server refused to list a folder in it — PROPFIND /dav/Public/x/ → 503 Service Unavailable.">—</span>`) {
		t.Errorf("the refused storage does not say why in the failure's colour:\n%s", html)
	}
}
