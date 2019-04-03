package main

import (
	"fmt"
	"strings"

	"github.com/gregdel/pushover"
	"github.com/hekmon/cunits"
	"github.com/hekmon/transmissionrpc"
)

func handleFreeseedCandidates(freeseedCandidates map[int64]string) {
	if len(freeseedCandidates) > 0 {
		// Build
		seedRatioMode := transmissionrpc.SeedRatioModeNoRatio
		IDList := make([]int64, len(freeseedCandidates))
		nameList := make([]string, len(freeseedCandidates))
		index := 0
		for id, name := range freeseedCandidates {
			IDList[index] = id
			nameList[index] = name
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
				butlerSendSuccessMsg(butlerMakeStrList(nameList), fmt.Sprintf("Switched %d torrent%s to free seed mode", len(nameList), suffix))
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
		nameList := make([]string, len(globalratioCandidates))
		index := 0
		for id, name := range globalratioCandidates {
			IDList[index] = id
			nameList[index] = name
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
				butlerSendSuccessMsg(butlerMakeStrList(nameList), fmt.Sprintf("Switched %d torrent%s to global ratio mode", len(globalratioCandidates), suffix))
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
	nameList := make([]string, len(customratioCandidates))
	index := 0
	for id, name := range customratioCandidates {
		IDList[index] = id
		nameList[index] = name
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
			butlerSendSuccessMsg(butlerMakeStrList(nameList), fmt.Sprintf("Switched %d torrent%s to custom ratio mode", len(customratioCandidates), suffix))
		}
	} else {
		butlerSendErrorMsg(fmt.Sprintf("Can't switch %d torrent%s to custom ratio mode: %v", len(customratioCandidates), suffix, err))
	}
}

func handleTodeleteCandidates(todeleteCandidates map[int64]string, dwnldDir *string) {
	if len(todeleteCandidates) > 0 {
		// Build
		IDList := make([]int64, len(todeleteCandidates))
		nameList := make([]string, len(todeleteCandidates))
		index := 0
		for id, name := range todeleteCandidates {
			IDList[index] = id
			nameList[index] = name
			index++
		}
		// Run
		err := transmission.TorrentRemove(&transmissionrpc.TorrentRemovePayload{
			IDs:             IDList,
			DeleteLocalData: true,
		})
		var suffix string
		if len(nameList) > 1 {
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
					butlerSendSuccessMsg(fmt.Sprintf("%s free after deleting:\n%s", freeSpace, strings.Join(nameList, "\n")),
						fmt.Sprintf("%d finished torrent%s deleted", len(nameList), suffix))
				}
			} else {
				butlerSendErrorMsg(fmt.Sprintf("Can't check free space in download dir: %v", err))
				butlerSendSuccessMsg(fmt.Sprintf("Deleted:\n%s", strings.Join(nameList, "\n")),
					fmt.Sprintf("%d finished torrent%s deleted", len(nameList), suffix))
			}
		} else {
			logger.Warning("[Butler] Can't fetch free space: session dwld dir is nil")
		}
	}
}

func butlerMakeStrList(items []string) string {
	for index, item := range items {
		items[index] = fmt.Sprintf("• %s", item)
	}
	return strings.Join(items, "\n")
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
