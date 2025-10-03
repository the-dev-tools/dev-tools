package serialdispatch

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrClosed is returned when dispatching after the dispatcher has been closed.
var ErrClosed = errors.New("serialdispatch: dispatcher closed")

// Dispatcher serializes function execution, ensuring only one task runs at a time.
// It supports an optimistic fast path: if no work is pending and the token is
// available, the task executes synchronously without going through the queue.
type Dispatcher struct {
	queue  chan job
	token  chan struct{}
	wg     sync.WaitGroup
	closed atomic.Bool
}

type job struct {
	fn    func() error
	errCh chan error
}

// New creates a Dispatcher with the provided queue capacity. If capacity <= 0,
// a default of 128 is used.
func New(capacity int) *Dispatcher {
	if capacity <= 0 {
		capacity = 128
	}
	d := &Dispatcher{
		queue: make(chan job, capacity),
		token: make(chan struct{}, 1),
	}
	// token available indicates stream is idle
	d.token <- struct{}{}

	d.wg.Add(1)
	go d.run()
	return d
}

func (d *Dispatcher) run() {
	defer d.wg.Done()
	for job := range d.queue {
		// Acquire token (blocks until available)
		<-d.token
		err := job.fn()
		d.token <- struct{}{}

		job.errCh <- err
		close(job.errCh)
	}
}

// Dispatch schedules fn for execution. If possible, fn runs synchronously; otherwise
// it is queued and executed by the worker. Returns ErrClosed if dispatcher is closed.
func (d *Dispatcher) Dispatch(fn func() error) error {
	if d.closed.Load() {
		return ErrClosed
	}

	// Fast path: no pending jobs and token available.
	if len(d.queue) == 0 {
		select {
		case <-d.token:
			err := fn()
			d.token <- struct{}{}
			return err
		default:
		}
	}

	job := job{
		fn:    fn,
		errCh: make(chan error, 1),
	}

	select {
	case d.queue <- job:
	default:
		// Queue full, force synchronous execution by acquiring token.
		select {
		case <-d.token:
			err := fn()
			d.token <- struct{}{}
			return err
		case d.queue <- job:
		}
	}

	return <-job.errCh
}

// Close stops the dispatcher after all pending jobs complete. Repeated calls are safe.
func (d *Dispatcher) Close() {
	if d.closed.Swap(true) {
		return
	}
	close(d.queue)
	d.wg.Wait()
}
