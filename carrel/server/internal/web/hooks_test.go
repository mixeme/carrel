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
