package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The move sheet is a form, as s-contacts-2 draws it: the collection is a
// radio under Target.Field, its value «connection/collection», and Move the
// form's button. The form carries the word the sheet said (warned); a card
// to another account needs none (Р-77), so the press moves it.
func TestTheMoveSheetIsAForm(t *testing.T) {
	f := newMoveFixture(t)
	ada := f.record(f.id, peopleBook, "ada")
	sheet := decode[view.MoveSheet](t, f.h.do(get(ada+"/move", f.sess)))
	form := formFields(sheet.Form)
	if form.Get("warned") != "nothing" || sheet.Form.Action != ada+"/move" {
		t.Fatalf("form = %s %v", sheet.Form.Action, form)
	}
	work := moveOption(t, sheet, "Work")
	if !strings.HasPrefix(work.Value, f.otherID+"/") || moveOption(t, sheet, "People").Here != true || moveOption(t, sheet, "Archive").Here {
		t.Fatalf("options = %+v", sheet.Target.Options)
	}
	form.Set(sheet.Target.Field, work.Value)
	if resp := f.h.do(postForm(sheet.Form.Action, f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("move to another account: %d %s", resp.Code, resp.Body.String())
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("the card is still here")
	}
}
