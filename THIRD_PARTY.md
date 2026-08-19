# Third-party notices

## Vendored assets

Carrel vendors the following third-party software in `internal/web/static/`.

### htmx 2.0.9

- **Source:** https://github.com/bigskysoftware/htmx
- **Files:** `htmx.min.js`
- **License:** Zero-Clause BSD (0BSD)
- **Copyright:** Copyright 2020–2024 James Carlson

### htmx Server-Sent Events extension

- **Source:** https://github.com/bigskysoftware/htmx-extensions (src/sse)
- **Files:** `htmx-sse.js`
- **License:** Zero-Clause BSD (0BSD)
- **Copyright:** Copyright 2020–2024 James Carlson

## Go modules

Resolved by the Go toolchain from `go.mod`; not vendored in this repository.

### golang.org/x/crypto v0.54.0

- **Source:** https://cs.opensource.google/go/x/crypto
- **Used for:** `argon2` (Argon2id password hashing and key derivation)
- **License:** BSD 3-Clause
- **Copyright:** Copyright (c) 2009 The Go Authors. All rights reserved.

### golang.org/x/sys v0.47.0 (indirect)

- **Source:** https://cs.opensource.google/go/x/sys
- **Used for:** `cpu` (CPU feature detection, pulled in by `argon2`)
- **License:** BSD 3-Clause
- **Copyright:** Copyright (c) 2009 The Go Authors. All rights reserved.

### github.com/emersion/go-webdav v0.7.0

- **Source:** https://github.com/emersion/go-webdav
- **Used for:** CalDAV/CardDAV/WebDAV client reference; stage 2 transport builds on the same protocol surface
- **License:** MIT
- **Copyright:** Copyright (c) 2016-2024 emersion

### github.com/emersion/go-vcard v0.0.0-20260618161152-d854b7e0e2d3

- **Source:** https://github.com/emersion/go-vcard
- **Used for:** vCard parsing and serialisation in `internal/model` (the payload behind `Object`)
- **License:** MIT
- **Copyright:** Copyright (c) 2016-2024 emersion

### github.com/emersion/go-ical v0.0.0-20250609112844-439c63cef608

- **Source:** https://github.com/emersion/go-ical
- **Used for:** iCalendar parsing, serialisation, and recurrence handling in `internal/model`
- **License:** MIT
- **Copyright:** Copyright (c) 2016-2024 emersion

### github.com/teambition/rrule-go v1.8.2

- **Source:** https://github.com/teambition/rrule-go
- **Used for:** RFC 5545 recurrence expansion through go-ical
- **License:** MIT
- **Copyright:** Copyright (c) 2017 Teambition

### github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd

- **Source:** https://github.com/rwcarlsen/goexif
- **Used for:** reading EXIF Orientation when processing contact photos (§11)
- **License:** BSD 2-Clause
- **Copyright:** Copyright (c) 2012 Callum Davies / rwcarlsen

### golang.org/x/image v0.34.0

- **Source:** https://cs.opensource.google/go/x/image
- **Used for:** WebP decode support in the contact photo pipeline
- **License:** BSD 3-Clause
- **Copyright:** Copyright (c) 2009 The Go Authors. All rights reserved.

### github.com/wailsapp/wails/v2 v2.10.2 (indirect; `cmd/carrel-desktop`)

- **Source:** https://github.com/wailsapp/wails
- **Used for:** desktop wrapper window and webview (§18)
- **License:** MIT
- **Copyright:** Copyright (c) Wails

Full license texts are available in the respective upstream repositories.
