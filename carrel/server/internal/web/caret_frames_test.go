package web_test

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The caret (FormRow.Focus): every form a frame draws with a field in focus
// — «is-focus» on a tc-in — has it in that field when the form is shown, in
// every state the frame draws; a field that came back with a reason takes
// it (s-auth-13, s-contacts-18, s-profile-11, s-admin-16, s-admin-20). The
// field is read off the frame, not copied here.

var (
	caretTag   = regexp.MustCompile(`<(input|textarea)\s[^>]*>`)
	caretClass = regexp.MustCompile(`class="([^"]*)"`)
	caretAria  = regexp.MustCompile(`aria-label="([^"]*)"`)
	caretLabel = regexp.MustCompile(`(?s)<label>(.*?)</label>`)
)

// frameCaret is the field a frame draws the caret in: the label of its
// field, else its aria-label, else «textarea» for a body without either;
// with whether it is drawn with a reason (is-error). A frame without a caret
// fails: the test that asks has read it wrong.
func frameCaret(t *testing.T, id string) (string, bool) {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	for _, at := range caretTag.FindAllStringIndex(frame, -1) {
		tag := frame[at[0]:at[1]]
		c := caretClass.FindStringSubmatch(tag)
		if c == nil {
			continue
		}
		class := " " + c[1] + " "
		if !strings.Contains(class, " tc-in ") || !strings.Contains(class, " is-focus ") {
			continue
		}
		reason := strings.Contains(class, " is-error ")
		before := frame[:at[0]]
		if i := strings.LastIndex(before, `<div class="tc-field`); i >= 0 {
			// The field the input stands in: its div is still open.
			if seg := before[i:]; strings.Count(seg, "<div") > strings.Count(seg, "</div>") {
				if l := caretLabel.FindStringSubmatch(seg); l != nil {
					return frameText(l[1]), reason
				}
			}
		}
		if a := caretAria.FindStringSubmatch(tag); a != nil {
			return html.UnescapeString(a[1]), reason
		}
		if strings.HasPrefix(tag, "<textarea") {
			return "textarea", reason
		}
		t.Fatalf("%s: the caret's field has no name: read the frame again", id)
	}
	t.Fatalf("%s draws no caret: read the frame again", id)
	return "", false
}

// sameCaret holds the caret of a form against the frame's: one field, the
// frame's, with a reason where the frame draws one.
func sameCaret(t *testing.T, id, what string, f view.Form) {
	t.Helper()
	want, reason := frameCaret(t, id)
	var got []string
	var row view.FormRow
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Focus {
				row = r
				label := string(r.Label)
				if want == "textarea" && r.Kind == view.InputArea {
					label = "textarea"
				}
				got = append(got, label)
			}
		}
	}
	if len(got) != 1 || got[0] != want || (row.Error != "") != reason {
		t.Errorf("%s: caret in %q (reason %q), want (%s) %q with a reason %v", what, got, row.Error, id, want, reason)
	}
}

