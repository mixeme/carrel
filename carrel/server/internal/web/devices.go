package web

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"rsc.io/qr"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-23: a new device. Everything discovery found, handed out four ways: an
// Apple profile, the addresses for DAVx5 — also as a code — the addresses
// for Thunderbird, and every address for anything else. The profile holds
// passwords, so it is the most dangerous file the service makes: it is
// issued only inside the unlock window (S-10.7), through a link that opens
// once and lives conf.DeviceProfileLinkTTL, and it is built in memory when
// the link is followed — never written to disk, never cached. A connection
// the person left out goes nowhere.

var deviceTabs = []view.DeviceTab{view.TabApple, view.TabAndroid, view.TabThunderbird, view.TabOther}

var deviceTabLabels = map[view.DeviceTab]string{
	view.TabApple: "Apple", view.TabAndroid: "Android", view.TabThunderbird: "Thunderbird", view.TabOther: "Anything else",
}

func deviceTab(r *http.Request) view.DeviceTab {
	t := view.DeviceTab(r.FormValue("tab"))
	for _, have := range deviceTabs {
		if t == have {
			return t
		}
	}
	return view.TabApple
}

// davType is how a collection is reached: CalDAV, CardDAV or WebDAV.
func davType(col sealed.Collection) string {
	switch {
	case col.Has(view.KindContact):
		return "CardDAV"
	case col.Has(view.KindFile):
		return "WebDAV"
	default:
		return "CalDAV"
	}
}

// fullURL is the collection's address on its server, whole.
func fullURL(conn sealed.Connection, path string) string {
	u, err := url.Parse(conn.Server)
	if err != nil || u.Host == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		return strings.TrimSuffix(u.String(), "/") + "/" + path
	}
	return u.Scheme + "://" + u.Host + path
}

// onTab reports whether a collection is set up by hand on the tab: file
// collections only under "anything else" — Apple has no payload for them and
// Thunderbird does not mount a drive.
func onTab(tab view.DeviceTab, col sealed.Collection) bool {
	return tab == view.TabOther || !col.Has(view.KindFile)
}

// davx5Link is the address DAVx5 opens with the server and the login filled
// in (manual.davx5.com, "Integration"): davx5:// is HTTPS, caldav:// plain
// HTTP. The password is never in it — the manual advises against it, and a
// code on a screen is seen by whoever stands behind the person.
func davx5Link(conn sealed.Connection) string {
	u, err := url.Parse(conn.Server)
	if err != nil || u.Host == "" {
		return ""
	}
	scheme := "davx5"
	if u.Scheme == "http" {
		scheme = "caldav"
	}
	link := url.URL{Scheme: scheme, User: url.User(conn.Login), Host: u.Host, Path: u.Path}
	if link.Path == "" {
		link.Path = "/"
	}
	return link.String()
}

// qrImage is the code of text as an image address: a data: URI, so the page
// needs no route of its own (C-5) and nothing is kept.
func qrImage(text string) string {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return ""
	}
	code.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
}

