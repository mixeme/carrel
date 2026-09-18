//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// dev/credentials.txt is written for people, not for programs: sections under
// "# Name", subsections under "## Name", then "key: value" lines and bare
// URLs. It is read here into CARREL_* variables when they are not set already,
// so a live test finds its server without anyone exporting anything first. A
// section the file does not hold sets nothing, and the tests of that server
// skip themselves.
//
// The shapes differ per server because the servers do: one names a token
// instead of a password, another splits into subsections with an address and
// an app password each. Every such turn is named below, next to the server it
// belongs to.

func liveCredEnv() {
	data, err := os.ReadFile(credentialsFile())
	if err != nil {
		return
	}
	for key, val := range credEnv(string(data)) {
		setIfEmpty(key, val)
	}
}

func credentialsFile() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// integration/live_creds.go → carrel/server → carrel_reborn
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(root, "dev", "credentials.txt")
}

func setIfEmpty(key, val string) {
	if val != "" && os.Getenv(key) == "" {
		_ = os.Setenv(key, val)
	}
}

// credSection is one block of the file: the section it stands under, the
// subsection when it has one, and its lines as key and value.
type credSection struct {
	name string
	sub  string
	keys map[string]string
	// urls are every bare address of the section, in order. A section may hold
	// several — Google names CardDAV, its well-known and an .ics feed — and
	// keeping only the last one silently picks the wrong server.
	urls []string
}

func (s credSection) get(names ...string) string {
	for _, n := range names {
		if v := s.keys[n]; v != "" {
			return v
		}
	}
	return ""
}

func parseCreds(text string) []credSection {
	var out []credSection
	cur := credSection{keys: map[string]string{}}
	flush := func() {
		if cur.name != "" {
			out = append(out, cur)
		}
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			cur = credSection{name: cur.name, sub: credKey(line[3:]), keys: map[string]string{}}
		case strings.HasPrefix(line, "# "):
			flush()
			cur = credSection{name: credKey(line[2:]), keys: map[string]string{}}
		case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
			// A bare address on its own line is an address of the section.
			cur.keys["url"] = line
			cur.urls = append(cur.urls, line)
		default:
			if k, v, ok := strings.Cut(line, ":"); ok {
				if v = strings.TrimSpace(v); v != "" {
					cur.keys[credKey(k)] = v
				}
			}
		}
	}
	flush()
	return out
}

func credKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// credEnv maps the file onto the variables the live tests read.
func credEnv(text string) map[string]string {
	sections := parseCreds(text)
	find := func(name, sub string) credSection {
		for _, s := range sections {
			if strings.Contains(s.name, name) && s.sub == sub {
				return s
			}
		}
		return credSection{keys: map[string]string{}}
	}
	out := map[string]string{}
	set := func(key, val string) {
		if val != "" {
			out[key] = val
		}
	}

	baikal := find("baikal", "")
	set("CARREL_BAIKAL_URL", baikal.get("url"))
	set("CARREL_BAIKAL_USER", baikal.get("login"))
	set("CARREL_BAIKAL_PASS", baikal.get("password"))

	sftpgo := find("sftpgo", "")
	set("CARREL_SFTPGO_URL", sftpgo.get("url"))
	set("CARREL_SFTPGO_USER", sftpgo.get("login"))
	set("CARREL_SFTPGO_PASS", sftpgo.get("password"))

	// OpenCloud: one space of a file server. Basic auth takes the app token,
	// not the password of the account — the password answers 401 (16.09.2026).
	oc := find("opencloud", "")
	set("CARREL_OPENCLOUD_URL", oc.get("url"))
	set("CARREL_OPENCLOUD_USER", oc.get("login"))
	set("CARREL_OPENCLOUD_PASS", oc.get("token", "password"))

	// Yandex: three addresses under one login, an app password each. The login
	// must be the whole address: "mixeum" answers 401 on CalDAV, and
	// "mixeum@yandex.ru" answers on both (16.09.2026). The Disk (WebDAV) answers
	// 402 for this account and is left out on purpose.
	yandex := find("yandex", "")
	user := yandexLogin(yandex.get("login"))
	cal, card := find("yandex", "caldav"), find("yandex", "carddav")
	set("CARREL_YANDEX_CALDAV_URL", credURL(cal.get("url")))
	set("CARREL_YANDEX_CALDAV_USER", user)
	set("CARREL_YANDEX_CALDAV_PASS", cal.get("app password, caldav", "app password"))
	set("CARREL_YANDEX_CARDDAV_URL", credURL(card.get("url")))
	set("CARREL_YANDEX_CARDDAV_USER", user)
	set("CARREL_YANDEX_CARDDAV_PASS", card.get("app password, carddav", "app password"))

	// Mail.ru names one calendar, not a home: the address is the collection.
	mail := find("mail.ru", "")
	set("CARREL_MAILRU_URL", mail.get("url"))
	set("CARREL_MAILRU_USER", mail.get("login"))
	set("CARREL_MAILRU_PASS", mail.get("app password, caldav", "app password"))

	// Google: CardDAV over Basic with an app password — the password of the
	// account itself is refused. The section also names a private .ics feed of
	// the calendar, which is a file to read and not a CalDAV server, so only
	// the CardDAV address is taken here. Google writes app passwords in groups
	// of four; the spaces are for reading, not for sending.
	google := find("google", "")
	// The file names the shared prefix; the account's own address is that plus
	// the login (the prefix alone answers 404).
	set("CARREL_GOOGLE_CARDDAV_URL", googleAddress(pickAddress(google, "carddav", "carddav/v1"), google.get("login")))
	set("CARREL_GOOGLE_CARDDAV_USER", google.get("login"))
	gpass := strings.ReplaceAll(google.get("app password, carreldav", "app password"), " ", "")
	set("CARREL_GOOGLE_CARDDAV_PASS", gpass)
	// Contacts and calendars sit on different hosts, so the file names both and
	// each is taken by what it contains. The documented
	// apidata.googleusercontent.com/caldav/v2 is not among them: it answers 401
	// loginRequired and wants OAuth2 (docs/servers.md, SV-24).
	set("CARREL_GOOGLE_CALDAV_URL", credURL(pickAddress(google, "caldav", "calendar/dav")))
	set("CARREL_GOOGLE_CALDAV_USER", google.get("login"))
	set("CARREL_GOOGLE_CALDAV_PASS", gpass)

	// Nextcloud on shared hosting: login and password of the account, and one
	// address — /remote.php/dav/, from which discovery walks to both homes.
	// sabre/dav underneath, like Baikal, so the same quirks are expected
	// (SV-1, SV-2) on a different deployment.
	nc := find("nextcloud", "")
	set("CARREL_NEXTCLOUD_URL", credURL(nc.get("url")))
	set("CARREL_NEXTCLOUD_USER", nc.get("login"))
	set("CARREL_NEXTCLOUD_PASS", nc.get("password"))

	// iCloud: contacts and calendars on hosts of their own, as with Google,
	// and an app password that Apple prints in groups of four.
	icloud := find("icloud", "")
	ipass := strings.ReplaceAll(icloud.get("app password, carrel", "app password"), " ", "")
	set("CARREL_ICLOUD_CARDDAV_URL", credURL(pickAddress(icloud, "carddav", "contacts.icloud")))
	set("CARREL_ICLOUD_CARDDAV_USER", icloud.get("login"))
	set("CARREL_ICLOUD_CARDDAV_PASS", ipass)
	set("CARREL_ICLOUD_CALDAV_URL", credURL(pickAddress(icloud, "caldav", "caldav.icloud")))
	set("CARREL_ICLOUD_CALDAV_USER", icloud.get("login"))
	set("CARREL_ICLOUD_CALDAV_PASS", ipass)

	// SoGo names one calendar and one address book of a mailbox account.
	sogo := find("sogo", "")
	set("CARREL_SOGO_CAL_URL", sogo.get("caldav"))
	set("CARREL_SOGO_CARD_URL", sogo.get("carddav"))
	set("CARREL_SOGO_USER", sogo.get("e-mail/login", "login"))
	set("CARREL_SOGO_PASS", sogo.get("password"))
	// The same account is the mailbox letters are delivered to (S-26): its
	// server takes IMAP as well as SMTP.
	set("CARREL_IMAP_HOST", sogo.get("smtp/imap server"))
	set("CARREL_IMAP_USER", sogo.get("e-mail/login", "login"))
	set("CARREL_IMAP_PASS", sogo.get("password"))

	// The SelfPost relay sends Carrel's letters in the live check of S-26.
	relay := find("релей", "")
	set("CARREL_SMTP_HOST", relay.get("host"))
	set("CARREL_SMTP_PORT", relay.get("port"))
	set("CARREL_SMTP_USER", relay.get("login"))
	set("CARREL_SMTP_PASS", relay.get("password"))
	// The address the relay lets this login send from ("not owned by user"
	// otherwise): the section names it as From.
	set("CARREL_SMTP_FROM", relay.get("from"))

	return out
}

// yandexLogin is the login Yandex wants: the whole address.
func yandexLogin(login string) string {
	if login == "" || strings.Contains(login, "@") {
		return login
	}
	return login + "@yandex.ru"
}

// credURL is an address written for a person: without a scheme, or with the
// full stop that ends the sentence it stood in.
func credURL(raw string) string {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ".")
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "http://") || strings.HasPrefix(strings.ToLower(raw), "https://") {
		return raw
	}
	return "https://" + raw
}

// pickAddress is the address a section names under a key of its own, or, where
// the file writes it as a bare line among others, the one that holds want. The
// file is written for people: a server named twice over the months may be
// under either.
func pickAddress(s credSection, key, want string) string {
	if v := s.get(key); v != "" {
		return v
	}
	return pickURL(s, want)
}

// pickURL is the address of the section that holds want; empty when none does.
// A section with several addresses would otherwise be read by whichever came
// last in the file.
func pickURL(s credSection, want string) string {
	for _, u := range s.urls {
		if strings.Contains(strings.ToLower(u), want) {
			return u
		}
	}
	return ""
}

// googleAddress completes the CardDAV prefix with the login it belongs to.
func googleAddress(raw, login string) string {
	raw = credURL(raw)
	if raw == "" || login == "" || !strings.HasSuffix(raw, "principals/") {
		return raw
	}
	return raw + login + "/"
}
