package main

import (
	"sync"
	"time"
)

func butler(frequency time.Duration, stopSignal <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create the ticker
	tick := time.NewTicker(frequency)
	defer tick.Stop()
	// Wait for ticks or cancellation
	for {
		select {
		case <-tick.C:
			logger.Debug("[Butler] new tick received, launching batch")
			////TODO
		case <-stopSignal:
			logger.Debug("[Butler] stop signal received, exiting")
			return
		}
	}
}
