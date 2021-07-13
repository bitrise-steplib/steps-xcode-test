package main

import (
	"errors"
	"io/ioutil"
	"testing"

	shellquote "github.com/kballard/go-shellquote"
	"github.com/stretchr/testify/assert"
)

func TestParseCommandLineOptions(t *testing.T) {
	t.Log("Parse complicated command")
	{
		expectedWords := []string{"/bin/sh", "-c", `echo "my complicated command" | tee log | cat > log2`}
		words, err := shellquote.Split("/bin/sh -c 'echo \"my complicated command\" | tee log | cat > log2'")
		if err != nil {
			t.Fatalf("Expected (no error), actual(%v)", err)
		}
		if len(words) != len(expectedWords) {
			t.Fatalf("Expected (%d), actual(%d)", len(expectedWords), len(words))
		}

		for i := 0; i < len(expectedWords); i++ {
			exceptedWord := expectedWords[i]
			word := words[i]

			if word != exceptedWord {
				t.Fatalf("Expected (%s), actual(%s)", exceptedWord, word)
			}
		}
	}

	t.Log("Parse invalid command")
	{
		_, err := shellquote.Split("/bin/sh -c 'echo")
		if err == nil {
			t.Fatalf("Expected (error), actual(%v)", err)
		}
	}
}

func Test_isStringFoundInOutput(t *testing.T) {
	t.Log("Should NOT find")
	{
		searchPattern := "something"
		isShouldFind := false
		for _, anOutStr := range []string{
			"",
			"a",
			"1",
			"somethin",
			"somethinx",
			"TEST FAILED",
		} {
			if isFound := isStringFoundInOutput(searchPattern, anOutStr); isFound != isShouldFind {
				t.Logf("Search pattern was: %s", searchPattern)
				t.Logf("Input string to search in was: %s", anOutStr)
				t.Fatalf("Expected (%v), actual (%v)", isShouldFind, isFound)
			}
		}
	}

	t.Log("Should find")
	{
		searchPattern := "search for this"
		isShouldFind := true
		for _, anOutStr := range []string{
			"search for this",
			"-search for this",
			"search for this-",
			"-search for this-",
		} {
			if isFound := isStringFoundInOutput(searchPattern, anOutStr); isFound != isShouldFind {
				t.Logf("Search pattern was: %s", searchPattern)
				t.Logf("Input string to search in was: %s", anOutStr)
				t.Fatalf("Expected (%v), actual (%v)", isShouldFind, isFound)
			}
		}
	}

	t.Log("Should find - empty pattern - always yes")
	{
		searchPattern := ""
		isShouldFind := true
		for _, anOutStr := range []string{
			"",
			"a",
			"1",
			"TEST FAILED",
		} {
			if isFound := isStringFoundInOutput(searchPattern, anOutStr); isFound != isShouldFind {
				t.Logf("Search pattern was: %s", searchPattern)
				t.Logf("Input string to search in was: %s", anOutStr)
				t.Fatalf("Expected (%v), actual (%v)", isShouldFind, isFound)
			}
		}
	}
}

