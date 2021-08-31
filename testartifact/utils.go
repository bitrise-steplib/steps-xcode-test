package testartifact

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/pretty"
	"github.com/bitrise-steplib/steps-xcode-test/xcodeutil/testsummaries"
)

const targetScreenshotTimeFormat = "2006-01-02_03-04-05"

// UpdateScreenshotNames ...
// Screenshot_uuid.png -> TestID_start_date_time_title_uuid.png
// Screenshot_uuid.jpg -> TestID_start_date_time_title_uuid.jpg
func UpdateScreenshotNames(testSummariesPath string, attachementDir string) (bool, error) {
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return false, fmt.Errorf("failed to check if file exists: %s", testSummariesPath)
	} else if !exist {
		return false, fmt.Errorf("no TestSummaries file found: %s", testSummariesPath)
	}
	testResults, err := testsummaries.New(testSummariesPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s, error: %s", testSummariesPath, err)
	}
	log.Debugf("Test results: %s", pretty.Object(testResults))

	return commitRename(createRenamePlan(testResults, attachementDir)), nil
}

func filterActivitiesWithScreenshotsFromTree(activities []testsummaries.Activity) []testsummaries.Activity {
	var activitiesWithScreensots []testsummaries.Activity
	for _, activity := range activities {
		if len(activity.Screenshots) > 0 {
			activitiesWithScreensots = append(activitiesWithScreensots, activity)
		}
		activitiesWithScreensots = append(activitiesWithScreensots,
			filterActivitiesWithScreenshotsFromTree(activity.SubActivities)...)
	}
	return activitiesWithScreensots
}

func createRenamePlan(testResults []testsummaries.TestResult, attachmentDir string) map[string]string {
	renameMap := make(map[string]string)
	for _, testResult := range testResults {
		activitiesWithScreensots := filterActivitiesWithScreenshotsFromTree(testResult.Activities)

		testRenames := make(map[string]string)
		for _, activity := range activitiesWithScreensots {
			for _, screenshot := range activity.Screenshots {
				toFileName := fmt.Sprintf("%s_%s_%s_%s%s", replaceUnsupportedFilenameCharacters(testResult.ID),
					screenshot.TimeCreated.Format(targetScreenshotTimeFormat),
					replaceUnsupportedFilenameCharacters(activity.Title),
					activity.UUID,
					filepath.Ext(screenshot.FileName))
				fromFileName := filepath.Join(attachmentDir, screenshot.FileName)
				if testResult.Status != "Success" {
					renameMap[fromFileName] = filepath.Join(attachmentDir, "Failures", toFileName)
				} else {
					renameMap[fromFileName] = filepath.Join(attachmentDir, toFileName)
				}
			}
		}

		for k, v := range testRenames {
			renameMap[k] = v
		}
	}
	return renameMap
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

// TODO: merge with testaddon.replaceUnsupportedFilenameCharacters
// Replaces characters '/' and ':', which are unsupported in filnenames on macOS
func replaceUnsupportedFilenameCharacters(s string) string {
	s = strings.Replace(s, "/", "-", -1)
	s = strings.Replace(s, ":", "-", -1)
	return s
}

// Zip ...
func Zip(targetDir, targetRelPathToZip, zipPath string) error {
	zipCmd := exec.Command("/usr/bin/zip", "-rTy", zipPath, targetRelPathToZip)
	zipCmd.Dir = targetDir
	if out, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Zip failed, out: %s, err: %#v", out, err)
	}
	return nil
}
