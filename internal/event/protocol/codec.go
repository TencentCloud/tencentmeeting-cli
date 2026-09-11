// codec.go — NDJSON line framing helpers.
//
// On the wire each message is exactly one JSON object terminated by '\n'.
// Encoding is a single Write() so partial frames are impossible; decoding caps
// individual frames at MaxFrameBytes to defang slowloris-style attacks from a
// misbehaving peer.
package protocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// MaxFrameBytes — single-frame upper bound (1 MiB).  Larger frames are rejected
// rather than silently truncated; bus and consumer both fail loudly so the bug
// is visible.
const MaxFrameBytes = 1 << 20

// WriteTimeout — wall-clock cap on a single Encode() call.  Without this a
// wedged peer kernel buffer would block the writer goroutine forever.
const WriteTimeout = 5 * time.Second

// ErrFrameTooLarge is returned by ReadFrame when the next frame exceeds MaxFrameBytes.
var ErrFrameTooLarge = errors.New("protocol: frame exceeds MaxFrameBytes")

// typeEnvelope is used purely for "peek the type" decoding — the actual
// payload is then unmarshalled into the concrete struct in Decode().
type typeEnvelope struct {
	Type string `json:"type"`
}

// Encode serialises msg as a single newline-terminated JSON line and writes it
// in a single Write() call so partial frames can never appear on the wire.
func Encode(w io.Writer, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("protocol encode: %w", err)
	}
	data = append(data, '\n')
	if _, err = w.Write(data); err != nil {
		return err
	}
	return nil
}

// EncodeWithDeadline applies a write deadline before delegating to Encode.
// Use this on bus → consumer fan-out so a single stuck consumer never blocks
// the hub goroutine indefinitely.
func EncodeWithDeadline(conn net.Conn, msg interface{}, timeout time.Duration) error {
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	return Encode(conn, msg)
}

// ReadFrame reads exactly one newline-delimited message from br.
//
// It transparently glues together the chunks returned by ReadSlice when the
// frame is larger than the bufio buffer, but bails out with ErrFrameTooLarge
// before allocations exceed MaxFrameBytes.
func ReadFrame(br *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := br.ReadSlice('\n')
		switch err {
		case nil:
			if len(buf) == 0 {
				// Common fast path: whole frame fit in the bufio buffer.
				// The slice is owned by the bufio.Reader and gets reused on
				// the next ReadSlice — copy so callers can hold onto it.
				out := make([]byte, len(chunk))
				copy(out, chunk)
				return out, nil
			}
			if len(buf)+len(chunk) > MaxFrameBytes {
				return nil, ErrFrameTooLarge
			}
			return append(buf, chunk...), nil
		case bufio.ErrBufferFull:
			if len(buf)+len(chunk) > MaxFrameBytes {
				return nil, ErrFrameTooLarge
			}
			buf = append(buf, chunk...)
		default:
			return nil, err
		}
	}
}

// Decode parses one frame line into the concrete protocol struct based on
// the "type" envelope field.  Unknown types are an error: tightening the
// envelope is preferred over silently ignoring frames the peer does not yet
// understand.
func Decode(line []byte) (interface{}, error) {
	var env typeEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, fmt.Errorf("protocol decode envelope: %w", err)
	}

	var msg interface{}
	switch env.Type {
	case MsgTypeHello:
		msg = &Hello{}
	case MsgTypeHelloAck:
		msg = &HelloAck{}
	case MsgTypeEvent:
		msg = &Event{}
	case MsgTypeControl:
		msg = &Control{}
	case MsgTypeBye:
		msg = &Bye{}
	case MsgTypeStatusQuery:
		msg = &StatusQuery{}
	case MsgTypeStatusResponse:
		msg = &StatusResponse{}
	case MsgTypeShutdown:
		msg = &Shutdown{}
	default:
		return nil, fmt.Errorf("protocol: unknown message type %q", env.Type)
	}

	if err := json.Unmarshal(line, msg); err != nil {
		return nil, fmt.Errorf("protocol decode %s: %w", env.Type, err)
	}
	return msg, nil
}