func TestIsStringFoundInOutput_timedOutRegisteringForTestingEvent(t *testing.T) {
	t.Log("load sample logs")
	{

	}
	sampleTestRunnerLog, err := loadFileContent("./_samples/xcodebuild-timed-out-registering-for-testing-event.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"Timed out registering for testing event accessibility notifications",
			".timed out registering for testing event accessibility notifications.",
			"(Timed out registering for testing event accessibility notifications)",
			"aaaTimed out registering for testing event accessibility notifications... test test test",
			"aaa timed out registering for testing event accessibility notificationstest",
			sampleTestRunnerLog,
		} {
			timedOutRegisteringForTestingEventWith(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"accessibility",
			"timed out",
			sampleOKBuildLog,
		} {
			timedOutRegisteringForTestingEventWith(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_testRunnerFailedToInitializeForUITesting(t *testing.T) {
	t.Log("load sample logs")
	{

	}
	sampleTestRunnerLog, err := loadFileContent("./_samples/xcodebuild-test-runner-failed-to-initialize-for-ui-testing.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"test runner failed to initialize for ui testing",
			"Test runner failed to initialize for ui testing.",
			"Test runner failed to initialize for UI testing, test test test",
			"aaaTest runner failed to initialize for UI testing... test test test",
			"aaa Test runner failed to initialize for UI testingtest",
			sampleTestRunnerLog,
		} {
			testRunnerFailedToInitializeForUITestingWith(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"UI testing:",
			"test runner failed",
			sampleOKBuildLog,
		} {
			testRunnerFailedToInitializeForUITestingWith(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_timeOutMessageIPhoneSimulator(t *testing.T) {
	t.Log("load sample logs")
	{

	}
	sampleIPhoneSimulatorLog, err := loadFileContent("./_samples/xcodebuild-iPhoneSimulator-timeout.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"iPhoneSimulator: Timed out waiting",
			"iphoneSimulator: timed out waiting",
			"iphoneSimulator: timed out waiting, test test test",
			"aaaiphoneSimulator: timed out waiting, test test test",
			"aaa iphoneSimulator: timed out waiting, test test test",
			sampleIPhoneSimulatorLog,
		} {
			testIPhoneSimulatorTimeoutWith(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"iphoneSimulator:",
			sampleOKBuildLog,
		} {
			testIPhoneSimulatorTimeoutWith(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_timeOutMessageUITest(t *testing.T) {
	// load sample logs
	sampleUITestTimeoutLog, err := loadFileContent("./_samples/xcodebuild-UITest-timeout.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"Terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
			"terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
			"aaTerminating app due to uncaught exception '_XCTestCaseInterruptionException'aa",
			"aa Terminating app due to uncaught exception '_XCTestCaseInterruptionException' aa",
			sampleUITestTimeoutLog,
		} {
			testTimeOutMessageUITestWith(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"Terminating app due to uncaught exception",
			"_XCTestCaseInterruptionException",
			sampleOKBuildLog,
		} {
			testTimeOutMessageUITestWith(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_earlyUnexpectedExit(t *testing.T) {
	// load sample logs
	sampleUITestEarlyUnexpectedExit1, err := loadFileContent("./_samples/xcodebuild-early-unexpected-exit_1.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleUITestEarlyUnexpectedExit2, err := loadFileContent("./_samples/xcodebuild-early-unexpected-exit_2.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"Early unexpected exit, operation never finished bootstrapping - no restart will be attempted",
			"Test target ios-xcode-8.0UITests encountered an error (Early unexpected exit, operation never finished bootstrapping - no restart will be attempted)",
			"aaEarly unexpected exit, operation never finished bootstrapping - no restart will be attemptedaa",
			"aa Early unexpected exit, operation never finished bootstrapping - no restart will be attempted aa",
			sampleUITestEarlyUnexpectedExit1,
			sampleUITestEarlyUnexpectedExit2,
		} {
			testEarlyUnexpectedExitMessageUITestWith(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"Early unexpected exit, operation never finished bootstrapping",
			"no restart will be attempted",
			sampleOKBuildLog,
		} {
			testEarlyUnexpectedExitMessageUITestWith(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_failureAttemptingToLaunch(t *testing.T) {
	// load sample logs
	sampleUITestFailureAttemptingToLaunch, err := loadFileContent("./_samples/xcodebuild-failure-attempting-tolaunch.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			"Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:",
			"t =    46.77s             Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl: 0x608000423da0 io.bitrise.ios-xcode-8-0 at ",
			"aaAssertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:aa",
			"aa Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl: aa",
			sampleUITestFailureAttemptingToLaunch,
		} {
			testFailureAttemptingToLaunch(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"Assertion Failure:",
			"Failure attempting to launch <XCUIApplicationImpl:",
			sampleOKBuildLog,
		} {
			testFailureAttemptingToLaunch(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_failedToBackgroundTestRunner(t *testing.T) {
	// load sample logs
	sampleUITestFailedToBackgroundTestRunner, err := loadFileContent("./_samples/xcodebuild-failed-to-background-test-runner.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			`Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.`,
			`2016-09-26 01:14:08.896 xcodebuild[1299:5953] Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner. If you believe this error represents a bug, please attach the log file at`,
			`aaError Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.aa`,
			`aa Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner. aa`,
			sampleUITestFailedToBackgroundTestRunner,
		} {
			testfailedToBackgroundTestRunner(t, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			"Assertion Failure:",
			"Failure attempting to launch <XCUIApplicationImpl:",
			sampleOKBuildLog,
		} {
			testFailureAttemptingToLaunch(t, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_appStateIsStillNotRunning(t *testing.T) {
	// load sample logs
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			`App state is still not running active, state = XCApplicationStateNotRunning`,
			`---App state is still not running active, state = XCApplicationStateNotRunning---`,
			`Asdf.SchemeUITests
  testThis, UI Testing Failure - '<XCUIApplicationImpl: 0x600000433440 com.this.Scheme.Target at /Users/vagrant/Library/Developer/Xcode/DerivedData/App-bfkuwgxmaiprwncotahtjjhoigbm/Build/Products/Debug-iphonesimulator/TheApp.app>' App state is still not running active, state = XCApplicationStateNotRunning
  MyAppUITests.swift:32`,
		} {
			testIsFoundWith(t, appStateIsStillNotRunning, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			`App state is still not running active, state = XCApplicationStateXXX`,
			sampleOKBuildLog,
		} {
			testIsFoundWith(t, appStateIsStillNotRunning, anOutStr, false)
		}
	}
}

func TestIsStringFoundInOutput_appAccessibilityIsNotLoaded(t *testing.T) {
	// load sample logs
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	t.Log("Should find")
	{
		for _, anOutStr := range []string{
			`UI Testing Failure - App accessibility isn't loaded`,
			`---UI Testing Failure - App accessibility isn't loaded---`,
			`    t =    65.80s                 Assertion Failure: LivescoreAppUITests.swift:23: UI Testing Failure - App accessibility isn't loaded`,
			`    t =     0.02s         Launch com.tapdm.LiveScore.WhatsTheScore
    t =     5.06s             Waiting for accessibility to load
    t =    65.80s                 Assertion Failure: LivescoreAppUITests.swift:23: UI Testing Failure - App accessibility isn't loaded
    t =    65.81s     Tear Down`,
		} {
			testIsFoundWith(t, appAccessibilityIsNotLoaded, anOutStr, true)
		}
	}

	t.Log("Should NOT find")
	{
		for _, anOutStr := range []string{
			"",
			`UI Testing Failure`,
			`App accessibility isn't loaded`,
			sampleOKBuildLog,
		} {
			testIsFoundWith(t, appAccessibilityIsNotLoaded, anOutStr, false)
		}
	}
}

func Test_GivenXcprettyInstallationCheckError_WhenTheErrorIsHandled_ThenExpectAnEmptyOutputToolAndErrorToBeReturned(t *testing.T) {
	// Given
	givenError := newXcprettyInstallationCheckError("an error occurred")

	// When
	outputTool, err := handleXcprettyInstallError(givenError)

	// Then
	assert.Equal(t, "", outputTool)
	assert.Equal(t, givenError, err)
}

func Test_GivenXcprettyDetermineVersionError_WhenTheErrorIsHandled_ThenExpectTheXcodeBuildOutputToolToBeReturned(t *testing.T) {
	// Given
	givenError := errors.New("determineVersionError")

	// When
	outputTool, err := handleXcprettyInstallError(givenError)

	// Then
	assert.Equal(t, xcodebuildTool, outputTool)
	assert.NoError(t, err)
}

//
// TESTING UTILITY FUNCS

func testIsFoundWith(t *testing.T, searchPattern, outputToSearchIn string, isShouldFind bool) {
	if isFound := isStringFoundInOutput(searchPattern, outputToSearchIn); isFound != isShouldFind {
		t.Logf("Search pattern was: %s", searchPattern)
		t.Logf("Input string to search in was: %s", outputToSearchIn)
		t.Fatalf("Expected (%v), actual (%v)", isShouldFind, isFound)
	}
}
func testRunnerFailedToInitializeForUITestingWith(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, testRunnerFailedToInitializeForUITesting, outputToSearchIn, isShouldFind)
}

func timedOutRegisteringForTestingEventWith(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, timedOutRegisteringForTestingEvent, outputToSearchIn, isShouldFind)
}

func testIPhoneSimulatorTimeoutWith(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, timeOutMessageIPhoneSimulator, outputToSearchIn, isShouldFind)
}

func testTimeOutMessageUITestWith(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, timeOutMessageUITest, outputToSearchIn, isShouldFind)
}

func testEarlyUnexpectedExitMessageUITestWith(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, earlyUnexpectedExit, outputToSearchIn, isShouldFind)
}

func testFailureAttemptingToLaunch(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, failureAttemptingToLaunch, outputToSearchIn, isShouldFind)
}

func testfailedToBackgroundTestRunner(t *testing.T, outputToSearchIn string, isShouldFind bool) {
	testIsFoundWith(t, failedToBackgroundTestRunner, outputToSearchIn, isShouldFind)
}

func loadFileContent(filePth string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filePth)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}
