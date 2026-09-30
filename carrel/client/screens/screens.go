package screens

import (
	"carrel.dev/carrel/client/screens/about"
	"carrel.dev/carrel/client/screens/login"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

// Screen is one screen of the client: the address of C-5 it draws, the frame
// it is drawn by, and how it is drawn from its view model.
type Screen struct {
	// Route is the address as C-5 writes it: "/login",
	// "/app/contacts/{account}/{col}".
	Route string
	// Frame is the data-frame of the route's frame in the mockup — the frame
	// whose data-screen names the route. The screen's draft is printed from
	// it, and the screen is checked against it on every fixture of the route.
	Frame string
	// Model is a new view model of the route, for a fixture to decode into.
	Model func() any
	// Draw draws the screen from a view model Model made.
	Draw func(model any) Root
	// Title is the document's title, from a view model Model made: the
	// model's Title, which a page outside the shell draws as the mark, not as
	// words (U-6.14).
	Title func(model any) string
}

// Root is what a screen draws: one of Typecase's roots — a shell, a page
// without a shell, an overlay. The interface is the library's and closed, so
// a screen cannot hand the page anything else (U-15.2, 1).
type Root = widget.Root

// All are the screens of the client, one per address of C-5 built. An
// address of C-5 is either here or in the list «ждёт библиотеку» of U-5.5,
// not both (U-15.2, 5).
var All = []Screen{
	{
		Route: "/login",
		Frame: "s-auth-1",
		Model: func() any { return new(view.LoginPage) },
		Draw:  func(model any) Root { return login.Screen(model.(*view.LoginPage)) },
		Title: func(model any) string { return string(model.(*view.LoginPage).Title) },
	},
	{
		Route: "/about",
		Frame: "rest-3",
		Model: func() any { return new(view.AboutPage) },
		Draw:  func(model any) Root { return about.Screen(model.(*view.AboutPage)) },
		Title: func(model any) string { return string(model.(*view.AboutPage).Title) },
	},
}
