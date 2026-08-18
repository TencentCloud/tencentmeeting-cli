// Package event — registry of built-in EventKeys.
//
// The registry is *static* (compiled in) for batch 1: tmeet ships a known set
// of EventKeys and there is no plug-in mechanism.  Keys are registered via
// init()-time RegisterKey calls from the schemas/* files in this package.
//
// All exported helpers (Lookup / List / DomainsOfKey) are read-only after init
// and therefore lock-free; mutation is restricted to RegisterKey, which panics
// on duplicate / invalid input so registration mistakes fail at process start.
package event

import (
	"fmt"
	"sort"
	"strings"
)

// registry holds all known EventKeys.  Populated by init() in registration files.
var registry = map[string]*KeyDef{}

// RegisterKey installs a KeyDef into the package-level registry.
//
// Panics on duplicate Key, empty Key/Domain, missing JQRootPath or out-of-range
// BufferSize: registration runs at process start, so a panic here surfaces as
// an immediate, loud failure during `tmeet --version` rather than a silent
// half-broken `event consume`.
func RegisterKey(def KeyDef) {
	if def.Key == "" {
		panic("event.RegisterKey: empty Key")
	}
	if def.Domain == "" {
		panic(fmt.Sprintf("event.RegisterKey(%q): empty Domain", def.Key))
	}
	if def.JQRootPath != "." && def.JQRootPath != ".payload" {
		panic(fmt.Sprintf("event.RegisterKey(%q): JQRootPath must be \".\" or \".payload\", got %q",
			def.Key, def.JQRootPath))
	}
	// SubscribeRole: empty defaults to "none" (no restriction); any other
	// value must be one of the known roles or it's a registration bug.
	if def.SubscribeRole == "" {
		def.SubscribeRole = SubscribeRoleNone
	}
	switch def.SubscribeRole {
	case SubscribeRoleMaster, SubscribeRoleAgent, SubscribeRoleNone:
	default:
		panic(fmt.Sprintf("event.RegisterKey(%q): SubscribeRole must be one of %q/%q/%q, got %q",
			def.Key, SubscribeRoleMaster, SubscribeRoleAgent, SubscribeRoleNone, def.SubscribeRole))
	}
	if _, dup := registry[def.Key]; dup {
		panic(fmt.Sprintf("event.RegisterKey: duplicate key %q", def.Key))
	}
	if def.BufferSize == 0 {
		def.BufferSize = defaultBufferSize
	}
	if def.BufferSize < 1 || def.BufferSize > maxBufferSize {
		panic(fmt.Sprintf("event.RegisterKey(%q): BufferSize %d out of [1,%d]",
			def.Key, def.BufferSize, maxBufferSize))
	}
	// Defensive copy of params map so post-registration mutation by callers
	// can't reach into the registry.
	if def.ParamsSchema != nil {
		clone := make(map[string]ParamDef, len(def.ParamsSchema))
		for k, v := range def.ParamsSchema {
			clone[k] = v
		}
		def.ParamsSchema = clone
	}
	registry[def.Key] = &def
}

// Lookup returns the KeyDef registered for key, or (nil, false) if absent.
// The pointer points into the registry; callers MUST treat it as read-only.
func Lookup(key string) (*KeyDef, bool) {
	d, ok := registry[key]
	return d, ok
}

// ListKeys returns all registered EventKeys, sorted by (Domain, Key) for stable
// output across `event list` invocations.  When domainFilter != "" only keys
// whose Domain matches (case-insensitive) are returned.
func ListKeys(domainFilter string) []EventListItem {
	out := make([]EventListItem, 0, len(registry))
	want := strings.ToLower(domainFilter)
	for _, d := range registry {
		if want != "" && strings.ToLower(d.Domain) != want {
			continue
		}
		out = append(out, EventListItem{
			Key:           d.Key,
			Domain:        d.Domain,
			Description:   d.Description,
			SubscribeRole: d.SubscribeRole,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// KnownDomains returns the deduplicated, sorted list of all registered domains.
// Used by `event list --domain <name>` to produce a friendly error when the
// requested domain is unknown (rather than silently returning an empty array).
func KnownDomains() []string {
	seen := map[string]struct{}{}
	for _, d := range registry {
		seen[d.Domain] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
