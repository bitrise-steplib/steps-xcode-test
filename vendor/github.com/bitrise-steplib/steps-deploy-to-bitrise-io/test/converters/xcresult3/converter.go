package xcresult3

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"howett.net/plist"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/xcresult3/model3"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/testreport"
)

// Converter ...
type Converter struct {
	xcresultPth               string
	useLegacyExtractionMethod bool
}

func majorVersion(document serialized.Object) (int, error) {
	version, err := document.Object("version")
	if err != nil {
		return -1, err
	}

	major, err := version.Value("major")
	if err != nil {
		return -1, err
	}
	return int(major.(uint64)), nil
}

func documentMajorVersion(pth string) (int, error) {
	content, err := fileutil.ReadBytesFromFile(pth)
	if err != nil {
		return -1, err
	}

	var info serialized.Object
	if _, err := plist.Unmarshal(content, &info); err != nil {
		return -1, err
	}

	return majorVersion(info)
}

func (c *Converter) Setup(useOldXCResultExtractionMethod bool) {
	c.useLegacyExtractionMethod = useOldXCResultExtractionMethod
}

// Detect ...
func (c *Converter) Detect(files []string) bool {
	if !isXcresulttoolAvailable() {
		log.Debugf("xcresult tool is not available")
		return false
	}

	for _, file := range files {
		if filepath.Ext(file) != ".xcresult" {
			continue
		}

		infoPth := filepath.Join(file, "Info.plist")
		if exist, err := pathutil.IsPathExists(infoPth); err != nil {
			log.Debugf("Failed to find Info.plist at %s: %s", infoPth, err)
			continue
		} else if !exist {
			log.Debugf("No Info.plist found at %s", infoPth)
			continue
		}

		version, err := documentMajorVersion(infoPth)
		if err != nil {
			log.Debugf("failed to get document version: %s", err)
			continue
		}

		if version < 3 {
			log.Debugf("version < 3: %d", version)
			continue
		}

		c.xcresultPth = file
		return true
	}
	return false
}

// XML ...
func (c *Converter) Convert() (testreport.TestReport, error) {
	supportsNewMethod, err := supportsNewExtractionMethods()
	if err != nil {
		return testreport.TestReport{}, err
	}

	useLegacyFlag := c.useLegacyExtractionMethod

	if supportsNewMethod && !useLegacyFlag {
		log.Infof("Using new extraction method")

		junitXml, err := parse(c.xcresultPth)
		if err == nil {
			return junitXml, nil
		}

		log.Warnf(fmt.Sprintf("Failed to parse extraction method: %s", err))
		log.Warnf("Falling back to legacy extraction method")

		sendRemoteWarning("xcresult3-parsing", "error: %s", err)

		useLegacyFlag = true
	}

	log.Infof("Using legacy extraction method")

	return legacyParse(c.xcresultPth, useLegacyFlag)
}

func legacyParse(path string, useLegacyFlag bool) (testreport.TestReport, error) {
	var (
		testResultDir = filepath.Dir(path)
		maxParallel   = runtime.NumCPU() * 2
	)

	log.Debugf("Maximum parallelism: %d.", maxParallel)

	_, summaries, err := Parse(path, useLegacyFlag)
	if err != nil {
		return testreport.TestReport{}, err
	}

	var xmlData testreport.TestReport
	{
		testSuiteCount := testSuiteCountInSummaries(summaries)
		xmlData.TestSuites = make([]testreport.TestSuite, 0, testSuiteCount)
	}

	summariesCount := len(summaries)
	log.Debugf("Summaries Count: %d", summariesCount)

	for _, summary := range summaries {
		testSuiteOrder, testsByName := summary.tests()

		for _, name := range testSuiteOrder {
			tests := testsByName[name]

			testSuite, err := genTestSuite(name, summary, tests, testResultDir, path, maxParallel, useLegacyFlag)
			if err != nil {
				return testreport.TestReport{}, err
			}

			xmlData.TestSuites = append(xmlData.TestSuites, testSuite)
		}
	}

	return xmlData, nil
}

func parse(path string) (testreport.TestReport, error) {
	results, err := ParseTestResults(path)
	if err != nil {
		return testreport.TestReport{}, err
	}

	testSummary, warnings, err := model3.Convert(results)
	if err != nil {
		return testreport.TestReport{}, err
	}

	if len(warnings) > 0 {
		sendRemoteWarning("xcresults3-data", "warnings: %s", warnings)
	}

	var xml testreport.TestReport

	for _, plan := range testSummary.TestPlans {
		for _, testBundle := range plan.TestBundles {
			xml.TestSuites = append(xml.TestSuites, parseTestBundle(testBundle))
		}
	}

	outputPath := filepath.Dir(path)
	if err := exportAttachments(path, outputPath); err != nil {
		return testreport.TestReport{}, err
	}

	return xml, nil
}

func parseTestBundle(testBundle model3.TestBundle) testreport.TestSuite {
	failedCount := 0
	skippedCount := 0
	var totalDuration time.Duration
	var tests []testreport.TestCase

	for _, testSuite := range testBundle.TestSuites {
		for _, testCase := range testSuite.TestCases {
			var testCasesToConvert []model3.TestCase
			if len(testCase.Retries) > 0 {
				testCasesToConvert = testCase.Retries
			} else {
				testCasesToConvert = []model3.TestCase{testCase.TestCase}
			}

			for _, testCaseToConvert := range testCasesToConvert {
				test := parseTestCase(testCaseToConvert)

				if test.Failure != nil {
					failedCount++
				} else if test.Skipped != nil {
					skippedCount++
				}
				totalDuration += testCaseToConvert.Time

				tests = append(tests, test)
			}
		}
	}

	return testreport.TestSuite{
		Name:      testBundle.Name,
		Tests:     len(tests),
		Failures:  failedCount,
		Skipped:   skippedCount,
		Time:      totalDuration.Seconds(),
		TestCases: tests,
	}
}

