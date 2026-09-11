package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"tmeet/internal/core/filelock"
)

// Level represents the logging severity level.
type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// levelName maps Level values to their string representations.
var levelName = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

const (
	maxFileSize   = 10 * 1024 * 1024 // 10 MB per log file
	maxRetainDay  = 7                // retain logs for the last 7 days
	logSubDir     = "logs"
	logPrefix     = "tmeet-"
	logExt        = ".log"
	logTimeFormat = "2006-01-02 15:04:05.000"
	chanSize      = 4096 // channel buffer size; callers rarely block
)

// entry is a log record delivered to the background goroutine via channel.
type entry struct {
	line string // fully-formatted log line
}

// Logger is a file-based logger that supports daily rotation and per-file size limits.
//
// Log files are stored under <logDir>/logs/ with the following naming convention:
//   - tmeet-2006-01-02.log      (first file of the day)
//   - tmeet-2006-01-02.1.log    (second file of the day, and so on)
//
// Multi-process concurrent write safety:
//   - Normal writes: the file is opened with O_APPEND; POSIX guarantees that a
//     single write() call is atomic as long as the line is shorter than PIPE_BUF
//     (~4 KB). bufio buffering is intentionally omitted because it would merge
//     multiple lines into one write() call, breaking atomicity.
//   - File rotation: exceeding 10MB requires "check size → create new file",
//     which has a TOCTOU race. filelock.WithLock serialises the rotation critical
//     section across processes so only one process creates the new file.
//
// Write model:
//   - Callers format an entry and send it to a buffered channel, returning immediately.
//   - A single background goroutine consumes the channel serially, handling rotation
//     and actual disk writes.
//   - Close() drains the channel and flushes to disk before returning, ensuring no
//     log records are lost.
type Logger struct {
	ch     chan entry    // channel for delivering log entries
	levelV atomic.Int32  // minimum log level, read/written atomically
	done   chan struct{} // closed when the background goroutine exits

	// The fields below are accessed only by the background goroutine; no locking needed.
	logDir   string
	lockPath string // path of the cross-process file lock used during rotation
	file     *os.File
	date     string
	seq      int
	written  int64

	// subDir / prefix override the package-level defaults (logSubDir / logPrefix)
	// for Loggers built via InitNamed.  Zero value means "use default".
	subDir string
	prefix string
}

// filePrefix returns the file-name prefix this Logger uses; falls back to the
// package default when the instance was constructed via the legacy Init path
// (which leaves prefix empty).
func (l *Logger) filePrefix() string {
	if l.prefix == "" {
		return logPrefix
	}
	return l.prefix
}

// fileSubDir returns the sub-directory name beneath logDir's parent.  Only
// used by InitNamed callers that want a custom layout; legacy Init keeps the
// historical "logs" sub-dir.
func (l *Logger) fileSubDir() string {
	if l.subDir == "" {
		return logSubDir
	}
	return l.subDir
}

// defaultLogger is the package-level Logger initialised by Init.
var defaultLogger *Logger

// Init initialises the global file logging system.
//
//   - logDir: root directory for logs (typically config.GetConfigDir())
//   - level:  minimum severity level to record
//
// It creates logDir/logs/ if it does not exist and removes log files older than
// 7 days. If initialisation fails the global Logger remains nil and subsequent
// log calls are silently ignored.
func Init(logDir string, level Level) error {
	l, err := newLogger(logDir, logSubDir, logPrefix, level)
	if err != nil {
		return err
	}
	defaultLogger = l
	return nil
}

