package mailbox

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"slices"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The names of the trace's steps (s-mailbox-25): what discovery asked —
// «srv», «autoconfig» —, then what signing in answered.
const (
	stepAddress    = "address"
	stepSRV        = "srv"
	stepAutoconfig = "autoconfig"
	stepIMAP       = "imap"
	stepSMTP       = "smtp"
)

// ErrRefused is the IMAP server refusing the login and password: said as
// it is, the step stays in the trace (as at a DAV connection, discover.Refused).
var ErrRefused = errors.New("mailbox: the server refused the login and password")

// ErrClosed is a provider that closed signing in by password (Microsoft):
// such a mailbox does not connect, and nothing is asked of its servers (M-3).
var ErrClosed = errors.New("mailbox: the provider closed signing in by password")

// Input is what the connection's form says: the address, the login (empty —
// the address), the password, the servers typed by hand — an empty Host is
// one to discover — and an accepted certificate.
type Input struct {
	Address, Login, Password string
	IMAP, SMTP               sealed.MailServer
	CertFingerprint          string
}

// Result is what the check found: the servers that answered, the special
// folders by role and the roles the server did not announce, which the
// person is asked once (M-3, s-mailbox-24), and why sending does not work,
// when it does not — the mailbox is kept, reading works (s-mailbox-25).
type Result struct {
	IMAP, SMTP sealed.MailServer
	// MaxLetter is the largest letter the SMTP server takes, from SIZE in
	// its answer to EHLO (limits.md: sending from one's own mailbox has no
	// ceiling of Carrel's); 0 — it said none.
	MaxLetter   int64
	Login       string
	Folders     sealed.MailFolders
	Unannounced []string // roles: "sent", "drafts", "trash", "archive", "junk"
	All         []string // every folder, for the question
	SendRefused string
	KeepsSent   bool
}

// CanSend says whether letters can go from the mailbox checked.
func (r Result) CanSend() bool { return r.SMTP.Host != "" && r.SendRefused == "" }

// Check finds the servers of a mailbox and signs in to them: IMAP — the
// login, then the folders and their roles (SPECIAL-USE, RFC 6154); SMTP —
// EHLO and AUTH, nothing sent (M-3). Every step goes into the trace with
// what the server answered. IMAP that does not let in is the refusal: no
// mailbox. SMTP that does not is not: the result says why, and reading
// without sending is kept.
func (p *Prober) Check(ctx context.Context, in Input) (*Result, *view.DiagnosticTrace, error) {
	trace := &view.DiagnosticTrace{}
	step := func(name, detail string, ok bool) {
		st := view.CheckBad
		if ok {
			st = view.CheckOK
		}
		trace.Steps = append(trace.Steps, view.TraceStep{Name: view.Text(name), State: st, Detail: view.Text(detail)})
	}
	provider, known := ProviderOf(in.Address)
	if known && provider.Closed {
		step(stepAddress, provider.Name+" closed signing in by password", false)
		return nil, trace, ErrClosed
	}
	login := in.Login
	var found Found
	if in.IMAP.Host == "" || in.SMTP.Host == "" {
		found = p.Discover(ctx, in.Address, step)
	}
	if login == "" {
		login = found.Login
	}
	if login == "" {
		login = in.Address
	}
	imaps := found.IMAP
	if in.IMAP.Host != "" {
		imaps = []sealed.MailServer{in.IMAP}
	}
	smtps := found.SMTP
	if in.SMTP.Host != "" {
		smtps = []sealed.MailServer{in.SMTP}
	}

	res := &Result{Login: login, KeepsSent: known && provider.KeepsSent}
	var imapErr error
	for _, srv := range imaps {
		folders, said, err := p.signIn(ctx, srv, login, in.Password, in.CertFingerprint)
		if err != nil {
			step(stepIMAP, said, false)
			imapErr = err
			if errors.Is(err, ErrRefused) {
				break // the password is wrong there; not tried on the guesses
			}
			continue
		}
		step(stepIMAP, said, true)
		res.IMAP, imapErr = srv, nil
		res.Folders, res.Unannounced, res.All = roles(folders)
		break
	}
	if res.IMAP.Host == "" {
		trace.Stopped = "Discovery stopped at IMAP."
		if imapErr == nil {
			imapErr = errors.New("mailbox: no IMAP server to sign in to")
		}
		return nil, trace, imapErr
	}

	var smtpErr error
	for _, srv := range smtps {
		said, size, err := p.checkSMTP(ctx, srv, login, in.Password, in.CertFingerprint)
		if err != nil {
			step(stepSMTP, said, false)
			smtpErr = err
			continue
		}
		step(stepSMTP, said, true)
		res.SMTP, res.MaxLetter, smtpErr = srv, size, nil
		break
	}
	switch {
	case res.SMTP.Host != "":
	case smtpErr != nil:
		res.SendRefused = smtpErr.Error()
		if len(smtps) > 0 {
			res.SMTP = smtps[0]
		}
	default:
		res.SendRefused = "No SMTP server was found for this address."
	}
	trace.OK = true
	return res, trace, nil
}

