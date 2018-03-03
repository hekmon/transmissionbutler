package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func handleSignals(mainStop *sync.Mutex) {
	// If we exit, allow main goroutine to do so
	defer mainStop.Unlock()
	// Register signals
	var sig os.Signal
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	// Waiting for signals to catch
	for {
		sig = <-signalChannel
		switch sig {
		case syscall.SIGTERM:
			fallthrough
		case syscall.SIGINT:
			logger.Infof("[Main] Signal '%v' caught: cleaning up before exiting", sig)
			stopButler()
			break
		default:
			logger.Warningf("[Main] Signal '%v' caught but no process set to handle it: skipping", sig)
		}
	}
}

func stopButler() {

}
