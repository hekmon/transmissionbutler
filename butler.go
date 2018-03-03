package main

import (
	"sync"
	"time"

	"github.com/hekmon/transmissionrpc"
)

func butler(frequency time.Duration, stopSignal <-chan struct{}, wg *sync.WaitGroup) {
	logger.Infof("[Butler] Will work every %v", frequency)
	defer wg.Done()
	// Create the ticker
	tick := time.NewTicker(frequency)
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

var fields = []string{"id", "name", "doneDate", "isFinished", "seedRatioLimit", "seedRatioMode", "uploadRatio"}

func butlerBatch() {
	// Get all torrents status
	logger.Debug("[Butler] Fetching torrents' data")
	torrents, err := transmission.TorrentGet(fields, nil)
	if err != nil {
		logger.Errorf("[Butler] Can't retreive torrent(s): %v", err)
		return
	}
	logger.Infof("[Butler] Fetched %d torrent(s) metadata", len(torrents))
	// Inspect each torrent
	for index, torrent := range torrents {
		// Checks
		if !torrentOK(torrent, index) {
			continue
		}
		// We can now safely access metadataa
		logger.Debugf("[Butler] Inspecting torrent %d:\n\tid: %d\n\tname: %s\n\tdoneDate: %v\n\tisFinished: %v\n\tseedRatioLimit: %f\n\tseedRatioMode: %d\n\tuploadRatio:%f",
			index, *torrent.ID, *torrent.Name, *torrent.DoneDate, *torrent.IsFinished, *torrent.SeedRatioLimit, *torrent.SeedRatioMode, *torrent.UploadRatio)
	}
}

func torrentOK(torrent *transmissionrpc.Torrent, index int) (ok bool) {
	if torrent == nil {
		logger.Warningf("[Butler] Encountered a nil torrent at index %d", index)
		return
	}
	if torrent.ID == nil {
		logger.Warningf("[Butler] Encountered a nil torrent id at index %d", index)
		return
	}
	if torrent.Name == nil {
		logger.Warningf("[Butler] Encountered a nil torrent name at index %d", index)
		return
	}
	if torrent.DoneDate == nil {
		logger.Warningf("[Butler] Encountered a nil torrent doneDate at index %d", index)
		return
	}
	if torrent.IsFinished == nil {
		logger.Warningf("[Butler] Encountered a nil torrent isFinished at index %d", index)
		return
	}
	if torrent.SeedRatioLimit == nil {
		logger.Warningf("[Butler] Encountered a nil torrent seedRatioLimit at index %d", index)
		return
	}
	if torrent.SeedRatioMode == nil {
		logger.Warningf("[Butler] Encountered a nil torrent seedRatioMode at index %d", index)
		return
	}
	if torrent.UploadRatio == nil {
		logger.Warningf("[Butler] Encountered a nil torrent ID at index %d", index)
		return
	}
	return true
}