// devicesPage is the page of one tab. Every account a tab sets up by hand
// carries its own credentials beside it (owner's decision 24.09.2026,
// s-plan-2, s-plan-4, s-plan-5): on Android a code, «Open in DAVx5» and the
// password under the code for each ticked account; on Thunderbird and
// «Anything else» the login and the password in the account's row. Nobody
// goes to Connections for a password. The password is shown inside the one
// unlock window (S-10.7) and every showing is journaled; outside it there
// are dots and Show, which opens the same sheet over this page. Apple has
// no password of its own: the profile is issued inside the window.
func (s *Server) devicesPage(sess *session.Session, tab view.DeviceTab, notices ...view.Notice) view.DevicesPage {
	blob := s.catalogOf(sess)
	key, live := sess.DataKey()
	unlocked := live && sess.UnlockUntil(s.now()) != nil
	page := view.DevicesPage{Shell: s.settingsShell(sess, "Set up a new device", "/app/settings/devices", "/app/settings/devices"), Tab: tab,
		Subtitle: "Everything discovery found — as one file or one code"}
	page.Notices = append(page.Notices, notices...)
	for _, t := range deviceTabs {
		// The tab shown now is on and stays a choice (C-6.2).
		page.Actions = append(page.Actions, view.Action{ID: "device.tab." + string(t), Label: text(deviceTabLabels[t]),
			Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(t)}}), Method: "GET", Enabled: true, On: t == tab})
	}
	if unlocked && tab == view.TabOther {
		// Under the rows while the window is open (s-plan-5).
		// The badge and Lock now in the head say how long (Typecase's
		// note 172).
		page.UnlockNote = join("Unlocked everywhere credentials are shown, not only here; every reveal goes to the audit log.")
	}
	here := "/app/settings/devices?tab=" + string(tab)
	unlock := s.unlockHref(unlockProfile, here)
	secret := func(conn sealed.Connection) view.SecretField {
		if unlocked {
			if password, err := s.users.RevealConnectionPassword(sess.AccountID(), key, conn.ID); err == nil {
				return view.SecretField{Shown: true, Value: text(password)}
			}
		}
		// The sheet says whose (s-profile-3).
		return view.SecretField{Reveal: view.Action{ID: "device.password", Label: "Show", Href: s.unlockHref(conn.ID, here), Method: "GET", Enabled: true}}
	}
	copyPassword := func(label string, shown bool) view.Action {
		a := view.Action{ID: "device.password.copy", Label: text(label), Method: "GET", Enabled: shown}
		if !shown {
			a.Reason = "Show the password first."
		}
		return a
	}
	profileAccounts := 0
	for _, conn := range blob.Connections {
		included := !blob.IsDeviceExcluded(conn.ID)
		acct := view.DeviceAccount{Title: text(connLabel(conn) + " · " + conn.Login), Label: text(connLabel(conn)), Included: included}
		// A server that refused the saved password goes in nowhere: its row
		// off with why (s-plan-1, -2, -4; Typecase's note 94).
		refused := s.passwordRefusedOf(sess, blob, conn)
		if refused {
			included = false
			acct.Included = false
			acct.Reason, acct.Refused = "The server refused the saved password.", "password refused"
		}
		dav := false
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !onTab(tab, col) {
				continue
			}
			u := fullURL(conn, col.Path)
			kind := view.KindEvent
			if len(col.Kinds) > 0 {
				kind = col.Kinds[0]
			}
			acct.Rows = append(acct.Rows, view.DeviceRow{
				Label: text(collectionLabel(col)), Colour: catalogColour(col), Kind: kind, Type: text(davType(col)), URL: text(u), Included: included, ReadOnly: col.ReadOnly,
				Copy: view.Action{ID: "device.copy", Label: "Copy", Href: u, Method: "GET", Enabled: true},
			})
			dav = dav || !col.Has(view.KindFile)
		}
		if len(acct.Rows) == 0 {
			continue
		}
		if refused && (tab == view.TabThunderbird || tab == view.TabOther) {
			// The account's row alone, without its password and its
			// addresses (s-plan-4, -5).
			acct.Rows, acct.Login, acct.Refused = nil, text(conn.Login), "the server refused the saved password"
			page.Accounts = append(page.Accounts, acct)
			continue
		}
		if dav && included {
			profileAccounts++
		}
		switch tab {
		case view.TabAndroid:
			// One code for each ticked account: DAVx5 makes an account per
			// code. The password is not in the code — a code on a screen is
			// seen by whoever stands behind — but under it.
			if cal, card := profileParts(blob, conn.ID); (cal || card) && included && davx5Link(conn) != "" {
				link := davx5Link(conn)
				acct.QR = qrImage(link)
				acct.Password = secret(conn)
				acct.Actions = []view.Action{
					{ID: "device.davx5", Label: "Open in DAVx5", Href: link, Method: "GET", Enabled: true},
					copyPassword("Copy", acct.Password.Shown),
				}
			}
		case view.TabThunderbird, view.TabOther:
			acct.Login = text(conn.Login)
			acct.Password = secret(conn)
			acct.Actions = []view.Action{copyPassword("Copy password", acct.Password.Shown)}
		}
		// The tick of the account (s-plan-1, -2): the profile and the codes
		// take an account whole. Thunderbird and anything else hand out
		// every address and tick nothing.
		if tab == view.TabApple || tab == view.TabAndroid {
			op, label := "exclude", "Leave out "+connLabel(conn)
			if !included {
				op, label = "include", "Include "+connLabel(conn)
			}
			acct.Toggle = &view.Action{
				ID: "device." + op, Label: text(label), Method: "POST", Enabled: true,
				Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {op}, "account": {conn.ID}}),
			}
			if refused {
				acct.Toggle.Enabled, acct.Toggle.Reason = false, acct.Reason
			}
		}
		page.Accounts = append(page.Accounts, acct)
	}

	switch tab {
	case view.TabApple:
		page.Warning = view.Notice{
			Role:  view.NoticeAlert,
			Title: "The profile carries passwords in cleartext.",
			// What to do with the file and why (Р-77, Typecase's note 167):
			// how long the link lives is on its card.
			Body: "Install it, then delete the file: otherwise it stays in Downloads and in the phone’s backup.",
		}
		gen := view.Action{ID: "device.profile", Label: "Generate profile", Method: "POST", Danger: true, Enabled: true,
			Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {"profile"}})}
		// The line above the button says why it asks (s-plan-6); inside the
		// unlock window there is none — the badge in the head says it
		// (s-plan-1, Р-77). A working button carries no reason (C-6.2).
		switch {
		case profileAccounts == 0:
			gen.Enabled, gen.Reason = false, "No included account has calendars or address books."
		case !unlocked:
			gen.Method, gen.Href = "GET", unlock
			page.Note = "Asks for your Carrel password first." // s-plan-6
		}
		page.Actions = append(page.Actions, gen)
		page.Link = s.profileLink(sess, tab)
	case view.TabAndroid:
		page.Warning = view.Notice{Role: view.NoticeInfo, Body: "One code for each ticked account. Scan it with the phone: DAVx5 opens with the address and the login filled in, and asks for the password shown under the code."}
	case view.TabThunderbird:
		page.Warning = view.Notice{Role: view.NoticeInfo, Body: thunderbirdNote}
	}
	// Anything else says nothing under its rows but the window (s-plan-5,
	// Нс44): a WebDAV address is what the person came for.
	return page
}

