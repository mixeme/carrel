package mailbox

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"slices"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The names of the trace's steps (s-mailbox-25): what discovery asked, then
// what signing in answered.
const (
	stepIMAP    = "IMAP"
	stepFolders = "folders"
	stepSMTP    = "SMTP"
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
	IMAP, SMTP  sealed.MailServer
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
		step("address", provider.Name+" closed signing in by password", false)
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
		folders, err := p.signIn(ctx, srv, login, in.Password, in.CertFingerprint)
		if err != nil {
			step(stepIMAP, serverSaid(srv)+" → "+short(err), false)
			imapErr = err
			if errors.Is(err, ErrRefused) {
				break // the password is wrong there; not tried on the guesses
			}
			continue
		}
		step(stepIMAP, serverSaid(srv)+" → signed in as "+login, true)
		res.IMAP, imapErr = srv, nil
		res.Folders, res.Unannounced, res.All = roles(folders)
		step(stepFolders, foldersSaid(res.Folders, res.Unannounced), true)
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
		if err := p.checkSMTP(ctx, srv, login, in.Password, in.CertFingerprint); err != nil {
			step(stepSMTP, serverSaid(srv)+" → "+err.Error(), false)
			smtpErr = err
			continue
		}
		step(stepSMTP, serverSaid(srv)+" → AUTH accepted, nothing sent", true)
		res.SMTP, smtpErr = srv, nil
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
// folders with their roles.
func (p *Prober) signIn(ctx context.Context, srv sealed.MailServer, login, password, pin string) ([]*imap.ListData, error) {
	c, err := p.dialIMAP(ctx, srv, pin)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if err := c.Login(login, password).Wait(); err != nil {
		var ie *imap.Error
		if errors.As(err, &ie) && ie.Type == imap.StatusResponseTypeNo {
			return nil, fmt.Errorf("%w: %s", ErrRefused, ie.Text)
		}
		return nil, err
	}
	opts := &imap.ListOptions{}
	if c.Caps().Has(imap.CapSpecialUse) {
		opts.ReturnSpecialUse = true
	}
	list, err := c.List("", "*", opts).Collect()
	if err != nil {
		return nil, err
	}
	_ = c.Logout().Wait()
	return list, nil
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
func (p *Prober) checkSMTP(ctx context.Context, srv sealed.MailServer, login, password, pin string) error {
	if srv.Security == sealed.MailPlain && !p.Guard.MailPlainAllowed(srv.Host) {
		return dav.ErrMailPlain
	}
	conn, err := p.Dial(ctx, srv.Host, srv.Port, srv.Security == sealed.MailTLS, pin)
	if err != nil {
		return errors.New(short(err))
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, srv.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()
	if err := c.Hello(helloName); err != nil {
		return err
	}
	if srv.Security == sealed.MailSTARTTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			// Announced and not carried out, or not announced: never plain
			// text instead (S-5.1).
			return errors.New("the server offers no STARTTLS")
		}
		if err := c.StartTLS(p.Guard.MailTLSConfig(srv.Host, pin)); err != nil {
			return err
		}
	}
	if ok, _ := c.Extension("AUTH"); !ok {
		return errors.New("the server offers no AUTH")
	}
	if err := c.Auth(smtp.PlainAuth("", login, password, srv.Host)); err != nil {
		return err
	}
	return c.Quit()
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

// foldersSaid is the trace's words of the roles: those announced, and those
// offered by name.
func foldersSaid(f sealed.MailFolders, unannounced []string) string {
	named := map[string]string{"sent": f.Sent, "drafts": f.Drafts, "trash": f.Trash, "archive": f.Archive, "junk": f.Junk}
	var out []string
	for _, role := range roleOrder {
		name := named[role]
		switch {
		case name == "":
			out = append(out, role+": none")
		case slices.Contains(unannounced, role):
			out = append(out, role+": "+name+" (by name)")
		default:
			out = append(out, role+": "+name)
		}
	}
	return strings.Join(out, ", ")
}
