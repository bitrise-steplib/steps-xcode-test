package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/steps-xcode-test/pretty"
	"github.com/bitrise-io/steps-xcode-test/xcodeutil/testsummaries"
)

const targetScreenshotTimeFormat = "2006-01-02_03-04-05"

// UpdateScreenshotNames ...
// Screenshot_uuid.png -> TestID_start_date_time_title_uuid.png
// Screenshot_uuid.jpg -> TestID_start_date_time_title_uuid.jpg
func UpdateScreenshotNames(testSummariesPath string, attachementDir string) (bool, error) {
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return false, fmt.Errorf("Failed to check if file exists: %s", testSummariesPath)
	} else if !exist {
		return false, fmt.Errorf("no TestSummaries file found: %s", testSummariesPath)
	}
	testResults, err := testsummaries.New(testSummariesPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s, error: %s", testSummariesPath, err)
	}
	log.Debugf("Test results: %s", pretty.Object(testResults))

	renameMap := make(map[string]string)
	for _, testResult := range testResults {
		var filterScreenshotsFromActivityTree func(activities []testsummaries.Activity) []testsummaries.Activity
		filterScreenshotsFromActivityTree = func(activities []testsummaries.Activity) []testsummaries.Activity {
			activitiesWithScreensots := make([]testsummaries.Activity, 0)
			for _, activity := range activities {
				if len(activity.Screenshots) > 0 {
					activitiesWithScreensots = append(activitiesWithScreensots, activity)
				}
				activitiesWithScreensots = append(activitiesWithScreensots,
					filterScreenshotsFromActivityTree(activity.SubActivities)...)
			}
			return activitiesWithScreensots
		}
		activitiesWithScreensots := filterScreenshotsFromActivityTree(testResult.Activities)

		var mapScreenshotsToTargetFileName = func(activities []testsummaries.Activity) map[string]string {
			renameMap := make(map[string]string)
			for _, activity := range activities {
				for _, screenshot := range activity.Screenshots {
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
