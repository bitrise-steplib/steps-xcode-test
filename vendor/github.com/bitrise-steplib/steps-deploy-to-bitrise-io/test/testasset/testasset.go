package testasset

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var AssetTypes = []string{".jpg", ".jpeg", ".png", ".txt", ".log"}
var VideoTypes = []string{".mp4", ".webm", ".ogg"} // These video types are also supported on the UI

func IsSupportedAssetType(fileName string) bool {
	ext := filepath.Ext(fileName)

	if slices.Contains(AssetTypes, strings.ToLower(ext)) {
		return true
	}

	if os.Getenv("ENABLE_TEST_VIDEO_UPLOAD") == "true" {
		return slices.Contains(VideoTypes, strings.ToLower(ext))
	}

	return false
}
