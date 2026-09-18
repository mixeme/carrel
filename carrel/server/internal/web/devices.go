package web

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-23: a new device. Everything discovery found, handed out four ways: an
// Apple profile, the addresses for DAVx5, the addresses for Thunderbird, and
// every address for anything else. The profile holds passwords, so it is the
// most dangerous file the service makes: it is issued only inside the unlock
// window (S-10.7), through a link that opens once and lives
// conf.DeviceProfileLinkTTL, and it is built in memory when the link is
// followed — never written to disk, never cached. A connection the person
// left out goes nowhere.

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

func (s *Server) devicesPage(sess *session.Session, tab view.DeviceTab, notices ...view.Notice) view.DevicesPage {
	blob := s.catalogOf(sess)
	unlocked := sess.UnlockUntil(time.Now()) != nil
	page := view.DevicesPage{Shell: s.buildShell(sess, "Set up a new device"), Tab: tab}
	page.Notices = append(page.Notices, notices...)
	for _, t := range deviceTabs {
		a := view.Action{ID: "device.tab." + string(t), Label: text(deviceTabLabels[t]), Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(t)}}), Method: "GET", Enabled: t != tab}
		if t == tab {
			a.Reason = "Shown now."
		}
		page.Actions = append(page.Actions, a)
	}
	profileAccounts := 0
	for _, conn := range blob.Connections {
		included := !blob.IsDeviceExcluded(conn.ID)
		acct := view.DeviceAccount{Title: text(serverHost(conn.Server) + " · " + conn.Login)}
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
				Label: text(collectionLabel(col)), Kind: kind, Type: text(davType(col)), URL: text(u), Included: included,
				Copy: view.Action{ID: "device.copy", Label: "Copy", Href: u, Method: "GET", Enabled: true},
			})
			dav = dav || !col.Has(view.KindFile)
		}
		if len(acct.Rows) == 0 {
			continue
		}
		if dav && included {
			profileAccounts++
		}
		page.Accounts = append(page.Accounts, acct)
		op, label := "exclude", "Leave out "+serverHost(conn.Server)
		if !included {
			op, label = "include", "Include "+serverHost(conn.Server)
		}
		page.Actions = append(page.Actions, view.Action{
			ID: "device." + op, Label: text(label), Method: "POST", Enabled: true,
			Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {op}, "account": {conn.ID}}),
		})
	}

	unlock := s.route("/app/settings/unlock")
	page.Password.Reveal = view.Action{ID: "device.password", Label: "Show password", Href: unlock, Method: "GET", Enabled: true}
	if unlocked {
		page.Password.Reveal.Href = s.route("/app/settings/connections")
	}
	switch tab {
	case view.TabApple:
		page.Warning = view.Notice{
			Role:  view.NoticeAlert,
			Title: "The profile carries passwords in cleartext",
			Body: text(fmt.Sprintf("It is an ordinary file: it lands in Downloads, in a messenger, in the phone's backup. Open it on the device and delete it. "+
				"The link opens once and lives %d minutes, for you signed in to Carrel; nothing is written to the server's disk, and the issue is recorded in the audit log.",
				int(conf.DeviceProfileLinkTTL/time.Minute))),
		}
		gen := view.Action{ID: "device.profile", Label: "Generate profile", Method: "POST", Danger: true, Enabled: true,
			Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {"profile"}})}
		switch {
		case profileAccounts == 0:
			gen.Enabled, gen.Reason = false, "No included account has calendars or address books."
		case !unlocked:
			gen.Method, gen.Href = "GET", unlock
			gen.Reason = "Confirm your password first: the profile hands out the credentials of every included account."
		}
		page.Actions = append(page.Actions, gen)
	case view.TabAndroid:
		page.Warning = view.Notice{Role: view.NoticeInfo, Body: "Add each account in DAVx5 with its address, login and password. The password is shown on the connections page, inside the unlock window."}
	case view.TabThunderbird:
		page.Warning = view.Notice{Role: view.NoticeInfo, Body: "Paste a CalDAV URL into Thunderbird Calendar, a CardDAV URL into Address Book. There is no import file and none is needed."}
	default:
		page.Warning = view.Notice{Role: view.NoticeInfo, Body: "A WebDAV address is for \"Map network drive\" in Windows, \"Connect to server\" in macOS and davfs in Linux."}
	}
	return page
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
		s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{Role: view.NoticeInfo, Title: "Profile links revoked", Body: "No link issued before this one opens any more."}))
	case "profile":
		if sess.UnlockUntil(time.Now()) == nil {
			s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{
				Role: view.NoticeAlert, Title: "Confirm your password first",
				Body:    "The profile hands out the credentials of every included account, so it is issued only inside the unlock window.",
				Actions: []view.Action{{ID: "settings.unlock", Label: "Confirm password", Href: s.route("/app/settings/unlock"), Method: "GET", Enabled: true}},
			}))
			return
		}
		token, expires, err := s.users.IssueDeviceProfile(sess.AccountID(), profileAccountIDs(s.catalogOf(sess)))
		if errors.Is(err, user.ErrNothingToIssue) {
			s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{Role: view.NoticeAlert, Title: "Nothing to put in a profile", Body: "No included account has calendars or address books."}))
			return
		}
		if err != nil {
			http.Error(w, "could not issue the link", http.StatusInternalServerError)
			return
		}
		tf := s.timeFor(sess)
		s.writePage(w, r, http.StatusOK, s.devicesPage(sess, tab, view.Notice{
			Role:  view.NoticeAccent,
			Title: text("Link ready until " + tf.clock(expires)),
			Body:  "It opens once, for you signed in to Carrel. Open it on the device, install the profile, then delete the file.",
			Actions: []view.Action{
				{ID: "device.profile.download", Label: "Download profile", Href: s.postQuery("/app/settings/devices/profile", url.Values{"t": {token}}), Method: "GET", Enabled: true},
				{ID: "device.profile.revoke", Label: "Revoke", Href: s.postQuery("/app/settings/devices", url.Values{"tab": {string(tab)}, "op": {"revoke"}}), Method: "POST", Enabled: true},
			},
		}))
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
			desc := serverHost(a.conn.Server) + " · " + part.prefix
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
