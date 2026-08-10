# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- **Stage 0 — project scaffold:** Go module `gitea.mixdep.ru/mix/carrel`, `cmd/carrel/main.go` with version/commit ldflags, package layout per spec §3, `go:embed` for templates and static assets, vendored htmx 2.0.9 and htmx-sse extension, `LICENSE` (AGPL-3.0-or-later), `THIRD_PARTY.md`, SPDX headers in source files.
- **Stage 1 — configuration:** `internal/config` with env-over-file loading (`CARREL_PORT`, `CARREL_DATA_DIR`, `CARREL_TRUSTED_PROXIES`, `CARREL_BASE_PATH`, `CARREL_LOG_LEVEL`), defaults (port 8080, data dir `/var/lib/carrel`), startup validation, unit tests.
- Minimal HTTP server: `/healthz` liveness, static file serving, structured JSON logging, graceful shutdown on SIGTERM (15s).
