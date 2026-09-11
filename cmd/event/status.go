// status.go — `tmeet event status`.
//
// Decision tree implemented by runStatus:
//
//  1. busdiscover.Scan() reports alive ⇒ probe via busctl.QueryStatus.
//     ├─ owner_hash matches current user ⇒ state=running
//     └─ owner_hash differs (or no current user)
//     ⇒ state=stale_owner   (live bus, wrong owner)
//  2. busdiscover.Scan() reports NOT alive but bus.meta is present
//     ⇒ state=orphan             (bus crashed, leftover state on disk)
//  3. nothing on disk ⇒ buses:[]   (no bus has ever run / stop --force scrubbed)
//
// stale_owner ≠ orphan: stale_owner means a bus is *currently* serving the
// previous user, so consume MUST refuse and ask for `event stop --force`;
// orphan means the bus is dead and consume can fork a fresh one safely.
//
// --fail-on-orphan covers both orphan and stale_owner because both indicate
// a state that requires operator action before consume can proceed safely.
package event

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/busdiscover"
	"tmeet/internal/event/transport"
	"tmeet/internal/output"
)

// toLocalRFC3339 renders an internally-stored UTC RFC3339 timestamp string
// as RFC3339 in the operator's local timezone.
//
// The bus daemon, ws.state and bus.meta all persist timestamps in UTC for
// cross-process / crash-forensics consistency (see owner.go, wssource.go,
// handlers.go).  But `event status` is the *human/Agent-facing* surface,
// and operators expect to read times in their own timezone — matching the
// convention used elsewhere in the CLI (auth status, meeting list).
//
// Behaviour:
//   - empty input → empty output (preserves omitempty semantics);
//   - parse failure → return input unchanged so a malformed upstream value
//     surfaces verbatim instead of being silently dropped.
func toLocalRFC3339(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format(time.RFC3339)
}

// busView is one entry of `event status`.buses.
type busView struct {
	OpenIDHash     string      `json:"openid_hash,omitempty"`
	State          string      `json:"state"`
	IsActiveLogin  bool        `json:"is_active_login,omitempty"`
	PID            int         `json:"pid,omitempty"`
	StartedAt      string      `json:"started_at,omitempty"`
	Sock           string      `json:"sock,omitempty"`
	ConsumerCount  int         `json:"consumer_count,omitempty"`
	SubscribedKeys []string    `json:"subscribed_keys,omitempty"`
	WSS            *busWSSView `json:"wss,omitempty"`
	Hint           string      `json:"hint,omitempty"`
}

type busWSSView struct {
	State          string `json:"state"`
	ConnectedAt    string `json:"connected_at,omitempty"`
	ReconnectCount int64  `json:"reconnect_count"`
}

// statusOutput is the top-level shape: {"buses":[...]}.
// `buses` length is 0 (nothing on disk) or 1 (single global bus per host).
type statusOutput struct {
	Buses []busView `json:"buses"`
}

// State values returned in busView.State (decision table — see file header).
const (
	stateRunning    = "running"
	stateOrphan     = "orphan"
	stateStaleOwner = "stale_owner"
)

// StatusOptions holds the options for `tmeet event status`.
type StatusOptions struct {
	tmeet        *internal.Tmeet
	FailOnOrphan bool // exit code 2 (instead of 0) when any bus is in state 'orphan' or 'stale_owner'
}

