package screens

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens/about"
	"carrel.dev/carrel/client/screens/admin"
	"carrel.dev/carrel/client/screens/agenda"
	"carrel.dev/carrel/client/screens/ask"
	"carrel.dev/carrel/client/screens/collection"
	"carrel.dev/carrel/client/screens/confirm"
	"carrel.dev/carrel/client/screens/conflict"
	"carrel.dev/carrel/client/screens/contact"
	"carrel.dev/carrel/client/screens/contactedit"
	"carrel.dev/carrel/client/screens/contacts"
	"carrel.dev/carrel/client/screens/copies"
	"carrel.dev/carrel/client/screens/deadlink"
	"carrel.dev/carrel/client/screens/duplicates"
	"carrel.dev/carrel/client/screens/event"
	"carrel.dev/carrel/client/screens/eventedit"
	"carrel.dev/carrel/client/screens/export"
	"carrel.dev/carrel/client/screens/fileop"
	"carrel.dev/carrel/client/screens/files"
	"carrel.dev/carrel/client/screens/folder"
	"carrel.dev/carrel/client/screens/forgot"
	"carrel.dev/carrel/client/screens/guest"
	"carrel.dev/carrel/client/screens/history"
	"carrel.dev/carrel/client/screens/importer"
	"carrel.dev/carrel/client/screens/invite"
	"carrel.dev/carrel/client/screens/login"
	"carrel.dev/carrel/client/screens/mail"
	"carrel.dev/carrel/client/screens/move"
	"carrel.dev/carrel/client/screens/note"
	"carrel.dev/carrel/client/screens/notes"
	"carrel.dev/carrel/client/screens/offline"
	"carrel.dev/carrel/client/screens/picker"
	"carrel.dev/carrel/client/screens/publish"
	"carrel.dev/carrel/client/screens/published"
	"carrel.dev/carrel/client/screens/quick"
	"carrel.dev/carrel/client/screens/register"
	"carrel.dev/carrel/client/screens/remove"
	"carrel.dev/carrel/client/screens/rewrite"
	"carrel.dev/carrel/client/screens/search"
	"carrel.dev/carrel/client/screens/settings"
	"carrel.dev/carrel/client/screens/setup"
	"carrel.dev/carrel/client/screens/task"
	"carrel.dev/carrel/client/screens/taskedit"
	"carrel.dev/carrel/client/screens/tasks"
	"carrel.dev/carrel/client/screens/timeline"
	"carrel.dev/carrel/client/screens/wipe"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

// Screen is one screen of the client: the screen of a view model. The
// server's render contract hands the renderer a view model, not a route, so
// the renderer picks the screen by the type of the model; a route of C-5 is
// built when every view model it can answer with has its screen
// (carrel.dev/carrel/client/guard, CheckAddresses).
type Screen struct {
	// Model is a new view model of the screen, for a fixture to decode into;
	// its type is the key of the registry.
	Model func() any
	// Frames are the data-frame of each frame the screen draws: the frame
	// of every fixture of its view model — its state's by STATE_FRAME of
	// docs/spec/check.py («register/sent» is s-auth-3), else the frame whose
	// data-screen names the fixture's route (FIXTURE_SCREEN): the combined
	// view's and one collection's are two frames since Typecase 80d1319
	// (s-tasks-1 and s-tasks-15). The screen's
	// drafts are printed from them, and it is checked against each on every
	// fixture it draws, and each is drawn by some fixture.
	Frames []string
	// Draw draws the screen from a view model Model made, in any of its
	// states.
	Draw func(model any) Root
	// Title is the document's title, from a view model Model made: the
	// model's Title, which a page outside the shell draws as the mark, not as
	// words (U-6.14).
	Title func(model any) string
	// Forms are the forms the page prints at its end, from the same view
	// model (page.Document.Forms): the PostForms a button outside a form
	// submits by their ID (Button.Form) — the one-button form of each card of
	// the forgotten password (s-auth-6), the wipe's confirmation (s-auth-10).
	// nil — the screen has none.
	Forms func(model any) []widget.PostForm
	// Drop is the page's drop sheet (Typecase's notes 60, 136; bind.DropSheet)
	// from the same view model, and the form it names when the screen
	// prints none of its own (the list's import, bind.ImportDrop); the
	// document puts the sheet among the page's own sheets (Shell.Dialogs)
	// and the form at the page's end. ok false — no file is dropped here.
	// nil — the screen takes no drop.
	Drop func(model any) (sheet widget.Scrim, form *widget.PostForm, ok bool)
	// Notices is the space the screen keeps around the shell's notices: the
	// page's gutters (zero), but flush with the sides of a dialog on the
	// conflict screen (s-conflict-8…-11). The zone notices is drawn so too.
	Notices widget.MsgSpace
	// NoticesOf is Notices by the model, for a screen whose notices stand
	// over the page or in its dialog by what is open: the connections — in
	// Connect account, over its fields (s-profile-24). nil — Notices.
	NoticesOf func(model any) widget.MsgSpace
	// Poll is the bar of the poll as the screen draws it, for a screen
	// whose bar is not the lists' (bind.Runbar): the search's stands in
	// every state of a pass, with Stop while it runs (s-find-1, -4); false —
	// no bar. The zone poll is drawn so too. nil — bind.Runbar.
	Poll func(model any) (widget.Runbar, bool)
	// Waits are what of the screen waits for the library (U-5.5, the
	// owner's decision of 01.10.2026): each a row of the list «ждёт
	// библиотеку», and the row's mark stands at the line here and at the
	// control in the screen's code. nil — nothing waits.
	Waits []Wait
}

