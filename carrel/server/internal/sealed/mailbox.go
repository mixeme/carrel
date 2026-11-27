package sealed

import "strings"

// Mailbox is a person's own mailbox (carrel-mailbox.md M-3): read over IMAP,
// written from over SMTP, in the same sealed blob as the DAV connections. It
// is a slice of its own, not a Connection of another kind: Blob.Connections
// is walked in dozens of places, and each of them expects DAV
// (architecture.md, «Почта — подготовка»). Blob.Connection does not find a
// mailbox; Blob.Mailbox does.
type Mailbox struct {
	ID string `json:"id"` // "m-…"
	// Label is the first half of the source's stamp («home · Inbox»), one
	// word, from the same space as the connections' labels (labelTaken).
	Label string `json:"label"`
	// Address is the mailbox's address: its From, and where discovery
	// starts (M-3).
	Address string `json:"address"`
	// Colour is chosen at connecting, from the collections' palette: IMAP
	// has no such property, so it lives here only (M-3).
	Colour string `json:"colour,omitempty"`
	// FromName is the name in From, by default the profile's (M-3, M-7);
	// Signature the plain-text signature of its letters, empty for none.
	FromName  string `json:"fromName,omitempty"`
	Signature string `json:"signature,omitempty"`
	Login     string `json:"login"`
	// Credentials are an object, so that an OAuth token stands beside the
	// password later without a change of format (M-3).
	Credentials MailCredentials `json:"credentials"`
	IMAP        MailServer      `json:"imap"`
	// SMTP is where letters go; its Host empty — sending was never found.
	SMTP MailServer `json:"smtp"`
	// SendRefused is what the SMTP server answered when the check of
	// sending failed: the mailbox is kept, reading works, and a letter from
	// it is off with this reason (M-3, M-7). Empty — sending works.
	SendRefused string `json:"sendRefused,omitempty"`
	// SendTrace is the SMTP step of that check, as the trace said it: the
	// card shows it under the refusal (s-mailbox-26).
	SendTrace string `json:"sendTrace,omitempty"`
	// MaxLetter is the largest letter the SMTP server takes, from SIZE of
	// EHLO: «up to 25 MB» on the card; 0 — it said none.
	MaxLetter int64 `json:"maxLetter,omitempty"`
	// ServersFrom is where the servers came from, in the screen's words:
	// «found by SRV», «from autoconfig.yandex.ru», «guessed from the
	// domain», «typed by hand» (s-mailbox-22, -27).
	ServersFrom string `json:"serversFrom,omitempty"`
	// Folders are the special folders by role, as the server announced
	// them (SPECIAL-USE, RFC 6154) or as the person answered once (M-3).
	Folders MailFolders `json:"folders,omitzero"`
	// ServerKeepsSent: the server puts the copy of a sent letter into Sent
	// itself (Gmail and the like); Carrel then puts none (M-3, M-7).
	ServerKeepsSent bool `json:"serverKeepsSent,omitempty"`
	// Autosave: the draft of a letter goes to Drafts by itself every minute
	// (Typecase's note 226, s-mailbox-27); off by default.
	Autosave bool `json:"autosave,omitempty"`
	// CertFingerprint is the accepted leaf certificate of a server the
	// system does not trust, as at a DAV connection (F-13).
	CertFingerprint string `json:"certFingerprint,omitempty"`
}

// MailCredentials are how Carrel signs in to a mailbox. Only the password
// today (M-3: AUTH PLAIN or LOGIN over TLS); an OAuth token comes as another
// Method beside it.
type MailCredentials struct {
	Method   string `json:"method"` // MailPassword
	Password string `json:"password,omitempty"`
}

// MailPassword is the only method of signing in to a mailbox today.
const MailPassword = "password"

// MailServer is one server of a mailbox: where and how.
type MailServer struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
	// Security is MailTLS (993, 465) or MailSTARTTLS (143, 587). Plain text
	// only to the hosts the guard lets through without TLS (S-5.1).
	Security string `json:"security,omitempty"`
}

// The security of a mail server's connection.
const (
	MailTLS      = "tls"
	MailSTARTTLS = "starttls"
	MailPlain    = "plain"
)

// MailFolders are the special folders of a mailbox by role. A role without a
// folder is not a refusal: no Archive — no action «to archive», no Drafts —
// no button of a draft (M-3).
type MailFolders struct {
	Sent    string `json:"sent,omitempty"`
	Drafts  string `json:"drafts,omitempty"`
	Trash   string `json:"trash,omitempty"`
	Archive string `json:"archive,omitempty"`
	Junk    string `json:"junk,omitempty"`
	// Asked: the person answered the question of the roles the server did
	// not announce; it is asked once, as the attachment folder (S-20).
	Asked bool `json:"asked,omitempty"`
	// Chosen are the roles the person set («you chose it», s-mailbox-26); a
	// role set and not chosen was announced by the server. What the folders'
	// names suggest is offered in the question only: Skip leaves the role
	// empty (s-mailbox-24).
	Chosen []string `json:"chosen,omitempty"`
}

// CanSend says whether letters can go from the mailbox: a server to send to,
// and the check of sending did not fail.
func (m Mailbox) CanSend() bool { return m.SMTP.Host != "" && m.SendRefused == "" }

// Name is how the mailbox is called on a screen: its label, else its address.
func (m Mailbox) Name() string {
	if m.Label != "" {
		return m.Label
	}
	return m.Address
}

// Mailbox returns the mailbox with id.
func (b Blob) Mailbox(id string) (Mailbox, bool) {
	for _, m := range b.Mailboxes {
		if m.ID == id {
			return m, true
		}
	}
	return Mailbox{}, false
}

// MailboxByAddress returns the mailbox of an address, case aside.
func (b Blob) MailboxByAddress(address string) (Mailbox, bool) {
	for _, m := range b.Mailboxes {
		if strings.EqualFold(m.Address, address) {
			return m, true
		}
	}
	return Mailbox{}, false
}
