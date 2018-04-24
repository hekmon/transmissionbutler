package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gregdel/pushover"
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

var fields = []string{"id", "name", "status", "doneDate", "seedRatioLimit", "seedRatioMode", "uploadRatio"}

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
		logger.Errorf("[Butler] Can't retreive torrent(s) metadata: %v", err)
		return
	}
	logger.Infof("[Butler] Fetched %d torrent(s) metadata", len(torrents))
	// Inspect each torrent
	youngTorrents, regularTorrents, finishedTorrents := inspectTorrents(torrents)
	// Updates what need to be updated
	updateYoungTorrents(youngTorrents)
	updateRegularTorrents(regularTorrents)
	deleteFinishedTorrents(finishedTorrents, session.DownloadDir)
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

func inspectTorrents(torrents []*transmissionrpc.Torrent) (youngTorrents, regularTorrents, finishedTorrents map[int64]string) {
	// Only 1 run at a time !
	defer butlerRun.Unlock()
	logger.Debugf("[Butler] Waiting for butlerRun lock")
	butlerRun.Lock()
	// Prepare
	youngTorrents = make(map[int64]string, len(torrents))
	regularTorrents = make(map[int64]string, len(torrents))
	finishedTorrents = make(map[int64]string, len(torrents))
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
			if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeCustom {
				logger.Infof("[Butler] Seeding torrent id %d (%s) has a custom ratio enabled: skipping", *torrent.ID, *torrent.Name)
				continue
			}
			// Does this torrent is under/over the free seed time range ?
			if torrent.DoneDate.Add(conf.Butler.UnlimitedSeed).After(now) {
				// Torrent is still within the unlimited seed time range
				if *torrent.SeedRatioMode != transmissionrpc.SeedRatioModeNoRatio {
					logger.Infof("[Butler] Seeding torrent id %d (%s) is still young: adding it to the unlimited seed ratio list",
						*torrent.ID, *torrent.Name)
					youngTorrents[*torrent.ID] = *torrent.Name
				}
			} else {
				// Torrent is over the unlimited seed time range
				if *torrent.SeedRatioMode != transmissionrpc.SeedRatioModeGlobal {
					logger.Infof("[Butler] Seeding torrent id %d (%s) is now over its unlimited seed period: adding it to the regular ratio list",
						*torrent.ID, *torrent.Name)
					regularTorrents[*torrent.ID] = *torrent.Name
				}
			}
		}
		// For stopped/finished torrents
		if conf.Butler.DeleteDone && *torrent.Status == 0 {
			// Should we handle this stopped torrent ?
			if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeCustom {
				targetRatio = *torrent.SeedRatioLimit
			} else if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeGlobal {
				targetRatio = conf.Butler.TargetRatio
			} else {
				if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeNoRatio {
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
				finishedTorrents[*torrent.ID] = *torrent.Name
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

func updateYoungTorrents(youngTorrents map[int64]string) {
	if len(youngTorrents) > 0 {
		// Build
		seedRatioMode := transmissionrpc.SeedRatioModeNoRatio
		IDList := make([]int64, len(youngTorrents))
		NameList := make([]string, len(youngTorrents))
		index := 0
		for id, name := range youngTorrents {
			IDList[index] = id
			NameList[index] = name
		}
		// Run
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           IDList,
			SeedRatioMode: &seedRatioMode,
		})
		if err == nil {
			logger.Infof("[Butler] Successfully applied the no ratio mutator to the %d young torrent(s)", len(youngTorrents))
			// Pushover
			if conf.isPushoverEnabled() {
				var prefix string
				if len(NameList) > 1 {
					prefix = "s"
				}
				pushoverTitle := fmt.Sprintf("%d young torrent%s detected", len(NameList), prefix)
				butlerSendSuccessMsg(strings.Join(NameList, "\n"), pushoverTitle)
			}
		} else {
			butlerSendErrorMsg(fmt.Sprintf("Can't apply no ratio mutator to the %d young torrent(s): %v", len(youngTorrents), err))
		}
	}
}

func updateRegularTorrents(regularTorrents map[int64]string) {
	if len(regularTorrents) > 0 {
		// Build
		seedRatioMode := transmissionrpc.SeedRatioModeGlobal
		IDList := make([]int64, len(regularTorrents))
		NameList := make([]string, len(regularTorrents))
		index := 0
		for id, name := range regularTorrents {
			IDList[index] = id
			NameList[index] = name
		}
		// Run
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           IDList,
			SeedRatioMode: &seedRatioMode,
		})
		if err == nil {
			logger.Infof("[Butler] Successfully applied the global ratio mutator to the %d regular torrent(s)", len(regularTorrents))
			if conf.isPushoverEnabled() {
				var prefix string
				if len(NameList) > 1 {
					prefix = "s"
				}
				pushoverTitle := fmt.Sprintf("%d regular torrent%s detected", len(NameList), prefix)
				butlerSendSuccessMsg(strings.Join(NameList, "\n"), pushoverTitle)
			}
		} else {
			butlerSendErrorMsg(fmt.Sprintf("Can't apply global ratio mutator to the %d regular torrent(s): %v", len(regularTorrents), err))
		}
	}
}

