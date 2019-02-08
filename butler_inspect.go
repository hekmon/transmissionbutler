package main

import (
	"fmt"
	"time"

	"github.com/hekmon/transmissionrpc"
)

func inspectTorrents(torrents []*transmissionrpc.Torrent) (
	freeseedCandidates, globalratioCandidates, customratioCandidates, todeleteCandidates map[int64]string) {
	// Only 1 run at a time !
	defer butlerRun.Unlock()
	logger.Debugf("[Butler] Waiting for butlerRun lock")
	butlerRun.Lock()
	// Prepare
	freeseedCandidates = make(map[int64]string, len(torrents))
	globalratioCandidates = make(map[int64]string, len(torrents))
	customratioCandidates = make(map[int64]string, len(torrents))
	todeleteCandidates = make(map[int64]string, len(torrents))
	now := time.Now()
	// Start inspection
	for index, torrent := range torrents {
		// Checks
		if !torrentOK(torrent, index) {
			continue
		}
		// We can now safely access metadata
		if logger.IsDebugShown() {
			logger.Debugf("[Butler] Inspecting torrent %d:\n\tid:\t\t%d\n\tname:\t\t%s\n\tsize:\t\t%s\n\tstatus:\t\t%s\n\tdoneDate:\t%v\n\tseedRatioLimit:\t%f\n\tseedRatioMode:\t%s\n\tuploadRatio:\t%f",
				index, *torrent.ID, *torrent.Name, *torrent.Status, *torrent.TotalSize, *torrent.DoneDate, *torrent.SeedRatioLimit, *torrent.SeedRatioMode, *torrent.UploadRatio)
		}
		// For seeding torrents
		if *torrent.Status == transmissionrpc.TorrentStatusSeed || *torrent.Status == transmissionrpc.TorrentStatusSeedWait {
			// Is this a custom torrent, should we leave it alone ?
			if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeCustom {
				if logger.IsDebugShown() {
					logger.Debugf("[Butler] Seeding torrent id %d (%s) has a custom ratio enabled: skipping", *torrent.ID, *torrent.Name)
				}
				continue
			}
			// Else process it
			inspectSeedingTorrent(torrent, now, freeseedCandidates, globalratioCandidates, customratioCandidates)
		}
		// For stopped/finished torrents
		if conf.Butler.DeleteDone && *torrent.Status == transmissionrpc.TorrentStatusStopped {
			inspectStoppedTorrent(torrent, todeleteCandidates)
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

func inspectSeedingTorrent(torrent *transmissionrpc.Torrent, now time.Time, freeseedCandidates, globalratioCandidates, customratioCandidates map[int64]string) {
	// Does this torrent is under/over the free seed time range ?
	if torrent.DoneDate.Add(conf.Butler.FreeSeed).Before(now) {
		// Torrent is over the unlimited seed time range
		if conf.Butler.RestoreCustom && *torrent.SeedRatioLimit != conf.Butler.TargetRatio {
			// This torrent had a custom ratio saved, let's check if this torrent does not need to be restored as custom ratio
			if *torrent.SeedRatioMode != transmissionrpc.SeedRatioModeCustom {
				logger.Infof("[Butler] Seeding torrent id %d (%s) is now over its unlimited seed period: adding it to the restore custom ratio list",
					*torrent.ID, *torrent.Name)
				customratioCandidates[*torrent.ID] = *torrent.Name
			} else if logger.IsDebugShown() {
				logger.Debugf("[Butler] Seeding torrent id %d (%s) is correctly set to use the custom ratio mode (free seed ending date: %v, RestoreCustom: %v, TorrentRatio: %v, GlobalRatio: %v)",
					*torrent.ID, *torrent.Name, torrent.DoneDate.Add(conf.Butler.FreeSeed), conf.Butler.RestoreCustom, *torrent.SeedRatioLimit, conf.Butler.TargetRatio)
			}
		} else {
			// Let's check if this torrent is in global ratio mode as it should be
			if *torrent.SeedRatioMode != transmissionrpc.SeedRatioModeGlobal {
				logger.Infof("[Butler] Seeding torrent id %d (%s) is now over its unlimited seed period: adding it to the global ratio list",
					*torrent.ID, *torrent.Name)
				globalratioCandidates[*torrent.ID] = *torrent.Name
			} else if logger.IsDebugShown() {
				logger.Debugf("[Butler] Seeding torrent id %d (%s) is correctly set to use the global ratio mode (free seed ending date: %v, RestoreCustom: %v, TorrentRatio: %v, GlobalRatio: %v)",
					*torrent.ID, *torrent.Name, torrent.DoneDate.Add(conf.Butler.FreeSeed), conf.Butler.RestoreCustom, *torrent.SeedRatioLimit, conf.Butler.TargetRatio)
			}
		}
	} else {
		// Torrent is still within the unlimited seed time range
		if *torrent.SeedRatioMode != transmissionrpc.SeedRatioModeNoRatio {
			logger.Infof("[Butler] Seeding torrent id %d (%s) is still young: adding it to the free seed ratio list",
				*torrent.ID, *torrent.Name)
			freeseedCandidates[*torrent.ID] = *torrent.Name
		} else if logger.IsDebugShown() {
			logger.Debugf("[Butler] Seeding torrent id %d (%s) is correctly set to use the free seed mode (free seed ending date: %v)",
				*torrent.ID, *torrent.Name, torrent.DoneDate.Add(conf.Butler.FreeSeed))
		}
	}
}

func inspectStoppedTorrent(torrent *transmissionrpc.Torrent, todeleteCandidates map[int64]string) {
	var targetRatio float64
	// Should we handle this stopped torrent ?
	if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeCustom {
		targetRatio = *torrent.SeedRatioLimit
	} else if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeGlobal {
		targetRatio = conf.Butler.TargetRatio
	} else {
		if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeNoRatio {
			if logger.IsDebugShown() {
				logger.Debugf("[Butler] Torrent id %d (%s) is finished (ratio %f) but it does not have a ratio target (custom or global): skipping",
					*torrent.ID, *torrent.Name, *torrent.UploadRatio)
			}
		} else {
			logger.Warningf("[Butler] Torrent id %d (%s) is finished but has an unknown seed ratio mode (%d): skipping",
				*torrent.ID, *torrent.Name, *torrent.SeedRatioMode)
		}
		return
	}
	// We should handle it but does it have seeded enought ?
	if *torrent.UploadRatio >= targetRatio {
		logger.Infof("[Butler] Torrent id %d (%s) is finished (ratio %f/%f): adding it to deletion list",
			*torrent.ID, *torrent.Name, *torrent.UploadRatio, targetRatio)
		todeleteCandidates[*torrent.ID] = fmt.Sprintf("%s (ratio: %.02f)", *torrent.Name, *torrent.UploadRatio)
	} else if logger.IsDebugShown() {
		logger.Debugf("[Butler] Torrent id %d (%s) is finished but it does not have reached its target ratio yet: %f/%f",
			*torrent.ID, *torrent.Name, *torrent.UploadRatio, targetRatio)
	}
}
