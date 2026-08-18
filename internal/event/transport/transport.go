// Package transport abstracts the north-bound IPC mechanism between the bus
// daemon (server) and the consumer process (client).
//
// The shape of the interface is shared with lark-cli: Listen/Dial return
// net.Listener / net.Conn so the bus and consumer code can be platform-agnostic.
// Two implementations exist:
//
//	transport_unix.go    — net.Listen("unix", BusSockPath()) on macOS/Linux
//	transport_windows.go — Named Pipe via go-winio (added in batch 2)
//
// During batch 1 the Windows implementation is a stub returning an explicit
// error; it is wired only so unix builds compile cleanly with the same
// import paths the rest of the subsystem will use later.
package transport

import "net"

// IPC is the platform-specific contract for hosting / connecting to the bus.
//
// Address(): consumers need only the well-known global path (BusSockPath).
// Cleanup(): on unix removes the stale socket file after a crash; no-op on
// Windows because the Named Pipe is a kernel object that disappears with the
// owning process.
type IPC interface {
	Listen(addr string) (net.Listener, error)
	Dial(addr string) (net.Conn, error)
	Cleanup(addr string)
}
