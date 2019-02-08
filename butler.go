package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gregdel/pushover"
	"github.com/hekmon/cunits"
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

func handleFreeseedCandidates(freeseedCandidates map[int64]string) {
	if len(freeseedCandidates) > 0 {
		// Build
		seedRatioMode := transmissionrpc.SeedRatioModeNoRatio
		IDList := make([]int64, len(freeseedCandidates))
		NameList := make([]string, len(freeseedCandidates))
		index := 0
		for id, name := range freeseedCandidates {
			IDList[index] = id
			NameList[index] = name
			index++
		}
		// Run
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           IDList,
			SeedRatioMode: &seedRatioMode,
		})
		var suffix string
		if len(freeseedCandidates) > 1 {
			suffix = "s"
		}
		if err == nil {
			logger.Infof("[Butler] Successfully switched %d torrent%s to free seed mode", len(freeseedCandidates), suffix)
			// Pushover
			if conf.isPushoverEnabled() {
				butlerSendSuccessMsg(strings.Join(NameList, "\n"), fmt.Sprintf("Switched %d torrent%s to free seed mode", len(NameList), suffix))
			}
		} else {
			butlerSendErrorMsg(fmt.Sprintf("Can't switch %d torrent%s to free seed mode: %v", len(freeseedCandidates), suffix, err))
		}
	}
}

func handleGlobalratioCandidates(globalratioCandidates map[int64]string) {
	if len(globalratioCandidates) > 0 {
		// Build
		seedRatioMode := transmissionrpc.SeedRatioModeGlobal
		IDList := make([]int64, len(globalratioCandidates))
		NameList := make([]string, len(globalratioCandidates))
		index := 0
		for id, name := range globalratioCandidates {
			IDList[index] = id
			NameList[index] = name
			index++
		}
		// Run
		err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
			IDs:           IDList,
			SeedRatioMode: &seedRatioMode,
		})
		var suffix string
		if len(globalratioCandidates) > 1 {
			suffix = "s"
		}
		if err == nil {
			logger.Infof("[Butler] Successfully switched %d torrent%s to global ratio mode", len(globalratioCandidates), suffix)
			if conf.isPushoverEnabled() {
				butlerSendSuccessMsg(strings.Join(NameList, "\n"), fmt.Sprintf("Switched %d torrent%s to global ratio mode", len(globalratioCandidates), suffix))
			}
		} else {
			butlerSendErrorMsg(fmt.Sprintf("Can't switch %d torrent%s to global ratio mode: %v", len(globalratioCandidates), suffix, err))
		}
	}
}

func handleCustomratioCandidates(customratioCandidates map[int64]string) {
	if len(customratioCandidates) == 0 {
		return
	}
	// Build
	seedRatioMode := transmissionrpc.SeedRatioModeCustom
	IDList := make([]int64, len(customratioCandidates))
	NameList := make([]string, len(customratioCandidates))
	index := 0
	for id, name := range customratioCandidates {
		IDList[index] = id
		NameList[index] = name
		index++
	}
	// Run
	err := transmission.TorrentSet(&transmissionrpc.TorrentSetPayload{
		IDs:           IDList,
		SeedRatioMode: &seedRatioMode,
	})
	var suffix string
	if len(customratioCandidates) > 1 {
		suffix = "s"
	}
	if err == nil {
		logger.Infof("[Butler] Successfully switched %d torrent%s to custom ratio mode", len(customratioCandidates), suffix)
		if conf.isPushoverEnabled() {
			butlerSendSuccessMsg(strings.Join(NameList, "\n"), fmt.Sprintf("Switched %d torrent%s to custom ratio mode", len(customratioCandidates), suffix))
		}
	} else {
		butlerSendErrorMsg(fmt.Sprintf("Can't switch %d torrent%s to custom ratio mode: %v", len(customratioCandidates), suffix, err))
	}
}

func handleTodeleteCandidates(todeleteCandidates map[int64]string, dwnldDir *string) {
	if len(todeleteCandidates) > 0 {
		// Build
		IDList := make([]int64, len(todeleteCandidates))
		NameList := make([]string, len(todeleteCandidates))
		index := 0
		for id, name := range todeleteCandidates {
			IDList[index] = id
			NameList[index] = name
			index++
		}
		// Run
		err := transmission.TorrentRemove(&transmissionrpc.TorrentRemovePayload{
			IDs:             IDList,
			DeleteLocalData: true,
		})
		var suffix string
		if len(NameList) > 1 {
			suffix = "s"
		}
		if err != nil {
			butlerSendErrorMsg(fmt.Sprintf("Can't delete %d finished torrent%s: %v", len(todeleteCandidates), suffix, err))
			return
		}
		logger.Infof("[Butler] Successfully deleted the %d finished torrent%s", len(todeleteCandidates), suffix)
		// Fetch free space
		if dwnldDir != nil {
			var freeSpace cunits.Bits
			if freeSpace, err = transmission.FreeSpace(*dwnldDir); err == nil {
				logger.Infof("[Butler] Remaining free space in download dir: %s", freeSpace)
				if conf.isPushoverEnabled() {
					butlerSendSuccessMsg(fmt.Sprintf("%s free after deleting:\n%s", freeSpace, strings.Join(NameList, "\n")),
						fmt.Sprintf("%d finished torrent%s deleted", len(NameList), suffix))
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
