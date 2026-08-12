// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package merge finds the records that describe the same person or the same
// meeting across collections, and merges what they say (§15).
//
// Detection happens on records that are already loaded: it costs no extra
// request to any server, which is the whole reason the scoring works on a
// Fingerprint rather than on an object. Nothing here writes anywhere and
// nothing here decides anything on its own — a group is a proposal, and §15 is
// explicit that the three verdicts belong to the person looking at it.
package merge
