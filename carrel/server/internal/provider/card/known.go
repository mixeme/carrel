package card

// Known vCard properties that C-6 already has a place for (Name, Fields, Tags).
// Adding a property is four places (S-7.2); listing it here alone hides it from Foreign.
var knownProps = map[string]bool{
	"FN": true, "N": true, "NICKNAME": true, "EMAIL": true, "TEL": true,
	"ADR": true, "ORG": true, "TITLE": true, "ROLE": true, "URL": true,
	"NOTE": true, "BDAY": true, "ANNIVERSARY": true, "IMPP": true,
	"CATEGORIES": true, "LANG": true, "TZ": true, "GEO": true,
	"RELATED": true, "GENDER": true,
}

var skipForeign = map[string]bool{
	"PHOTO": true, "UID": true, "VERSION": true, "PRODID": true, "REV": true,
}
