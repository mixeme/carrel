# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- **Stage 0 — project scaffold:** Go module `gitea.mixdep.ru/mix/carrel`, `cmd/carrel/main.go` with version/commit ldflags, package layout per spec §3, `go:embed` for templates and static assets, vendored htmx 2.0.9 and htmx-sse extension, `LICENSE` (AGPL-3.0-or-later), `THIRD_PARTY.md`, SPDX headers in source files.
- **Stage 1 — configuration:** `internal/config` with env-over-file loading (`CARREL_PORT`, `CARREL_DATA_DIR`, `CARREL_TRUSTED_PROXIES`, `CARREL_BASE_PATH`, `CARREL_LOG_LEVEL`), defaults (port 8080, data dir `/var/lib/carrel`), startup validation, unit tests.
- Minimal HTTP server: `/healthz` liveness, static file serving, structured JSON logging, graceful shutdown on SIGTERM (15s).
- **Stage 2 — crypto layer:** `internal/crypto` implementing the key schedule of spec §4. Argon2id with separate cost profiles for login verification, KEK derivation and the escrow master password (strengthened: 256 MiB / 6 passes), stored alongside each salt so costs can be raised later. Separate salts for the two uses of a password, constant-time digest comparison, `Equal` for invite-token digests.
- DEK handling: 32-byte random DEK, AES-256-GCM wrap/unwrap under the KEK, per-use AAD constants so a wrapped DEK cannot be replayed as sealed state. Changing a password re-wraps the DEK only and leaves data encrypted under it readable.
- Server key: generated on first run in the data directory (`server.key`, `O_EXCL`, mode 0600), never overwritten; a short or unreadable key file is reported as corrupt instead of being replaced. `SealState`/`OpenState` protect service data that must be readable before anyone logs in.
- Escrow (§5.4): X25519 key pair, private key sealed under the master password and never cached, DEK copies encrypted to the public key so depositing needs no secret. Master password change re-seals the private key only; minimum master password length enforced at 16 characters.
- `Key.Zero`/`Zero` for explicit wiping of key material (§24.6); uniform `ErrDecrypt` for every authentication failure.
- Unit tests: derivation and salt separation, wrap/unwrap round trips, password change, wrong password and tampered ciphertext, AAD domain separation, server key persistence and corruption, escrow round trip, cross-escrow isolation and master password rotation.

### Dependencies

- `golang.org/x/crypto` v0.54.0 for `argon2` (and its indirect `golang.org/x/sys`), recorded in `THIRD_PARTY.md`.
