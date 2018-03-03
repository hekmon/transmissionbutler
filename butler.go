package main

import (
	"sync"
	"time"

	"github.com/hekmon/transmissionrpc"
)

func butler(conf *butlerConfig, stopSignal <-chan struct{}, wg *sync.WaitGroup) {
	logger.Infof("[Butler] Will work every %v", conf.CheckFrequency)
	defer wg.Done()
	// Create the ticker
	tick := time.NewTicker(conf.CheckFrequency)
	defer tick.Stop()
	// Start first batch
	butlerBatch(conf)
	// Wait for ticks or cancellation
	for {
		select {
		case <-tick.C:
			butlerBatch(conf)
		case <-stopSignal:
			logger.Debug("[Butler] stop signal received")
			return
		}
	}
}

var fields = []string{"id", "name", "doneDate", "isFinished", "seedRatioLimit", "seedRatioMode", "uploadRatio"}

// seedRatioMode
//  0 : global limit
//	1 : custom limit
//	2 : no ratio limit

func butlerBatch(conf *butlerConfig) {
	// Check that global ratio limit is activated and set with correct value
	//// TODO
	// Get all torrents status
	logger.Debug("[Butler] Fetching torrents' data")
	torrents, err := transmission.TorrentGet(fields, nil)
	if err != nil {
		logger.Errorf("[Butler] Can't retreive torrent(s): %v", err)
		return
	}
	logger.Infof("[Butler] Fetched %d torrent(s) metadata", len(torrents))
	// Inspect each torrent
	finishedTorrents := make([]int64, 0, len(torrents))
	for index, torrent := range torrents {
		// Checks
		if !torrentOK(torrent, index) {
			continue
		}
		// We can now safely access metadata
		logger.Debugf("[Butler] Inspecting torrent %d:\n\tid: %d\n\tname: %s\n\tdoneDate: %v\n\tisFinished: %v\n\tseedRatioLimit: %f\n\tseedRatioMode: %d\n\tuploadRatio:%f",
			index, *torrent.ID, *torrent.Name, *torrent.DoneDate, *torrent.IsFinished, *torrent.SeedRatioLimit, *torrent.SeedRatioMode, *torrent.UploadRatio)
		// Is this a custom torrent, should we left it alone ?
		if *torrent.SeedRatioMode == 1 {
			logger.Infof("[Butler] Torent id %d (%s) has a custom ratio limit: considering it as custom (skipping)", *torrent.ID, *torrent.Name)
			continue
		}
		// Does this torrent is under/over the free seed time range ?
		//// TODO
		// Does this torrent is finished ?
		if *torrent.IsFinished {
			if conf.DeleteDone {
				logger.Infof("[Butler] Torent id %d (%s) is finished (ratio %f): adding it to deletion list", *torrent.ID, *torrent.Name, *torrent.UploadRatio)
				finishedTorrents = append(finishedTorrents, *torrent.ID)
			} else {
				logger.Infof("[Bulter] Torent id %d (%s) is finished (ratio %f) but auto deletion is disable: skipping", *torrent.ID, *torrent.Name, *torrent.UploadRatio)
			}
		}
	}
	// Delete finished torrents
	if conf.DeleteDone {
		err = transmission.TorrentDelete(&transmissionrpc.TorrentDeletePayload{
			IDs:             finishedTorrents,
			DeleteLocalData: true,
		})
		if err != nil {
			logger.Errorf("[Butler] Can't delete the %d finished torrent(s): %v", len(finishedTorrents), err)
		} else {
			logger.Errorf("[Butler] Successfully deleted the %d finished torrent(s)", len(finishedTorrents))
		}
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
