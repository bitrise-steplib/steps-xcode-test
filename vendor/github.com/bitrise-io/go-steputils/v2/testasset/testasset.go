// Package testasset provides helpers for filtering test result attachments by file type.
package testasset

import (
	"path/filepath"
	"slices"
	"strings"
)

// AssetTypes is the list of supported attachment file extensions.
var AssetTypes = []string{".jpg", ".jpeg", ".png", ".txt", ".log", ".mp4", ".webm", ".ogg"}

// IsSupportedAssetType reports whether the given file name has a supported attachment extension.
func IsSupportedAssetType(fileName string) bool {
	ext := filepath.Ext(fileName)
	return slices.Contains(AssetTypes, strings.ToLower(ext))
}
