module carrel.dev/carrel/server

go 1.27

toolchain go1.27.1

require (
	carrel.dev/carrel/client v0.0.0
	filippo.io/age v1.3.2
	github.com/emersion/go-ical v0.0.0-20250609112844-439c63cef608
	github.com/emersion/go-vcard v0.1.0
	github.com/emersion/go-webdav v0.0.0-20260625083450-1901d193ce99
	github.com/google/go-cmp v0.7.0
	github.com/teambition/rrule-go v1.8.2
	github.com/yuin/goldmark v1.8.6
	golang.org/x/crypto v0.56.0
	golang.org/x/image v0.46.0
	golang.org/x/sync v0.23.0
	golang.org/x/text v0.42.0
	rsc.io/qr v0.2.0
)

require (
	carrel.dev/typecase v0.0.0-00010101000000-000000000000 // indirect
	filippo.io/hpke v0.4.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

// The client (carrel/client) is wired into the binary by cmd/carrel alone,
// and Typecase comes with it from the neighbouring clone. A dependency's
// replace directives are not applied to this module, so the client's own
// replace of Typecase is repeated here: the image build and the Linux
// machine need the clone beside this repository (docs/development.md).
replace carrel.dev/carrel/client => ../client

replace carrel.dev/typecase => ../../../typecase
