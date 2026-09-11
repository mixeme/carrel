package card

// Known vCard properties with named C-6 fields. PHOTO, UID, VERSION stay out
// of Foreign via skipForeign.
var knownProps = map[string]bool{
	"FN": true, "N": true, "NICKNAME": true, "EMAIL": true, "TEL": true,
	"ADR": true, "ORG": true, "TITLE": true, "ROLE": true, "URL": true,
	"NOTE": true, "BDAY": true, "ANNIVERSARY": true, "IMPP": true,
	"CATEGORIES": true, "LANG": true, "TZ": true, "GEO": true,
	"RELATED": true, "GENDER": true, "MEMBER": true, "LOGO": true,
	"SOUND": true, "KEY": true, "FBURL": true, "CALADRURI": true, "CALURI": true,
	"KIND": true, "XML": true, "SOURCE": true, "CLIENTPIDMAP": true,
}

var skipForeign = map[string]bool{
	"PHOTO": true, "UID": true, "VERSION": true, "PRODID": true, "REV": true,
}
