package output

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
)

func saveRawOutputToLogFile(rawXcodebuildOutput string) (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("xcodebuild-output")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir, error: %s", err)
	}
	logFileName := "raw-xcodebuild-output.log"
	logPth := filepath.Join(tmpDir, logFileName)
	if err := fileutil.WriteStringToFile(logPth, rawXcodebuildOutput); err != nil {
		return "", fmt.Errorf("failed to write xcodebuild output to file, error: %s", err)
	}

	return logPth, nil
}
