package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// s-contacts-7, the client's stop C16: the card in the timeline's panel has
// its cross, and without scripts the cross is a link — the same timeline,
// the same range and kind, without the panel; the screen keeps its sense
// («закрыли — экран не изменил смысла»). Any other link of the screen opens
// the panel again.
func TestTimelineCardCloses(t *testing.T) {
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-7"), "<figcaption")
	_, side, ok := strings.Cut(frame, `<div class="tc-side">`)
	if !ok || !strings.Contains(side, `tc-close`) {
		t.Fatal("s-contacts-7 draws no cross on the panel: read the frame again")
	}
	app := newMockupApp(t)
	tl := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7/timeline"
	page := decode[view.ContactTimelinePage](t, app.h.do(get(tl+"?kind=event", app.sess)))
	if page.CardClosed || page.Card.Close == "" || !strings.HasPrefix(page.Card.Close, tl+"?") || !strings.Contains(page.Card.Close, "kind=event") {
		t.Fatalf("the card's cross = %q, closed %v", page.Card.Close, page.CardClosed)
	}
	closed := decode[view.ContactTimelinePage](t, app.h.do(get(page.Card.Close, app.sess)))
	if !closed.CardClosed || closed.Card.Title != "" || closed.Name != page.Name || len(closed.Bar.Segments) == 0 {
		t.Errorf("closed: %v, card %q, name %q", closed.CardClosed, closed.Card.Title, closed.Name)
	}
	for _, s := range closed.Bar.Segments {
		if strings.Contains(s.Href, "card=") {
			t.Errorf("%s keeps the panel closed: %s", s.ID, s.Href)
		}
	}
}
