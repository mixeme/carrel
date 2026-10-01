package screens

import (
	"carrel.dev/carrel/client/screens/about"
	"carrel.dev/carrel/client/screens/confirm"
	"carrel.dev/carrel/client/screens/deadlink"
	"carrel.dev/carrel/client/screens/forgot"
	"carrel.dev/carrel/client/screens/invite"
	"carrel.dev/carrel/client/screens/login"
	"carrel.dev/carrel/client/screens/register"
	"carrel.dev/carrel/client/screens/setup"
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
	// data-screen names the fixture's route (FIXTURE_SCREEN). The screen's
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
		Frames: []string{"s-auth-12"},
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
}
