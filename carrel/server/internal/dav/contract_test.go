package dav

// Nothing in the module refers to Transport as a type yet — the request layer
// and the providers that will are not written. Without this, a change to
// Client could quietly stop satisfying the S-5 surface and nothing would say
// so until those packages exist.
var (
	_ Transport    = (*Client)(nil)
	_ Creator      = (*Client)(nil)
	_ CalReporter  = calReports{}
	_ CardReporter = cardReports{}
)
