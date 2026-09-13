package discover

import "carrel.dev/carrel/server/view"

// Collection is one discovered DAV collection.
type Collection struct {
	Path        string
	DisplayName string
	Kinds       []view.Kind
	ReadOnly    bool
	Components  []string
	ChangeTag   string
	// Colour is calendar-color as the server sent it; empty when it sent none
	// (C-10 then derives one from the path).
	Colour          string
	MaxResourceSize int64
}

// Result is a successful discovery outcome.
type Result struct {
	BaseURL         string
	Principal       string
	CalendarHome    string
	AddressBookHome string
	Collections     []Collection
	FileOnly        bool
}

// Diff is the proposed change set for re-discovery (S-6.1).
type Diff struct {
	Added   []Collection
	Removed []string // collection paths no longer found
}
