package serialdispatch

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcherSerializesExecution(t *testing.T) {
	d := New(16)
	defer d.Close()

	var executing int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := d.Dispatch(func() error {
				if atomic.AddInt32(&executing, 1) != 1 {
					t.Fatalf("concurrent execution detected")
				}
				time.Sleep(100 * time.Microsecond)
				atomic.AddInt32(&executing, -1)
				return nil
			})
			if err != nil {
				t.Fatalf("dispatch failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestDispatcherFastPath(t *testing.T) {
	d := New(8)
	defer d.Close()

	var count int32
	for i := 0; i < 10; i++ {
		if err := d.Dispatch(func() error {
			atomic.AddInt32(&count, 1)
			return nil
		}); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}
	}

	if got := atomic.LoadInt32(&count); got != 10 {
		t.Fatalf("expected 10 executions, got %d", got)
	}
}

func TestDispatcherQueueFallback(t *testing.T) {
	d := New(2)
	defer d.Close()

	// Fill token by starting a long-running task synchronously.
	done := make(chan struct{})
	go func() {
		err := d.Dispatch(func() error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Errorf("dispatch: %v", err)
		}
		close(done)
	}()

	// These should fall back to queue and still complete.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.Dispatch(func() error { return nil }); err != nil {
				t.Errorf("dispatch: %v", err)
			}
		}()
	}

	wg.Wait()
	<-done
}

func TestDispatcherClose(t *testing.T) {
	d := New(4)

	err := d.Dispatch(func() error { return nil })
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	d.Close()

	if err := d.Dispatch(func() error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}
