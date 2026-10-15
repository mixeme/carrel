package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The move sheet is a form, as s-contacts-2 draws it: the collection is a
// radio under Target.Field, its value «connection/collection», and Move the
// form's button. A collection whose move needs another word than the sheet
// said — another account, where the sheet spoke of one server — is not
// moved: the sheet comes back with it chosen and its word said (S-18:
// before it starts). The word it was shown with moves it.
func TestTheMoveSheetIsAForm(t *testing.T) {
	f := newMoveFixture(t)
	ada := f.record(f.id, peopleBook, "ada")
	sheet := decode[view.MoveSheet](t, f.h.do(get(ada+"/move", f.sess)))
	form := formFields(sheet.Form)
	if form.Get("warned") != "server" || sheet.Form.Action != ada+"/move" {
		t.Fatalf("form = %s %v", sheet.Form.Action, form)
	}
	work := moveOption(t, sheet, "Work")
	if !strings.HasPrefix(work.Value, f.otherID+"/") || moveOption(t, sheet, "People").Here != true || moveOption(t, sheet, "Archive").Here {
		t.Fatalf("options = %+v", sheet.Target.Options)
	}
	form.Set(sheet.Target.Field, work.Value)
	again := decode[view.MoveSheet](t, f.h.do(postForm(sheet.Form.Action, f.sess, form)))
	if again.Target.Chosen != work.ID || !strings.Contains(plainText(again.Warning), "another account") || formFields(again.Form).Get("warned") != "account" {
		t.Fatalf("another account named after the press: %+v", again)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); !ok {
		t.Fatal("moved before the sheet named another account")
	}
	confirm := formFields(again.Form)
	confirm.Set(again.Target.Field, work.Value)
	if resp := f.h.do(postForm(again.Form.Action, f.sess, confirm)); resp.Code != http.StatusSeeOther {
		t.Fatalf("move as said: %d %s", resp.Code, resp.Body.String())
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("the card is still here")
	}
}
