package ocr

import "time"

type Timer struct {
    ticker *time.Ticker
    ch     chan time.Time
}

func NewTimer(interval time.Duration) *Timer {
    c := &Timer{
        ticker: time.NewTicker(interval),
        ch:     make(chan time.Time),
    }
    go func() {
        for t := range c.ticker.C {
            c.ch <- t
        }
    }()
    return c
}

func (t *Timer) Subscribe() <-chan time.Time {
    return t.ch
}