// passwordRefusedOf is whether conn's server refused the saved password at
// its last answer to this session: a reading of one of its collections
// refused as unauthorised, after the last that went through.
func (s *Server) passwordRefusedOf(sess *session.Session, blob sealed.Blob, conn sealed.Connection) bool {
	for _, col := range blob.CollectionsOf(conn.ID) {
		if r, ok := sess.ReadingOf(conn.ID, col.Path); ok && r.Unauthorized && !r.Refused.Before(r.At) {
			return true
		}
	}
	return false
}

// profileLink is the card of the link last issued while it is still to be
// followed (s-plan-1, Нс40): after a reload too, gone once it is followed,
// revoked or out of time.
func (s *Server) profileLink(sess *session.Session, tab view.DeviceTab) *view.DeviceLink {
	token := string(sess.Draft(session.DraftProfileLink))
	if token == "" {
		return nil
	}
	expires, ok := s.users.DeviceProfileLive(sess.AccountID(), token)
	if !ok {
		sess.SetDraft(session.DraftProfileLink, nil)
		return nil
	}
	return &view.DeviceLink{
		Title: text("Link ready until " + s.timeFor(sess).clock(expires)),
		Local: s.instanceLocal(),
		Hint:  "Opens once, for you signed in to Carrel.",
		Actions: []view.Action{
			{ID: "device.profile.download", Label: "Download profile", Icon: "i-download", Href: s.postQuery("/app/settings/devices/profile", url.Values{"t": {token}}), Method: "GET", Enabled: true},
			{ID: "device.profile.revoke", Label: "Revoke", Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {"revoke"}}), Method: "POST", Enabled: true, Danger: true},
		},
	}
}

