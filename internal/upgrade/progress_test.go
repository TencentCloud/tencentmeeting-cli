package upgrade

import (
	"strings"
	"testing"
)

func Test_HumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KiB"},
		{int64(1<<20) + int64(1<<19), "1.50 MiB"},
		{int64(1 << 30), "1.00 GiB"},
	}
	for _, tc := range cases {
		if got := HumanBytes(tc.in); got != tc.want {
			t.Fatalf("HumanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func Test_ProgressWriter_FinishAlwaysEmitsOnce(t *testing.T) {
	logf, buf := captureLogf()
	p := NewProgressWriter(logf, 1024)

	// Write less than the byte threshold — intermediate emit should NOT fire.
	_, _ = p.Write(make([]byte, 10))
	if buf.Len() != 0 {
		t.Fatalf("expected no intermediate emit under threshold, got %q", buf.String())
	}

	// Finish always prints exactly one line regardless of thresholding.
	p.Finish(10)
	if !strings.Contains(buf.String(), "Downloaded") {
		t.Fatalf("expected Finish line, got %q", buf.String())
	}
}

func Test_ProgressWriter_UnknownContentLength(t *testing.T) {
	logf, buf := captureLogf()
	p := NewProgressWriter(logf, -1) // chunked / unknown total

	p.Finish(2048)
	s := buf.String()
	// Must NOT contain a percentage marker when total is unknown.
	if !strings.Contains(s, "Downloaded") {
		t.Fatalf("expected Downloaded marker: %q", s)
	}
	if strings.Contains(s, "%") {
		t.Fatalf("unexpected percentage with unknown total: %q", s)
	}
}

func Test_ProgressWriter_NilLogfIsNoOp(t *testing.T) {
	// Nil logf must be tolerated — it is a documented "silent" mode
	// for callers (tests, non-interactive paths) that don't want
	// progress noise but still need the byte counting.
	p := NewProgressWriter(nil, 100)
	if _, err := p.Write(make([]byte, 10)); err != nil {
		t.Fatalf("Write under nil logf: %v", err)
	}
	// Finish must not panic.
	p.Finish(10)
}