// signIn opens IMAP on srv through the guard, signs in and lists the
// folders with their roles. said is the step of the trace as the frame
// writes it (s-mailbox-25): the server, how it was reached and whether its
// certificate matched; then what LOGIN answered and what the server can.
func (p *Prober) signIn(ctx context.Context, srv sealed.MailServer, login, password, pin string) ([]*imap.ListData, string, error) {
	said := reached(srv, pin)
	c, err := p.dialIMAP(ctx, srv, pin)
	if err != nil {
		return nil, said + " → " + short(err), err
	}
	defer c.Close()
	if err := c.Login(login, password).Wait(); err != nil {
		var ie *imap.Error
		if errors.As(err, &ie) && ie.Type == imap.StatusResponseTypeNo {
			return nil, said + "\nLOGIN → NO " + ie.Text, fmt.Errorf("%w: %s", ErrRefused, ie.Text)
		}
		return nil, said + "\nLOGIN → " + short(err), err
	}
	said += "\nLOGIN → OK · CAPABILITY " + capsSaid(c.Caps())
	opts := &imap.ListOptions{}
	if c.Caps().Has(imap.CapSpecialUse) {
		opts.ReturnSpecialUse = true
	}
	list, err := c.List("", "*", opts).Collect()
	if err != nil {
		return nil, said + "\nLIST → " + short(err), err
	}
	_ = c.Logout().Wait()
	return list, said, nil
}

// reached is how a server was reached, in the trace's words:
// «mail.office.example:993  TLS · certificate matches»; STARTTLS says its
// certificate on the line of the command itself.
func reached(srv sealed.MailServer, pin string) string {
	s := fmt.Sprintf("%s:%d  %s", srv.Host, srv.Port, securitySaid(srv.Security))
	if srv.Security == sealed.MailTLS {
		s += " · " + certSaid(pin)
	}
	return s
}

// securitySaid is a server's security as a screen says it: «TLS»,
// «STARTTLS», «plain text».
func securitySaid(security string) string {
	switch security {
	case sealed.MailTLS:
		return "TLS"
	case sealed.MailSTARTTLS:
		return "STARTTLS"
	}
	return "plain text"
}

// SecuritySaid is a server's security as the screens say it.
func SecuritySaid(security string) string { return securitySaid(security) }

// certSaid is what the trace says of the certificate: it matched the host's
// name, or it is the one the person accepted before (F-13).
func certSaid(pin string) string {
	if pin != "" {
		return "certificate accepted before"
	}
	return "certificate matches"
}

// capsSaid is what an IMAP server can, its version first, the rest in order.
func capsSaid(caps imap.CapSet) string {
	var first, rest []string
	for c := range caps {
		if strings.HasPrefix(string(c), "IMAP4") {
			first = append(first, string(c))
		} else {
			rest = append(rest, string(c))
		}
	}
	slices.Sort(first)
	slices.Sort(rest)
	return strings.Join(append(first, rest...), " ")
}

// dialIMAP is an IMAP client on srv: implicit TLS, STARTTLS, or plain text to
// a host the guard excepts only.
func (p *Prober) dialIMAP(ctx context.Context, srv sealed.MailServer, pin string) (*imapclient.Client, error) {
	if srv.Security == sealed.MailPlain && !p.Guard.MailPlainAllowed(srv.Host) {
		return nil, dav.ErrMailPlain
	}
	conn, err := p.Dial(ctx, srv.Host, srv.Port, srv.Security == sealed.MailTLS, pin)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if srv.Security == sealed.MailSTARTTLS {
		c, err := imapclient.NewStartTLS(conn, &imapclient.Options{TLSConfig: p.Guard.MailTLSConfig(srv.Host, pin)})
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return c, nil
	}
	return imapclient.New(conn, nil), nil
}

