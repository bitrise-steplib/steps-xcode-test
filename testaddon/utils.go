package testaddon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
)

// Replaces characters '/' and ':', which are unsupported in filnenames on macOS
func replaceUnsupportedFilenameCharacters(s string) string {
	s = strings.Replace(s, "/", "-", -1)
	s = strings.Replace(s, ":", "-", -1)
	return s
}

func copyDirectory(sourceBundle string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory (%s): %w", targetDir, err)
	}

	// the leading `/` means to copy not the content but the whole dir
	// -a means a better recursive, with symlinks handling and everything
	cmd := command.NewFactory(env.NewRepository()).Create("cp", []string{"-a", sourceBundle, targetDir + "/"}, nil)
	//cmd := command.New("cp", "-a", sourceBundle, targetDir+"/")
	// TODO: migrate log
	log.Donef("$ %s", cmd.PrintableCommandArgs())
	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		return fmt.Errorf("copy failed: %w, output: %s", err, out)
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
		return fmt.Errorf("could not encode metadata: %w", err)
	}
	if err = ioutil.WriteFile(filepath.Join(outputDir, "test-info.json"), bytes, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
