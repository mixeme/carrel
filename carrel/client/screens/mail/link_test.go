package mail

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The link letter has the same sending feedback as the full letter
// (Typecase's note 148, s-mail-4).
func TestLinkSendSaysItIsSending(t *testing.T) {
	p := &view.MailLinkSheet{Form: view.Form{
		Action: "/app/mail/link",
		Actions: []view.Action{{ID: sendID, Label: "Send", Enabled: true, Href: "/app/mail/link"}},
	}}
	if html := LinkSheet(p).Render(); !strings.Contains(html, `data-tc-busy="Sending…"`) {
		t.Fatal("the link letter's Send has no sending feedback")
	}
}