// checkSMTP says EHLO and AUTH to srv, sends nothing and says goodbye. The
// error is the server's answer as it is: «not sent» says nothing (M-7).
// said is the step of the trace, a line a command (s-mailbox-25); size is
// what SIZE of EHLO allows, 0 when it says nothing.
func (p *Prober) checkSMTP(ctx context.Context, srv sealed.MailServer, login, password, pin string) (said string, size int64, err error) {
	said = reached(srv, pin)
	if srv.Security == sealed.MailPlain && !p.Guard.MailPlainAllowed(srv.Host) {
		return said + " → " + dav.ErrMailPlain.Error(), 0, dav.ErrMailPlain
	}
	conn, err := p.Dial(ctx, srv.Host, srv.Port, srv.Security == sealed.MailTLS, pin)
	if err != nil {
		return said + " → " + short(err), 0, errors.New(short(err))
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, srv.Host)
	if err != nil {
		_ = conn.Close()
		return said + " → " + err.Error(), 0, err
	}
	defer c.Close()
	if err := c.Hello(helloName); err != nil {
		return said + "  EHLO → " + err.Error(), 0, err
	}
	// What EHLO offered that matters here: STARTTLS where it is the way in,
	// and the largest letter.
	ehlo := func() string {
		var offered []string
		if ok, _ := c.Extension("STARTTLS"); ok && srv.Security == sealed.MailSTARTTLS {
			offered = append(offered, "STARTTLS")
		}
		size = 0
		if ok, param := c.Extension("SIZE"); ok {
			size, _ = strconv.ParseInt(strings.TrimSpace(param), 10, 64)
			offered = append(offered, strings.TrimSpace("SIZE "+param))
		}
		return strings.TrimSpace("EHLO → 250 " + strings.Join(offered, " · "))
	}
	if srv.Security == sealed.MailSTARTTLS {
		said += "  " + ehlo()
		if ok, _ := c.Extension("STARTTLS"); !ok {
			// Announced and not carried out, or not announced: never plain
			// text instead (S-5.1).
			err := errors.New("the server offers no STARTTLS")
			return said + "\nSTARTTLS → " + err.Error(), 0, err
		}
		if err := c.StartTLS(p.Guard.MailTLSConfig(srv.Host, pin)); err != nil {
			return said + "\nSTARTTLS → " + err.Error(), 0, err
		}
		said += "\nSTARTTLS → 220 · " + certSaid(pin)
		// After STARTTLS the client says EHLO again: what it offers now.
		ehlo()
	} else {
		said += "\n" + ehlo()
	}
	if ok, _ := c.Extension("AUTH"); !ok {
		err := errors.New("the server offers no AUTH")
		return said + "\nAUTH → " + err.Error(), size, err
	}
	if err := c.Auth(smtp.PlainAuth("", login, password, srv.Host)); err != nil {
		return said + "\nAUTH PLAIN → " + err.Error(), size, err
	}
	return said + "\nAUTH PLAIN → 235 · nothing sent", size, c.Quit()
}

// helloName is what Carrel calls itself in EHLO: no name of the instance
// and no address of it (M-7: nothing of the instance in the letter's way).
const helloName = "carrel.invalid"

// roleNames are the folders a server that announces no roles is offered by
// name, for each role (M-3: «предлагает папку по имени»).
var roleNames = map[string][]string{
	"sent":    {"sent", "sent items", "sent messages", "sent mail", "отправленные"},
	"drafts":  {"drafts", "черновики"},
	"trash":   {"trash", "deleted items", "deleted messages", "bin", "корзина", "удаленные", "удалённые"},
	"archive": {"archive", "archives", "архив"},
	"junk":    {"junk", "spam", "junk e-mail", "спам"},
}

// roleOrder is the order the roles are asked and said in (s-mailbox-24).
var roleOrder = []string{"sent", "drafts", "trash", "archive", "junk"}

// roles are the special folders by the server's attributes, the roles it
// did not announce — each with the folder its name offers, when there is
// one — and every folder, for the question.
func roles(list []*imap.ListData) (sealed.MailFolders, []string, []string) {
	var f sealed.MailFolders
	var all []string
	set := func(role, name string) {
		switch role {
		case "sent":
			f.Sent = name
		case "drafts":
			f.Drafts = name
		case "trash":
			f.Trash = name
		case "archive":
			f.Archive = name
		case "junk":
			f.Junk = name
		}
	}
	get := func(role string) string {
		return map[string]string{"sent": f.Sent, "drafts": f.Drafts, "trash": f.Trash, "archive": f.Archive, "junk": f.Junk}[role]
	}
	attrRole := map[imap.MailboxAttr]string{imap.MailboxAttrSent: "sent", imap.MailboxAttrDrafts: "drafts",
		imap.MailboxAttrTrash: "trash", imap.MailboxAttrArchive: "archive", imap.MailboxAttrJunk: "junk"}
	for _, d := range list {
		noSelect := false
		for _, a := range d.Attrs {
			if a == imap.MailboxAttrNoSelect || a == imap.MailboxAttrNonExistent {
				noSelect = true
			}
		}
		if noSelect {
			continue
		}
		all = append(all, d.Mailbox)
		for _, a := range d.Attrs {
			if role, ok := attrRole[a]; ok && get(role) == "" {
				set(role, d.Mailbox)
			}
		}
	}
	var unannounced []string
	for _, role := range roleOrder {
		if get(role) != "" {
			continue
		}
		unannounced = append(unannounced, role)
		for _, name := range all {
			leaf := strings.ToLower(name)
			if i := strings.LastIndexAny(leaf, "/."); i >= 0 {
				leaf = leaf[i+1:]
			}
			if slices.Contains(roleNames[role], leaf) {
				set(role, name)
				break
			}
		}
	}
	return f, unannounced, all
}
