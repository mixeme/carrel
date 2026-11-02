package web

import (
	"errors"

	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// unreadableCardsNotice is the cards the server returned and Carrel could not
// read (the owner's words of 02.11.2026): «1 record not shown: invalid
// vCard.» · «Unchanged on the server.», each by path and parser error.
func unreadableCardsNotice(partial *card.PartialRead) view.Notice {
	n := view.Notice{
		Role:    view.NoticeAlert,
		Title:   text(plural(len(partial.Records), "record") + " not shown: invalid vCard."),
		Body:    "Unchanged on the server.",
		Details: &view.Diagnostic{Summary: "Not shown"},
	}
	for _, b := range partial.Records {
		n.Details.Lines = append(n.Details.Lines, text(b.Path+" · "+b.Err.Error()))
	}
	return n
}

// Export uses the explicit omission flow of unreadable calendar resources.
// Other errors remain fatal, and incomplete reads are not cached.
func unreadableCards(err error) ([]cal.Unreadable, error) {
	var partial *card.PartialRead
	if !errors.As(err, &partial) {
		return nil, err
	}
	bad := make([]cal.Unreadable, 0, len(partial.Records))
	for _, r := range partial.Records {
		bad = append(bad, cal.Unreadable{Path: r.Path, Err: r.Err})
	}
	return bad, nil
}