func (s *Server) getSettingsDevices(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writePage(w, r, http.StatusOK, s.devicesPage(sess, deviceTab(r)))
}

// postSettingsDevices takes op=exclude or include with account=, or
// op=profile, which issues the link — only inside the unlock window — and
// shows it once, or op=revoke, which burns the links not yet followed.
func (s *Server) postSettingsDevices(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	tab := deviceTab(r)
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	switch r.FormValue("op") {
	case "exclude", "include":
		err := s.users.SetDeviceExcluded(sess.AccountID(), key, r.FormValue("account"), r.FormValue("op") == "exclude")
		if errors.Is(err, user.ErrNotFound) {
			s.notFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "could not save the choice", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/settings/devices?tab="+string(tab))
	case "revoke":
		_ = s.users.RevokeDeviceProfiles(sess.AccountID())
		sess.SetDraft(session.DraftProfileLink, nil)
		s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{Role: view.NoticeInfo, Title: "Profile links revoked", Body: "No link issued before this one opens any more."}))
	case "profile":
		if sess.UnlockUntil(s.now()) == nil {
			s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{
				Role: view.NoticeAlert, Title: "Confirm your password first",
				Body:    "The profile hands out the credentials of every included account, so it is issued only inside the unlock window.",
				Actions: []view.Action{{ID: "settings.unlock", Label: "Confirm password", Href: s.unlockHref(unlockProfile, "/app/settings/devices?tab="+string(tab)), Method: "GET", Enabled: true}},
			}))
			return
		}
		token, _, err := s.users.IssueDeviceProfile(sess.AccountID(), profileAccountIDs(s.catalogOf(sess)))
		if errors.Is(err, user.ErrNothingToIssue) {
			s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{Role: view.NoticeAlert, Title: "Nothing to put in a profile", Body: "No included account has calendars or address books."}))
			return
		}
		if err != nil {
			http.Error(w, "could not issue the link", http.StatusInternalServerError)
			return
		}
		// The card is the page's while the link lives (Нс40): a reload
		// keeps it.
		sess.SetDraft(session.DraftProfileLink, []byte(token))
		s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab))
	default:
		s.redirect(w, r, "/app/settings/devices?tab="+string(tab))
	}
}

// profileAccountIDs are the connections a profile carries now: included, with
// at least one calendar or address book.
func profileAccountIDs(blob sealed.Blob) []string {
	var out []string
	for _, conn := range blob.Connections {
		if blob.IsDeviceExcluded(conn.ID) {
			continue
		}
		cal, card := profileParts(blob, conn.ID)
		if cal || card {
			out = append(out, conn.ID)
		}
	}
	return out
}

func profileParts(blob sealed.Blob, accountID string) (cal, card bool) {
	for _, col := range blob.CollectionsOf(accountID) {
		if col.Gone {
			continue
		}
		card = card || col.Has(view.KindContact)
		cal = cal || col.Has(view.KindEvent) || col.Has(view.KindTask) || col.Has(view.KindNote)
	}
	return cal, card
}

