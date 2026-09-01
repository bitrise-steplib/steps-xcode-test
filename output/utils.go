package output

import (
	"fmt"
	"os"
	"path/filepath"
)

func saveRawOutputToLogFile(rawXcodebuildOutput string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "xcodebuild-output")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	logFileName := "raw-xcodebuild-output.log"
	logPth := filepath.Join(tmpDir, logFileName)
	if err := os.WriteFile(logPth, []byte(rawXcodebuildOutput), 0644); err != nil {
		return "", fmt.Errorf("failed to write xcodebuild output to file: %w", err)
	}

	return logPth, nil
}