// InitNamed builds a standalone *Logger that writes to <logDir>/<subDir>/ with
// files named "<prefix><date>.log" (and rotated as "<prefix><date>.N.log").
//
// Unlike Init, the returned Logger is NOT registered as the global
// defaultLogger; the caller owns it and MUST call (*Logger).Close() before
// process exit to flush buffered entries.  This is intended for long-running
// auxiliary processes (e.g. the event bus daemon) that want their own log
// namespace while still benefiting from rotation, retention and async writes.
//
// Empty subDir / prefix fall back to the package defaults ("logs" / "tmeet-").
func InitNamed(logDir, subDir, prefix string, level Level) (*Logger, error) {
	if subDir == "" {
		subDir = logSubDir
	}
	if prefix == "" {
		prefix = logPrefix
	}
	return newLogger(logDir, subDir, prefix, level)
}

// newLogger is the shared constructor behind Init and InitNamed.  It creates
// the on-disk directory, runs cleanup, opens the first file and starts the
// background writer goroutine.
func newLogger(logDir, subDir, prefix string, level Level) (*Logger, error) {
	dir := filepath.Join(logDir, subDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("log: failed to create log directory %s: %w", dir, err)
	}

	l := &Logger{
		ch:       make(chan entry, chanSize),
		done:     make(chan struct{}),
		logDir:   dir,
		lockPath: filepath.Join(dir, "rotate.lock"),
		subDir:   subDir,
		prefix:   prefix,
	}
	l.levelV.Store(int32(level))

	// Clean up expired logs before starting the background goroutine.
	l.cleanup()

	// Open (or create) today's log file.
	if err := l.rotate(); err != nil {
		return nil, err
	}

	// Start the background writer goroutine.
	go l.loop()

	return l, nil
}

// SetLevel dynamically changes the minimum log level of the global Logger.
func SetLevel(level Level) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.levelV.Store(int32(level))
}

// Close drains the channel, waits for all pending writes to be flushed to disk,
// and then closes the file handle. Must be called before the process exits to
// avoid losing buffered log records.
func Close() {
	if defaultLogger == nil {
		return
	}
	defaultLogger.Close()
}

// Close drains the channel and waits for the background goroutine to finish
// flushing buffered entries to disk.  Safe to call multiple times: subsequent
// calls are no-ops once the channel has been closed.
//
// Must be invoked before the owning process exits when the Logger was created
// via InitNamed; legacy callers that go through the package-level Init/Close
// pair never touch this method directly.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	// Recover from a double-close: closing an already-closed channel panics,
	// but we still want the second Close() to wait for done so callers can
	// rely on "after Close returns, nothing is writing".
	defer func() { _ = recover() }()
	close(l.ch)
	<-l.done
}

// Debug writes a DEBUG-level log record.
func Debug(ctx context.Context, args ...interface{}) {
	defaultLogger.send(ctx, LevelDebug, fmt.Sprint(args...), 2)
}

// Debugf writes a formatted DEBUG-level log record.
func Debugf(ctx context.Context, format string, args ...interface{}) {
	defaultLogger.send(ctx, LevelDebug, fmt.Sprintf(format, args...), 2)
}

// Info writes an INFO-level log record.
func Info(ctx context.Context, args ...interface{}) {
	defaultLogger.send(ctx, LevelInfo, fmt.Sprint(args...), 2)
}

// Infof writes a formatted INFO-level log record.
func Infof(ctx context.Context, format string, args ...interface{}) {
	defaultLogger.send(ctx, LevelInfo, fmt.Sprintf(format, args...), 2)
}

// Warn writes a WARN-level log record.
func Warn(ctx context.Context, args ...interface{}) {
	defaultLogger.send(ctx, LevelWarn, fmt.Sprint(args...), 2)
}

// Warnf writes a formatted WARN-level log record.
func Warnf(ctx context.Context, format string, args ...interface{}) {
	defaultLogger.send(ctx, LevelWarn, fmt.Sprintf(format, args...), 2)
}

// Error writes an ERROR-level log record.
func Error(ctx context.Context, args ...interface{}) {
	defaultLogger.send(ctx, LevelError, fmt.Sprint(args...), 2)
}

// Errorf writes a formatted ERROR-level log record.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	defaultLogger.send(ctx, LevelError, fmt.Sprintf(format, args...), 2)
}

