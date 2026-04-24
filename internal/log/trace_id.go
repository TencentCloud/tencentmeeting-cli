package log

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// CtxTraceIDKey is the key for trace ID in context.
const CtxTraceIDKey = "traceID"

// GenerateTraceID generates a 32-character trace ID.
// Format: 8-bit timestamp(hex) + 4-bit pid(hex) + 20-bit random(hex)
// Example: 6613a2f400050a3f1c9e2b4d7e8f1a2b
func GenerateTraceID() string {
	// 8-char hex timestamp (seconds)
	timestamp := fmt.Sprintf("%08x", time.Now().Unix())

	// 4-char hex process ID
	pid := fmt.Sprintf("%04x", os.Getpid()&0xffff)

	// 10-byte random = 20-char hex
	randBytes := make([]byte, 10)
	_, err := rand.Read(randBytes)
	if err != nil {
		// fallback: use nanosecond timestamp as random source
		nano := time.Now().UnixNano()
		for i := range randBytes {
			randBytes[i] = byte(nano >> (uint(i) * 8))
		}
	}
	random := hex.EncodeToString(randBytes)

	return timestamp + pid + random
}
