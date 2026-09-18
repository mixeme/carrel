package web

import (
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
)

// The DAVx5 link (manual.davx5.com, "Integration"): davx5:// for HTTPS,
// caldav:// for plain HTTP, the login in the user part — escaped, since a
// login is often an address — and never the password.
func TestDAVx5Link(t *testing.T) {
	for _, c := range []struct{ server, login, want string }{
		{"https://baikal.example.org/dav.php", "ann", "davx5://ann@baikal.example.org/dav.php"},
		{"https://cloud.example.org", "ann@mail.example", "davx5://ann%40mail.example@cloud.example.org/"},
		{"http://192.168.1.5:5232/", "ann", "caldav://ann@192.168.1.5:5232/"},
		{"not a url", "ann", ""},
	} {
		got := davx5Link(sealed.Connection{Server: c.server, Login: c.login, Password: "secret"})
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.server, got, c.want)
		}
	}
}