// importDrop is a list's drop sheet (s-io-1; Typecase's notes 60, 136): the
// import its bar names (ListBar.Drop) and the wizard's form it sends; a list
// one does not write into names none.
func importDrop(bar view.ListBar) (widget.Scrim, *widget.PostForm, bool) {
	if bar.Drop == nil {
		return widget.Scrim{}, nil, false
	}
	sheet, form := bind.ImportDrop(*bar.Drop, importIcon)
	return sheet, &form, true
}

// ownForm is a drop sheet that names a form the screen prints itself: the
// page's Attach (s-io-20), an edit's, whose file field takes the dropped
// file (s-io-21, -22).
func ownForm(sheet widget.Scrim, ok bool) (widget.Scrim, *widget.PostForm, bool) {
	return sheet, nil, ok
}

// importIcon is the drop sheet's icon of an import (s-io-1).
const importIcon widget.Icon = "import"

// Wait is one thing of a screen the library cannot draw yet, by its row of
// the list «ждёт библиотеку» (U-5.5):
//
//   - a state with no frame of its own, or one its frame draws cut short —
//     Fixture, «app-contacts/empty»: the screen draws it, and it is not held
//     against a frame (no frame comparison, no kinship); the meaning guard
//     and the classes of the library hold it as any other;
//   - a control the library cannot give its meaning yet — Control, the end
//     of its path in the screen's tree as the meaning guard names it, without
//     the indexes («.Main(Main).Items(Bar).More(Button)»): the meaning
//     guard's finding there is excused, on every fixture.
//
// A wait that excuses nothing is a finding: the row is to be closed.
type Wait struct {
	Row     int
	Fixture string
	Control string
}

// Root is what a screen draws: one of Typecase's roots — a shell, a page
// without a shell, an overlay. The interface is the library's and closed, so
// a screen cannot hand the page anything else (U-15.2, 1).
type Root = widget.Root

