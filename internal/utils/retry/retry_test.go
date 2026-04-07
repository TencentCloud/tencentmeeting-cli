package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errFake = errors.New("fake error")

// --- Do ---

// 首次执行成功，不触发重试
func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	}, Options{MaxAttempts: 3})
	if err != nil {
		t.Fatalf("期望 nil，得到 %v", err)
	}
	if calls != 1 {
		t.Fatalf("期望调用 1 次，实际 %d 次", calls)
	}
}

// 第 N 次成功
func TestDo_SuccessAfterRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errFake
		}
		return nil
	}, Options{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("期望 nil，得到 %v", err)
	}
	if calls != 3 {
		t.Fatalf("期望调用 3 次，实际 %d 次", calls)
	}
}

// 超过最大重试次数，返回 ErrMaxAttemptsReached
func TestDo_ExceedMaxAttempts(t *testing.T) {
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errFake
	}, Options{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
	})
	if err == nil {
		t.Fatal("期望返回错误，得到 nil")
	}
	// 总调用次数 = 1 次首次 + 3 次重试
	if calls != 4 {
		t.Fatalf("期望调用 4 次，实际 %d 次", calls)
	}
	if !IsMaxAttemptsReached(err) {
		t.Fatalf("期望 ErrMaxAttemptsReached，得到 %T: %v", err, err)
	}
	// 原始错误可被 errors.Is 解包
	if !errors.Is(err, errFake) {
		t.Fatalf("期望 errors.Is(err, errFake) == true")
	}
}

// MaxAttempts=0 时只执行一次，失败直接返回
func TestDo_ZeroMaxAttempts(t *testing.T) {
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errFake
	}, Options{MaxAttempts: 0})
	if err == nil {
		t.Fatal("期望返回错误，得到 nil")
	}
	if calls != 1 {
		t.Fatalf("期望调用 1 次，实际 %d 次", calls)
	}
	if !IsMaxAttemptsReached(err) {
		t.Fatalf("期望 ErrMaxAttemptsReached，得到 %T", err)
	}
}

// RetryIf 返回 false 时立即停止重试
func TestDo_RetryIf_StopsEarly(t *testing.T) {
	nonRetryErr := errors.New("不可重试错误")
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		return nonRetryErr
	}, Options{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
		RetryIf: func(err error) bool {
			return !errors.Is(err, nonRetryErr)
		},
	})
	if !errors.Is(err, nonRetryErr) {
		t.Fatalf("期望返回 nonRetryErr，得到 %v", err)
	}
	if calls != 1 {
		t.Fatalf("RetryIf=false 时应只调用 1 次，实际 %d 次", calls)
	}
}

// RetryIf 对部分错误重试，对另一部分不重试
func TestDo_RetryIf_RetryOnSpecificError(t *testing.T) {
	retryErr := errors.New("可重试")
	calls := 0
	err := Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return retryErr
		}
		return nil
	}, Options{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
		RetryIf: func(err error) bool {
			return errors.Is(err, retryErr)
		},
	})
	if err != nil {
		t.Fatalf("期望 nil，得到 %v", err)
	}
	if calls != 3 {
		t.Fatalf("期望调用 3 次，实际 %d 次", calls)
	}
}

// ctx 取消时立即退出，返回 ctx.Err()
func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := Do(ctx, func(ctx context.Context) error {
		calls++
		cancel() // 第一次执行后立即取消
		return errFake
	}, Options{
		MaxAttempts:  10,
		InitialDelay: 100 * time.Millisecond, // 等待期间会被 ctx 取消
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("期望 context.Canceled，得到 %v", err)
	}
	if calls != 1 {
		t.Fatalf("ctx 取消后不应继续重试，实际调用 %d 次", calls)
	}
}

// ctx 超时
func TestDo_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Do(ctx, func(ctx context.Context) error {
		return errFake
	}, Options{
		MaxAttempts:  100,
		InitialDelay: 30 * time.Millisecond, // 每次等待 30ms，超时后退出
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 context.DeadlineExceeded，得到 %v", err)
	}
}

