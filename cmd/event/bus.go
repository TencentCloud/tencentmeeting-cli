package event

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal"
	"tmeet/internal/auth"
	"tmeet/internal/config"
	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/bus"
	"tmeet/internal/event/source"
	"tmeet/internal/exception"
)

// newBusCmd builds the hidden `tmeet event _bus` daemon command.
//
// This subcommand is forked by `tmeet event consume` (batch 3) and is NOT
// meant to be invoked by humans — it stays Hidden=true in cobra so it does
// not pollute --help output.  It can be invoked directly during development:
//
//	$ tmeet event _bus           # blocks until idle timeout / SIGTERM
//	$ tmeet event _bus --interval 1s
//
// Skips the global preCheck so we can produce a focused error message when
// UserConfig is missing rather than the generic "user config not found".
func newBusCmd(tmeet *internal.Tmeet) *cobra.Command {
	var (
		mockInterval time.Duration
		idleTimeout  time.Duration
	)
	cmd := &cobra.Command{
		Use:    "_bus",
		Short:  "Internal event-bus daemon (do not call directly)",
		Hidden: true,
		// We do our own UserConfig validation below so we can include
		// daemon-specific guidance in the error message.
		Annotations: map[string]string{"skipPreCheck": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBusDaemon(cmd, tmeet, mockInterval, idleTimeout)
		},
	}

	// --interval: mock source emit cadence.  Hidden because real users
	// shouldn't twiddle it; kept for development & tests.
	cmd.Flags().DurationVar(&mockInterval, "interval", 5*time.Second,
		"mock source emit interval")
	_ = cmd.Flags().MarkHidden("interval")

	// --idle-timeout: how long the bus stays alive with no consumers.
	// Hidden because the default (30s) is the contract; overriding is for
	// tests that want a snappier idle exit.
	cmd.Flags().DurationVar(&idleTimeout, "idle-timeout", bus.IdleTimeout,
		"shut down after this long with no active consumers")
	_ = cmd.Flags().MarkHidden("idle-timeout")

	return cmd
}

// runBusDaemon is the actual entry point of the bus process.
//
// Steps:
//
//  1. Resolve UserConfig — error out cleanly if not logged in.
//  2. Compute OpenIDHash for owner binding.
//  3. Acquire BusForkLock (best-effort, poll briefly): two _bus invocations
//     started at the same instant should not BOTH try to bind the socket.
//  4. Set up the bus logger; create the bus instance with the mock source.
//  5. Hook SIGTERM/SIGINT → ctx cancel; bus.Run blocks until exit.
//
// Returns nil on graceful exit (idle timeout, ctx cancel, lost-the-fork-race).
// Returns a non-nil error only on real startup failures.
func runBusDaemon(cmd *cobra.Command, tmeet *internal.Tmeet, mockInterval, idleTimeout time.Duration) error {
	// 1. UserConfig.
	usr, err := config.GetUserConfig()
	if err != nil {
		return exception.ParseUserConfigError.With("event _bus: load user config: %v", err)
	}
	if usr == nil || usr.OpenId == "" {
		return exception.GetUserConfigEmptyError.With(
			"event _bus requires an authenticated user; run 'tmeet auth login' first")
	}

	// 2. Owner hash.
	ownerHash := eventruntime.OpenIDHash(usr.OpenId)
	if ownerHash == "" {
		// Defensive: GetUserConfig succeeded but OpenId is empty → corrupted
		// keychain blob.  We refuse to start a bus with no owner because
		// every consumer's WrongOwner check would trivially pass.
		return exception.EventBusError.With("refusing to start with empty OpenIDHash (config corruption?)")
	}

	// 3. Fork lock — short polling acquire.  WithLock's defaultTimeout of 5s
	//    is enough to absorb a stop-then-restart sequence (the previous bus
	//    is on its way out and will release the alive lock before then).
	//
	//    Ensure BusDir exists first: filelock.WithLock opens the lock file
	//    with O_CREATE which does NOT create parent directories.  On a
	//    fresh install (or after the user wiped ~/.tmeet/event/) the open
	//    would fail with ENOENT before we ever reach SetupBusLogger which
	//    is the other place that does MkdirAll.  Using 0700 to match the
	//    permissions the logging subsystem applies later.
	busDir := eventruntime.BusDir()
	if err := os.MkdirAll(busDir, 0700); err != nil {
		return exception.EventBusError.With("event _bus: mkdir %s: %v", busDir, err)
	}
	forkLockPath := eventruntime.BusForkLock()
	return filelock.WithLock(forkLockPath, func() error {
		return runBusUnderForkLock(cmd, tmeet, ownerHash, mockInterval, idleTimeout)
	})
}

