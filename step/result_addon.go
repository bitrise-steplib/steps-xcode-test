package step

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
)

type addonCopy struct {
	sourceTestOutputDir   string
	targetAddonPath       string
	targetAddonBundleName string
}

func copyAndSaveMetadata(info addonCopy) error {
	info.targetAddonBundleName = replaceUnsupportedFilenameCharacters(info.targetAddonBundleName)
	addonPerStepOutputDir := filepath.Join(info.targetAddonPath, info.targetAddonBundleName)

	if err := copyDirectory(info.sourceTestOutputDir, addonPerStepOutputDir); err != nil {
		return err
	}
	if err := saveBundleMetadata(addonPerStepOutputDir, info.targetAddonBundleName); err != nil {
		return err
	}
	return nil
}

func copyDirectory(sourceBundle string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory (%s), error: %s", targetDir, err)
	}

	// the leading `/` means to copy not the content but the whole dir
	// -a means a better recursive, with symlinks handling and everything
	cmd := command.NewFactory(env.NewRepository()).Create("cp", []string{"-a", sourceBundle, targetDir + "/"}, nil)
	//cmd := command.New("cp", "-a", sourceBundle, targetDir+"/")
	// TODO: migrate log
	log.Donef("$ %s", cmd.PrintableCommandArgs())
	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		return fmt.Errorf("copy failed, error: %s, output: %s", err, out)
	}

	return nil
}

func saveBundleMetadata(outputDir string, bundleName string) error {
	// Save test bundle metadata
	type testBundle struct {
		BundleName string `json:"test-name"`
	}
	bytes, err := json.Marshal(testBundle{
		BundleName: bundleName,
	})
	if err != nil {
		return fmt.Errorf("could not encode metadata, error: %s", err)
	}
	if err = ioutil.WriteFile(filepath.Join(outputDir, "test-info.json"), bytes, 0600); err != nil {
		return fmt.Errorf("failed to write file, error: %s", err)
	}
	return nil
}