func parseTestCase(testCase model3.TestCase) testreport.TestCase {
	test := testreport.TestCase{
		Name:      testCase.Name,
		ClassName: testCase.ClassName,
		Time:      testCase.Time.Seconds(),
	}

	if testCase.Result == model3.TestResultFailed {
		test.Failure = &testreport.Failure{Value: testCase.Message}
	} else if testCase.Result == model3.TestResultSkipped {
		test.Skipped = &testreport.Skipped{}
	}

	return test
}

func exportAttachments(xcresultPath, outputPath string) error {
	if err := xcresulttoolExport(xcresultPath, "", outputPath, false); err != nil {
		return err
	}

	return renameFiles(outputPath)
}

func renameFiles(outputPath string) error {
	manifestPath := filepath.Join(outputPath, "manifest.json")
	bytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest []model3.TestAttachmentDetails
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return fmt.Errorf("failed to unmarshal manifest.json: %w", err)
	}

	for _, attachmentDetail := range manifest {
		for _, attachment := range attachmentDetail.Attachments {
			oldPath := filepath.Join(outputPath, attachment.ExportedFileName)
			newPath := filepath.Join(outputPath, attachment.SuggestedHumanReadableName)

			if err := os.Rename(oldPath, newPath); err != nil {
				// It is not a critical error if the rename fails because the file will be still exported just by its
				// unique ID.
				log.Warnf("Failed to rename %s to %s", oldPath, newPath)
			}
		}
	}

	if err := os.Remove(manifestPath); err != nil {
		return err
	}

	return nil
}

func testSuiteCountInSummaries(summaries []ActionTestPlanRunSummaries) int {
	testSuiteCount := 0
	for _, summary := range summaries {
		testSuiteOrder, _ := summary.tests()
		testSuiteCount += len(testSuiteOrder)
	}
	return testSuiteCount
}

func genTestSuite(name string,
	summary ActionTestPlanRunSummaries,
	tests []ActionTestSummaryGroup,
	testResultDir string,
	xcresultPath string,
	maxParallel int,
	useLegacyFlag bool,
) (testreport.TestSuite, error) {
	var (
		start           = time.Now()
		genTestSuiteErr error
		wg              sync.WaitGroup
		mtx             sync.RWMutex
	)

	testSuite := testreport.TestSuite{
		Name:      name,
		Tests:     len(tests),
		Failures:  summary.failuresCount(name),
		Skipped:   summary.skippedCount(name),
		Time:      summary.totalTime(name),
		TestCases: make([]testreport.TestCase, len(tests)),
	}

	testIdx := 0
	for testIdx < len(tests) {
		for i := 0; i < maxParallel && testIdx < len(tests); i++ {
			test := tests[testIdx]
			wg.Add(1)

			go func(test ActionTestSummaryGroup, testIdx int) {
				defer wg.Done()

				testCase, err := genTestCase(test, xcresultPath, testResultDir, useLegacyFlag)
				if err != nil {
					mtx.Lock()
					genTestSuiteErr = err
					mtx.Unlock()
				}

				testSuite.TestCases[testIdx] = testCase
			}(test, testIdx)

			testIdx++
		}

		wg.Wait()
	}

	log.Debugf("Generating test suite [%s] (%d tests) - DONE %v", name, len(tests), time.Since(start))

	return testSuite, genTestSuiteErr
}

func genTestCase(test ActionTestSummaryGroup, xcresultPath, testResultDir string, useLegacyFlag bool) (testreport.TestCase, error) {
	var duartion float64
	if test.Duration.Value != "" {
		var err error
		duartion, err = strconv.ParseFloat(test.Duration.Value, 64)
		if err != nil {
			return testreport.TestCase{}, err
		}
	}

	testSummary, err := test.loadActionTestSummary(xcresultPath, useLegacyFlag)
	// Ignoring the SummaryNotFoundError error is on purpose because not having an action summary is a valid use case.
	// For example, failed tests will always have a summary, but successful ones might have it or might not.
	// If they do not have it, then that means that they did not log anything to the console,
	// and they were not executed as device configuration tests.
	if err != nil && !errors.Is(err, ErrSummaryNotFound) {
		return testreport.TestCase{}, err
	}

	var failure *testreport.Failure
	var skipped *testreport.Skipped
	switch test.TestStatus.Value {
	case "Failure":
		failureMessage := ""
		for _, aTestFailureSummary := range testSummary.FailureSummaries.Values {
			file := aTestFailureSummary.FileName.Value
			line := aTestFailureSummary.LineNumber.Value
			message := aTestFailureSummary.Message.Value

			if len(failureMessage) > 0 {
				failureMessage += "\n"
			}
			failureMessage += fmt.Sprintf("%s:%s - %s", file, line, message)
		}

		failure = &testreport.Failure{
			Value: failureMessage,
		}
	case "Skipped":
		skipped = &testreport.Skipped{}
	}

	if err := test.exportScreenshots(xcresultPath, testResultDir, useLegacyFlag); err != nil {
		return testreport.TestCase{}, err
	}

	return testreport.TestCase{
		Name:              test.Name.Value,
		ConfigurationHash: testSummary.Configuration.Hash,
		ClassName:         strings.Split(test.Identifier.Value, "/")[0],
		Failure:           failure,
		Skipped:           skipped,
		Time:              duartion,
	}, nil
}
