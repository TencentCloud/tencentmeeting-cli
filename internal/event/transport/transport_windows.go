// Copyright (c) 2026 Tencent.
// SPDX-License-Identifier: MIT

//go:build windows

// transport_windows.go — Windows Named Pipe transport via go-winio.
//
// A Named Pipe is a kernel object (no on-disk inode), so:
//   - Listen creates the pipe with the calling user's security descriptor by
//     leaving SecurityDescriptor empty in PipeConfig.  This restricts access
//     to the same user the bus runs under, matching the unix 0700 dir model.
//   - Dial uses a bounded timeout so a missing-pipe error surfaces quickly
//     instead of wedging consume.
//   - Cleanup is a no-op: when the bus process exits the kernel deletes the
//     pipe object automatically.  No file to unlink, no stale-inode race.
package transport

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// pipeBufferSize is per-direction (input/output) on each pipe instance.  64
// KiB comfortably fits a single event payload (events are JSON, far below
// 1 MiB).  Sizing this larger doesn't help throughput on Windows IPC — it
// just consumes non-paged pool — and sizing it smaller forces extra
// kernel<->userspace round-trips.
const pipeBufferSize = 65536

// dialTimeout — symmetric with the unix transport's net.DialTimeout.  Five
// seconds is generous: a healthy bus answers Dial in microseconds; anything
// longer almost certainly means the bus is gone or wedged and consume should
// fail over to its fork-new-bus path.
const dialTimeout = 5 * time.Second

type windowsTransport struct{}

// New returns the Named Pipe IPC implementation.
func New() IPC { return &windowsTransport{} }

// Listen creates a Named Pipe at addr (typically `\\.\pipe\tmeet-event-bus`).
//
// SecurityDescriptor is left empty so go-winio falls back to the calling
// user's SID — only that user can connect.  This is the Windows equivalent
// of the 0700 unix-socket directory permissions used in transport_unix.go.
func (t *windowsTransport) Listen(addr string) (net.Listener, error) {
	return winio.ListenPipe(addr, &winio.PipeConfig{
		InputBufferSize:  pipeBufferSize,
		OutputBufferSize: pipeBufferSize,
	})
}

// Dial connects with a bounded timeout.
//
// winio.DialPipe takes a *time.Duration (so callers can pass nil for "no
// timeout"); we always pass a non-nil dialTimeout so a stale pipe never
// blocks consume indefinitely.
func (t *windowsTransport) Dial(addr string) (net.Conn, error) {
	timeout := dialTimeout
	return winio.DialPipe(addr, &timeout)
}

// Cleanup is a no-op on Windows.  Named Pipes are kernel objects that
// disappear with the owning process; there's no inode to unlink.
func (t *windowsTransport) Cleanup(addr string) {}