// s-auth-2, s-auth-12, s-auth-4, s-auth-5, s-auth-11, account-3: the pages
// outside the shell and the password page. A refusal moves the caret to the
// field it is said at, as s-auth-13 draws it.
func TestCaretOutsideTheShellIsTheFrames(t *testing.T) {
	h := newHarness(t)
	sameCaret(t, "s-auth-4", "setup", decode[view.SetupPage](t, h.do(get("/setup", nil))).Form)
	setupRefused := decode[view.SetupPage](t, h.do(h.anonPostForm("/setup", "/setup",
		url.Values{"login": {"mix"}, "password": {"password123"}, "password2": {"password124"}}))).Form
	if r := rowNamed(t, setupRefused, "password2"); !r.Focus || r.Error == "" {
		t.Errorf("setup, the repeat differs: %+v, want the caret at the reason (s-auth-13)", r)
	}

	adminID := h.admin("mix", "password123")
	openRegistration(t, h)
	sameCaret(t, "s-auth-2", "registration", decode[view.RegisterPage](t, h.do(get("/register", nil))).Form)
	refused := decode[view.RegisterPage](t, h.do(h.anonPostForm("/register", "/register",
		url.Values{"login": {"viktor"}, "email": {"v@example.org"}, "password": {"password123"}, "password2": {"password124"}}))).Form
	if r := rowNamed(t, refused, "password2"); !r.Focus || r.Error == "" || rowNamed(t, refused, "login").Focus {
		t.Errorf("registration, the repeat differs: %+v, want the caret at the reason (s-auth-13)", r)
	}

	byLink, err := h.users.Invite(adminID, "", store.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	sameCaret(t, "s-auth-12", "invitation by link", decode[view.InvitePage](t, h.do(get("/invite/"+byLink, nil))).Form)
	byLetter, err := h.users.Invite(adminID, "anna@example.org", store.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	sameCaret(t, "s-auth-12", "invitation by letter", decode[view.InvitePage](t, h.do(get("/invite/"+byLetter, nil))).Form)
	// A login the invitation names does not change: the caret is in the
	// first field that does.
	named, err := h.users.CreateInvite(adminID, "bob", "")
	if err != nil {
		t.Fatal(err)
	}
	if f := decode[view.InvitePage](t, h.do(get("/invite/"+named, nil))).Form; !rowNamed(t, f, "password").Focus || rowNamed(t, f, "login").Focus {
		t.Errorf("an invitation naming the login: %+v", f.Sets)
	}

	sameCaret(t, "s-auth-5", "forgot", decode[view.ForgotPage](t, h.do(get("/forgot", nil))).Form)

	link, err := h.users.IssueConsoleWipeLink("mix")
	if err != nil {
		t.Fatal(err)
	}
	confirm := decode[view.ForgotWipePage](t, h.do(get(link, nil)))
	next := decode[view.ForgotWipePage](t, h.do(h.anonPostForm(link, link, formFields(confirm.Form))))
	sameCaret(t, "s-auth-11", "a new password", next.Form)
	differs := decode[view.ForgotWipePage](t, h.do(h.anonPostForm(link, link, url.Values{"password": {"password123"}, "password2": {"password124"}}))).Form
	if r := rowNamed(t, differs, "password2"); !r.Focus || r.Error == "" || rowNamed(t, differs, "password").Focus {
		t.Errorf("a new password, the repeat differs: %+v", differs.Sets)
	}

	h2 := newHarness(t)
	h2.admin("vera", "password123")
	vera := h2.login("vera", "password123")
	sameCaret(t, "account-3", "change password", decode[view.PasswordPage](t, h2.do(get("/app/password", vera))).Form)
	if err := h2.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByLogin("vera").MustChangePassword = true
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	sameCaret(t, "account-3", "a temporary password", decode[view.PasswordPage](t, h2.do(get("/app/password", vera))).Form)
	wrong := decode[view.PasswordPage](t, h2.do(postForm("/app/password", vera,
		url.Values{"current": {"wrong-one-1"}, "password": {"password456"}, "password2": {"password456"}}))).Form
	if r := rowNamed(t, wrong, "current"); !r.Focus || r.Error == "" {
		t.Errorf("a wrong current password: %+v, want the caret at the reason", r)
	}
	if _, _, _, ok := h2.users.Authenticate(context.Background(), "vera", "password123"); !ok {
		t.Fatal("a refused change changed the password")
	}
}

// s-contacts-3, s-contacts-4, s-agenda-3, s-tasks-3: the forms of a record,
// the caret in their first field; s-contacts-18: a day that is not there
// takes it.
func TestCaretInRecordFormsIsTheFrames(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:anna\nFN:Анна Ковалёва\nBDAY:1987-03-14\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bare\nFN:Анна Ковалёва\nEND:VCARD\n")
	anna := f.path("contacts") + "/anna"
	edit := decode[view.ContactEditPage](t, f.h.do(get(anna+"/edit", f.sess))).Form
	sameCaret(t, "s-contacts-3", "contact", edit)
	sameCaret(t, "s-contacts-4", "contact without a photo", decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/bare/edit", f.sess))).Form)
	sameCaret(t, "s-notes-8", "new contact", decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess))).Form)
	form := formFields(edit)
	form.Set("bday", "31.02.1987")
	sameCaret(t, "s-contacts-18", "a birthday that is not there", decode[view.ContactEditPage](t, f.h.do(postForm(anna+"/edit", f.sess, form))).Form)

	c := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000Z\nDTEND:20260810T110000Z\nSUMMARY:Sync-up\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"))
	sameCaret(t, "s-agenda-3", "event", decode[view.EventEditPage](t, c.h.do(get(c.path("calendar")+"/e1/edit", c.sess))).Form)
	sameCaret(t, "s-tasks-3", "task", decode[view.TaskEditPage](t, c.h.do(get(c.path("tasks")+"/t1/edit", c.sess))).Form)
	// The edit of a note (s-notes-20) draws the caret in its text, which is
	// NoteDoc.Body and no row: C-6 says it there. Its rows have none.
	for _, r := range decode[view.NotePage](t, c.h.do(get(c.path("notes")+"/new", c.sess))).Doc.Fields {
		if r.Focus {
			t.Errorf("a note's %q has the caret; s-notes-20 draws it in the text", r.Name)
		}
	}
}

