// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"fmt"
	"net"
	"strings"
)

const loopbackBind = "127.0.0.1"

// PickFreePort asks the OS for a free TCP port on host. Desktop local mode
// binds the sidecar to loopback only (docs/plans/desktop-wrapper.md §7).
func PickFreePort(host string) (int, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = loopbackBind
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, fmt.Errorf("desktop: pick port on %s: %w", host, err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if err := ln.Close(); err != nil {
		return 0, fmt.Errorf("desktop: close listener: %w", err)
	}
	if !ok || addr.Port == 0 {
		return 0, fmt.Errorf("desktop: pick port: invalid address %v", ln.Addr())
	}
	return addr.Port, nil
}

// LocalBaseURL returns the Carrel base URL for a loopback sidecar port.
func LocalBaseURL(port int) string {
	return fmt.Sprintf("http://%s:%d", loopbackBind, port)
}
