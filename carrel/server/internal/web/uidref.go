package web

import "strings"

// A record names another by UID in RELATED-TO of a calendar, and in
// RELATED, MEMBER or Apple's X-ADDRESSBOOKSERVER-MEMBER of a card — bare,
// or as a urn:uuid: URI (RFC 6350 §6.6.6, §6.7.5; RFC 4122 §3). Every
// reading of such a value goes through splitUIDRef, so the form is decided
// in one place: the import keeps it when it points a link elsewhere
// (linkPatch), and Mentions (mockup 7.23, deferred) will read @-links and
// RELATED of a card by it.

const urnUUID = "urn:uuid:"

// splitUIDRef reads a value that names a record by UID as the form it was
// written in and the UID: prefix is "urn:uuid:" in the case it was written
// («URN:UUID:» stays capitals), or "" for a bare UID. The UID is as written:
// no case is folded, because a UID is compared as a string (RFC 5545
// §3.8.4.7) and a urn:uuid:'s hex may come in either case. A value that is
// the prefix alone names nothing of its own and is read as a bare value.
func splitUIDRef(v string) (prefix, uid string) {
	if len(v) > len(urnUUID) && strings.EqualFold(v[:len(urnUUID)], urnUUID) {
		return v[:len(urnUUID)], v[len(urnUUID):]
	}
	return "", v
}
