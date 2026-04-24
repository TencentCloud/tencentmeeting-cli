package log

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestGenerateTraceID_Length verifies the trace ID is exactly 32 characters.
func TestGenerateTraceID_Length(t *testing.T) {
	id := GenerateTraceID()
	if len(id) != 32 {
		t.Errorf("expected length 32, got %d: %s", len(id), id)
	}
}

// TestGenerateTraceID_IsHex verifies the trace ID contains only hex characters.
func TestGenerateTraceID_IsHex(t *testing.T) {
	id := GenerateTraceID()
	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("trace ID is not valid hex: %s, err: %v", id, err)
	}
}

// TestGenerateTraceID_TimestampSegment verifies the first 8 chars encode a reasonable Unix timestamp.
func TestGenerateTraceID_TimestampSegment(t *testing.T) {
	before := time.Now().Unix()
	id := GenerateTraceID()
	after := time.Now().Unix()

	tsHex := id[:8]
	ts, err := strconv.ParseInt(tsHex, 16, 64)
	if err != nil {
		t.Fatalf("failed to parse timestamp segment %q: %v", tsHex, err)
	}

	if ts < before || ts > after {
		t.Errorf("timestamp segment %d out of range [%d, %d]", ts, before, after)
	}
}

// TestGenerateTraceID_PIDSegment verifies chars 8-12 encode the current process ID.
func TestGenerateTraceID_PIDSegment(t *testing.T) {
	id := GenerateTraceID()
	pidHex := id[8:12]

	pid, err := strconv.ParseInt(pidHex, 16, 64)
	if err != nil {
		t.Fatalf("failed to parse pid segment %q: %v", pidHex, err)
	}

	expectedPID := int64(os.Getpid() & 0xffff)
	if pid != expectedPID {
		t.Errorf("pid segment: got %d, want %d", pid, expectedPID)
	}
}

// TestGenerateTraceID_RandomSegmentLength verifies the random segment is 20 chars.
func TestGenerateTraceID_RandomSegmentLength(t *testing.T) {
	id := GenerateTraceID()
	randomPart := id[12:]
	if len(randomPart) != 20 {
		t.Errorf("expected random segment length 20, got %d", len(randomPart))
	}
}

// TestGenerateTraceID_Uniqueness verifies that multiple calls produce unique IDs.
func TestGenerateTraceID_Uniqueness(t *testing.T) {
	const count = 10000
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		id := GenerateTraceID()
		if _, exists := seen[id]; exists {
			t.Errorf("duplicate trace ID detected: %s", id)
		}
		seen[id] = struct{}{}
	}
}

// TestGenerateTraceID_ConcurrentUniqueness verifies uniqueness under concurrent goroutines.
func TestGenerateTraceID_ConcurrentUniqueness(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 200

	var (
		mu   sync.Mutex
		seen = make(map[string]struct{}, goroutines*perGoroutine)
		wg   sync.WaitGroup
	)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := make([]string, perGoroutine)
			for i := 0; i < perGoroutine; i++ {
				ids[i] = GenerateTraceID()
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range ids {
				if _, exists := seen[id]; exists {
					t.Errorf("duplicate trace ID detected in concurrent test: %s", id)
				}
				seen[id] = struct{}{}
			}
		}()
	}
	wg.Wait()

	total := goroutines * perGoroutine
	if len(seen) != total {
		t.Errorf("expected %d unique IDs, got %d", total, len(seen))
	}
}

// TestGenerateTraceID_Format verifies the overall format by printing a sample.
func TestGenerateTraceID_Format(t *testing.T) {
	for i := 0; i < 5; i++ {
		id := GenerateTraceID()
		fmt.Printf("sample trace ID [%d]: %s\n", i+1, id)
		if len(id) != 32 {
			t.Errorf("sample[%d] length error: %s", i, id)
		}
	}
}
