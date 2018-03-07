package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func handleSignals(butlerSignal chan<- struct{}, butlerStopped *sync.WaitGroup, mainStop *sync.Mutex) {
	// If we exit, allow main goroutine to do so
	defer mainStop.Unlock()
	// Register signals
	var sig os.Signal
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	// Waiting for signals to catch
	for {
		sig = <-signalChannel
		switch sig {
		case syscall.SIGUSR1:
			logger.Infof("[Main] Signal '%v' caught: forcing the butler to run now", sig)
			go butlerBatch()
		case syscall.SIGTERM:
			fallthrough
		case syscall.SIGINT:
			logger.Infof("[Main] Signal '%v' caught: cleaning up before exiting", sig)
			butlerSignal <- struct{}{}
			logger.Debug("[Main] Stop signal sent to butler, waiting for its goroutine to finish")
			butlerStopped.Wait()
			logger.Debug("[Main] butler has stopped, unlocking main goroutine to exit")
			return
		default:
			logger.Warningf("[Main] Signal '%v' caught but no process set to handle it: skipping", sig)
		}
	}
}
