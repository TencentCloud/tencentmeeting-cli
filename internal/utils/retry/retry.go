package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Options is the retry configuration.
type Options struct {
	// MaxAttempts is the maximum number of retries (excluding the first execution); 0 means no retry.
	MaxAttempts int
	// InitialDelay is the initial wait time, default 100ms.
	InitialDelay time.Duration
	// MaxDelay is the maximum wait time cap, default 30s.
	MaxDelay time.Duration
	// Multiplier is the exponential base, default 2.0.
	Multiplier float64
	// Jitter enables random jitter (±25%) to avoid thundering herd, default true.
	Jitter bool
	// RetryIf is a custom function to determine whether to retry; returning false stops retrying immediately.
	// When nil, retries on all errors.
	RetryIf func(err error) bool
}

var DefaultOptions = Options{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     3 * time.Second,
	Multiplier:   2.0,
	Jitter:       true,
	RetryIf:      nil,
}

func (o *Options) applyDefaults() {
	if o.InitialDelay <= 0 {
		o.InitialDelay = 100 * time.Millisecond
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = 30 * time.Second
	}
	if o.Multiplier <= 1 {
		o.Multiplier = 2.0
	}
}

// ErrMaxAttemptsReached wraps the original error when the maximum number of retries is exceeded.
type ErrMaxAttemptsReached struct {
	Attempts int
	Err      error
}

func (e *ErrMaxAttemptsReached) Error() string {
	return e.Err.Error()
}

func (e *ErrMaxAttemptsReached) Unwrap() error {
	return e.Err
}

// IsMaxAttemptsReached checks whether the failure was due to exceeding the maximum number of retries.
func IsMaxAttemptsReached(err error) bool {
	var e *ErrMaxAttemptsReached
	return errors.As(err, &e)
}

// Do executes fn with exponential backoff until success, ctx cancellation, or max retries exceeded.
//
// Wait time formula:
//
//	delay = min(InitialDelay * Multiplier^attempt, MaxDelay)
//	If Jitter is enabled, randomly picks a value in [0.75*delay, 1.25*delay]
func Do(ctx context.Context, fn func(ctx context.Context) error, opts Options) error {
	opts.applyDefaults()

	var lastErr error
	for attempt := 0; attempt <= opts.MaxAttempts; attempt++ {
		// Execute the target function.
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check whether to continue retrying.
		if opts.RetryIf != nil && !opts.RetryIf(lastErr) {
			return lastErr
		}

		// Max retries reached; no more waiting.
		if attempt >= opts.MaxAttempts {
			break
		}

		// Calculate the backoff time for this attempt.
		delay := calcDelay(opts.InitialDelay, opts.MaxDelay, opts.Multiplier, opts.Jitter, attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return &ErrMaxAttemptsReached{
		Attempts: opts.MaxAttempts + 1,
		Err:      lastErr,
	}
}

// calcDelay calculates the wait time before the attempt-th retry.
func calcDelay(initial, maxDelay time.Duration, multiplier float64, jitter bool, attempt int) time.Duration {
	// delay = initial * multiplier^attempt
	delay := float64(initial) * math.Pow(multiplier, float64(attempt))

	// Add random jitter ±25%.
	if jitter {
		// Range: [0.75, 1.25)
		factor := 0.75 + rand.Float64()*0.5
		delay *= factor
	}

	// Do not exceed the upper limit.
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	return time.Duration(delay)
}
