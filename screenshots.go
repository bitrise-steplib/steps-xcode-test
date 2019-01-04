package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/steps-xcode-test/pretty"
	"github.com/bitrise-io/steps-xcode-test/xcodeutil/testresults"
)

const targetScreenshotTimeFormat = "2006-01-02_03-04-05"

// UpdateScreenshotNames ...
// Screenshot_uuid.png -> TestID_start_date_time_title_uuid.png
// Screenshot_uuid.jpg -> TestID_start_date_time_title_uuid.jpg
func UpdateScreenshotNames(testLogsDir string, attachementDir string) (bool, error) {
	testSummariesPath := filepath.Join(testLogsDir, testSummaryFileName)
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return false, fmt.Errorf("Failed to check if file exists: %s", testSummariesPath)
	} else if !exist {
		return false, fmt.Errorf("no TestSummaries file found: %s", testSummariesPath)
	}
	testResults, err := testresults.New(testSummariesPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s, error: %s", testSummariesPath, err)
	}
	log.Debugf("Test results: %s", pretty.Object(testResults))

	renameMap := make(map[string]string)
	for _, testResult := range testResults {
		var filterScreenshotsFromActivityTree func(activities []testresults.Activity) []testresults.Activity
		filterScreenshotsFromActivityTree = func(activities []testresults.Activity) []testresults.Activity {
			activitiesWithScreensots := make([]testresults.Activity, 0)
			for _, activity := range activities {
				if len(activity.Screenhsots) > 0 {
					activitiesWithScreensots = append(activitiesWithScreensots, activity)
				}
				activitiesWithScreensots = append(activitiesWithScreensots,
					filterScreenshotsFromActivityTree(activity.SubActivities)...)
			}
			return activitiesWithScreensots
		}
		activitiesWithScreensots := filterScreenshotsFromActivityTree(testResult.Activities)

		var mapScreenshotsToTargetFileName = func(activities []testresults.Activity) map[string]string {
			renameMap := make(map[string]string)
			for _, activity := range activities {
				for _, screenshot := range activity.Screenhsots {
					toFileName := fmt.Sprintf("%s_%s_%s_%s%s", replaceUnsupportedFilenameCharacters(testResult.ID),
						screenshot.Timestamp.Format(targetScreenshotTimeFormat),
						replaceUnsupportedFilenameCharacters(activity.Title),
						activity.UUID,
						filepath.Ext(screenshot.FilePath))
					fromFileName := filepath.Join(attachementDir, screenshot.FilePath)
					if testResult.TestStatus != "Success" {
						renameMap[fromFileName] = filepath.Join(attachementDir, "Failures", toFileName)
					} else {
						renameMap[fromFileName] = filepath.Join(attachementDir, toFileName)
					}
				}
			}
			return renameMap
		}
		testRenameMap := mapScreenshotsToTargetFileName(activitiesWithScreensots)
		for k, v := range testRenameMap {
			renameMap[k] = v
		}
	}
	return commitRename(renameMap), nil
}

// Replaces characters '/' and ':', which are unsupported in filnenames on MacOS
func replaceUnsupportedFilenameCharacters(s string) string {
	s = strings.Replace(s, "/", "-", -1)
	s = strings.Replace(s, ":", "-", -1)
	return s
}

func commitRename(renameMap map[string]string) bool {
	var succesfulRenames int
	for fromFileName, toFileName := range renameMap {
		if exists, err := pathutil.IsPathExists(fromFileName); err != nil {
			log.Warnf("Error checking if file exists, error: ", err)
		} else if !exists {
			log.Infof("Screenshot file does not exists: %s", fromFileName)
			continue
		}
		if err := os.Mkdir(filepath.Dir(toFileName), os.ModePerm); !os.IsExist(err) && err != nil {
			log.Warnf("Failed to create directory: %s", filepath.Dir(toFileName))
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

/*
func updateScreenshotNames(testLogsDir string) (bool, error) {
	testSummariesPath := filepath.Join(testLogsDir, testSummaryFileName)
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return false, fmt.Errorf("Failed to check if file exists: %s", testSummariesPath)
	} else if !exist {
		return false, fmt.Errorf("no TestSummaries file found: %s", testSummariesPath)
	}

	// TestSummaries
	testSummaries, err := testresults.New(testSummariesPath)
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
		startTime, err := testresults.TimestampToTime(startTimeInterval)
		if err != nil {
			return false, err
		}

		{
			var renameMap map[string]string
			switch testSummaries.Type {
			case testresults.ScreenshotsLegacy:
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
			case testresults.ScreenshotsAsAttachments:
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
*/

/*
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
*/
