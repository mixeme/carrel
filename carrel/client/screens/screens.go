package screens

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens/about"
	"carrel.dev/carrel/client/screens/agenda"
	"carrel.dev/carrel/client/screens/confirm"
	"carrel.dev/carrel/client/screens/contact"
	"carrel.dev/carrel/client/screens/contactedit"
	"carrel.dev/carrel/client/screens/contacts"
	"carrel.dev/carrel/client/screens/copies"
	"carrel.dev/carrel/client/screens/deadlink"
	"carrel.dev/carrel/client/screens/event"
	"carrel.dev/carrel/client/screens/eventedit"
	"carrel.dev/carrel/client/screens/forgot"
	"carrel.dev/carrel/client/screens/invite"
	"carrel.dev/carrel/client/screens/login"
	"carrel.dev/carrel/client/screens/move"
	"carrel.dev/carrel/client/screens/note"
	"carrel.dev/carrel/client/screens/notes"
	"carrel.dev/carrel/client/screens/register"
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
	// Waits are what of the screen waits for the library (U-5.5, the
	// owner's decision of 01.10.2026): each a row of the list «ждёт
	// библиотеку», and the row's mark stands at the line here and at the
	// control in the screen's code. nil — nothing waits.
	Waits []Wait
}

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
		Frames: []string{"s-contacts-32", "s-tasks-16"},
		Draw: func(model any) Root {
			p := model.(*view.CopiesSheet)
			return over(p.Under, copies.Sheet(p))
		},
		Title: func(model any) string { return string(model.(*view.CopiesSheet).Title) },
		Forms: func(model any) []widget.PostForm { return underForms(model.(*view.CopiesSheet).Under) },
	},
	{
		Model:  func() any { return new(view.LoginPage) },
		Frames: []string{"s-auth-1", "s-auth-13"},
		Draw:   func(model any) Root { return login.Screen(model.(*view.LoginPage)) },
		Title:  func(model any) string { return string(model.(*view.LoginPage).Title) },
		Waits: []Wait{
			{Row: 17, Fixture: "login/normal"},  // ждёт библиотеку: U-5.5 #17
			{Row: 17, Fixture: "login/refused"}, // ждёт библиотеку: U-5.5 #17
		},
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
		Model:  func() any { return new(view.DeadLinkPage) },
		Frames: []string{"rest-1"},
		Draw:   func(model any) Root { return deadlink.Screen(model.(*view.DeadLinkPage)) },
		Title:  func(model any) string { return string(model.(*view.DeadLinkPage).Title) },
	},
	{
		Model:  func() any { return new(view.ContactsListPage) },
		Frames: []string{"s-contacts-1", "states-4", "states-7", "states-8", "states-9", "states-10", "states-11"},
		Draw:   func(model any) Root { return contacts.Screen(model.(*view.ContactsListPage)) },
		Title:  func(model any) string { return string(model.(*view.ContactsListPage).Title) },
		Forms:  func(model any) []widget.PostForm { return contacts.Forms(model.(*view.ContactsListPage)) },
		Waits:  []Wait{},
	},
	{
		Model:  func() any { return new(view.ContactPage) },
		Frames: []string{"s-contacts-6", "s-contacts-16"},
		Draw:   func(model any) Root { return contact.Screen(model.(*view.ContactPage)) },
		Title:  func(model any) string { return string(model.(*view.ContactPage).Title) },
		Forms:  func(model any) []widget.PostForm { return contact.Forms(model.(*view.ContactPage)) },
		Waits: []Wait{
			{Row: 20, Fixture: "app-contact/normal"},  // ждёт библиотеку: U-5.5 #20
			{Row: 20, Fixture: "app-contact/outcome"}, // ждёт библиотеку: U-5.5 #20
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
			{Row: 20, Fixture: "app-contact-timeline/normal"},                        // ждёт библиотеку: U-5.5 #20
		},
	},
	{
		Model:  func() any { return new(view.ContactEditPage) },
		Frames: []string{"s-contacts-3", "s-contacts-4", "s-contacts-27", "states-12"},
		Draw:   func(model any) Root { return contactedit.Screen(model.(*view.ContactEditPage)) },
		Title:  func(model any) string { return string(model.(*view.ContactEditPage).Title) },
		Forms:  func(model any) []widget.PostForm { return contactedit.Forms(model.(*view.ContactEditPage)) },
		Waits: []Wait{
			{Row: 21, Fixture: "app-contact-edit/normal"},   // ждёт библиотеку: U-5.5 #21
			{Row: 21, Fixture: "app-contact-edit/no-photo"}, // ждёт библиотеку: U-5.5 #21
			{Row: 21, Fixture: "app-contact-edit/loss"},     // ждёт библиотеку: U-5.5 #21
			{Row: 21, Fixture: "app-contact-new/normal"},    // ждёт библиотеку: U-5.5 #21
		},
	},
	{
		Model:  func() any { return new(view.AgendaPage) },
		Frames: []string{"s-agenda-1", "s-agenda-17", "states-13", "states-14", "states-15"},
		Draw:   func(model any) Root { return agenda.Screen(model.(*view.AgendaPage)) },
		Title:  func(model any) string { return string(model.(*view.AgendaPage).Title) },
		Forms:  func(model any) []widget.PostForm { return agenda.Forms(model.(*view.AgendaPage)) },
		Waits:  []Wait{},
	},
	{
		Model:  func() any { return new(view.EventPage) },
		Frames: []string{"s-agenda-2"},
		Draw:   func(model any) Root { return event.Screen(model.(*view.EventPage)) },
		Title:  func(model any) string { return string(model.(*view.EventPage).Title) },
		Forms:  func(model any) []widget.PostForm { return event.Forms(model.(*view.EventPage)) },
	},
	{
		Model:  func() any { return new(view.EventEditPage) },
		Frames: []string{"s-agenda-3", "s-agenda-15", "states-17", "states-36"},
		Draw:   func(model any) Root { return eventedit.Screen(model.(*view.EventEditPage)) },
		Title:  func(model any) string { return string(model.(*view.EventEditPage).Title) },
		Forms:  func(model any) []widget.PostForm { return eventedit.Forms(model.(*view.EventEditPage)) },
		Waits: []Wait{
			{Row: 26, Fixture: "app-event-edit/normal"},    // ждёт библиотеку: U-5.5 #26
			{Row: 26, Fixture: "app-event-edit/loss"},      // ждёт библиотеку: U-5.5 #26
			{Row: 26, Fixture: "app-event-edit/loss-days"}, // ждёт библиотеку: U-5.5 #26
			{Row: 44, Fixture: "app-event-new/normal"},     // ждёт библиотеку: U-5.5 #44
		},
	},
	{
		Model:  func() any { return new(view.TasksPage) },
		Frames: []string{"s-tasks-1", "s-tasks-15", "states-18", "states-22"},
		Draw:   func(model any) Root { return tasks.Screen(model.(*view.TasksPage)) },
		Title:  func(model any) string { return string(model.(*view.TasksPage).Title) },
		Forms:  func(model any) []widget.PostForm { return tasks.Forms(model.(*view.TasksPage)) },
		Waits: []Wait{
			{Row: 31, Fixture: "app-tasks-all/normal"}, // ждёт библиотеку: U-5.5 #31
			{Row: 32, Fixture: "app-tasks/normal"},     // ждёт библиотеку: U-5.5 #32
		},
	},
	{
		Model:  func() any { return new(view.TaskPage) },
		Frames: []string{"s-tasks-2"},
		Draw:   func(model any) Root { return task.Screen(model.(*view.TaskPage)) },
		Title:  func(model any) string { return string(model.(*view.TaskPage).Title) },
		Forms:  func(model any) []widget.PostForm { return task.Forms(model.(*view.TaskPage)) },
	},
	{
		Model:  func() any { return new(view.TaskEditPage) },
		Frames: []string{"s-tasks-3", "s-tasks-14", "states-24"},
		Draw:   func(model any) Root { return taskedit.Screen(model.(*view.TaskEditPage)) },
		Title:  func(model any) string { return string(model.(*view.TaskEditPage).Title) },
		Forms:  func(model any) []widget.PostForm { return taskedit.Forms(model.(*view.TaskEditPage)) },
		Waits: []Wait{
			{Row: 35, Fixture: "app-task-edit/loss"}, // ждёт библиотеку: U-5.5 #35
		},
	},
	{
		Model:  func() any { return new(view.NotesPage) },
		Frames: []string{"s-notes-1", "s-notes-31", "states-25", "states-29"},
		Draw:   func(model any) Root { return notes.Screen(model.(*view.NotesPage)) },
		Title:  func(model any) string { return string(model.(*view.NotesPage).Title) },
		Forms:  func(model any) []widget.PostForm { return notes.Forms(model.(*view.NotesPage)) },
	},
	{
		Model:  func() any { return new(view.NotePage) },
		Frames: []string{"s-notes-2", "s-notes-20", "s-notes-30"},
		Draw:   func(model any) Root { return note.Screen(model.(*view.NotePage)) },
		Title:  func(model any) string { return string(model.(*view.NotePage).Title) },
		Forms:  func(model any) []widget.PostForm { return note.Forms(model.(*view.NotePage)) },
		Waits: []Wait{
			{Row: 42, Fixture: "app-note-edit/new"},                                     // ждёт библиотеку: U-5.5 #42
			{Row: 36, Control: ".Title(Rubric).Items(Drop).Menu(Menu).Items(MenuItem)"}, // ждёт библиотеку: U-5.5 #36
			{Row: 36, Control: ".Items(Docbar).Items(Drop).Menu(Menu).Items(MenuItem)"}, // ждёт библиотеку: U-5.5 #36
			{Row: 41, Control: ".Items(Head).Acts(Acts).Items(Button)"},                 // ждёт библиотеку: U-5.5 #41
			{Row: 41, Control: ".Items(Head).Acts(Acts).Items(Seg)"},                    // ждёт библиотеку: U-5.5 #41
		},
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
	}
	return bind.Over(shell, sheet)
}

// underForms are the forms the screen under a sheet prints at its end: what
// its top's menu submits (Sign out) stands under the scrim as well.
func underForms(u *view.Under) []widget.PostForm {
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
	case u.Note != nil:
		return note.Forms(u.Note)
	}
	return nil
}
