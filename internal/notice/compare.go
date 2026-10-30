package notice

import (
	"strconv"
	"strings"
)

// Compare compares two version strings with CLI-flavoured semantics. It
// returns -1 if current < target, 0 if equal, +1 if current > target.
//
// Rules:
//  1. Both strings are stripped of a single leading "v" if present.
//  2. The "release" part (before the first "-") is split on "." and
//     compared as integers. Non-numeric segments are treated as 0 so a
//     malformed version does not blow up the entire comparison.
//  3. If the release parts tie, any version carrying a pre-release suffix
//     ("-...") is considered smaller than one without ("-pre.1" < "" of
//     equal release). Two pre-release suffixes are compared segment-by
//     -segment on ".": numeric segments compare as integers (so
//     "slug.11" > "slug.8"), non-numeric segments fall back to lexical
//     comparison. We deliberately do NOT try to pick between different
//     pre-release tags because the server owns release-channel policy —
//     those labels end up compared lexically just like before.
//
// IsNewer wraps Compare for the common "should we prompt the user to
// upgrade?" question.
func Compare(current, target string) int {
	cRel, cPre := splitVersion(current)
	tRel, tPre := splitVersion(target)

	// Compare release parts segment-by-segment. The shorter version is
	// zero-padded on the right so "1.0" and "1.0.0" are equal.
	cs := strings.Split(cRel, ".")
	ts := strings.Split(tRel, ".")
	n := len(cs)
	if len(ts) > n {
		n = len(ts)
	}
	for i := 0; i < n; i++ {
		ci := segInt(cs, i)
		ti := segInt(ts, i)
		if ci != ti {
			if ci < ti {
				return -1
			}
			return 1
		}
	}

	// Release equal — a pre-release suffix is strictly older than none.
	switch {
	case cPre == "" && tPre == "":
		return 0
	case cPre == "" && tPre != "":
		return 1 // release > pre-release
	case cPre != "" && tPre == "":
		return -1
	default:
		return comparePreRelease(cPre, tPre)
	}
}

// comparePreRelease compares two pre-release identifiers segment-by
// -segment on ".". Numeric segments are compared as integers so the
// build counter in e.g. "tag.slug.11" sorts after "tag.slug.8" instead
// of lexically. Non-numeric segments (pre-release tags and branch
// slugs) fall back to byte-wise lexical comparison — this preserves
// the pre-existing behaviour that the client never editorialises
// release-channel ordering.
//
// When one side has fewer segments than the other and the shared prefix
// is equal, the shorter identifier is considered smaller, matching
// semver §11 ("pre" < "pre.1").
func comparePreRelease(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		// Shorter identifier with a matching prefix is older.
		if i >= len(as) {
			return -1
		}
		if i >= len(bs) {
			return 1
		}
		av, bv := as[i], bs[i]
		ai, aerr := strconv.Atoi(av)
		bi, berr := strconv.Atoi(bv)
		switch {
		case aerr == nil && berr == nil:
			// Both numeric — compare as integers.
			if ai != bi {
				if ai < bi {
					return -1
				}
				return 1
			}
		case aerr == nil && berr != nil:
			// Numeric < alphanumeric per semver. Defensive: the current
			// CLI version grammar never actually mixes the two at the
			// same segment index, but keep the rule for safety.
			return -1
		case aerr != nil && berr == nil:
			return 1
		default:
			// Both non-numeric — lexical, same as the previous behaviour
			// for channel / branch slugs.
			if c := strings.Compare(av, bv); c != 0 {
				return c
			}
		}
	}
	return 0
}

// IsNewer reports whether target represents a strictly newer version than
// current. "dev" / "" / malformed current never prompts an upgrade — those
// are explicitly handled by callers upstream of Compare, but IsNewer
// double-checks here so a misuse cannot slip through.
func IsNewer(current, target string) bool {
	if current == "" || current == "dev" || target == "" {
		return false
	}
	return Compare(current, target) < 0
}

// splitVersion drops a leading "v" / "V" and splits off the first "-"
// delimited pre-release tail.
func splitVersion(v string) (release, pre string) {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

// segInt returns the integer value of segs[i], or 0 when i is out of range
// or the segment is non-numeric. Keeps Compare total so a weird version
// string never panics.
func segInt(segs []string, i int) int {
	if i >= len(segs) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(segs[i]))
	if err != nil {
		return 0
	}
	return n
}
