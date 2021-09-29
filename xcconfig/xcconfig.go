package xcconfig

import (
	"fmt"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"path/filepath"
)

// Writer ...
type Writer interface {
	Write(content string) (string, error)
}

type writer struct {
	pathProvider pathutil.PathProvider
	fileWriter   fileutil.FileWriter
}

// NewWriter ...
func NewWriter(pathProvider pathutil.PathProvider, fileWriter fileutil.FileWriter) Writer {
	return &writer{pathProvider: pathProvider, fileWriter: fileWriter}
}

func (w writer) Write(content string) (string, error) {
	dir, err := w.pathProvider.CreateTempDir("")
	if err != nil {
		return "", fmt.Errorf("unable to create temp dir for writing XCConfig: %v", err)
	}
	xcconfigPath := filepath.Join(dir, "temp.xcconfig")
	if err = w.fileWriter.Write(xcconfigPath, content, 0644); err != nil {
		return "", fmt.Errorf("unable to write XCConfig content into file: %v", err)
	}
	return xcconfigPath, nil
}
