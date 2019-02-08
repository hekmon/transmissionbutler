package main

import (
	"sync"
	"time"

	"github.com/hekmon/transmissionrpc"
)

func butler(stopSignal <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create the ticker
	logger.Infof("[Butler] Will work every %v", conf.Butler.CheckFrequency)
	tick := time.NewTicker(conf.Butler.CheckFrequency)
	defer tick.Stop()
	// Start first batch
	butlerBatch()
	// Wait for ticks or cancellation
	for {
		select {
		case <-tick.C:
			butlerBatch()
		case <-stopSignal:
			logger.Debug("[Butler] stop signal received")
			return
		}
	}
}

var fields = []string{"id", "name", "totalSize", "status", "doneDate", "seedRatioLimit", "seedRatioMode", "uploadRatio"}

func butlerBatch() {
	// Check that global ratio limit is activated and set with correct value
	logger.Debug("[Butler] Fetching session data")
	session, err := transmission.SessionArgumentsGet()
	if err == nil {
		globalRatio(session)
	} else {
		logger.Errorf("[Butler] Can't check global ratio: can't get sessions values: %v", err)
	}
	// Get all torrents status
	logger.Debug("[Butler] Fetching torrents metadata")
	torrents, err := transmission.TorrentGet(fields, nil)
	if err != nil {
		logger.Errorf("[Butler] Can't retrieve torrent(s) metadata: %v", err)
		return
	}
	logger.Infof("[Butler] Fetched %d torrent(s) metadata", len(torrents))
	// Inspect each torrent
	freeseedCandidates, globalratioCandidates, customratioCandidates, todeleteCandidates := inspectTorrents(torrents)
	// Updates what need to be updated
	handleFreeseedCandidates(freeseedCandidates)
	handleGlobalratioCandidates(globalratioCandidates)
	handleCustomratioCandidates(customratioCandidates)
	handleTodeleteCandidates(todeleteCandidates, session.DownloadDir)
}

func globalRatio(session *transmissionrpc.SessionArguments) {
	var updateRatio, updateRatioEnabled bool
	// Ratio value
	if session.SeedRatioLimit != nil {
		if *session.SeedRatioLimit != conf.Butler.TargetRatio {
			logger.Infof("[Butler] Global ratio is invalid (%f instead of %f): scheduling update",
				*session.SeedRatioLimit, conf.Butler.TargetRatio)
			updateRatio = true
		} else {
			logger.Debugf("[Butler] Session SeedRatioLimit: %v", *session.SeedRatioLimit)
		}
	} else {
		logger.Error("[Butler] Can't check global ratio value: SeedRatioLimit session value is nil")
	}
	// Global ratio enabled
	if session.SeedRatioLimited != nil {
		if !*session.SeedRatioLimited {
			logger.Infof("[Butler] Global ratio is disabled: scheduling activation")
			updateRatioEnabled = true
		} else {
			logger.Debugf("[Butler] Session SeedRatioLimited: %v", *session.SeedRatioLimited)
		}
	} else {
		logger.Error("[Butler] Can't check global ratio value: SeedRatioLimited session value is nil")
	}
	// Update
	if updateRatio || updateRatioEnabled {
		updateRatioEnabled = true
		err := transmission.SessionArgumentsSet(&transmissionrpc.SessionArguments{
			SeedRatioLimit:   &conf.Butler.TargetRatio,
			SeedRatioLimited: &updateRatioEnabled,
		})
		if err == nil {
			logger.Infof("[Butler] Global ratio set and activated")
		} else {
			logger.Errorf("[Butler] Can't update global ratio: %v", err)
		}
	}
}
