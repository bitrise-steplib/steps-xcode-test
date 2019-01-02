package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/steps-xcode-test/pretty"
	"github.com/bitrise-io/steps-xcode-test/xcodeutil"
)

func attachementDir(testOutputDir string) string {
	return filepath.Join(testOutputDir, "Attachments")
}

func updateScreenshotNames(testLogsDir string) (bool, error) {
	testSummariesPath := filepath.Join(testLogsDir, testSummaryFileName)
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return false, fmt.Errorf("Failed to check if file exists: %s", testSummariesPath)
	} else if !exist {
		return false, fmt.Errorf("no TestSummaries file found: %s", testSummariesPath)
	}

	// TestSummaries
	testSummaries, err := xcodeutil.NewTestSummaries(testSummariesPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s, error: %s", filepath.Base(testSummariesPath), err)
	}

	log.Debugf("Test items with screenshots: %s", pretty.Object(testSummaries.TestItemsWithScreenshots))
	log.Debugf("TestSummaries version has been set to: %s\n", testSummaries.Type)

	if len(testSummaries.TestItemsWithScreenshots) > 0 {
		log.Printf("Renaming screenshots")
	} else {
		log.Printf("No screenshot found")
		return false, nil
	}

	for _, testItem := range testSummaries.TestItemsWithScreenshots {
		startTimeIntervalObj, found := testItem["StartTimeInterval"]
		if !found {
			return false, fmt.Errorf("missing StartTimeInterval")
		}
		startTimeInterval, casted := startTimeIntervalObj.(float64)
		if !casted {
			return false, fmt.Errorf("StartTimeInterval is not a float64")
		}
		startTime, err := xcodeutil.TimestampToTime(startTimeInterval)
		if err != nil {
			return false, err
		}

		{
			var renameMap map[string]string
			switch testSummaries.Type {
			case xcodeutil.TestSummariesWithScreenshotData:
				{
					title, err := getTitleOldSummaryType(testItem)
					if err != nil {
						log.Warnf("%s", err)
						continue
					}
					uuidObj, found := testItem["UUID"]
					if !found {
						return false, fmt.Errorf("missing UUID")
					}
					uuid, casted := uuidObj.(string)
					if !casted {
						return false, fmt.Errorf("UUID is not a string")
					}
					renameMap = prepareRenameOldSummaryType(title, uuid, startTime, attachementDir(testLogsDir))
				}
			case xcodeutil.TestSummariesWithAttachemnts:
				{
					var fromFileNames []string
					if fromFileNames, err = collectNewSummaryTypeFilenames(testItem); err != nil {
						log.Warnf("%s", err)
						continue
					}
					renameMap = prepareRenameNewSummaryType(fromFileNames, startTime, attachementDir(testLogsDir))
				}
			}
			hasSuccesfulRename := renameTestItemAttachments(renameMap)
			if !hasSuccesfulRename {
				log.Warnf("No screenshots renamed for test item: %s", pretty.Object(testItem))
			}
		}
	}

	return true, nil
}

func collectNewSummaryTypeFilenames(testItem map[string]interface{}) ([]string, error) {
	attachmentsObj, found := testItem["Attachments"]
	if !found {
		return nil, fmt.Errorf("Attachments not found in the TestSummaries.plist")
	}

	attachments, casted := attachmentsObj.([]interface{})
	if !casted {
		return nil, fmt.Errorf("Failed to cast attachmentsObj")
	}

	originalFileNames := make([]string, 0)
	for _, attachmentObj := range attachments {
		attachment, casted := attachmentObj.(map[string]interface{})
		if !casted {
			return nil, fmt.Errorf("Failed to cast attachmentObj")
		}

		var fileName string
		fileNameObj, found := attachment["Filename"]
		if found {
			fileName, casted = fileNameObj.(string)
			if casted {
				originalFileNames = append(originalFileNames, fileName)
			}
		}
	}
	return originalFileNames, nil
}

const timeFormat = "2006-01-02_03-04-05"

func prepareRenameNewSummaryType(originalFileNames []string, startTime time.Time, attachmentDir string) map[string]string {
	fileRenameMap := make(map[string]string)
	for _, fileName := range originalFileNames {
		formattedDate := startTime.Format(timeFormat)
		fromPath := fileName
		toPath := formattedDate + "_" + fileName
		fileRenameMap[filepath.Join(attachmentDir, fromPath)] = filepath.Join(attachmentDir, toPath)
	}
	return fileRenameMap
}

func getTitleOldSummaryType(testItem map[string]interface{}) (string, error) {
	titleObj, found := testItem["Title"]
	if !found {
		return "", fmt.Errorf("missing Title")
	}
	title, casted := titleObj.(string)
	if !casted {
		return "", fmt.Errorf("Title is not a string")
	}
	return title, nil
}

func prepareRenameOldSummaryType(title string, uuid string, startTime time.Time, attachmentDir string) map[string]string {
	fileRenameMap := make(map[string]string)
	for _, ext := range []string{"png", "jpg"} {
		fromPath := fmt.Sprintf("Screenshot_%s.%s", uuid, ext)
		formattedDate := startTime.Format(timeFormat)
		fixedTitle := strings.Replace(title, " ", "_", -1)
		toPath := fmt.Sprintf("%s_%s_%s", formattedDate, fixedTitle, uuid) + "." + ext
		fileRenameMap[filepath.Join(attachmentDir, fromPath)] = filepath.Join(attachmentDir, toPath)
	}
	return fileRenameMap
}

func renameTestItemAttachments(renameMap map[string]string) bool {
	var succesfulRenames int
	for fromFileName, toFileName := range renameMap {
		if exists, err := pathutil.IsPathExists(fromFileName); err != nil {
			log.Warnf("Error checking if file exists, error: ", err)
		} else if !exists {
			log.Infof("Screenshot file does not exists: %s", fromFileName)
			continue
		}
		if err := os.Rename(fromFileName, toFileName); err != nil {
			log.Warnf("Failed to rename the screenshot: %s, error: %s", fromFileName, err)
			continue
		}
		succesfulRenames++
		log.Printf("Screenshot renamed: %s => %s", filepath.Base(fromFileName), filepath.Base(toFileName))
	}
	return succesfulRenames > 0
}
