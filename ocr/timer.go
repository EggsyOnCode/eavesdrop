package ocr

import "time"

type Timer struct {
	ticker *time.Ticker
	ch     chan time.Time
}
