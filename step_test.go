package main

import (
	"io/ioutil"
	"runtime/debug"
	"testing"
)

//
// --- TESTS

func Test_isStringFoundInOutput(t *testing.T) {
	// Should NOT find
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

	// Should find
	searchPattern = "search for this"
	isShouldFind = true
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

	// Should find - empty pattern - always "yes"
	searchPattern = ""
	isShouldFind = true
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

func Test_findTestSummaryInOutput(t *testing.T) {
	// load sample logs
	sampleIPhoneSimulatorLog, err := loadFileContent("./_samples/xcodebuild-iPhoneSimulator-timeout.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleUITestTimeoutLog, err := loadFileContent("./_samples/xcodebuild-UITest-timeout.txt")
	if err != nil {
		t.Fatalf("Failed to load (UITest timeout) error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	// Should NOT find
	for _, anOutStr := range []string{
		"",
		"xyz",
		"TEST SUCCEEDED",
		"TEST FAILED",
		sampleIPhoneSimulatorLog,
	} {
		testFindTestSummaryInOutput(t, false, anOutStr, true)
		testFindTestSummaryInOutput(t, false, anOutStr, false)
	}

	// should find
	testFindTestSummaryInOutput(t, true, "** TEST SUCCEEDED **", true)
	testFindTestSummaryInOutput(t, true, "** TEST FAILED **", false)
	testFindTestSummaryInOutput(t, true, "Failing tests:", false)
	testFindTestSummaryInOutput(t, true, "Testing failed:", false)
	testFindTestSummaryInOutput(t, true, sampleOKBuildLog, true)
	testFindTestSummaryInOutput(t, true, sampleUITestTimeoutLog, false)
}

func TestIsStringFoundInOutput_timeOutMessageIPhoneSimulator(t *testing.T) {
	// load sample logs
	sampleIPhoneSimulatorLog, err := loadFileContent("./_samples/xcodebuild-iPhoneSimulator-timeout.txt")
	if err != nil {
		t.Fatalf("Failed to load error sample log : %s", err)
	}
	sampleOKBuildLog, err := loadFileContent("./_samples/xcodebuild-ok.txt")
	if err != nil {
		t.Fatalf("Failed to load xcodebuild-ok.txt : %s", err)
	}

	// Should find
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

	// Should not
	for _, anOutStr := range []string{
		"",
		"iphoneSimulator:",
		sampleOKBuildLog,
	} {
		testIPhoneSimulatorTimeoutWith(t, anOutStr, false)
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

	// Should find
	for _, anOutStr := range []string{
		"Terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
		"terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
		"aaTerminating app due to uncaught exception '_XCTestCaseInterruptionException'aa",
		"aa Terminating app due to uncaught exception '_XCTestCaseInterruptionException' aa",
		sampleUITestTimeoutLog,
	} {
		testTimeOutMessageUITestWith(t, anOutStr, true)
	}

	// Should not
	for _, anOutStr := range []string{
		"",
		"Terminating app due to uncaught exception",
		"_XCTestCaseInterruptionException",
		sampleOKBuildLog,
	} {
		testTimeOutMessageUITestWith(t, anOutStr, false)
	}
}

func Test_printableCommandArgs(t *testing.T) {
	orgCmdArgs := []string{
		"xcodebuild", "-project", "MyProj.xcodeproj", "-scheme", "MyScheme",
		"build", "test",
		"-destination", "platform=iOS Simulator,name=iPhone 6,OS=latest",
		"-sdk", "iphonesimulator",
	}
	resStr := printableCommandArgs(orgCmdArgs)
	expectedStr := `xcodebuild "-project" "MyProj.xcodeproj" "-scheme" "MyScheme" "build" "test" "-destination" "platform=iOS Simulator,name=iPhone 6,OS=latest" "-sdk" "iphonesimulator"`

	if resStr != expectedStr {
		t.Log("printableCommandArgs failed to generate the expected string!")
		t.Logf(" -> expectedStr: %s", expectedStr)
		t.Logf(" -> resStr: %s", resStr)
		t.Fatalf("Expected string does not match the generated string. Original args: (%#v)", orgCmdArgs)
	}
}

//
// --- TESTING UTILITY FUNCS

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

func testFindTestSummaryInOutput(t *testing.T, isExpectToFind bool, fullOutput string, isRunSucess bool) {
	resStr := findTestSummaryInOutput(fullOutput, isRunSucess)
	if isExpectToFind && resStr == "" {
		t.Logf("Expected to find Test Summary in provided log.")
		debug.PrintStack()
		t.Fatalf("Provided output was: %s", fullOutput)
	}
	if !isExpectToFind && resStr != "" {
		t.Logf("Expected to NOT find Test Summary in provided log.")
		debug.PrintStack()
		t.Fatalf("Provided output was: %s", fullOutput)
	}
}

func loadFileContent(filePth string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filePth)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}
