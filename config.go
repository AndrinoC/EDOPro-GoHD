package main

import "time"

const (
	ProgramVersion      = "1.0.0-Go"
	YGOProDeckCardsURL  = "https://db.ygoprodeck.com/api/v7/cardinfo.php"
	ImagesBaseURL       = "https://images.ygoprodeck.com/images/cards"
	CardCachePath       = "./hd_cards_downloader_tracker.tmp"
	FieldCachePath      = "./hd_fields_downloader_tracker.tmp"
	PicsDir             = "./pics"
	FieldPicsDir        = "./pics/field"
	DefaultTimeout      = 30 * time.Second
	DownloadConcurrency = 100
	StatsUpdateInterval = 200 * time.Millisecond
)

var RequestHeaders = map[string]string{
	"User-Agent": "AndrinoC-YGO-HDL-Go/" + ProgramVersion,
}

var idConversionMap = map[int]int{
	904186: 31533705,
}

func convertID(cardID int) int {
	if converted, ok := idConversionMap[cardID]; ok {
		return converted
	}
	return cardID
}

type DownloadItem struct {
	ID      int
	IsField bool
}

var RequiredDirs = []string{PicsDir, FieldPicsDir}

var TempCacheFiles = []string{CardCachePath, FieldCachePath}

const IntroString = `EDOPro GoHD v%s
Fork by Andrino
Automatically downloading all missing cards and fields...
`
