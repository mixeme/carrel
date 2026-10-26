//go:build integration

package integration

import "testing"

// The credentials file is edited by hand, and a live test that cannot find its
// server skips itself — silently, which is the danger. This holds the shapes
// the file actually uses: "# Section", "## Subsection", bare addresses, keys
// with a comma in them, a token instead of a password, a login that must grow
// its domain. A shape that stops being read here stops the whole live set.
const credsSample = `# Carrel panel
## Admin user
login: mix
password: Access123456

Тестовые учётные записи. Можно делать всё, что необходимо для тестирования.
# Baikal, CardDAV/CalDAV
url: https://baikal.example.org/dav.php
login: baikal-user
password: baikal-pass

# SFTPGo, WebDAV
url: https://webdav.example.org/
login: sftpgo-user
password: sftpgo-pass

# OpenCloud, WebDAV
url: https://opencloud.example.org/dav/spaces/space$id
login: oc-user
password: oc-password
token: oc token with spaces

# Yandex, CardDAV/CalDAV/WebDAV
login: someone

## CardDAV
url: https://carddav.yandex.ru
app password: card-pass

## CalDAV
url: https://caldav.yandex.ru.
app password: cal-pass

## WebDAV (may not work)
url: https://webdav.yandex.ru
app password: disk-pass

# Mail.ru, CalDAV
url: https://calendar.mail.ru/principals/mail.ru/person/calendars/c97b/
login: person@mail.ru
app password: mailru-pass

# Почтовый ящик для тестов + SoGO, SMTP/IMAP/CardDAV/CalDAV
SMTP/IMAP: mc.example.org
CardDAV: https://sogo.example.org/SOGo/dav/carrel@example.org/Contacts/personal/
CalDAV: https://sogo.example.org/SOGo/dav/carrel@example.org/Calendar/personal/
e-mail/login: carrel@example.org
password: sogo-pass

# Google, CardDAV/CalDAV
login: someone@gmail.com
app password, CarrelDAV: abcd efgh ijkl mnop

Проверено 17.09.2026 паролем приложения: оба адреса ниже отвечают.
OAuth не нужен.

CardDAV: https://www.googleapis.com/carddav/v1/principals/
CalDAV: https://www.google.com/calendar/dav/

не работает, 401 loginRequired — нужен OAuth2: https://apidata.googleusercontent.com/caldav/v2/
не работает, 404 HTML: https://www.google.com/carddav/v1/
Лента календаря — файл, не сервер CalDAV.

https://calendar.google.com/calendar/ical/someone/private-feed/basic.ics

# Nextcloud, CardDAV/CalDAV/WebDAV
url: https://nextcloud.example.org/remote.php/dav/
login: nc-user
password: nc-pass

# iCloud, CardDAV/CalDAV
login: robot@example.org
app password, Carrel: qrst uvwx yzab cdef

CardDAV: https://contacts.icloud.com
CalDAV: https://caldav.icloud.com

# Релей SelfPost, SMTP
host: relay.example.org
port: 587
from: carrel@example.org
login: relay-user
password: relay-pass
`

