package testaddon

import "path/filepath"

// Exporter ...
type Exporter interface {
	CopyAndSaveMetadata(info AddonCopy) error
}

type exporter struct {
}

// NewExporter ...
func NewExporter() Exporter {
	return &exporter{}
}

// AddonCopy ...
type AddonCopy struct {
	SourceTestOutputDir   string
	TargetAddonPath       string
	TargetAddonBundleName string
}

func (e exporter) CopyAndSaveMetadata(info AddonCopy) error {
	info.TargetAddonBundleName = replaceUnsupportedFilenameCharacters(info.TargetAddonBundleName)
	addonPerStepOutputDir := filepath.Join(info.TargetAddonPath, info.TargetAddonBundleName)

	if err := copyDirectory(info.SourceTestOutputDir, addonPerStepOutputDir); err != nil {
		return err
	}
	if err := saveBundleMetadata(addonPerStepOutputDir, info.TargetAddonBundleName); err != nil {
		return err
	}
	return nil
}
