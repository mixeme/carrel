// Package screens is where the screens of the Carrel client go: one screen
// per view model of C-6 a page or sheet of C-5 answers with, drawn by the
// frames of docs/visual/carrel-ui-mockups.html (U-6) — the screen of a model
// draws all its states. Typecase is connected by a replace onto the
// neighbouring clone (U-5.1, docs/plan/client.md); the screens built are
// listed in All (screens.go), each with its view model and its frames: the
// renderer picks a screen by the type of the model the server hands it.
//
// The rules a screen is written by, and what checks them (U-15.2):
//
//   - U-5.1. A screen starts with the draft cmd/frame2go of Typecase prints
//     for its frame, committed as printed, alone; the second commit puts the
//     view model's data where the frame's data was. The difference between
//     the two is all the author's work. The guard of kinship prints the draft
//     again and holds the screen's skeleton against it; the frame comparison
//     holds the screen, drawn on every fixture of its view model, against its
//     frame (carrel.dev/carrel/client/guard, screens_test.go).
//   - U-5.2. A screen is calls of Typecase widgets with the view model's data
//     in their fields, and nothing else: no markup, no style, no script, no
//     name of the library's space written by hand. The guards of
//     carrel.dev/carrel/client/guard hold it: the packages here import only
//     carrel.dev/carrel/server/view, carrel.dev/typecase/widget and the
//     standard packages strconv and slices; no string or rune constant here
//     holds markup, «tc-», «class=», «style=» or «data-tc»; the client module
//     holds no stylesheet, page, script or picture file.
//   - U-5.5. A frame that cannot be built from the widgets is a defect of the
//     screen, not a gap of the library. The author stops and writes; a row in
//     the list «ждёт библиотеку» is opened by the owner only, and the place in
//     code is marked «ждёт библиотеку: U-5.5 #N».
//   - U-5.7. Everything common and repeated is Typecase's. What is purely
//     specific to one screen is a row of the registry of screen specifics,
//     opened by the owner, and its place in code is marked «специфика экрана:
//     U-5.7 #N» — the only thing that excuses a finding of the guards above.
//     Marks and rows are held against each other both ways.
//
// A screen shares nothing with another screen through a package of its own
// here: what two screens share is common, and common is Typecase's.
package screens
