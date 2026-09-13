package discover

import "testing"

// go-webdav's FindAddressBooks never asks for rights, so every address book
// used to come back writable. writable reads what the home PROPFIND returns.
func TestWritableReadsPrivilegeSet(t *testing.T) {
	cases := []struct {
		name            string
		inner           string
		canWrite, known bool
	}{
		{"not sent", "", false, false},
		{"read only", `<D:privilege><D:read/></D:privilege>`, false, true},
		{"write", `<D:privilege><D:read /></D:privilege><D:privilege><D:write /></D:privilege>`, true, true},
		{"write-content, lower-case prefix", `<d:privilege><d:read/></d:privilege><d:privilege><d:write-content/></d:privilege>`, true, true},
		{"all", `<privilege xmlns="DAV:"><all/></privilege>`, true, true},
	}
	for _, c := range cases {
		canWrite, known := writable([]byte(c.inner))
		if canWrite != c.canWrite || known != c.known {
			t.Errorf("%s: writable = %v, %v; want %v, %v", c.name, canWrite, known, c.canWrite, c.known)
		}
	}
}