func deleteFinishedTorrents(finishedTorrents map[int64]string, dwnldDir *string) {
	if len(finishedTorrents) > 0 {
		// Build
		IDList := make([]int64, len(finishedTorrents))
		NameList := make([]string, len(finishedTorrents))
		index := 0
		for id, name := range finishedTorrents {
			IDList[index] = id
			NameList[index] = name
		}
		// Run
		err := transmission.TorrentDelete(&transmissionrpc.TorrentDeletePayload{
			IDs:             IDList,
			DeleteLocalData: true,
		})
		if err != nil {
			butlerSendErrorMsg(fmt.Sprintf("Can't delete the %d finished torrent(s): %v", len(finishedTorrents), err))
			return
		}
		logger.Infof("[Butler] Successfully deleted the %d finished torrent(s)", len(finishedTorrents))
		// Fetch free space
		if dwnldDir != nil {
			var sizeBytes int64
			if sizeBytes, err = transmission.FreeSpace(*dwnldDir); err == nil {
				freeSpace := float64(sizeBytes) / 1024 / 1024 / 1024
				logger.Infof("[Butler] Remaining free space in download dir: %fGB", freeSpace)
				// pushover
				if conf.isPushoverEnabled() {
					var prefix string
					if len(NameList) > 1 {
						prefix = "s"
					}
					pushoverTitle := fmt.Sprintf("%d finished torrent%s deleted", len(NameList), prefix)
					pushoverMsg := fmt.Sprintf("%fGB free after deleting:\n%s", freeSpace, strings.Join(NameList, "\n"))
					butlerSendSuccessMsg(pushoverMsg, pushoverTitle)
				}
			} else {
				butlerSendErrorMsg(fmt.Sprintf("Can't check free space in download dir: %v", err))
			}
		} else {
			logger.Warning("[Butler] Can't fetch free space: session dwld dir is nil")
		}
	}
}

func butlerSendSuccessMsg(pushoverMsg, pushoverTitle string) {
	if answer, err := pushoverApp.SendMessage(pushover.NewMessageWithTitle(pushoverMsg, pushoverTitle), pushoverDest); err == nil {
		logger.Debugf("[Butler] Successfully sent the success message to pushover: %s", answer)
	} else {
		logger.Errorf("[Butler] Can't send success msg to pushover: %v", err)
	}
}

func butlerSendErrorMsg(msg string) {
	logger.Errorf("[Butler] %s", msg)
	if conf.isPushoverEnabled() {
		if answer, err := pushoverApp.SendMessage(pushover.NewMessage(msg), pushoverDest); err == nil {
			logger.Debugf("[Butler] Successfully sent the error message to pushover: %s", answer)
		} else {
			logger.Errorf("[Butler] Can't send error msg to pushover: %v", err)
		}
	}
}
