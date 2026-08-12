// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package contacts reads and writes address objects over CardDAV
// (spec §8, §9, §12, §13).
//
// The provider owns the read path — collection tag, ETag map, batched
// addressbook-multiget — and the write path, where every mutation carries a
// precondition and every stored object is read back and compared with what was
// sent, so a server that drops properties is noticed at the time (§8).
package contacts