// All are the screens of the client, one per view model built. An address of
// C-5 is built when each view model it answers with is here, and is either
// built or in the list «ждёт библиотеку» of U-5.5, not both (U-15.2, 5).
var All = []Screen{
	{
		Model:  func() any { return new(view.MoveSheet) },
		Frames: []string{"s-contacts-2"},
		Draw: func(model any) Root {
			p := model.(*view.MoveSheet)
			return over(p.Under, move.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.MoveSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MoveSheet).Under) },
	},
	{
		Model:  func() any { return new(view.CopiesSheet) },
		Frames: []string{"s-contacts-35", "s-tasks-19"},
		Draw: func(model any) Root {
			p := model.(*view.CopiesSheet)
			return over(p.Under, copies.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.CopiesSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CopiesSheet).Under) },
	},
	{
		Model:  func() any { return new(view.CollectionRenameSheet) },
		Frames: []string{"s-profile-7", "s-profile-18", "s-profile-30", "s-profile-33"},
		Draw: func(model any) Root {
			p := model.(*view.CollectionRenameSheet)
			return over(p.Under, collection.Rename(p))
		},
		Title: func(model any) string { return string(model.(*view.CollectionRenameSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CollectionRenameSheet).Under) },
	},
	{
		Model:  func() any { return new(view.CollectionDeleteSheet) },
		Frames: []string{"s-profile-9", "s-profile-19"},
		Draw: func(model any) Root {
			p := model.(*view.CollectionDeleteSheet)
			return over(p.Under, collection.Delete(p))
		},
		Title: func(model any) string { return string(model.(*view.CollectionDeleteSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CollectionDeleteSheet).Under) },
	},
	{
		Model:  func() any { return new(view.NewSheet) },
		Frames: []string{"s-notes-5", "s-notes-6", "s-notes-8", "s-notes-9", "s-notes-10", "s-notes-34"},
		Draw: func(model any) Root {
			p := model.(*view.NewSheet)
			return over(p.Under, quick.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.NewSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.NewSheet).Under) },
	},
	{
		Model:  func() any { return new(view.ExportSheet) },
		Frames: []string{"s-agenda-30"},
		Draw: func(model any) Root {
			p := model.(*view.ExportSheet)
			return over(p.Under, export.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.ExportSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.ExportSheet).Under) },
	},
	{
		Model:  func() any { return new(view.CollectionMoveSheet) },
		Frames: []string{"s-profile-8", "s-profile-34", "s-profile-35"},
		Draw: func(model any) Root {
			p := model.(*view.CollectionMoveSheet)
			return over(p.Under, collection.Move(p))
		},
		Title: func(model any) string { return string(model.(*view.CollectionMoveSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CollectionMoveSheet).Under) },
	},
	{
		Model:  func() any { return new(view.CollectionNewSheet) },
		Frames: []string{"s-profile-5", "s-profile-6", "s-profile-20", "s-profile-21", "s-profile-28", "s-profile-29"},
		Draw: func(model any) Root {
			p := model.(*view.CollectionNewSheet)
			return over(p.Under, collection.New(p))
		},
		Title: func(model any) string { return string(model.(*view.CollectionNewSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CollectionNewSheet).Under) },
	},
	{
		Model:  func() any { return new(view.RecordDeleteSheet) },
		Frames: []string{"s-agenda-26", "s-tasks-18"},
		Draw: func(model any) Root {
			p := model.(*view.RecordDeleteSheet)
			return over(p.Under, remove.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.RecordDeleteSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.RecordDeleteSheet).Under) },
	},
	{
		Model:  func() any { return new(view.RewriteSheet) },
		Frames: []string{"s-contacts-15"},
		Draw: func(model any) Root {
			p := model.(*view.RewriteSheet)
			return over(p.Under, rewrite.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.RewriteSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.RewriteSheet).Under) },
	},
	{
		Model:  func() any { return new(view.AskSheet) },
		Frames: []string{"s-agenda-27", "s-agenda-23", "s-agenda-28"},
		Draw: func(model any) Root {
			p := model.(*view.AskSheet)
			return over(p.Under, ask.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.AskSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.AskSheet).Under) },
	},
	{
		Model:  func() any { return new(view.PublishSheet) },
		Frames: []string{"s-publish-2", "s-publish-3", "s-publish-4", "s-publish-5", "s-publish-13", "s-publish-19", "s-publish-22"},
		Draw: func(model any) Root {
			p := model.(*view.PublishSheet)
			return over(p.Under, publish.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.PublishSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.PublishSheet).Under) },
	},
	{
		Model:  func() any { return new(view.MailAttachSheet) },
		Frames: []string{"s-mail-5", "s-mail-6", "s-mail-10", "s-mail-12", "s-mail-13"},
		Draw: func(model any) Root {
			p := model.(*view.MailAttachSheet)
			return over(p.Under, mail.Attach(p))
		},
		Title: func(model any) string { return mail.AttachTitle(model.(*view.MailAttachSheet)) },
		Forms: func(model any) []widget.PostForm {
			p := model.(*view.MailAttachSheet)
			return append(mail.AttachForms(p), underForms(p.Under)...)
		},
	},
	{
		Model:  func() any { return new(view.MailSheet) },
		Frames: []string{"s-mail-1", "s-mail-17"},
		Draw: func(model any) Root {
			p := model.(*view.MailSheet)
			return over(p.Under, mail.Letter(p))
		},
		Title: func(model any) string { return mail.LetterTitle },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MailSheet).Under) },
	},
	{
		Model:  func() any { return new(view.MailLinkSheet) },
		Frames: []string{"s-mail-4"},
		Draw: func(model any) Root {
			p := model.(*view.MailLinkSheet)
			return over(p.Under, mail.LinkSheet(p))
		},
		Title: func(model any) string { return mail.LinkTitle },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MailLinkSheet).Under) },
	},
	{
		Model:  func() any { return new(view.MailPreviewSheet) },
		Frames: []string{"s-mail-14"},
		Draw: func(model any) Root {
			p := model.(*view.MailPreviewSheet)
			return over(p.Under, mail.Preview(p))
		},
		Title: func(model any) string { return mail.PreviewTitle },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MailPreviewSheet).Under) },
	},
	{
		Model:  func() any { return new(view.MailCloseSheet) },
		Frames: []string{"s-mail-15"},
		Draw: func(model any) Root {
			p := model.(*view.MailCloseSheet)
			return over(p.Under, mail.Close(p))
		},
		Title: func(model any) string { return mail.CloseTitle },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MailCloseSheet).Under) },
	},
	{
		Model:  func() any { return new(view.MailSentSheet) },
		Frames: []string{"s-mail-18"},
		Draw: func(model any) Root {
			p := model.(*view.MailSentSheet)
			return over(p.Under, mail.Sent(p))
		},
		Title: func(model any) string { return mail.LetterTitle },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.MailSentSheet).Under) },
	},
	{
		Model:  func() any { return new(view.LoginPage) },
		Frames: []string{"s-auth-1", "s-auth-13"},
		Draw:   func(model any) Root { return login.Screen(model.(*view.LoginPage)) },
		Title:  func(model any) string { return string(model.(*view.LoginPage).Title) },
	},
	{
		Model:  func() any { return new(view.AboutPage) },
		Frames: []string{"rest-3"},
		Draw:   func(model any) Root { return about.Screen(model.(*view.AboutPage)) },
		Title:  func(model any) string { return string(model.(*view.AboutPage).Title) },
	},
	{
		Model:  func() any { return new(view.InvitePage) },
		Frames: []string{"s-auth-12", "s-auth-14"},
		Draw:   func(model any) Root { return invite.Screen(model.(*view.InvitePage)) },
		Title:  func(model any) string { return string(model.(*view.InvitePage).Title) },
	},
	{
		Model:  func() any { return new(view.ConfirmEmailPage) },
		Frames: []string{"rest-2"},
		Draw:   func(model any) Root { return confirm.Screen(model.(*view.ConfirmEmailPage)) },
		Title:  func(model any) string { return string(model.(*view.ConfirmEmailPage).Title) },
	},
	{
		Model:  func() any { return new(view.RegisterPage) },
		Frames: []string{"s-auth-2", "s-auth-3"},
		Draw:   func(model any) Root { return register.Screen(model.(*view.RegisterPage)) },
		Title:  func(model any) string { return string(model.(*view.RegisterPage).Title) },
	},
	{
		Model:  func() any { return new(view.SetupPage) },
		Frames: []string{"s-auth-4"},
		Draw:   func(model any) Root { return setup.Screen(model.(*view.SetupPage)) },
		Title:  func(model any) string { return string(model.(*view.SetupPage).Title) },
	},
	{
		Model:  func() any { return new(view.ForgotPage) },
		Frames: []string{"s-auth-5", "s-auth-6", "s-auth-7", "s-auth-8", "s-auth-9"},
		Draw:   func(model any) Root { return forgot.Screen(model.(*view.ForgotPage)) },
		Title:  func(model any) string { return string(model.(*view.ForgotPage).Title) },
		Forms:  func(model any) []widget.PostForm { return forgot.Forms(model.(*view.ForgotPage)) },
	},
	{
		Model:  func() any { return new(view.ForgotWipePage) },
		Frames: []string{"s-auth-10", "s-auth-11"},
		Draw:   func(model any) Root { return wipe.Screen(model.(*view.ForgotWipePage)) },
		Title:  func(model any) string { return string(model.(*view.ForgotWipePage).Title) },
		Forms:  func(model any) []widget.PostForm { return wipe.Forms(model.(*view.ForgotWipePage)) },
	},
	{
		Model:  func() any { return new(view.PasswordHistoryPage) },
		Frames: []string{"s-auth-15"},
		Draw:   func(model any) Root { return history.Screen(model.(*view.PasswordHistoryPage)) },
		Title:  func(model any) string { return string(model.(*view.PasswordHistoryPage).Title) },
	},
	{
		Model: func() any { return new(view.PublishedPage) },
		Frames: []string{"s-publish-6", "s-publish-7", "s-publish-8", "s-publish-23", "s-publish-9", "s-publish-10", "s-publish-11", "s-publish-12",
			"s-publish-16", "s-publish-24", "s-publish-25", "s-publish-26", "s-publish-27", "s-publish-28", "s-publish-29", "s-publish-30",
			"s-publish-31", "s-publish-33", "s-publish-34", "s-publish-35", "s-publish-36", "s-publish-37", "s-publish-38", "s-publish-39"},
		Draw:  func(model any) Root { return guest.Screen(model.(*view.PublishedPage)) },
		Title: func(model any) string { return string(model.(*view.PublishedPage).Title) },
	},
	{
		Model:  func() any { return new(view.OfflinePage) },
		Frames: []string{"offline-1"},
		Draw:   func(model any) Root { return offline.Screen(model.(*view.OfflinePage)) },
		Title:  func(model any) string { return string(model.(*view.OfflinePage).Title) },
	},
	{
		Model:  func() any { return new(view.DeadLinkPage) },
		Frames: []string{"rest-1"},
		Draw:   func(model any) Root { return deadlink.Screen(model.(*view.DeadLinkPage)) },
		Title:  func(model any) string { return string(model.(*view.DeadLinkPage).Title) },
	},
	{
		Model:  func() any { return new(view.ContactsListPage) },
		Frames: []string{"s-contacts-1", "states-4", "states-7", "states-8", "states-9", "states-10", "states-11", "print-2"},
		Draw: func(model any) Root {
			if p := model.(*view.ContactsListPage); p.Print.Paper {
				return contacts.Paper(p)
			}
			return contacts.Screen(model.(*view.ContactsListPage))
		},
		Title: func(model any) string { return string(model.(*view.ContactsListPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return importDrop(model.(*view.ContactsListPage).Bar)
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.ContactsListPage); !p.Print.Paper {
				return contacts.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.ContactPage) },
		Frames: []string{"s-contacts-6", "s-contacts-16", "s-contacts-33", "s-dup-8", "print-3"},
		Draw: func(model any) Root {
			if p := model.(*view.ContactPage); p.Print.Paper {
				return contact.Paper(p)
			}
			return contact.Screen(model.(*view.ContactPage))
		},
		Title: func(model any) string { return string(model.(*view.ContactPage).Title) },
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.ContactPage); !p.Print.Paper {
				return contact.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
	},
	{
		Model:  func() any { return new(view.ContactTimelinePage) },
		Frames: []string{"s-contacts-7"},
		Draw:   func(model any) Root { return timeline.Screen(model.(*view.ContactTimelinePage)) },
		Title:  func(model any) string { return string(model.(*view.ContactTimelinePage).Title) },
		Forms:  func(model any) []widget.PostForm { return timeline.Forms(model.(*view.ContactTimelinePage)) },
		Waits: []Wait{
			{Row: 22, Control: ".Items(Bar).Items(Drop).Menu(Menu).Items(MenuItem)"}, // ждёт библиотеку: U-5.5 #22
		},
	},
	{
		Model:  func() any { return new(view.PhotoPreviewPage) },
		Frames: []string{"s-contacts-5"},
		Draw:   func(model any) Root { return contactedit.Crop(model.(*view.PhotoPreviewPage)) },
		Title:  func(model any) string { return string(model.(*view.PhotoPreviewPage).Title) },
		Forms:  func(model any) []widget.PostForm { return contactedit.CropForms(model.(*view.PhotoPreviewPage)) },
	},
	{
		Model:  func() any { return new(view.ContactEditPage) },
		Frames: []string{"s-contacts-3", "s-contacts-4", "s-contacts-27", "s-contacts-38", "states-12"},
		Draw:   func(model any) Root { return contactedit.Screen(model.(*view.ContactEditPage)) },
		Title:  func(model any) string { return string(model.(*view.ContactEditPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(contactedit.Drop(model.(*view.ContactEditPage)))
		},
		Forms: func(model any) []widget.PostForm { return contactedit.Forms(model.(*view.ContactEditPage)) },
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.AgendaPage) },
		Frames: []string{"s-agenda-1", "s-agenda-17", "s-agenda-31", "states-13", "states-14", "states-15", "print-1", "print-7"},
		Draw: func(model any) Root {
			p := model.(*view.AgendaPage)
			if p.Print.Paper {
				return agenda.Paper(p)
			}
			return agenda.Screen(p)
		},
		Title: func(model any) string { return string(model.(*view.AgendaPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return importDrop(model.(*view.AgendaPage).Bar)
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.AgendaPage); !p.Print.Paper {
				return agenda.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.EventPage) },
		Frames: []string{"s-agenda-2", "s-agenda-25"},
		Draw:   func(model any) Root { return event.Screen(model.(*view.EventPage)) },
		Title:  func(model any) string { return string(model.(*view.EventPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(event.Drop(model.(*view.EventPage)))
		},
		Forms: func(model any) []widget.PostForm { return event.Forms(model.(*view.EventPage)) },
	},
	{
		Model:  func() any { return new(view.EventEditPage) },
		Frames: []string{"s-agenda-3", "s-agenda-15", "s-agenda-32", "states-17", "states-36"},
		Draw:   func(model any) Root { return eventedit.Screen(model.(*view.EventEditPage)) },
		Title:  func(model any) string { return string(model.(*view.EventEditPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(eventedit.Drop(model.(*view.EventEditPage)))
		},
		Forms: func(model any) []widget.PostForm { return eventedit.Forms(model.(*view.EventEditPage)) },
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.TasksPage) },
		Frames: []string{"s-tasks-1", "s-tasks-15", "s-tasks-20", "states-18", "states-19", "states-20", "states-21", "states-22", "states-23", "print-4"},
		Draw: func(model any) Root {
			if p := model.(*view.TasksPage); p.Print.Paper {
				return tasks.Paper(p)
			}
			return tasks.Screen(model.(*view.TasksPage))
		},
		Title: func(model any) string { return string(model.(*view.TasksPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return importDrop(model.(*view.TasksPage).Bar)
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.TasksPage); !p.Print.Paper {
				return tasks.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.TaskPage) },
		Frames: []string{"s-tasks-2", "s-tasks-17"},
		Draw:   func(model any) Root { return task.Screen(model.(*view.TaskPage)) },
		Title:  func(model any) string { return string(model.(*view.TaskPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(task.Drop(model.(*view.TaskPage)))
		},
		Forms: func(model any) []widget.PostForm { return task.Forms(model.(*view.TaskPage)) },
	},
	{
		Model:  func() any { return new(view.TaskEditPage) },
		Frames: []string{"s-tasks-3", "s-tasks-14", "states-24"},
		Draw:   func(model any) Root { return taskedit.Screen(model.(*view.TaskEditPage)) },
		Title:  func(model any) string { return string(model.(*view.TaskEditPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(taskedit.Drop(model.(*view.TaskEditPage)))
		},
		Forms: func(model any) []widget.PostForm { return taskedit.Forms(model.(*view.TaskEditPage)) },
	},
	{
		Model:  func() any { return new(view.NotesPage) },
		Frames: []string{"s-notes-1", "s-notes-31", "s-notes-33", "states-25", "states-26", "states-27", "states-28", "states-29", "states-30", "print-8"},
		Draw: func(model any) Root {
			if p := model.(*view.NotesPage); p.Print.Paper {
				return notes.Paper(p)
			}
			return notes.Screen(model.(*view.NotesPage))
		},
		Title: func(model any) string { return string(model.(*view.NotesPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return importDrop(model.(*view.NotesPage).Bar)
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.NotesPage); !p.Print.Paper {
				return notes.Forms(p)
			}
			return nil // nothing on paper is pressed, as the tasks'
		},
		Waits: []Wait{},
	},
	{
		Model:  func() any { return new(view.NotePage) },
		Frames: []string{"s-notes-2", "s-notes-20", "s-notes-30", "s-notes-32", "s-notes-27", "states-31", "print-5"},
		Draw: func(model any) Root {
			if p := model.(*view.NotePage); p.Print.Paper {
				return note.Paper(p)
			}
			return note.Screen(model.(*view.NotePage))
		},
		Title: func(model any) string { return string(model.(*view.NotePage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(note.Drop(model.(*view.NotePage)))
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.NotePage); !p.Print.Paper {
				return note.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
		Waits: []Wait{},
	},
	{
		Model: func() any { return new(view.ConflictPage) },
		Frames: []string{"s-conflict-1", "s-conflict-3", "s-conflict-4", "s-conflict-5", "s-conflict-6", "s-conflict-7",
			"s-conflict-8", "s-conflict-9", "s-conflict-10", "s-conflict-11"},
		Draw:    func(model any) Root { return conflict.Screen(model.(*view.ConflictPage)) },
		Title:   func(model any) string { return string(model.(*view.ConflictPage).Title) },
		Forms:   func(model any) []widget.PostForm { return conflict.Forms(model.(*view.ConflictPage)) },
		Notices: conflict.NoticeSpace,
	},
	{
		Model:  func() any { return new(view.AccountPage) },
		Frames: []string{"account-1", "account-7", "account-8", "account-9", "account-10", "account-13", "s-mail-9"},
		Draw:   func(model any) Root { return settings.Account(model.(*view.AccountPage)) },
		Title:  func(model any) string { return string(model.(*view.AccountPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.AccountForms(model.(*view.AccountPage)) },
	},
	{
		Model:  func() any { return new(view.AttachmentsPage) },
		Frames: []string{"attachments-1", "attachments-5", "attachments-6"},
		Draw:   func(model any) Root { return settings.Attachments(model.(*view.AttachmentsPage)) },
		Title:  func(model any) string { return string(model.(*view.AttachmentsPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.AttachmentsForms(model.(*view.AttachmentsPage)) },
	},
	{
		Model:     func() any { return new(view.ConnectionsPage) },
		Frames:    []string{"s-profile-1", "s-profile-2", "s-profile-10", "s-profile-11", "s-profile-16", "s-profile-23", "s-profile-24", "s-profile-25", "s-profile-27", "s-profile-31", "s-profile-32", "s-profile-4"},
		Draw:      func(model any) Root { return settings.Connections(model.(*view.ConnectionsPage)) },
		Title:     func(model any) string { return string(model.(*view.ConnectionsPage).Title) },
		Forms:     func(model any) []widget.PostForm { return settings.ConnectionsForms(model.(*view.ConnectionsPage)) },
		NoticesOf: func(model any) widget.MsgSpace { return settings.ConnectionsNotices(model.(*view.ConnectionsPage)) },
	},
	{
		Model:  func() any { return new(view.DevicesPage) },
		Frames: []string{"s-plan-1", "s-plan-2", "s-plan-4", "s-plan-5", "s-plan-6"},
		Draw:   func(model any) Root { return settings.Devices(model.(*view.DevicesPage)) },
		Title:  func(model any) string { return string(model.(*view.DevicesPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.DevicesForms(model.(*view.DevicesPage)) },
	},
	{
		Model:  func() any { return new(view.BackupPage) },
		Frames: []string{"s-backup-1", "s-backup-2", "s-backup-6", "s-backup-7", "s-backup-8", "s-backup-15", "s-backup-16", "s-backup-17", "s-backup-18", "s-backup-19", "s-backup-20"},
		Draw:   func(model any) Root { return settings.Backup(model.(*view.BackupPage)) },
		Title:  func(model any) string { return string(model.(*view.BackupPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(settings.RestoreDrop(model.(*view.BackupPage)))
		},
		Forms: func(model any) []widget.PostForm { return settings.BackupForms(model.(*view.BackupPage)) },
	},
	{
		Model:  func() any { return new(view.BackupTriggerSheet) },
		Frames: []string{"s-backup-12", "s-backup-13", "s-backup-14"},
		Draw: func(model any) Root {
			p := model.(*view.BackupTriggerSheet)
			return over(p.Under, settings.TriggerSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.BackupTriggerSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.BackupTriggerSheet).Under) },
	},
	{
		Model:  func() any { return new(view.ConnectionRemoveSheet) },
		Frames: []string{"s-profile-26"},
		Draw: func(model any) Root {
			p := model.(*view.ConnectionRemoveSheet)
			return over(p.Under, settings.DisconnectSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.ConnectionRemoveSheet).Subject) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.ConnectionRemoveSheet).Under) },
	},
	{
		Model:  func() any { return new(view.DavloomScopeSheet) },
		Frames: []string{"s-davloom-7"},
		Draw: func(model any) Root {
			p := model.(*view.DavloomScopeSheet)
			return over(p.Under, settings.DavloomScopeSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.DavloomScopeSheet).Subject) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.DavloomScopeSheet).Under) },
	},
	{
		Model:  func() any { return new(view.DavloomTreeSheet) },
		Frames: []string{"s-davloom-9"},
		Draw: func(model any) Root {
			p := model.(*view.DavloomTreeSheet)
			return over(p.Under, settings.DavloomTreeSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.DavloomTreeSheet).Address) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.DavloomTreeSheet).Under) },
	},
	{
		Model:  func() any { return new(view.EscrowPage) },
		Frames: []string{"account-4", "account-12", "account-14", "account-16"},
		Draw:   func(model any) Root { return settings.Escrow(model.(*view.EscrowPage)) },
		Title:  func(model any) string { return string(model.(*view.EscrowPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.EscrowForms(model.(*view.EscrowPage)) },
	},
	{
		Model:  func() any { return new(view.EscrowWithdrawSheet) },
		Frames: []string{"account-11"},
		Draw: func(model any) Root {
			p := model.(*view.EscrowWithdrawSheet)
			return over(p.Under, settings.WithdrawSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.EscrowWithdrawSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.EscrowWithdrawSheet).Under) },
	},
	{
		Model:  func() any { return new(view.PasswordPage) },
		Frames: []string{"account-3", "account-17"},
		Draw:   func(model any) Root { return settings.Password(model.(*view.PasswordPage)) },
		Title:  func(model any) string { return string(model.(*view.PasswordPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.PasswordForms(model.(*view.PasswordPage)) },
	},
	{
		Model:  func() any { return new(view.UnlockSheet) },
		Frames: []string{"s-plan-3", "s-davloom-5", "account-15", "account-2"},
		Draw: func(model any) Root {
			p := model.(*view.UnlockSheet)
			return over(p.Under, settings.UnlockSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.UnlockSheet).Purpose) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.UnlockSheet).Under) },
	},
	{
		Model:  func() any { return new(view.DavloomPage) },
		Frames: []string{"s-davloom-1", "s-davloom-2", "s-davloom-3", "s-davloom-6", "s-davloom-8"},
		Draw:   func(model any) Root { return settings.Davloom(model.(*view.DavloomPage)) },
		Title:  func(model any) string { return string(model.(*view.DavloomPage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.DavloomForms(model.(*view.DavloomPage)) },
	},
	{
		Model:  func() any { return new(view.AppearancePage) },
		Frames: []string{"appearance-1"},
		Draw:   func(model any) Root { return settings.Appearance(model.(*view.AppearancePage)) },
		Title:  func(model any) string { return string(model.(*view.AppearancePage).Title) },
		Forms:  func(model any) []widget.PostForm { return settings.AppearanceForms(model.(*view.AppearancePage)) },
	},
	{
		Model:  func() any { return new(view.AdminInvitesPage) },
		Frames: []string{"s-admin-1"},
		Draw:   func(model any) Root { return admin.Invites(model.(*view.AdminInvitesPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminInvitesPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.InvitesForms(model.(*view.AdminInvitesPage)) },
	},
	{
		Model:  func() any { return new(view.AdminSettingsPage) },
		Frames: []string{"s-admin-3", "s-admin-32", "s-admin-33"},
		Draw:   func(model any) Root { return admin.Settings(model.(*view.AdminSettingsPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminSettingsPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.SettingsForms(model.(*view.AdminSettingsPage)) },
	},
	{
		Model:  func() any { return new(view.AdminInstallPage) },
		Frames: []string{"s-admin-4", "s-admin-25", "s-admin-26"},
		Draw:   func(model any) Root { return admin.Install(model.(*view.AdminInstallPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminInstallPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.InstallForms(model.(*view.AdminInstallPage)) },
	},
	{
		Model:  func() any { return new(view.AdminDavPage) },
		Frames: []string{"s-admin-5", "s-admin-27", "s-admin-28", "s-admin-29"},
		Draw:   func(model any) Root { return admin.Dav(model.(*view.AdminDavPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminDavPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.DavForms(model.(*view.AdminDavPage)) },
	},
	{
		Model:  func() any { return new(view.AdminEscrowPage) },
		Frames: []string{"s-admin-6", "s-admin-30", "s-admin-31"},
		Draw:   func(model any) Root { return admin.Escrow(model.(*view.AdminEscrowPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminEscrowPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.EscrowForms(model.(*view.AdminEscrowPage)) },
	},
	{
		Model:  func() any { return new(view.PasswordResetSheet) },
		Frames: []string{"s-admin-13", "s-admin-14", "s-admin-16", "s-admin-20"},
		Draw: func(model any) Root {
			p := model.(*view.PasswordResetSheet)
			return over(p.Under, admin.ResetSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.PasswordResetSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.PasswordResetSheet).Under) },
	},
	{
		Model:  func() any { return new(view.AccountDeleteSheet) },
		Frames: []string{"s-admin-34"},
		Draw: func(model any) Root {
			p := model.(*view.AccountDeleteSheet)
			return over(p.Under, admin.DeleteSheet(p))
		},
		Title: func(model any) string { return string(model.(*view.AccountDeleteSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.AccountDeleteSheet).Under) },
	},
	{
		Model:  func() any { return new(view.AdminUsersPage) },
		Frames: []string{"s-admin-2"},
		Draw:   func(model any) Root { return admin.Users(model.(*view.AdminUsersPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminUsersPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.UsersForms(model.(*view.AdminUsersPage)) },
	},
	{
		Model:  func() any { return new(view.AdminAuditPage) },
		Frames: []string{"s-admin-7"},
		Draw:   func(model any) Root { return admin.Audit(model.(*view.AdminAuditPage)) },
		Title:  func(model any) string { return string(model.(*view.AdminAuditPage).Title) },
		Forms:  func(model any) []widget.PostForm { return admin.AuditForms(model.(*view.AdminAuditPage)) },
	},
	{
		Model:  func() any { return new(view.FilesRootPage) },
		Frames: []string{"s-files-6", "states-35"},
		Draw:   func(model any) Root { return files.Screen(model.(*view.FilesRootPage)) },
		Title:  func(model any) string { return string(model.(*view.FilesRootPage).Title) },
		Forms:  func(model any) []widget.PostForm { return files.Forms(model.(*view.FilesRootPage)) },
	},
	{
		Model:  func() any { return new(view.FilesPage) },
		Frames: []string{"s-files-1", "s-files-2", "s-files-5", "states-32", "states-33", "states-34", "print-6"},
		Draw: func(model any) Root {
			if p := model.(*view.FilesPage); p.Print.Paper {
				return folder.Paper(p)
			}
			return folder.Screen(model.(*view.FilesPage))
		},
		Title: func(model any) string { return string(model.(*view.FilesPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(folder.Drop(model.(*view.FilesPage)))
		},
		Forms: func(model any) []widget.PostForm {
			if p := model.(*view.FilesPage); !p.Print.Paper {
				return folder.Forms(p)
			}
			return nil // nothing on paper is pressed
		},
	},
	{
		Model:  func() any { return new(view.FileOpSheet) },
		Frames: []string{"s-files-4", "s-files-11", "s-files-12", "s-files-13", "s-files-21", "s-files-22", "s-files-23", "s-files-27"},
		Draw:   func(model any) Root { return fileop.Screen(model.(*view.FileOpSheet)) },
		Title:  func(model any) string { return string(model.(*view.FileOpSheet).Title) },
		Forms:  func(model any) []widget.PostForm { return fileop.Forms(model.(*view.FileOpSheet)) },
	},
	{
		Model:  func() any { return new(view.FilePickerSheet) },
		Frames: []string{"s-notes-11", "s-files-16", "s-files-19"},
		Draw:   func(model any) Root { return picker.Screen(model.(*view.FilePickerSheet)) },
		Title:  func(model any) string { return string(model.(*view.FilePickerSheet).Title) },
		Forms:  func(model any) []widget.PostForm { return picker.Forms(model.(*view.FilePickerSheet)) },
	},
	{
		Model:  func() any { return new(view.PublishedLinksPage) },
		Frames: []string{"s-publish-1", "s-publish-18", "s-publish-20", "s-publish-21"},
		Draw:   func(model any) Root { return published.Screen(model.(*view.PublishedLinksPage)) },
		Title:  func(model any) string { return string(model.(*view.PublishedLinksPage).Title) },
		Forms:  func(model any) []widget.PostForm { return published.Forms(model.(*view.PublishedLinksPage)) },
	},
	{
		Model: func() any { return new(view.ImportPage) },
		Frames: []string{"s-io-3", "s-io-4", "s-io-5", "s-io-6", "s-io-7", "s-io-8", "s-io-9",
			"s-io-14", "s-io-15", "s-io-16", "s-io-17", "s-io-18", "s-io-19"},
		Draw:  func(model any) Root { return importer.Screen(model.(*view.ImportPage)) },
		Title: func(model any) string { return string(model.(*view.ImportPage).Title) },
		Drop: func(model any) (widget.Scrim, *widget.PostForm, bool) {
			return ownForm(importer.Drop(model.(*view.ImportPage), importIcon))
		},
		Forms:   func(model any) []widget.PostForm { return importer.Forms(model.(*view.ImportPage)) },
		Notices: widget.MsgFlushSides,
	},
	{
		Model: func() any { return new(view.SearchPage) },
		Frames: []string{"s-find-1", "s-find-4", "s-find-5", "s-find-6", "s-find-7", "s-find-8",
			"s-find-9", "s-find-10"},
		Draw:    func(model any) Root { return search.Screen(model.(*view.SearchPage)) },
		Title:   func(model any) string { return string(model.(*view.SearchPage).Title) },
		Forms:   func(model any) []widget.PostForm { return search.Forms(model.(*view.SearchPage)) },
		Notices: widget.MsgFlushSides,
		Poll:    func(model any) (widget.Runbar, bool) { return search.Bar(model.(*view.SearchPage)) },
	},
	{
		Model:   func() any { return new(view.DuplicatesPage) },
		Frames:  []string{"s-dup-1", "s-dup-6"},
		Draw:    func(model any) Root { return duplicates.Screen(model.(*view.DuplicatesPage)) },
		Title:   func(model any) string { return string(model.(*view.DuplicatesPage).Title) },
		Forms:   func(model any) []widget.PostForm { return duplicates.Forms(model.(*view.DuplicatesPage)) },
		Notices: widget.MsgSpacePage,
	},
	{
		Model:   func() any { return new(view.DuplicatesMergePage) },
		Frames:  []string{"s-dup-2", "s-dup-4", "s-dup-5"},
		Draw:    func(model any) Root { return duplicates.Merge(model.(*view.DuplicatesMergePage)) },
		Title:   func(model any) string { return string(model.(*view.DuplicatesMergePage).Title) },
		Forms:   func(model any) []widget.PostForm { return duplicates.MergeForms(model.(*view.DuplicatesMergePage)) },
		Notices: widget.MsgSpacePage,
	},
}

// Part is a piece of a page an answer prints alone.
type Part interface{ Render() string }

// Answer is what a view model answers that is not a page: a part the live
// layer asks for by an address the page names and puts into the page — the
// search line's suggestions (Typecase's note 118). It is printed alone, with
// no document; its frames draw it inside a page, and it is held against
// that part of them.
type Answer struct {
	Model  func() any
	Frames []string
	Draw   func(model any) Part
}

// Answers are the answers of the client that are not pages, one per view
// model.
var Answers = []Answer{
	{
		Model:  func() any { return new(view.SearchSuggest) },
		Frames: []string{"s-find-2", "s-find-3"},
		Draw:   func(model any) Part { return search.Suggestions(model.(*view.SearchSuggest)) },
	},
}

// over is a sheet opened by its address over the screen it was opened from
// (C-6 Under, the owner's decision of 15.10.2026; Typecase's Overlay,
// s-contacts-2): that page drawn by its own screen, under the scrim, the
// sheet on it. A sheet that came without the screen stands alone.
func over(u *view.Under, sheet widget.Sheet) Root {
	var shell widget.Shell
	switch {
	case u == nil:
		return bind.Alone(sheet)
	case u.Contacts != nil:
		shell = contacts.Screen(u.Contacts)
	case u.Contact != nil:
		shell = contact.Screen(u.Contact)
	case u.Agenda != nil:
		shell = agenda.Screen(u.Agenda)
	case u.Event != nil:
		shell = event.Screen(u.Event)
	case u.Tasks != nil:
		shell = tasks.Screen(u.Tasks)
	case u.Task != nil:
		shell = task.Screen(u.Task)
	case u.Notes != nil:
		shell = notes.Screen(u.Notes)
	case u.Note != nil:
		shell = note.Screen(u.Note)
	case u.EventEdit != nil:
		shell = eventedit.Screen(u.EventEdit)
	case u.TaskEdit != nil:
		shell = taskedit.Screen(u.TaskEdit)
	case u.ContactEdit != nil:
		shell = contactedit.Screen(u.ContactEdit)
	case u.AdminUsers != nil:
		shell = admin.Users(u.AdminUsers)
	case u.Account != nil:
		shell = settings.Account(u.Account)
	case u.Connections != nil:
		shell = settings.Connections(u.Connections)
	case u.Devices != nil:
		shell = settings.Devices(u.Devices)
	case u.Backup != nil:
		shell = settings.Backup(u.Backup)
	case u.Davloom != nil:
		shell = settings.Davloom(u.Davloom)
	case u.Published != nil:
		shell = published.Screen(u.Published)
	case u.Files != nil:
		shell = folder.Screen(u.Files)
	}
	return bind.Over(shell, sheet)
}

// underForms are the forms the screen under a sheet prints at its end: what
// its top's menu submits (Sign out) stands under the scrim as well. The
// zones of a page under a sheet are not named — the sheet is what is open —
// so its forms ask none: the rail's goes the plain way.
func underForms(u *view.Under) []widget.PostForm {
	forms := pageForms(u)
	for i := range forms {
		forms[i].Target = ""
	}
	return forms
}

// pageForms are the forms of the page under a sheet as its screen prints them.
func pageForms(u *view.Under) []widget.PostForm {
	switch {
	case u == nil:
		return nil
	case u.Contacts != nil:
		return contacts.Forms(u.Contacts)
	case u.Contact != nil:
		return contact.Forms(u.Contact)
	case u.Agenda != nil:
		return agenda.Forms(u.Agenda)
	case u.Event != nil:
		return event.Forms(u.Event)
	case u.Tasks != nil:
		return tasks.Forms(u.Tasks)
	case u.Task != nil:
		return task.Forms(u.Task)
	case u.Notes != nil:
		return notes.Forms(u.Notes)
	case u.EventEdit != nil:
		return eventedit.Forms(u.EventEdit)
	case u.TaskEdit != nil:
		return taskedit.Forms(u.TaskEdit)
	case u.ContactEdit != nil:
		return contactedit.Forms(u.ContactEdit)
	case u.Note != nil:
		return note.Forms(u.Note)
	case u.AdminUsers != nil:
		return admin.UsersForms(u.AdminUsers)
	case u.Account != nil:
		return settings.AccountForms(u.Account)
	case u.Connections != nil:
		return settings.ConnectionsForms(u.Connections)
	case u.Devices != nil:
		return settings.DevicesForms(u.Devices)
	case u.Backup != nil:
		return settings.BackupForms(u.Backup)
	case u.Davloom != nil:
		return settings.DavloomForms(u.Davloom)
	case u.Published != nil:
		return published.Forms(u.Published)
	case u.Files != nil:
		return folder.Forms(u.Files)
	}
	return nil
}
