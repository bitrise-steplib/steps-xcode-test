package main

import (
	"io/ioutil"
	"testing"

	cmd "github.com/bitrise-io/xcode-test/command"
	shellquote "github.com/kballard/go-shellquote"
)

//
// --- TESTS

func TestParseCommandLineOptions(t *testing.T) {
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

	words, err = shellquote.Split("/bin/sh -c 'echo")
	if err == nil {
		t.Fatalf("Expected (error), actual(%v)", err)
	}
}

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
	resStr := cmd.PrintableCommandArgs(orgCmdArgs)
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

func loadFileContent(filePth string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filePth)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}
