package notice

import "testing"

// TestCompare_Basic covers the "common" strict ordering a user would
// expect: lexicographic within equal major.minor.patch is irrelevant
// (we only split "release" vs "pre"), and 1.0.19 < 1.0.20 regardless of
// the leading "v".
func TestCompare_Basic(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"equal plain", "1.0.0", "1.0.0", 0},
		{"equal with v", "v1.0.0", "v1.0.0", 0},
		{"a older patch", "v1.0.19", "v1.0.20", -1},
		{"a newer patch", "v1.0.20", "v1.0.19", 1},
		{"a older minor", "v1.1.0", "v1.2.0", -1},
		{"a older major", "v1.9.9", "v2.0.0", -1},
		{"padded short vs long equal", "1.0", "1.0.0", 0},
		{"padded short older", "1.0", "1.0.1", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.a, tc.b); got != tc.want {
				t.Fatalf("Compare(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestCompare_PreRelease verifies the "pre-release is strictly older than
// the release of the same number" rule, which is the only piece of
// non-trivial policy baked into Compare. Pre-release tags are kept
// deliberately generic ("pre1" / "pre2" / "pre3") so the test does not
// pin itself to any particular release-channel vocabulary.
func TestCompare_PreRelease(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"pre less than release (pre1)", "v1.0.19-pre1.1", "v1.0.19", -1},
		{"pre less than release (pre2)", "v1.0.19-pre2.2", "v1.0.19", -1},
		{"pre less than release (pre3)", "v1.0.19-pre3.1", "v1.0.19", -1},
		{"release greater than pre1", "v1.0.19", "v1.0.19-pre1.1", 1},
		{"two pres same tag lexical", "v1.0.19-pre1.1", "v1.0.19-pre1.2", -1},
		{"different pre tags lexical", "v1.0.19-pre1.1", "v1.0.19-pre2.1", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.a, tc.b); got != tc.want {
				t.Fatalf("Compare(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestCompare_PreReleaseNumeric locks in the fix for the lexical-vs
// -numeric trap in multi-segment pre-release identifiers. The build
// counter in e.g. "pre1.br1.11" must sort AFTER "pre1.br1.8", not
// before. Same story for any pre-release tag once its counter crosses
// a 10x boundary.
func TestCompare_PreReleaseNumeric(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		// The original bug report: 11 vs 8 under a shared "tag.slug" prefix.
		{"pre counter 11 > 8", "v1.0.20-pre1.br1.11", "v1.0.20-pre1.br1.8", 1},
		{"pre counter 8 < 11", "v1.0.20-pre1.br1.8", "v1.0.20-pre1.br1.11", -1},
		// Mirror of the same trap in a smaller range.
		{"pre counter 2 < 10", "v1.0.20-pre1.br1.2", "v1.0.20-pre1.br1.10", -1},
		// Same tag, no branch slug — pre.9 vs pre.10.
		{"pre2 9 < pre2 10", "v1.0.20-pre2.9", "v1.0.20-pre2.10", -1},
		{"pre2 10 > pre2 9", "v1.0.20-pre2.10", "v1.0.20-pre2.9", 1},
		// Larger counter crossing a 100x boundary.
		{"pre1 9 < pre1 100", "v1.0.20-pre1.br1.9", "v1.0.20-pre1.br1.100", -1},
		// Another pre tag, same pattern.
		{"pre3 2 < pre3 11", "v1.0.20-pre3.2", "v1.0.20-pre3.11", -1},
		// Equal pre-releases still tie.
		{"pre equal", "v1.0.20-pre1.br1.11", "v1.0.20-pre1.br1.11", 0},
		// Shorter identifier with equal prefix is older (semver §11).
		{"pre shorter is older", "v1.0.20-pre2", "v1.0.20-pre2.1", -1},
		{"pre longer is newer", "v1.0.20-pre2.1", "v1.0.20-pre2", 1},
		// Different non-numeric slug under the same pre tag: non-numeric
		// segment falls back to lexical, result is stable (we don't
		// assert business meaning, just that it stays total).
		{"pre different slug lexical", "v1.0.20-pre1.br1.1", "v1.0.20-pre1.main.1", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.a, tc.b); got != tc.want {
				t.Fatalf("Compare(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestCompare_Malformed documents the "never panic, treat garbage as 0"
// policy. Non-numeric segments become 0 so the comparison stays total.
func TestCompare_Malformed(t *testing.T) {
	// "abc" segment -> 0, so "1.abc.0" == "1.0.0"
	if got := Compare("1.abc.0", "1.0.0"); got != 0 {
		t.Fatalf("expected malformed to tie, got %d", got)
	}
	// Leading whitespace is trimmed.
	if got := Compare("  v1.0.0  ", "1.0.0"); got != 0 {
		t.Fatalf("whitespace: expected 0, got %d", got)
	}
}

// TestIsNewer wraps the common "should we prompt the user?" question and
// also verifies the hard-coded dev/empty short-circuit.
func TestIsNewer(t *testing.T) {
	cases := []struct {
		cur, tgt string
		want     bool
	}{
		{"v1.0.19", "v1.0.20", true},
		{"v1.0.20", "v1.0.19", false},
		{"v1.0.19", "v1.0.19", false},
		{"dev", "v1.0.19", false},
		{"", "v1.0.19", false},
		{"v1.0.19", "", false},
		{"v1.0.19-pre1.1", "v1.0.19", true},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.cur, tc.tgt); got != tc.want {
			t.Fatalf("IsNewer(%q,%q) = %v, want %v", tc.cur, tc.tgt, got, tc.want)
		}
	}
}
