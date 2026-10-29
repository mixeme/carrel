package card

import (
	"fmt"
	"strings"
)

// Unreadable names one resource which the server returned but could not be
// decoded. The resource is left untouched.
type Unreadable struct {
	Path string
	Err  error
}

// PartialRead accompanies the healthy cards of an incomplete read. Callers
// must either display the diagnostic with those cards or refuse the operation;
// they must not cache or export the result as a complete address book.
type PartialRead struct {
	Records []Unreadable
}

func (e *PartialRead) Error() string {
	lines := make([]string, 0, len(e.Records))
	for _, r := range e.Records {
		lines = append(lines, fmt.Sprintf("%s: %v", r.Path, r.Err))
	}
	return "Unreadable vCard records: " + strings.Join(lines, "; ")
}
