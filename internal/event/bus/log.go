// log.go — bus log redirection through tmeet's internal/log subsystem.
//
// Why a dedicated logger instance instead of internal/log's global default:
//   The bus daemon is forked as a separate process (`tmeet event _bus`).
//   cmd/root.go has already called log.Init for that process which targets
//   <configDir>/logs/tmeet-<date>.log — meant for the cli foreground flow.
//   Mixing bus-daemon diagnostics into that file would scramble post-mortem
//   analysis of either side, so the bus owns its own *tlog.Logger (one
//   instance per process) wired through bus.Config.Logger and propagated
//   into Hub / Conn / Source via plain field assignment.  This is the
//   single reason callers see `b.logger.Warnf(ctx, ...)` here while the
//   rest of the cli uses package-level `log.Warnf(ctx, ...)`.
//
// On-disk layout:
//   <BusDir>/logs/bus-YYYY-MM-DD.log     (rotated by size at 10 MiB)
//   <BusDir>/logs/bus-YYYY-MM-DD.N.log   (subsequent files of the same day)
// The 7-day retention, daily rotation and async non-blocking writes are all
// inherited from internal/log — see logging.go for the policy details.
//
// Lifetime:
//   The returned *tlog.Logger MUST be Close()'d before the daemon exits so
//   the async channel is drained and on-disk state is consistent.  See
//   cmd/event/bus.go for the wiring (`defer logger.Close()`).

package bus

import (
	eventruntime "tmeet/internal/event"
	"tmeet/internal/exception"
	tlog "tmeet/internal/log"
)

const (
	// busLogSubDir / busLogPrefix are the on-disk layout for bus diagnostics.
	// Files end up at <BusDir>/logs/bus-YYYY-MM-DD.log (and bus-YYYY-MM-DD.N.log
	// once 10 MiB is reached, see internal/log/logging.go for the exact policy).
	busLogSubDir = "logs"
	busLogPrefix = "bus-"
)

// SetupBusLogger initialises and returns the bus daemon's *tlog.Logger.
//
// On rotation/setup failure the call is fatal: bus diagnostics are essential
// for post-mortem debugging, so refusing to start is preferable to running
// silently.
//
// The caller (cmd/event/bus.go) MUST defer Close() on the returned logger so
// buffered entries reach disk before the process exits.
func SetupBusLogger() (*tlog.Logger, error) {
	logger, err := tlog.InitNamed(eventruntime.BusDir(), busLogSubDir, busLogPrefix, tlog.LevelInfo)
	if err != nil {
		return nil, exception.EventInternalError.With("bus log: init: %v", err)
	}
	return logger, nil
}
