package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	systemd "github.com/iguanesolutions/go-systemd"
)

func handleSignals(butlerSignal chan<- struct{}, butlerStopped *sync.WaitGroup, mainStop *sync.Mutex) {
	// If we exit, allow main goroutine to do so
	defer mainStop.Unlock()
	// Register signals
	var sig os.Signal
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	// Waiting for signals to catch
	var err error
	for {
		sig = <-signalChannel
		switch sig {
		case syscall.SIGUSR1:
			if err = systemd.NotifyReloading(); err != nil {
				logger.Errorf("[Main] Sending reloading notification to systemd failed: %v", err)
			}
			logger.Infof("[Main] Signal '%v' caught: forcing the butler to run now", sig)
			butlerBatch()
			if err = systemd.NotifyReady(); err != nil {
				logger.Errorf("[Main] Sending ready notification to systemd failed: %v", err)
			}
		case syscall.SIGTERM:
			fallthrough
		case syscall.SIGINT:
			// Notify stop
			logger.Infof("[Main] Signal '%v' caught: cleaning up before exiting", sig)
			if err = systemd.NotifyStopping(); err != nil {
				logger.Errorf("[Main] Sending stopping notification to systemd failed: %v", err)
			}
			// Start stop (haha)
			butlerSignal <- struct{}{}
			logger.Debug("[Main] Stop signal sent to butler, waiting for its goroutine to finish")
			// Wait stop
			butlerStopped.Wait()
			logger.Debug("[Main] butler has stopped, unlocking main goroutine to exit")
			return
		default:
			logger.Warningf("[Main] Signal '%v' caught but no process set to handle it: skipping", sig)
		}
	}
}