// ---- instance-level formatted helpers (used by InitNamed callers) ----
//
// These mirror the package-level Debugf/Infof/Warnf/Errorf but write to a
// specific Logger instance instead of the global defaultLogger.  They are
// intended for long-running processes (e.g. the event bus daemon) that own
// their own Logger via InitNamed.  Behaviour — level filtering, caller
// capture, async dispatch — is identical to the package-level helpers.

// Debugf writes a formatted DEBUG-level record to this Logger instance.
func (l *Logger) Debugf(ctx context.Context, format string, args ...interface{}) {
	l.send(ctx, LevelDebug, fmt.Sprintf(format, args...), 2)
}

// Infof writes a formatted INFO-level record to this Logger instance.
func (l *Logger) Infof(ctx context.Context, format string, args ...interface{}) {
	l.send(ctx, LevelInfo, fmt.Sprintf(format, args...), 2)
}

// Warnf writes a formatted WARN-level record to this Logger instance.
func (l *Logger) Warnf(ctx context.Context, format string, args ...interface{}) {
	l.send(ctx, LevelWarn, fmt.Sprintf(format, args...), 2)
}

// Errorf writes a formatted ERROR-level record to this Logger instance.
func (l *Logger) Errorf(ctx context.Context, format string, args ...interface{}) {
	l.send(ctx, LevelError, fmt.Sprintf(format, args...), 2)
}

// ---- internal implementation ----

// send formats a log line and delivers it to the channel without blocking.
// skip is the number of call-stack frames to skip relative to send itself.
func (l *Logger) send(ctx context.Context, level Level, msg string, skip int) {
	if l == nil {
		return
	}

	// Level filtering happens here to avoid unnecessary formatting.
	if level < Level(l.levelV.Load()) {
		return
	}

	caller := getCaller(skip + 1) // +1 to skip send itself
	now := time.Now().Format(logTimeFormat)
	traceID := ctx.Value(CtxTraceIDKey)
	line := fmt.Sprintf("%s [%v] [%s] %s %s\n", now, traceID, levelName[level], caller, msg)

	// Non-blocking send: drop the record if the channel is full to avoid
	// stalling the caller (should only happen under extreme load).
	select {
	case l.ch <- entry{line: line}:
	default:
		// channel full – record dropped
	}
}

// loop is the main loop of the background writer goroutine; it consumes entries serially.
func (l *Logger) loop() {
	defer func() {
		if l.file != nil {
			_ = l.file.Close()
		}
		close(l.done)
	}()

	for e := range l.ch {
		l.write(e.line)
	}
}

// write writes a single formatted log line to the current file.
// Must only be called from the background goroutine.
//
// Write strategy:
//   - Writes directly to os.File (no bufio buffering). Combined with O_APPEND
//     this makes each write atomic, preventing interleaved lines from concurrent
//     processes.
//   - File rotation (>10 MB) is protected by a cross-process filelock to prevent
//     multiple processes from creating the new file simultaneously.
func (l *Logger) write(line string) {
	if err := l.checkRotate(); err != nil {
		return
	}
	n, err := fmt.Fprint(l.file, line)
	if err == nil {
		l.written += int64(n)
	}
}

// checkRotate checks whether the current log file needs to be rotated.
// Must only be called from the background goroutine.
func (l *Logger) checkRotate() error {
	today := time.Now().Format("2006-01-02")

	// Date changed: switch to a new daily file and reset the sequence number.
	if today != l.date {
		return l.rotate()
	}

	// File exceeds 10 MB: use a cross-process file lock to protect the rotation.
	if l.written >= maxFileSize {
		return l.rotateWithLock()
	}

	return nil
}

// rotate switches to today's log file, resetting the sequence number.
// No locking is needed; this is only called during process startup or on a date change.
func (l *Logger) rotate() error {
	l.date = time.Now().Format("2006-01-02")
	l.seq = l.findLastSeq(l.date)
	return l.openFile()
}

