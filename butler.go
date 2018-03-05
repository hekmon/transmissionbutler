package main

import (
	"sync"
	"time"

	"github.com/hekmon/transmissionrpc"
)

func butler(conf *butlerConfig, stopSignal <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create the ticker
	logger.Infof("[Butler] Will work every %v", conf.CheckFrequency)
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

var fields = []string{"id", "name", "status", "doneDate", "seedRatioLimit", "seedRatioMode", "uploadRatio"}

const (
	seedRatioModeGlobal  = int64(0)
	seedRatioModeCustom  = int64(1)
	seedRatioModeNoRatio = int64(2)
)

func butlerBatch(conf *butlerConfig) {
	// Check that global ratio limit is activated and set with correct value
	logger.Debug("[Butler] Fetching session data")
	session, err := transmission.SessionArgumentsGet()
	if err == nil {
		globalRatio(session, conf)
	} else {
		logger.Errorf("[Butler] Can't check global ratio: can't get sessions values: %v", err)
	}
	// Get all torrents status
	logger.Debug("[Butler] Fetching torrents metadata")
	torrents, err := transmission.TorrentGet(fields, nil)
	if err != nil {
		logger.Errorf("[Butler] Can't retreive torrent(s) metadata: %v", err)
		return
	}
	logger.Infof("[Butler] Fetched %d torrent(s) metadata", len(torrents))
	// Inspect each torrent
	youngTorrents, regularTorrents, finishedTorrents := inspectTorrents(torrents, conf)
	// Updates what need to be updated
	updateYoungTorrents(youngTorrents)
	updateRegularTorrents(regularTorrents)
	deleteFinishedTorrents(finishedTorrents, session.DownloadDir)
}

func globalRatio(session *transmissionrpc.SessionArguments, conf *butlerConfig) {
	var updateRatio, updateRatioEnabled bool
	// Ratio value
	if session.SeedRatioLimit != nil {
		if *session.SeedRatioLimit != conf.TargetRatio {
			logger.Infof("[Butler] Global ratio is invalid (%f instead of %f): scheduling update", *session.SeedRatioLimit, conf.TargetRatio)
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
			SeedRatioLimit:   &conf.TargetRatio,
			SeedRatioLimited: &updateRatioEnabled,
		})
		if err == nil {
			logger.Infof("[Butler] Global ratio set and activated")
		} else {
			logger.Errorf("[Butler] Can't update global ratio: %v", err)
		}
	}
}

