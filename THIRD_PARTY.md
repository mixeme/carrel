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

Full license texts are available in the respective upstream repositories.
