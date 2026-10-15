package web

import (
	"errors"
	"time"
)

// RefuseApart makes the event apart of the day at fail as a server that did
// not take it (s-agenda-24) until the function it returns is called.
func RefuseApart(at time.Time, why string) (restore func()) {
	apartRefused = func(day time.Time) error {
		if day.Equal(at) {
			return errors.New(why)
		}
		return nil
	}
	return func() { apartRefused = nil }
}

// DrawSheets turns on the pages' way to the sheets the client does not draw
// yet (askOnPages) until the function it returns is called.
func DrawSheets() (restore func()) {
	was := askOnPages
	askOnPages = true
	return func() { askOnPages = was }
}
