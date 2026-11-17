package document

import (
	"carrel.dev/carrel/client/screens"
	"carrel.dev/typecase/widget"
)

// withDrop is a page of the shell with its drop sheet among its own closed
// sheets (Shell.Dialogs), as the keys' one (Typecase's notes 60, 136): the
// library's script opens it while a file is dragged over the page and sends
// the form it names with the file. The frames draw it alone (s-io-1,
// -20…-23), so the screen's comparison with its frame does not see it.
func withDrop(root screens.Root, scr screens.Screen, model any) screens.Root {
	shell, ok := root.(widget.Shell)
	if !ok || scr.Drop == nil {
		return root
	}
	sheet, _, ok := scr.Drop(model)
	if !ok {
		return root
	}
	shell.Dialogs = append(append([]widget.Scrim(nil), shell.Dialogs...), sheet)
	return shell
}

// dropForm is the form of a page's drop sheet when the screen prints none
// of its own that takes the file (the list's import).
func dropForm(scr screens.Screen, model any) (widget.PostForm, bool) {
	if scr.Drop == nil {
		return widget.PostForm{}, false
	}
	_, form, ok := scr.Drop(model)
	if !ok || form == nil {
		return widget.PostForm{}, false
	}
	return *form, true
}