func TestCredentialsFileIsRead(t *testing.T) {
	got := credEnv(credsSample)
	want := map[string]string{
		"CARREL_BAIKAL_URL":  "https://baikal.example.org/dav.php",
		"CARREL_BAIKAL_USER": "baikal-user",
		"CARREL_BAIKAL_PASS": "baikal-pass",

		"CARREL_SFTPGO_URL":  "https://webdav.example.org/",
		"CARREL_SFTPGO_USER": "sftpgo-user",
		"CARREL_SFTPGO_PASS": "sftpgo-pass",

		// The token, not the password: Basic auth of OpenCloud takes that one.
		"CARREL_OPENCLOUD_URL":  "https://opencloud.example.org/dav/spaces/space$id",
		"CARREL_OPENCLOUD_USER": "oc-user",
		"CARREL_OPENCLOUD_PASS": "oc token with spaces",

		// The address grows a scheme, loses the full stop that ended the line,
		// and the login grows its domain.
		"CARREL_YANDEX_CALDAV_URL":   "https://caldav.yandex.ru",
		"CARREL_YANDEX_CALDAV_USER":  "someone@yandex.ru",
		"CARREL_YANDEX_CALDAV_PASS":  "cal-pass",
		"CARREL_YANDEX_CARDDAV_URL":  "https://carddav.yandex.ru",
		"CARREL_YANDEX_CARDDAV_USER": "someone@yandex.ru",
		"CARREL_YANDEX_CARDDAV_PASS": "card-pass",

		"CARREL_MAILRU_URL":  "https://calendar.mail.ru/principals/mail.ru/person/calendars/c97b/",
		"CARREL_MAILRU_USER": "person@mail.ru",
		"CARREL_MAILRU_PASS": "mailru-pass",

		"CARREL_SOGO_CAL_URL":  "https://sogo.example.org/SOGo/dav/carrel@example.org/Calendar/personal/",
		"CARREL_SOGO_CARD_URL": "https://sogo.example.org/SOGo/dav/carrel@example.org/Contacts/personal/",
		"CARREL_SOGO_USER":     "carrel@example.org",
		"CARREL_SOGO_PASS":     "sogo-pass",
		// The same mailbox takes IMAP: letters are delivered there (S-26).
		"CARREL_IMAP_HOST": "mc.example.org",
		"CARREL_IMAP_USER": "carrel@example.org",
		"CARREL_IMAP_PASS": "sogo-pass",

		// The relay sends the letters, from the address it lets this login use.
		"CARREL_SMTP_HOST": "relay.example.org",
		"CARREL_SMTP_PORT": "587",
		"CARREL_SMTP_USER": "relay-user",
		"CARREL_SMTP_PASS": "relay-pass",
		"CARREL_SMTP_FROM": "carrel@example.org",

		// Google names its protocols under keys of their own, and the CardDAV
		// prefix is completed with the login: the prefix alone answers 404
		// (docs/servers.md, SV-24). The app password loses its spaces, and the
		// .ics feed on its own line is not taken for a server.
		"CARREL_GOOGLE_CARDDAV_URL":  "https://www.googleapis.com/carddav/v1/principals/someone@gmail.com/",
		"CARREL_GOOGLE_CARDDAV_USER": "someone@gmail.com",
		"CARREL_GOOGLE_CARDDAV_PASS": "abcdefghijklmnop",
		"CARREL_GOOGLE_CALDAV_URL":   "https://www.google.com/calendar/dav/",
		"CARREL_GOOGLE_CALDAV_USER":  "someone@gmail.com",
		"CARREL_GOOGLE_CALDAV_PASS":  "abcdefghijklmnop",

		// Nextcloud: a section whose name carries the protocols after a comma,
		// and one bare address.
		"CARREL_NEXTCLOUD_URL":  "https://nextcloud.example.org/remote.php/dav/",
		"CARREL_NEXTCLOUD_USER": "nc-user",
		"CARREL_NEXTCLOUD_PASS": "nc-pass",

		// iCloud: two hosts, one app password, spaces dropped.
		"CARREL_ICLOUD_CARDDAV_URL":  "https://contacts.icloud.com",
		"CARREL_ICLOUD_CARDDAV_USER": "robot@example.org",
		"CARREL_ICLOUD_CARDDAV_PASS": "qrstuvwxyzabcdef",
		"CARREL_ICLOUD_CALDAV_URL":   "https://caldav.icloud.com",
		"CARREL_ICLOUD_CALDAV_USER":  "robot@example.org",
		"CARREL_ICLOUD_CALDAV_PASS":  "qrstuvwxyzabcdef",
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %q, want %q", key, got[key], w)
		}
	}
	// The Disk answers 402 for this account and has no variable of its own:
	// nothing reads it, so nothing pretends it is there.
	for _, key := range []string{"CARREL_YANDEX_WEBDAV_URL", "CARREL_YANDEX_URL"} {
		if v, ok := got[key]; ok {
			t.Errorf("%s = %q, want nothing", key, v)
		}
	}
}

// A file without a server sets nothing for it: its tests skip themselves
// instead of dialling an empty address.
func TestMissingSectionSetsNothing(t *testing.T) {
	got := credEnv("# Baikal DAV\nhttps://baikal.example.org/dav.php\nlogin: u\npassword: p\n")
	for _, key := range []string{"CARREL_SFTPGO_URL", "CARREL_SOGO_CAL_URL", "CARREL_MAILRU_URL", "CARREL_OPENCLOUD_URL",
		"CARREL_NEXTCLOUD_URL", "CARREL_ICLOUD_CARDDAV_URL", "CARREL_GOOGLE_CARDDAV_URL"} {
		if v, ok := got[key]; ok {
			t.Errorf("%s = %q, want nothing", key, v)
		}
	}
	if got["CARREL_BAIKAL_URL"] == "" {
		t.Error("the section that is there was not read")
	}
}
