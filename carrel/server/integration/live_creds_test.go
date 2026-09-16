//go:build integration

package integration

import "testing"

// The credentials file is edited by hand, and a live test that cannot find its
// server skips itself — silently, which is the danger. This holds the shapes
// the file actually uses: "# Section", "## Subsection", bare addresses, keys
// with a comma in them, a token instead of a password, a login that must grow
// its domain. A shape that stops being read here stops the whole live set.
const credsSample = `Carrel admin
mix
Access123456

Тестовые учётные записи. Можно делать всё, что необходимо для тестирования.
# Baikal DAV
https://baikal.example.org/dav.php
login: baikal-user
password: baikal-pass

# SFTPGo WebDAV
https://webdav.example.org/
login: sftpgo-user
password: sftpgo-pass

# OpenCloud WebDAV
login: oc-user
password: oc-password
token: oc token with spaces
url: https://opencloud.example.org/dav/spaces/space$id

# Yandex
login: someone

## WebDAV (may not work)
url: https://webdav.yandex.ru
app password, WebDAV: disk-pass

## CardDAV
url: carddav.yandex.ru.
app password, CardDAV: card-pass

## CalDAV
url: caldav.yandex.ru
app password, CalDAV: cal-pass

# Mail.ru
login: person@mail.ru
app password, CalDAV: mailru-pass
https://calendar.mail.ru/principals/mail.ru/person/calendars/c97b/

# Почтовый ящик для тестов + SoGO
E-mail/login: carrel@example.org
password: sogo-pass
SMTP/IMAP server: mc.example.org
CalDAV: https://sogo.example.org/SOGo/dav/carrel@example.org/Calendar/personal/
CardDAV: https://sogo.example.org/SOGo/dav/carrel@example.org/Contacts/personal/
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
	for _, key := range []string{"CARREL_SFTPGO_URL", "CARREL_SOGO_CAL_URL", "CARREL_MAILRU_URL", "CARREL_OPENCLOUD_URL"} {
		if v, ok := got[key]; ok {
			t.Errorf("%s = %q, want nothing", key, v)
		}
	}
	if got["CARREL_BAIKAL_URL"] == "" {
		t.Error("the section that is there was not read")
	}
}