// newStatusCmd implements `tmeet event status [--fail-on-orphan]`.
//
// Annotated skipPreCheck so admins / Agents can run `event status` to look
// for stale state even after `auth logout` has cleared local credentials.
// Without skipPreCheck, the very tool you'd reach for to investigate a
// post-logout orphan bus would itself error out on missing UserConfig.
func newStatusCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &StatusOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report the local bus daemon state",
		Long: `Report the bus daemon's state on this host.

The output schema always carries a top-level "buses" array of length 0 or 1
(tmeet has at most one bus per host).

States:
  running       bus is alive and bound to the currently logged-in user.
  stale_owner   bus is alive but bound to a different user (or no user is
				logged in).  Resolve by 'tmeet event stop --force' (after
				confirming with the previous user) or by re-logging in to
				the original account.
  orphan        bus is dead but left bus.pid / bus.meta on disk.  Resolve
				by 'tmeet event stop --force' to scrub the leftover files.

With --fail-on-orphan, exits with code 2 (instead of 0) when any bus is in
state 'orphan' or 'stale_owner', so health-check scripts can branch.`,
		Annotations: map[string]string{"skipPreCheck": "true"},
		Args:        cobra.NoArgs,
		RunE:        opts.Run,
	}
	cmd.Flags().BoolVar(&opts.FailOnOrphan, "fail-on-orphan", false,
		"exit with code 2 when any bus is in state 'orphan' or 'stale_owner'")
	return cmd
}

// Run executes `event status`.  Pure plumbing — every case is a
// composition of (Scan, ReadBusMeta, QueryStatus) results, no IO of its own.
func (o *StatusOptions) Run(cmd *cobra.Command, args []string) error {
	out := statusOutput{Buses: []busView{}}

	view, ok, err := computeBusView(transport.New())
	if err != nil {
		return err
	}
	if ok {
		out.Buses = append(out.Buses, view)
	}

	// --fail-on-orphan triggers exit 2 if any bus is in orphan / stale_owner.
	if o.FailOnOrphan {
		for _, b := range out.Buses {
			if b.State == stateOrphan || b.State == stateStaleOwner {
				return exitWithCodeAfterJSON(cmd, out, "", exitCodeOrphan)
			}
		}
	}
	return output.EventPrint(cmd, out)
}

// runStatus is a thin shim kept for tests (status_stop_test.go drives the
// status flow directly via runStatus(cmd, false) without going through
// cobra).  Production code paths reach Run via newStatusCmd's RunE.
func runStatus(cmd *cobra.Command, failOnOrphan bool) error {
	return (&StatusOptions{FailOnOrphan: failOnOrphan}).Run(cmd, nil)
}

// computeBusView returns:
//   - (view, true,  nil)  — there is on-disk evidence of a bus; view describes it.
//   - ({},   false, nil)  — nothing on disk; caller emits buses:[].
//   - ({},   false, err)  — unexpected IO failure; caller surfaces the error.
//
// Kept as a package-level function because `event stop` reuses the same
// logic (see runStop in stop.go); making it a method on StatusOptions would
// force stop to fabricate an unrelated StatusOptions just to call it.
func computeBusView(tr transport.IPC) (busView, bool, error) {
	proc, alive, err := busdiscover.Default().Scan()
	if err != nil {
		return busView{}, false, err
	}

	meta, metaPresent, metaErr := eventruntime.ReadBusMeta()
	// metaErr (corrupted JSON) is not fatal: we degrade to "metadata missing"
	// and let the caller surface a hint.  ReadBusMeta returns metaPresent=false
	// alongside the error so the rest of the logic can proceed.
	_ = metaErr

	currentHash := currentOpenIDHash()

	switch {
	case alive:
		return liveBusView(tr, proc, meta, metaPresent, currentHash), true, nil

	case metaPresent:
		// Lock not held but bus.meta lingers ⇒ orphan.  The previous bus
		// crashed without releasing its file lock (well, the OS released
		// the lock — that's how we got here — but bus.meta wasn't unlinked
		// in its defer).
		return orphanBusView(meta, currentHash), true, nil

	default:
		// Nothing on disk.  Could be a fresh install or a clean stop --force.
		return busView{}, false, nil
	}
}