// rotateWithLock performs file rotation under a cross-process file lock.
//
// TOCTOU race addressed:
//
//	Process A and process B both see written >= 10 MB and want to create .1.log.
//	With the lock, A goes first: it creates .1.log and starts writing.
//	When B acquires the lock it re-scans the directory, finds .1.log already
//	exists and is not yet full, and reuses it instead of creating a duplicate.
func (l *Logger) rotateWithLock() error {
	return filelock.WithLock(l.lockPath, func() error {
		// Re-scan the directory to get the latest sequence number
		// (another process may have already rotated).
		latestSeq := l.findLastSeq(l.date)
		l.seq = latestSeq

		// Open the latest file; if it is already full, openFile will increment seq.
		return l.openFile()
	})
}

// findLastSeq scans the log directory and returns the highest sequence number
// found for the given date.
func (l *Logger) findLastSeq(date string) int {
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return 0
	}
	datePrefix := l.filePrefix() + date
	maxSeq := -1
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, datePrefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, datePrefix)
		if suffix == logExt {
			// tmeet-2026-04-17.log → seq 0
			if maxSeq < 0 {
				maxSeq = 0
			}
		} else if strings.HasPrefix(suffix, ".") && strings.HasSuffix(suffix, logExt) {
			// tmeet-2026-04-17.1.log → seq 1, etc.
			seqStr := strings.TrimSuffix(strings.TrimPrefix(suffix, "."), logExt)
			var seq int
			if _, err2 := fmt.Sscanf(seqStr, "%d", &seq); err2 == nil && seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	if maxSeq < 0 {
		return 0
	}
	return maxSeq
}

// openFile opens (or creates) the log file for the current date and sequence number.
// If the file is already full it increments seq and recurses.
// Must only be called from the background goroutine.
func (l *Logger) openFile() error {
	// Close the previous file handle if one is open.
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	path := l.currentPath()
	// O_APPEND: the kernel moves the write offset to EOF before every write(),
	// guaranteeing atomic appends from multiple concurrent processes.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("log: failed to open log file %s: %w", path, err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("log: failed to stat log file %s: %w", path, err)
	}

	// If the existing file is already full, roll over immediately.
	if info.Size() >= maxFileSize {
		_ = f.Close()
		l.seq++
		return l.openFile()
	}

	l.file = f
	l.written = info.Size()
	return nil
}

// currentPath returns the full path of the current log file.
func (l *Logger) currentPath() string {
	prefix := l.filePrefix()
	if l.seq == 0 {
		return filepath.Join(l.logDir, prefix+l.date+logExt)
	}
	return filepath.Join(l.logDir, fmt.Sprintf("%s%s.%d%s", prefix, l.date, l.seq, logExt))
}

// cleanup removes log files older than maxRetainDay days.
func (l *Logger) cleanup() {
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -maxRetainDay)

	type logFile struct {
		name string
		date time.Time
	}
	var files []logFile

	prefix := l.filePrefix()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, logExt) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if len(rest) < 10 {
			continue
		}
		dateStr := rest[:10]
		t, err2 := time.Parse("2006-01-02", dateStr)
		if err2 != nil {
			continue
		}
		files = append(files, logFile{name: name, date: t})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].date.Before(files[j].date)
	})

	for _, f := range files {
		if f.date.Before(cutoff) {
			_ = os.Remove(filepath.Join(l.logDir, f.name))
		}
	}
}

// getCaller returns a "filename:line" string for the caller.
// skip is the number of stack frames to skip above getCaller itself.
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown:0"
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	return fmt.Sprintf("%s:%d", short, line)
}

// GetLogConfig get log config
func GetLogConfig() (subDir, prefix, ext, timeFormat string) {
	return logSubDir, logPrefix, logExt, logTimeFormat
}
