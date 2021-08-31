package testartifact

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

type Exporter interface {
	SaveAttachments(scheme, testSummariesPath, attachementDir string) (string, error)
	GetSummariesAndAttachmentPath(testOutputDir string) (testSummariesPath string, attachmentDir string, err error)
}

type exporter struct {
}

func NewExporter() Exporter {
	return &exporter{}
}

func (e *exporter) SaveAttachments(scheme, testSummariesPath, attachementDir string) (string, error) {
	if exist, err := pathutil.IsDirExists(attachementDir); err != nil {
		return "", err
	} else if !exist {
		return "", fmt.Errorf("no test attachments found at: %s", attachementDir)
	}

	if found, err := UpdateScreenshotNames(testSummariesPath, attachementDir); err != nil {
		log.Warnf("Failed to update screenshot names, error: %s", err)
	} else if !found {
		return "", nil
	}

	// deploy zipped attachments
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		return "", errors.New("no BITRISE_DEPLOY_DIR found")
	}

	zipedTestsDerivedDataPath := filepath.Join(deployDir, fmt.Sprintf("%s-xc-test-Attachments.zip", scheme))
	if err := Zip(filepath.Dir(attachementDir), filepath.Base(attachementDir), zipedTestsDerivedDataPath); err != nil {
		return "", err
	}

	return zipedTestsDerivedDataPath, nil
}

func (e *exporter) GetSummariesAndAttachmentPath(testOutputDir string) (testSummariesPath string, attachmentDir string, err error) {
	const testSummaryFileName = "TestSummaries.plist"
	if exist, err := pathutil.IsDirExists(testOutputDir); err != nil {
		return "", "", err
	} else if !exist {
		return "", "", fmt.Errorf("no test logs found at: %s", testOutputDir)
	}

	testSummariesPath = path.Join(testOutputDir, testSummaryFileName)
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return "", "", err
	} else if !exist {
		return "", "", fmt.Errorf("no test summaries found at: %s", testSummariesPath)
	}

	var attachementDir string
	{
		attachementDir = filepath.Join(testOutputDir, "Attachments")
		if exist, err := pathutil.IsDirExists(attachementDir); err != nil {
			return "", "", err
		} else if !exist {
			return "", "", fmt.Errorf("no test attachments found at: %s", attachementDir)
		}
	}

	log.Debugf("Test summaries path: %s", testSummariesPath)
	log.Debugf("Attachment dir: %s", attachementDir)
	return testSummariesPath, attachementDir, nil
}
