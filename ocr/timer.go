package ocr

import "time"

type Timer struct {
	ticker  *time.Ticker
	ch      chan time.Time
	stopCh  chan struct{} // signal for stopping the timer
	stopped bool
}

func NewTimer(interval time.Duration) *Timer {
	tmer := &Timer{
		ticker: time.NewTicker(interval),
		ch:     make(chan time.Time),
		stopCh: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case t := <-tmer.ticker.C:
				select {
				case tmer.ch <- t:
				default: // Avoid blocking if no one is listening
				}
			case <-tmer.stopCh:
				tmer.ticker.Stop()
				close(tmer.ch) // Close channel when stopping to notify listeners
				return
			}
		}
	}()
	return tmer
}

// Subscribe returns the timer's event channel.
func (t *Timer) Subscribe() <-chan time.Time {
	return t.ch
}

// Stop stops the timer and releases resources.
func (t *Timer) Stop() {
	if !t.stopped {
		close(t.stopCh) // Signal the goroutine to exit
		t.stopped = true
	}
}