// liveBusView is invoked when alive=true; we ALWAYS return a view here even
// if QueryStatus fails — the alive lock proves there's a process behind the
// IPC, and the user wants to see "running but unreachable" rather than a
// silent buses:[].
func liveBusView(tr transport.IPC, proc *busdiscover.Process, meta eventruntime.BusMeta, metaPresent bool, currentHash string) busView {
	v := busView{
		Sock: eventruntime.BusSockPath(),
	}
	if proc != nil && proc.PID > 0 {
		v.PID = proc.PID
	}
	if proc != nil && !proc.StartedAt.IsZero() {
		v.StartedAt = proc.StartedAt.Local().Format(time.RFC3339)
	}

	// Owner-hash decision: prefer bus.meta (the bus's authoritative claim)
	// over Hello-time hashes; fall back to QueryStatus if meta is missing.
	ownerHash := ""
	if metaPresent {
		ownerHash = meta.OpenIDHash
		if v.StartedAt == "" && meta.StartedAt != "" {
			v.StartedAt = toLocalRFC3339(meta.StartedAt)
		}
	}

	// Probe the bus for richer info — best-effort, never fatal.
	resp, qerr := busctl.QueryStatus(tr)
	if qerr == nil && resp != nil {
		v.ConsumerCount = resp.ActiveConns
		v.SubscribedKeys = resp.SubscribedKeys
		if v.PID == 0 {
			v.PID = resp.PID
		}
		if v.StartedAt == "" && resp.StartedAt != "" {
			v.StartedAt = toLocalRFC3339(resp.StartedAt)
		}
		if ownerHash == "" {
			ownerHash = resp.OwnerHash
		}
	} else if qerr != nil && !errors.Is(qerr, busctl.ErrNotRunning) {
		// Lock held but we can't talk to it — exotic.  Surface as hint.
		v.Hint = "bus appears alive but failed StatusQuery: " + qerr.Error()
	}
	v.OpenIDHash = ownerHash

	// Owner check.
	switch {
	case ownerHash == "":
		// We can't determine the owner — neither bus.meta nor QueryStatus
		// gave us a hash.  Treat as stale_owner (the safe pessimistic
		// default; the alternative is silent "running" which would hide a
		// real misconfiguration).
		v.State = stateStaleOwner
		v.Hint = "bus is alive but owner_hash is unknown; run 'tmeet event stop --force' if you don't recognise this bus"

	case currentHash == "":
		// Bus has an owner but no user is logged in locally.  This is the
		// "logged out while bus running" scenario.
		v.State = stateStaleOwner
		v.IsActiveLogin = false
		v.Hint = "bus is bound to a previous user; run 'tmeet auth login' to resume, or 'tmeet event stop --force' to release"

	case currentHash != ownerHash:
		// Different user logged in.
		v.State = stateStaleOwner
		v.IsActiveLogin = false
		v.Hint = "bus is bound to a different user; run 'tmeet event stop --force' to release"

	default:
		v.State = stateRunning
		v.IsActiveLogin = true
	}

	// Best-effort WSS diagnostic snapshot.  Missing / corrupt ws.state is
	// downgraded to "no wss field in output" — same status output is
	// emitted, just without the sub-object — so a fresh bus that hasn't
	// written its first state yet still reports cleanly.
	if ws, ok, _ := eventruntime.ReadWSState(); ok {
		v.WSS = &busWSSView{
			State:          ws.State,
			ConnectedAt:    toLocalRFC3339(ws.ConnectedAt),
			ReconnectCount: ws.ReconnectCount,
		}
	}
	return v
}

// orphanBusView is invoked when the alive lock is NOT held but bus.meta is
// still on disk.  The PID in meta is informational only — by the time we're
// here the process is gone, otherwise the lock would still be held.
func orphanBusView(meta eventruntime.BusMeta, currentHash string) busView {
	v := busView{
		State:      stateOrphan,
		OpenIDHash: meta.OpenIDHash,
		PID:        meta.PID,
		StartedAt:  toLocalRFC3339(meta.StartedAt),
		Sock:       eventruntime.BusSockPath(),
		Hint:       "leftover bus state from a previous (now-dead) bus; run 'tmeet event stop --force' to scrub",
	}
	if currentHash != "" && meta.OpenIDHash == currentHash {
		v.IsActiveLogin = true
	}
	return v
}
