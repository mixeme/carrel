package web

import (
	"errors"

	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

func unreadableCardsNotice(partial *card.PartialRead) view.Notice {
	n := view.Notice{
		Role:    view.NoticeAlert,
		Title:   text(plural(len(partial.Records), "record") + " could not be read"),
		Body:    "The server returned them, but they could not be read as vCard. They are left as they are; other clients may still open them.",
		Details: &view.Diagnostic{Summary: "Unreadable records"},
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
