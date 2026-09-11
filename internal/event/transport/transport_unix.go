// Copyright (c) 2026 Tencent.
// SPDX-License-Identifier: MIT

//go:build !windows

// transport_unix.go — Unix Domain Socket transport.
//
// We use the standard library only: no extra dependencies on macOS/Linux.
// The bus directory is created with 0700 so the socket inherits per-user
// permissions — multiple users on the same host get their own bus.
package transport

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

// dialTimeout — keep symmetric with the Windows winio.DialPipe default in batch 2.
const dialTimeout = 5 * time.Second

type unixTransport struct{}

// New returns the platform-default IPC implementation.
func New() IPC { return &unixTransport{} }

// Listen creates the parent directory (0700) and binds a stream-oriented unix
// socket.  Caller is responsible for Cleanup() on shutdown.
func (t *unixTransport) Listen(addr string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(addr), 0700); err != nil {
		return nil, err
	}
	return net.Listen("unix", addr)
}

// Dial connects with a bounded timeout.  Without the deadline a stale lock-file
// scenario could wedge consume forever.
func (t *unixTransport) Dial(addr string) (net.Conn, error) {
	return net.DialTimeout("unix", addr, dialTimeout)
}

// Cleanup removes the on-disk socket inode after a clean shutdown.  Errors are
// swallowed: the inode might not exist (idle-timeout exit), and a left-over
// socket merely surfaces as ECONNREFUSED on the next consume which has its own
// recovery path (fork a new bus).
func (t *unixTransport) Cleanup(addr string) {
	_ = os.Remove(addr)
}