// --- calcDelay ---

func TestCalcDelay_ExponentialGrowth(t *testing.T) {
	initial := 100 * time.Millisecond
	maxDelay := 10 * time.Second

	d0 := calcDelay(initial, maxDelay, 2.0, false, 0) // 100ms * 2^0 = 100ms
	d1 := calcDelay(initial, maxDelay, 2.0, false, 1) // 100ms * 2^1 = 200ms
	d2 := calcDelay(initial, maxDelay, 2.0, false, 2) // 100ms * 2^2 = 400ms

	if d0 != 100*time.Millisecond {
		t.Errorf("attempt=0 期望 100ms，得到 %v", d0)
	}
	if d1 != 200*time.Millisecond {
		t.Errorf("attempt=1 期望 200ms，得到 %v", d1)
	}
	if d2 != 400*time.Millisecond {
		t.Errorf("attempt=2 期望 400ms，得到 %v", d2)
	}
}

func TestCalcDelay_MaxDelayCap(t *testing.T) {
	initial := 1 * time.Second
	maxDelay := 3 * time.Second

	// 1s * 2^10 = 1024s，远超 maxDelay
	d := calcDelay(initial, maxDelay, 2.0, false, 10)
	if d != maxDelay {
		t.Errorf("期望被截断为 %v，得到 %v", maxDelay, d)
	}
}

func TestCalcDelay_JitterInRange(t *testing.T) {
	initial := 1 * time.Second
	maxDelay := 10 * time.Second

	// 多次采样，验证抖动范围在 [0.75, 1.25] 之间
	for i := 0; i < 200; i++ {
		d := calcDelay(initial, maxDelay, 2.0, true, 0) // base = 1s
		low := time.Duration(float64(initial) * 0.75)
		high := time.Duration(float64(initial) * 1.25)
		if d < low || d > high {
			t.Fatalf("抖动超出范围 [%v, %v]，得到 %v", low, high, d)
		}
	}
}

// --- IsMaxAttemptsReached ---

func TestIsMaxAttemptsReached(t *testing.T) {
	wrapped := &ErrMaxAttemptsReached{Attempts: 3, Err: errFake}

	if !IsMaxAttemptsReached(wrapped) {
		t.Error("期望 true，得到 false")
	}
	if IsMaxAttemptsReached(errFake) {
		t.Error("普通 error 期望 false，得到 true")
	}
	if IsMaxAttemptsReached(nil) {
		t.Error("nil 期望 false，得到 true")
	}
}

// ErrMaxAttemptsReached.Error() 透传原始错误信息
func TestErrMaxAttemptsReached_ErrorMessage(t *testing.T) {
	e := &ErrMaxAttemptsReached{Attempts: 2, Err: errFake}
	if e.Error() != errFake.Error() {
		t.Errorf("期望 %q，得到 %q", errFake.Error(), e.Error())
	}
}

// --- applyDefaults ---

func TestApplyDefaults(t *testing.T) {
	o := &Options{}
	o.applyDefaults()

	if o.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay 默认值期望 100ms，得到 %v", o.InitialDelay)
	}
	if o.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay 默认值期望 30s，得到 %v", o.MaxDelay)
	}
	if o.Multiplier != 2.0 {
		t.Errorf("Multiplier 默认值期望 2.0，得到 %v", o.Multiplier)
	}
}

func TestApplyDefaults_NoOverrideWhenSet(t *testing.T) {
	o := &Options{
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     1 * time.Minute,
		Multiplier:   3.0,
	}
	o.applyDefaults()

	if o.InitialDelay != 500*time.Millisecond {
		t.Errorf("不应覆盖已设置的 InitialDelay，得到 %v", o.InitialDelay)
	}
	if o.MaxDelay != 1*time.Minute {
		t.Errorf("不应覆盖已设置的 MaxDelay，得到 %v", o.MaxDelay)
	}
	if o.Multiplier != 3.0 {
		t.Errorf("不应覆盖已设置的 Multiplier，得到 %v", o.Multiplier)
	}
}
