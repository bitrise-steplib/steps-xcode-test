package main

import (
	"io/ioutil"
	"testing"

	shellquote "github.com/kballard/go-shellquote"
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

//
// TESTING UTILITY FUNCS

func testIsFoundWith(t *testing.T, searchPattern, outputToSearchIn string, isShouldFind bool) {
	if isFound := isStringFoundInOutput(searchPattern, outputToSearchIn); isFound != isShouldFind {
		t.Logf("Search pattern was: %s", searchPattern)
		t.Logf("Input string to search in was: %s", outputToSearchIn)
		t.Fatalf("Expected (%v), actual (%v)", isShouldFind, isFound)
	}
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