// s-notes-5, s-notes-8, s-notes-9, s-notes-10: the quick sheets, the caret
// in their first field — the note's body.
func TestCaretInQuickSheetsIsTheFrames(t *testing.T) {
	f := contactsFixture(t, false)
	sameCaret(t, "s-notes-8", "contact quick sheet", decode[view.NewSheet](t, f.h.do(get(f.path("contacts")+"/new/quick", f.sess))).Form)
	c := calendarFixture(t, nil)
	for kind, id := range map[string]string{"calendar": "s-notes-9", "tasks": "s-notes-10", "notes": "s-notes-5"} {
		sameCaret(t, id, kind+" quick sheet", decode[view.NewSheet](t, c.h.do(get(c.path(kind)+"/new/quick", c.sess))).Form)
	}
}

// s-plan-3 and s-profile-3: the password that opens the window; s-profile-5,
// s-profile-7, s-profile-9: a new collection, its name and colour, its
// deletion; s-profile-2: Connect account; s-profile-10: a subscription;
// s-davloom-2: a device; s-backup-2: a job.
func TestCaretInSettingsIsTheFrames(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.DAVloomPort = 8081 })
	unlock := decode[view.UnlockSheet](t, f.h.do(get("/app/settings/unlock", f.sess))).Form
	sameCaret(t, "s-plan-3", "unlock", unlock)
	sameCaret(t, "s-profile-3", "unlock", unlock)
	type unlockFragment struct {
		Body view.UnlockSheet `json:"body"`
	}
	wrong := decode[unlockFragment](t, f.h.do(fragment(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"wrong-one"}}), "sheet"))).Body.Form
	if r := rowNamed(t, wrong, "password"); !r.Focus || r.Error == "" {
		t.Errorf("a wrong password: %+v", r)
	}

	q := url.Values{"a": {f.id}, "c": {f.col}}.Encode()
	sameCaret(t, "s-profile-5", "new calendar", decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=event&a="+f.id, f.sess))).Form)
	sameCaret(t, "s-profile-7", "rename", decode[view.CollectionRenameSheet](t, f.h.do(get("/app/collections/rename?"+q, f.sess))).Form)
	sameCaret(t, "s-profile-9", "delete", decode[view.CollectionDeleteSheet](t, f.h.do(get("/app/collections/delete?"+q, f.sess))).Form)

	add := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?op=add", f.sess)))
	if add.Add == nil {
		t.Fatal("Connect account opens no form")
	}
	sameCaret(t, "s-profile-2", "connect account", *add.Add)
	feed := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?op=feed", f.sess)))
	if feed.Add == nil {
		t.Fatal("Subscribe opens no form")
	}
	sameCaret(t, "s-profile-10", "subscribe", *feed.Add)

	sameCaret(t, "s-davloom-2", "add device", decode[view.DavloomPage](t, f.h.do(get("/app/settings/davloom", f.sess))).Add)
	sameCaret(t, "s-backup-2", "backup job", decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Add)
}