func inspectTorrents(torrents []*transmissionrpc.Torrent, conf *butlerConfig) (youngTorrents, regularTorrents, finishedTorrents []int64) {
	// Prepare
	youngTorrents = make([]int64, 0, len(torrents))
	regularTorrents = make([]int64, 0, len(torrents))
	finishedTorrents = make([]int64, 0, len(torrents))
	now := time.Now()
	var targetRatio float64
	// Start inspection
	for index, torrent := range torrents {
		// Checks
		if !torrentOK(torrent, index) {
			continue
		}
		// We can now safely access metadata
		if logger.IsDebugShown() {
			logger.Debugf("[Butler] Inspecting torrent %d:\n\tid: %d\n\tname: %s\n\tstatus: %d\n\tdoneDate: %v\n\tseedRatioLimit: %f\n\tseedRatioMode: %d\n\tuploadRatio:%f",
				index, *torrent.ID, *torrent.Name, *torrent.Status, *torrent.DoneDate, *torrent.SeedRatioLimit, *torrent.SeedRatioMode, *torrent.UploadRatio)
		}
		// For seeding torrents
		if *torrent.Status == 6 {
			// Is this a custom torrent, should we leave it alone ?
			if *torrent.SeedRatioMode == seedRatioModeCustom {
				logger.Infof("[Butler] Seeding torrent id %d (%s) has a custom ratio enabled: skipping", *torrent.ID, *torrent.Name)
				continue
			}
			// Does this torrent is under/over the free seed time range ?
			if torrent.DoneDate.Add(conf.UnlimitedSeed).After(now) {
				// Torrent is still within the unlimited seed time range
				if *torrent.SeedRatioMode != seedRatioModeNoRatio {
					logger.Infof("[Butler] Seeding torrent id %d (%s) is still young: adding it to the unlimited seed ratio list",
						*torrent.ID, *torrent.Name)
					youngTorrents = append(youngTorrents, *torrent.ID)
				}
			} else {
				// Torrent is over the unlimited seed time range
				if *torrent.SeedRatioMode != seedRatioModeGlobal {
					logger.Infof("[Butler] Seeding torrent id %d (%s) is now over its unlimited seed period: adding it to the regular ratio list",
						*torrent.ID, *torrent.Name)
					regularTorrents = append(regularTorrents, *torrent.ID)
				}
			}
		}
		// For stopped/finished torrents
		if conf.DeleteDone && *torrent.Status == 0 {
			// Should we handle this stopped torrent ?
			if *torrent.SeedRatioMode == seedRatioModeCustom {
				targetRatio = *torrent.SeedRatioLimit
			} else if *torrent.SeedRatioMode == seedRatioModeGlobal {
				targetRatio = conf.TargetRatio
			} else {
				if *torrent.SeedRatioMode == seedRatioModeNoRatio {
					logger.Infof("[Butler] Torrent id %d (%s) is finished (ratio %f) but it does not have a ratio target (custom or global): skipping",
						*torrent.ID, *torrent.Name, *torrent.UploadRatio)
				} else {
					logger.Warningf("[Butler] Torrent id %d (%s) is finished but has an unknown seed ratio mode (%d): skipping",
						*torrent.ID, *torrent.Name, *torrent.SeedRatioMode)
				}
				continue
			}
			// We should handle it but does it have seeded enought ?
			if *torrent.UploadRatio >= targetRatio {
				logger.Infof("[Butler] Torrent id %d (%s) is finished (ratio %f/%f): adding it to deletion list",
					*torrent.ID, *torrent.Name, *torrent.UploadRatio, targetRatio)
				finishedTorrents = append(finishedTorrents, *torrent.ID)
			} else {
				logger.Debugf("[Butler] Torrent id %d (%s) is finished but it does not have reached its target ratio yet: %f/%f",
					*torrent.ID, *torrent.Name, *torrent.UploadRatio, targetRatio)
			}
		}
	}
	return
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
	if torrent.Status == nil {
		logger.Warningf("[Butler] Encountered a nil torrent status at index %d", index)
		return
	}
	if torrent.DoneDate == nil {
		logger.Warningf("[Butler] Encountered a nil torrent doneDate at index %d", index)
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

func updateYoungTorrents(youngTorrents []int64) {
	if len(youngTorrents) > 0 {
		seedRatioMode := seedRatioModeNoRatio
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           youngTorrents,
			SeedRatioMode: &seedRatioMode,
		})
		if err != nil {
			logger.Errorf("[Butler] Can't apply no ratio mutator to the %d young torrent(s): %v", len(youngTorrents), err)
		} else {
			logger.Infof("[Butler] Successfully applied the no ratio mutator to the %d young torrent(s)", len(youngTorrents))
		}
	}
}

func updateRegularTorrents(regularTorrents []int64) {
	if len(regularTorrents) > 0 {
		seedRatioMode := seedRatioModeGlobal
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           regularTorrents,
			SeedRatioMode: &seedRatioMode,
		})
		if err != nil {
			logger.Errorf("[Butler] Can't apply global ratio mutator to the %d regular torrent(s): %v", len(regularTorrents), err)
		} else {
			logger.Infof("[Butler] Successfully applied the global ratio mutator to the %d regular torrent(s)", len(regularTorrents))
		}
	}
}

func deleteFinishedTorrents(finishedTorrents []int64, dwnldDir *string) {
	if len(finishedTorrents) > 0 {
		// Delete
		err := transmission.TorrentDelete(&transmissionrpc.TorrentDeletePayload{
			IDs:             finishedTorrents,
			DeleteLocalData: true,
		})
		if err != nil {
			logger.Errorf("[Butler] Can't delete the %d finished torrent(s): %v", len(finishedTorrents), err)
		} else {
			logger.Infof("[Butler] Successfully deleted the %d finished torrent(s)", len(finishedTorrents))
		}
		// Fetch free space
		if dwnldDir != nil {
			var sizeBytes int64
			if sizeBytes, err = transmission.FreeSpace(*dwnldDir); err == nil {
				logger.Infof("[Butler] Remaing free space in download dir: %fGB", float64(sizeBytes)/1024/1024/1024)
			} else {
				logger.Errorf("[Butler] Can't check free space in download dir: %v", err)
			}
		} else {
			logger.Warning("[Butler] Can't fetch free space: session dwld dir is nil")
		}
	}
}
