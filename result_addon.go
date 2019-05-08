package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
)

type addonCopy struct {
	sourceTestOutputDir   string
	targetAddonPath       string
	targetAddonBundleName string
}

func copyToResultAddonDir(info addonCopy) error {
	addonPerStepOutputDir := filepath.Join(info.targetAddonPath, info.targetAddonBundleName)
	if err := os.MkdirAll(addonPerStepOutputDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory (%s), error: %s", addonPerStepOutputDir, err)
	}

	// the leading `/` means to copy not the content but the whole dir
	// -a means a better recursive, with symlinks handling and everything
	cmd := command.New("cp", "-a", info.sourceTestOutputDir, addonPerStepOutputDir+"/")
	log.Donef("$ %s", cmd.PrintableCommandArgs())
	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		return fmt.Errorf("copy failed, error: %s, output: %s", err, out)
	}

	// Save test bundle metadata
	type testBundle struct {
		BundleName string `json:"test-name"`
	}
	bytes, err := json.Marshal(testBundle{
		BundleName: info.targetAddonBundleName,
	})
	if err != nil {
		return fmt.Errorf("could not encode metadata, error: %s", err)
	}
	if err = ioutil.WriteFile(filepath.Join(addonPerStepOutputDir, "test-info.json"), bytes, 0600); err != nil {
		return fmt.Errorf("failed to write file, error: %s", err)
	}

	return nil
}
