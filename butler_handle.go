package main

import (
	"fmt"
	"strings"

	"github.com/hekmon/cunits"
	"github.com/hekmon/transmissionrpc"
)

func handleFreeseedCandidates(freeseedCandidates []*transmissionrpc.Torrent) {
	if len(freeseedCandidates) == 0 {
		return
	}
	// Build
	seedRatioMode := transmissionrpc.SeedRatioModeNoRatio
	IDList := make([]int64, len(freeseedCandidates))
	nameList := make([]string, len(freeseedCandidates))
	index := 0
	for _, torrent := range freeseedCandidates {
		IDList[index] = *torrent.ID
		nameList[index] = fmt.Sprintf("%s (ratio: %.02f/+∞)", *torrent.Name, *torrent.UploadRatio)
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
	if err != nil {
		logger.Errorf("[Butler] Free seed switch for %d torrent%s failed: %v", len(freeseedCandidates), suffix, err)
		pushoverClient.SendHighPriorityMsg(
			fmt.Sprintf("Can't switch %d torrent%s to free seed mode: %v", len(freeseedCandidates), suffix, err),
			"",
			"free seed candidates",
		)
		return
	}
	// Success
	logger.Infof("[Butler] Successfully switched %d torrent%s to free seed mode", len(freeseedCandidates), suffix)
	pushoverClient.SendNormalPriorityMsg(
		butlerMakeStrList(nameList),
		fmt.Sprintf("Switched %d torrent%s to free seed mode", len(nameList), suffix),
		"free seed candidates",
	)
}

func handleGlobalratioCandidates(globalratioCandidates []*transmissionrpc.Torrent) {
	if len(globalratioCandidates) == 0 {
		return
	}
	// Build
	seedRatioMode := transmissionrpc.SeedRatioModeGlobal
	IDList := make([]int64, len(globalratioCandidates))
	nameList := make([]string, len(globalratioCandidates))
	index := 0
	for _, torrent := range globalratioCandidates {
		IDList[index] = *torrent.ID
		nameList[index] = fmt.Sprintf("%s (ratio: %.02f/%.02f)", *torrent.Name, *torrent.UploadRatio, getTorrentTargetRatio(torrent))
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
	if err != nil {
		logger.Errorf("[Butler] global ratio switch for %d torrent%s failed: %v", len(globalratioCandidates), suffix, err)
		pushoverClient.SendHighPriorityMsg(
			fmt.Sprintf("Can't switch %d torrent%s to global ratio mode: %v", len(globalratioCandidates), suffix, err),
			"",
			"global ratio candidates",
		)
		return
	}
	// Success
	logger.Infof("[Butler] Successfully switched %d torrent%s to global ratio mode", len(globalratioCandidates), suffix)
	pushoverClient.SendNormalPriorityMsg(
		butlerMakeStrList(nameList),
		fmt.Sprintf("Switched %d torrent%s to global ratio mode", len(globalratioCandidates), suffix),
		"global ratio candidates",
	)
}

func handleCustomratioCandidates(customratioCandidates []*transmissionrpc.Torrent) {
	if len(customratioCandidates) == 0 {
		return
	}
	// Build
	seedRatioMode := transmissionrpc.SeedRatioModeCustom
	IDList := make([]int64, len(customratioCandidates))
	nameList := make([]string, len(customratioCandidates))
	index := 0
	for _, torrent := range customratioCandidates {
		IDList[index] = *torrent.ID
		nameList[index] = fmt.Sprintf("%s (ratio: %.02f/%.02f)", *torrent.Name, *torrent.UploadRatio, getTorrentTargetRatio(torrent))
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
	if err != nil {
		logger.Errorf("[Butler] custom ratio switch for %d torrent%s failed: %v", len(customratioCandidates), suffix, err)
		pushoverClient.SendHighPriorityMsg(
			fmt.Sprintf("Can't switch %d torrent%s to custom ratio mode: %v", len(customratioCandidates), suffix, err),
			"",
			"custom ratio candidates",
		)
		return
	}
	// Success
	logger.Infof("[Butler] Successfully switched %d torrent%s to custom ratio mode", len(customratioCandidates), suffix)
	pushoverClient.SendNormalPriorityMsg(
		butlerMakeStrList(nameList),
		fmt.Sprintf("Switched %d torrent%s to custom ratio mode", len(customratioCandidates), suffix),
		"custom ratio candidates",
	)
}

func handleTodeleteCandidates(todeleteCandidates []*transmissionrpc.Torrent, dwnldDir *string) {
	if len(todeleteCandidates) == 0 {
		return
	}
	// Build
	IDList := make([]int64, len(todeleteCandidates))
	nameList := make([]string, len(todeleteCandidates))
	index := 0
	for _, torrent := range todeleteCandidates {
		IDList[index] = *torrent.ID
		nameList[index] = fmt.Sprintf("%s (ratio: %.02f/%.02f)", *torrent.Name, *torrent.UploadRatio, getTorrentTargetRatio(torrent))
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
		logger.Errorf("[Butler] Failted to deleted the %d finished torrent%s: %s", len(todeleteCandidates), suffix, err)
		pushoverClient.SendHighPriorityMsg(
			fmt.Sprintf("Can't delete %d finished torrent%s: %v", len(todeleteCandidates), suffix, err),
			"",
			"delete candidates",
		)
		return
	}
	logger.Infof("[Butler] Successfully deleted the %d finished torrent%s", len(todeleteCandidates), suffix)
	// Fetch free space
	if dwnldDir == nil {
		logger.Warning("[Butler] Can't fetch free space: session dwld dir is nil")
		return
	}
	var freeSpace cunits.Bits
	if freeSpace, err = transmission.FreeSpace(*dwnldDir); err != nil {
		pushoverClient.SendNormalPriorityMsg(
			fmt.Sprintf("Deleted:\n%s", butlerMakeStrList(nameList)),
			fmt.Sprintf("%d finished torrent%s deleted", len(nameList), suffix),
			"delete candidates",
		)
		pushoverClient.SendHighPriorityMsg(
			fmt.Sprintf("Can't check free space in '%s' dir: %v", *dwnldDir, err),
			"",
			"delete candidates",
		)
		return
	}
	// success
	logger.Infof("[Butler] Remaining free space in download dir: %s", freeSpace)
	pushoverClient.SendNormalPriorityMsg(
		fmt.Sprintf("%s free after deleting:\n%s", freeSpace, butlerMakeStrList(nameList)),
		fmt.Sprintf("%d finished torrent%s deleted", len(nameList), suffix),
		"delete candidates",
	)
}

func butlerMakeStrList(items []string) string {
	for index, item := range items {
		items[index] = fmt.Sprintf("• %s", item)
	}
	return strings.Join(items, "\n")
}

func getTorrentTargetRatio(torrent *transmissionrpc.Torrent) float64 {
	if torrent == nil || torrent.SeedRatioMode == nil {
		return -1
	}
	if *torrent.SeedRatioMode == transmissionrpc.SeedRatioModeCustom {
		if torrent.SeedRatioLimit == nil {
			return -2
		}
		return *torrent.SeedRatioLimit
	}
	return conf.Butler.TargetRatio
}
