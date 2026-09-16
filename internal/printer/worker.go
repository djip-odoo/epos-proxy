package printer

import (
	"sync"
	"time"

	"epos-proxy/internal/logger"
)

type JobResult struct {
	OK  bool
	Err error
}

type JobFunc func(p Printer) JobResult

type Job struct {
	run   JobFunc
	reply chan JobResult
}

type printerWorker struct {
	printer Printer
	jobs    chan Job
	mu      sync.Mutex
	closed  bool
}

func newPrinterWorker(p Printer) *printerWorker {
	w := &printerWorker{
		printer: p,
		jobs:    make(chan Job, QueueSize),
	}
	go w.loop()
	return w
}

func (w *printerWorker) Enqueue(fn JobFunc, reply chan JobResult) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrNotFound
	}
	w.mu.Unlock()

	j := Job{run: fn, reply: reply}
	select {
	case w.jobs <- j:
		logger.Debugf("Enqueued print job for printer %s", w.printer.ID())
		return nil
	default:
		logger.Warnf("Printer queue full for printer %s", w.printer.ID())
		return ErrQueueFull
	}
}

func (w *printerWorker) loop() {
	logger.Debugf("Printer loop started for %s", w.printer.ID())
	var idleTimer *time.Timer
	var idleC <-chan time.Time

	startIdleTimer := func() {
		if idleTimer == nil {
			idleTimer = time.NewTimer(IdleTimeout)
		} else {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(IdleTimeout)
		}
		idleC = idleTimer.C
	}

	stopIdleTimer := func() {
		if idleTimer != nil {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleC = nil
		}
	}

	if len(w.jobs) == 0 {
		startIdleTimer()
	}

	for {
		select {
		case j, ok := <-w.jobs:
			if !ok {
				stopIdleTimer()
				w.printer.Close()
				return
			}
			stopIdleTimer()
			result := j.run(w.printer)
			if j.reply != nil {
				j.reply <- result
				close(j.reply)
			}
			if len(w.jobs) == 0 {
				startIdleTimer()
			}
		case <-idleC:
			logger.Debugf("Idle timeout reached for printer %s, closing device connection", w.printer.ID())
			w.printer.Close()
			idleC = nil
		}
	}
}

func (w *printerWorker) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	close(w.jobs)
	w.mu.Unlock()
	w.printer.Close()
}