// runBusUnderForkLock is the body executed once we hold BusForkLock.  Split
// out so the fork-lock acquisition stays a tight one-liner readable above.
func runBusUnderForkLock(cmd *cobra.Command, tmeet *internal.Tmeet, ownerHash string, mockInterval, idleTimeout time.Duration) error {
	// 4. Logger + bus instance.
	//
	// SetupBusLogger returns a *tlog.Logger writing to <BusDir>/logs/bus-<date>.log
	// with daily + size rotation, async non-blocking writes, and 7-day retention
	// inherited from internal/log.  defer Close() guarantees the buffered async
	// channel is fully drained even when ctx cancellation wakes b.Run before
	// pending log lines have reached disk.
	logger, err := bus.SetupBusLogger()
	if err != nil {
		return exception.InitializeFailedError.With("event _bus: setup logger: %v", err)
	}
	defer logger.Close()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	logger.Infof(ctx, "event _bus starting: pid=%d cli_version=%s mock_interval=%s idle_timeout=%s",
		os.Getpid(), tmeet.CLIVersion, mockInterval, idleTimeout)

	// Build the production WSS source with credentials taken from the
	// authenticated user.  An empty token (e.g. config corruption) falls
	// back to MockSource via the safety rail in source.Build, so we
	// don't accidentally fire unauth WSS dials.
	token := ""
	openID := ""
	if tmeet.UserConfig != nil {
		token = tmeet.UserConfig.AccessToken
		openID = tmeet.UserConfig.OpenId
	}
	machineID := ""
	if tmeet.SystemInfo != nil {
		machineID = tmeet.SystemInfo.MachineID
	}

	// TokenProvider drives RefreshToken-on-(re)connect and
	// RefreshToken-on-heartbeat inside WSSource.  The closure captures
	// the live *TmeetAuth so it always reads the current cached token;
	// RefreshToken itself is a no-op when the cached access-token is
	// still within its TTL, so the per-heartbeat call is cheap.
	//
	// On success we return the freshest AccessToken from UserConfig
	// (RefreshToken updates UserConfig in place under filelock).  On
	// failure we propagate the error so WSSource surfaces it through
	// the standard reconnect / shutdown path — a hard
	// UserIdentityExpiredError will eventually exhaust the daemon's
	// reconnect budget and the user will see "auth failed" the next
	// time they invoke `event status`.
	tmeetAuth := auth.NewTmeetAuth(tmeet)
	// tokenMu serialises the compound refresh+read operation inside
	// tokenProvider so that concurrent calls from multiple sources
	// do not race on config.userConfig or tmeet.UserConfig.
	var tokenMu sync.Mutex
	tokenProvider := func(ctx context.Context) (string, error) {
		tokenMu.Lock()
		defer tokenMu.Unlock()
		if err := tmeetAuth.RefreshToken(ctx, config.ClearUserConfigUnResource); err != nil {
			return "", err
		}
		// Defensive check: verify the on-disk credential has not been
		// removed by an external process.  RefreshToken's fast path
		// (Expires > now) only inspects the in-memory cache and cannot
		// detect out-of-band deletion (e.g. manual rm of ~/.tmeet/,
		// keychain cleanup).  Force a disk re-read to catch this.
		config.ResetCache()
		freshCfg, err := config.GetUserConfig()
		if err != nil {
			return "", exception.GetUserConfigEmptyError
		}
		if freshCfg == nil {
			return "", exception.GetUserConfigEmptyError
		}
		// Sync the in-memory reference so subsequent code sees the freshest value.
		tmeet.UserConfig = freshCfg
		return freshCfg.AccessToken, nil
	}

	// Envelope-level reject at the WSS gateway (Head.Status != 0).
	// We only treat code 10006 ("token invalid") as authoritative
	// proof that the local credential is unusable and wipe it via
	// ClearUserConfigUnResource — other non-zero statuses (e.g.
	// transient gateway errors, malformed-request style codes)
	// may not mean the token itself is dead, so we just log and
	// let the daemon's normal teardown path handle them; forcing
	// a re-login on those would degrade UX.
	//
	// ClearUserConfigUnResource (not ClearUserConfig) on purpose:
	// ClearUserConfig fires the "stop bus daemon" resource-release
	// hook, which here would re-enter ourselves.
	onAuthFailed := func(ctx context.Context, code int, err error) {
		logger.Errorf(ctx, "event _bus: token rejected by gateway (status=%d), clearing local user config", code)
		_ = config.ClearUserConfigUnResource()
	}

	mockSrc := source.Build(source.Params{
		Token:         token,
		OpenID:        openID,
		MachineID:     machineID,
		MockInterval:  mockInterval,
		Logger:        logger,
		TokenProvider: tokenProvider,
		OnAuthFailed:  onAuthFailed,
	})
	b := bus.New(bus.Config{
		OpenIDHash:  ownerHash,
		BusVersion:  tmeet.CLIVersion,
		Source:      []source.Source{mockSrc},
		IdleTimeout: idleTimeout,
		Logger:      logger,
	})

	// 5. Signal handling.  We listen for SIGTERM / SIGINT and translate them
	//    into ctx cancellation so bus.Run's main select treats them
	//    identically to a graceful Shutdown frame.  SIGHUP intentionally
	//    NOT trapped: the consume process catches it itself for terminal
	//    detach handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go func() {
		select {
		case sig := <-sigCh:
			logger.Infof(ctx, "event _bus: received signal %s, initiating shutdown", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := b.Run(ctx); err != nil {
		logger.Errorf(ctx, "event _bus: Run returned error: %v", err)
		return err
	}
	logger.Infof(ctx, "event _bus: exited cleanly")
	return nil
}
