package filelock

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFileLockTimeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	held := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = WithLock(lockPath, func() error {
			close(held)
			<-release
			return nil
		})
	}()

	<-held

	err := WithLock(lockPath, func() error {
		return nil
	})
	if err == nil {
		t.Fatal("WithLock should timeout when lock is held")
	}

	close(release)
}

func TestWithLockConcurrentExclusive(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "exclusive.lock")
	outPath := filepath.Join(dir, "out.log")

	const n = 16
	var wg sync.WaitGroup
	var inCritical int32
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = WithLock(lockPath, func() error {
				entered := atomic.AddInt32(&inCritical, 1)
				if entered != 1 {
					return fmt.Errorf("mutex violation: concurrent entries=%d", entered)
				}
				defer atomic.AddInt32(&inCritical, -1)

				f, err := os.OpenFile(outPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(f, "%d\n", idx)
				_ = f.Close()
				return err
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

func TestWithLockReleasedAfterCallbackError(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "err.lock")

	err := WithLock(lockPath, func() error {
		return fmt.Errorf("intentional failure")
	})
	if err == nil {
		t.Fatal("expected error from callback")
	}

	// 锁应已释放，第二次应能立即拿到。
	err = WithLock(lockPath, func() error { return nil })
	if err != nil {
		t.Fatalf("second WithLock after failed fn: %v", err)
	}
}

func TestWithLockEmptyPathStillOpensLockFile(t *testing.T) {
	// lockPath 落在 TempDir 下，避免污染工作目录。
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "empty_callback.lock")
	err := WithLock(lockPath, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}