// getSettingsDevicesProfile follows the link: once, by the person it was
// issued to, signed in. The profile is built here, from this session's blob,
// for the connections the link names that are still there and still
// included; a link to nothing is the same 404 as a dead one.
func (s *Server) getSettingsDevicesProfile(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", referrerPolicyNoReferrer)
	accounts, ok := s.users.TakeDeviceProfile(sess.AccountID(), r.URL.Query().Get("t"))
	if !ok {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(sess)
	var parts []profileAccount
	for _, id := range accounts {
		conn, ok := blob.Connection(id)
		if !ok || blob.IsDeviceExcluded(id) {
			continue
		}
		cal, card := profileParts(blob, id)
		if cal || card {
			parts = append(parts, profileAccount{conn: conn, cal: cal, card: card})
		}
	}
	if len(parts) == 0 {
		s.notFound(w, r)
		return
	}
	body, err := appleProfile(parts)
	if err != nil {
		http.Error(w, "could not build the profile", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", `attachment; filename="carrel.mobileconfig"`)
	w.Header().Set("Pragma", "no-cache")
	_, _ = w.Write(body)
}

// profileAccount is one connection of a profile and the parts it gets.
type profileAccount struct {
	conn      sealed.Connection
	cal, card bool
}

// appleProfile is a configuration profile with a CalDAV and a CardDAV
// account per connection, written by hand: a plist is small, and a library
// for it is a dependency for one file. Unsigned — signing needs a
// certificate the instance does not have, and iOS says so on install.
func appleProfile(accounts []profileAccount) ([]byte, error) {
	var b bytes.Buffer
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	str := func(k, v string) { fmt.Fprintf(&b, "<key>%s</key><string>%s</string>\n", k, esc(v)) }
	num := func(k string, v int) { fmt.Fprintf(&b, "<key>%s</key><integer>%d</integer>\n", k, v) }
	boolean := func(k string, v bool) { fmt.Fprintf(&b, "<key>%s</key><%t/>\n", k, v) }
	top, err := newUUID()
	if err != nil {
		return nil, err
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\"><dict>\n<key>PayloadContent</key><array>\n")
	for _, a := range accounts {
		u, err := url.Parse(a.conn.Server)
		if err != nil || u.Host == "" {
			continue
		}
		ssl := u.Scheme == "https"
		port := 80
		if ssl {
			port = 443
		}
		if p := u.Port(); p != "" {
			fmt.Sscanf(p, "%d", &port)
		}
		for _, part := range []struct {
			on           bool
			prefix, kind string
		}{{a.cal, "CalDAV", "com.apple.caldav.account"}, {a.card, "CardDAV", "com.apple.carddav.account"}} {
			if !part.on {
				continue
			}
			id, err := newUUID()
			if err != nil {
				return nil, err
			}
			desc := connLabel(a.conn) + " · " + part.prefix
			b.WriteString("<dict>\n")
			str(part.prefix+"AccountDescription", desc)
			str(part.prefix+"HostName", u.Hostname())
			num(part.prefix+"Port", port)
			boolean(part.prefix+"UseSSL", ssl)
			str(part.prefix+"Username", a.conn.Login)
			str(part.prefix+"Password", a.conn.Password)
			str(part.prefix+"PrincipalURL", a.conn.Server)
			str("PayloadDisplayName", desc)
			str("PayloadIdentifier", "dev.carrel.device."+id)
			str("PayloadType", part.kind)
			str("PayloadUUID", id)
			num("PayloadVersion", 1)
			b.WriteString("</dict>\n")
		}
	}
	b.WriteString("</array>\n")
	str("PayloadDisplayName", "Carrel accounts")
	str("PayloadDescription", "Calendars and address books from Carrel. Holds the passwords of these accounts.")
	str("PayloadIdentifier", "dev.carrel.profile."+top)
	str("PayloadType", "Configuration")
	str("PayloadUUID", top)
	num("PayloadVersion", 1)
	boolean("PayloadRemovalDisallowed", false)
	b.WriteString("</dict></plist>\n")
	return b.Bytes(), nil
}

// newUUID is a random (version 4) UUID, upper case as Apple writes them.
func newUUID() (string, error) {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return "", err
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])), nil
}

// thunderbirdNote is the message of Thunderbird's tab (s-plan-4).
const thunderbirdNote = "Paste a CalDAV URL into Thunderbird Calendar, a CardDAV URL into Address Book, with the login and the password from the account row. No import file is needed."
