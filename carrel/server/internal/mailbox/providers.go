// Package mailbox is a person's own mailbox (carrel-mailbox.md): finding its
// servers, signing in to them, and — later — reading and writing letters.
// Nothing of the mail is written to disk; every connection goes through the
// guard (S-5.1).
package mailbox

import "strings"

// Provider is a mail provider Carrel knows by the domain of an address, from
// a table in the service itself: the shared database the mail clients keep
// is not asked — it would learn the domains of everyone on the instance
// (M-3).
type Provider struct {
	Name string
	// AppPassword: the provider lets in by an app password, as its CalDAV
	// does; the connection screen says so before the password is typed.
	AppPassword bool
	// Closed: the provider closed signing in by password (Microsoft); such a
	// mailbox does not connect, and the screen says it in plain words.
	Closed bool
	// KeepsSent: the server puts the copy of a sent letter into Sent itself;
	// Carrel's own copy there would be a twin (M-3).
	KeepsSent bool
}

// providers are the providers known, by domain.
var providers = func() map[string]Provider {
	out := map[string]Provider{}
	add := func(p Provider, domains ...string) {
		for _, d := range domains {
			out[d] = p
		}
	}
	add(Provider{Name: "Gmail", AppPassword: true, KeepsSent: true}, "gmail.com", "googlemail.com")
	add(Provider{Name: "Yandex", AppPassword: true}, "yandex.ru", "ya.ru", "yandex.com", "yandex.by", "yandex.kz")
	add(Provider{Name: "Mail.ru", AppPassword: true}, "mail.ru", "bk.ru", "inbox.ru", "list.ru", "internet.ru")
	add(Provider{Name: "iCloud", AppPassword: true}, "icloud.com", "me.com", "mac.com")
	add(Provider{Name: "Microsoft", Closed: true}, "outlook.com", "hotmail.com", "live.com", "msn.com", "outlook.ru")
	return out
}()

// ProviderOf is the provider of an address, when its domain is in the table.
func ProviderOf(address string) (Provider, bool) {
	p, ok := providers[Domain(address)]
	return p, ok
}

// Domain is the part of an address after its last @, lower-cased.
func Domain(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(address[at+1:]), "."))
}
