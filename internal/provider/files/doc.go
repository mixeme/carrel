// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package files reads and writes plain WebDAV collections (§6, §7).
//
// It is the one provider that never buffers a body. The contacts and calendar
// providers read a whole object into memory because they have to parse it;
// a file is passed between the DAV server and the browser as a stream, which is
// why §7 fixes Get at io.ReadCloser rather than []byte — a 200 MB download must
// not become 200 MB of resident memory per person reading it.
//
// Its role is bounded on purpose (§23.10): it serves attachments and lets a
// person fetch and place a file. Previews, permissions and renaming trees at
// scale belong to a file manager, which is a different product.
package files
