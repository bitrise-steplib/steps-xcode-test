package main

import "testing"

func testIsFoundWith(t *testing.T, searchPattern, outputToSearchIn string, isShouldFind bool) {
	if isFound, err := isStringFoundInOutput(searchPattern, outputToSearchIn); err != nil {
		t.Logf("Search pattern was: %s", searchPattern)
		t.Logf("Input string to search in was: %s", outputToSearchIn)
		t.Fatalf("Error: Expected (nil), actual (%s)", err)
	} else if isFound != isShouldFind {
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

func TestIsStringFoundInOutput_timeOutMessageIPhoneSimulator(t *testing.T) {
	// Should find
	for _, anOutStr := range []string{
		"iPhoneSimulator: Timed out waiting",
		"iphoneSimulator: timed out waiting",
		"iphoneSimulator: timed out waiting, test test test",
		"aaaiphoneSimulator: timed out waiting, test test test",
		"aaa iphoneSimulator: timed out waiting, test test test",
		longOutputIPhoneSimulatorStrDidFind,
	} {
		testIPhoneSimulatorTimeoutWith(t, anOutStr, true)
	}

	// Should not
	for _, anOutStr := range []string{
		"",
		"iphoneSimulator:",
		longOutputIPhoneSimulatorStrNotFound,
	} {
		testIPhoneSimulatorTimeoutWith(t, anOutStr, false)
	}
}

func TestIsStringFoundInOutput_timeOutMessageUITest(t *testing.T) {
	// Should find
	for _, anOutStr := range []string{
		"Terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
		"terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
		"aaTerminating app due to uncaught exception '_XCTestCaseInterruptionException'aa",
		"aa Terminating app due to uncaught exception '_XCTestCaseInterruptionException' aa",
		longOutputUITestStrFound,
	} {
		testTimeOutMessageUITestWith(t, anOutStr, true)
	}

	// Should not
	for _, anOutStr := range []string{
		"",
		"Terminating app due to uncaught exception",
		"_XCTestCaseInterruptionException",
		longOutputUITestStrNotFound,
	} {
		testTimeOutMessageUITestWith(t, anOutStr, false)
	}
}

// -------------------------------------------------

const (
	longOutputUITestStrNotFound = `Copying libswiftDispatch.dylib from /Applications/Xcodes/Xcode-beta.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest/Frameworks
	Copying libswiftObjectiveC.dylib from /Applications/Xcodes/Xcode-beta.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest/Frameworks
	Copying libswiftCoreImage.dylib from /Applications/Xcodes/Xcode-beta.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest/Frameworks
	Copying libswiftDarwin.dylib from /Applications/Xcodes/Xcode-beta.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest/Frameworks
	Touch /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest
	    cd /Users/vagrant/git
	    export PATH="/Applications/Xcodes/Xcode-beta.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/usr/bin:/Applications/Xcodes/Xcode-beta.app/Contents/Developer/usr/bin:/Users/vagrant/.rvm/gems/ruby-2.1.5/bin:/Users/vagrant/.rvm/gems/ruby-2.1.5@global/bin:/Users/vagrant/.rvm/rubies/ruby-2.1.5/bin:/usr/local/bin:/usr/local/sbin:~/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/opt/go/libexec/bin:/Users/vagrant/go/bin:/Users/vagrant/bitrise/tools/cmd-bridge/bin/osx:/Users/vagrant/.rvm/bin:/Users/vagrant/.rvm/bin"
	    /usr/bin/touch -c /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest
	** TEST SUCCEEDED **
	Test Suite 'All tests' started at 2015-09-18 04:12:54.760
	Test Suite 'BitriseXcode7SampleTests.xctest' started at 2015-09-18 04:12:54.761
	Test Suite 'BitriseXcode7SampleTests' started at 2015-09-18 04:12:54.761
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testExample]' started.
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testExample]' passed (0.000 seconds).
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' started.
	/Users/vagrant/git/BitriseXcode7SampleTests/BitriseXcode7SampleTests.swift:33: Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' measured [Time, seconds] average: 0.000, relative standard deviation: 214.598%, values: [0.000009, 0.000001, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000], performanceMetricID:com.apple.XCTPerformanceMetric_WallClockTime, baselineName: "", baselineAverage: , maxPercentRegression: 10.000%, maxPercentRelativeStandardDeviation: 10.000%, maxRegression: 0.100, maxStandardDeviation: 0.100
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' passed (0.324 seconds).
	Test Suite 'BitriseXcode7SampleTests' passed at 2015-09-18 04:12:55.087.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.324 (0.326) seconds
	Test Suite 'BitriseXcode7SampleTests.xctest' passed at 2015-09-18 04:12:55.088.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.324 (0.327) seconds
	Test Suite 'All tests' passed at 2015-09-18 04:12:55.089.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.324 (0.329) seconds
	2015-09-18 04:12:57.059 XCTRunner[1104:6372] Running tests...
	2015-09-18 04:12:57.061 XCTRunner[1104:6372] Looking for test bundles in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/E7E1469D-ED5F-4432-AF52-F4791E5B744B/BitriseXcode7SampleUITests-Runner.app/PlugIns
	2015-09-18 04:12:57.062 XCTRunner[1104:6372] Found test bundle at /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/E7E1469D-ED5F-4432-AF52-F4791E5B744B/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-18 04:12:57.063 XCTRunner[1104:6372] Looking for configurations in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/E7E1469D-ED5F-4432-AF52-F4791E5B744B/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-18 04:12:57.065 XCTRunner[1104:6372] Found configuration
		                  testBundleURL:file:///Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
		              productModuleName:BitriseXcode7SampleUITests
		                    testsToSkip:(null)
		                     testsToRun:(null)
		             reportResultsToIDE:YES
		              sessionIdentifier:<__NSConcreteUUID 0x7fa403d328e0> CB505AC8-9CBC-4B78-9715-1CEE606F116E
		     pathToXcodeReportingSocket:(null)
		      disablePerformanceMetrics:no
		treatMissingBaselinesAsFailures:no
		                baselineFileURL:(null)
		          targetApplicationPath:/Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7Sample.app
		      targetApplicationBundleID:com.bitrise.BitriseXcode7Sample
		               reportActivities:no
	Test Suite 'All tests' started at 2015-09-18 04:12:57.259
	Test Suite 'BitriseXcode7SampleUITests.xctest' started at 2015-09-18 04:12:57.260
	Test Suite 'BitriseXcode7SampleUITests' started at 2015-09-18 04:12:57.260
	Test Case '-[BitriseXcode7SampleUITests.BitriseXcode7SampleUITests testAddAnItemGoToDetailsThenDeleteIt]' started.
	    t =     0.00s     Start Test
	    t =     0.00s     Set Up
	    t =     0.01s         Launch com.bitrise.BitriseXcode7Sample
	2015-09-18 04:12:59.203 XCTRunner[1104:6372] Continuing to run tests in the background with task ID 1
	    t =     3.12s             Waiting for accessibility to load
	    t =    10.15s             Wait for app to idle
	    t =    10.83s     Tap "Add" Button
	    t =    10.83s         Wait for app to idle
	    t =    11.05s         Find the "Add" Button
	    t =    11.05s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    11.12s             Find: Descendants matching type NavigationBar
	    t =    11.12s             Find: Elements matching predicate '"Master" IN identifiers'
	    t =    11.12s             Find: Descendants matching type Button
	    t =    11.12s             Find: Elements matching predicate '"Add" IN identifiers'
	    t =    11.16s             Wait for app to idle
	    t =    11.22s         Synthesize event
	    t =    11.48s         Wait for app to idle
	    t =    11.98s     Tap Cell
	    t =    11.98s         Wait for app to idle
	    t =    12.03s         Find the Cell
	    t =    12.03s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    12.08s             Find: Descendants matching type Table
	    t =    12.08s             Find: Descendants matching type Cell
	    t =    12.08s             Find: Element at index 0
	    t =    12.09s             Wait for app to idle
	    t =    12.16s         Synthesize event
	    t =    12.43s         Wait for app to idle
	    t =    12.97s     Tap "Master" Button
	    t =    12.97s         Wait for app to idle
	    t =    13.02s         Find the "Master" Button
	    t =    13.02s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    13.06s             Find: Descendants matching type NavigationBar
	    t =    13.08s             Find: Elements matching predicate '"Detail" IN identifiers'
	    t =    13.09s             Find: Descendants matching type Button
	    t =    13.09s             Find: Elements matching predicate '"Master" IN identifiers'
	    t =    13.11s             Wait for app to idle
	    t =    13.17s         Synthesize event
	    t =    13.43s         Wait for app to idle
	    t =    13.95s     Tap "Edit" Button
	    t =    13.95s         Wait for app to idle
	    t =    14.00s         Find the "Edit" Button
	    t =    14.00s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    14.03s             Find: Descendants matching type NavigationBar
	    t =    14.03s             Find: Elements matching predicate '"Master" IN identifiers'
	    t =    14.04s             Find: Descendants matching type Button
	    t =    14.04s             Find: Elements matching predicate '"Edit" IN identifiers'
	    t =    14.06s             Wait for app to idle
	    t =    14.12s         Synthesize event
	    t =    14.38s         Wait for app to idle
	    t =    14.86s     Tap Button
	    t =    14.86s         Wait for app to idle
	    t =    14.91s         Find the Button
	    t =    14.91s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    14.94s             Find: Descendants matching type Table
	    t =    14.95s             Find: Descendants matching type Cell
	    t =    14.96s             Find: Element at index 0
	    t =    14.96s             Find: Descendants matching type Button
	    t =    14.96s             Find: Element at index 0
	    t =    14.97s             Wait for app to idle
	    t =    15.03s         Synthesize event
	    t =    15.31s         Wait for app to idle
	    t =    16.22s     Tap "Delete" Button
	    t =    16.22s         Wait for app to idle
	    t =    16.32s         Find the "Delete" Button
	    t =    16.32s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    16.36s             Find: Descendants matching type Table
	    t =    16.37s             Find: Descendants matching type Cell
	    t =    16.37s             Find: Element at index 0
	    t =    16.37s             Find: Descendants matching type Button
	    t =    16.37s             Find: Elements matching predicate '"Delete" IN identifiers'
	    t =    16.39s             Wait for app to idle
	    t =    16.45s         Synthesize event
	    t =    16.71s         Wait for app to idle
	    t =    17.22s     Tap "Done" Button
	    t =    17.22s         Wait for app to idle
	    t =    17.26s         Find the "Done" Button
	    t =    17.26s             Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    17.29s             Find: Descendants matching type NavigationBar
	    t =    17.29s             Find: Elements matching predicate '"Master" IN identifiers'
	    t =    17.30s             Find: Descendants matching type Button
	    t =    17.30s             Find: Elements matching predicate '"Done" IN identifiers'
	    t =    17.31s             Wait for app to idle
	    t =    17.38s         Synthesize event
	    t =    17.63s         Wait for app to idle
	    t =    17.81s     Get number of matches for: Descendants matching type Cell
	    t =    17.85s         Snapshot accessibility hierarchy for com.bitrise.BitriseXcode7Sample
	    t =    17.89s         Find: Descendants matching type Table
	    t =    17.89s         Find: Descendants matching type Cell
	    t =    17.90s     Tear Down
	Test Case '-[BitriseXcode7SampleUITests.BitriseXcode7SampleUITests testAddAnItemGoToDetailsThenDeleteIt]' passed (17.902 seconds).
	Test Suite 'BitriseXcode7SampleUITests' passed at 2015-09-18 04:13:15.163.
		 Executed 1 test, with 0 failures (0 unexpected) in 17.902 (17.903) seconds
	Test Suite 'BitriseXcode7SampleUITests.xctest' passed at 2015-09-18 04:13:15.165.
		 Executed 1 test, with 0 failures (0 unexpected) in 17.902 (17.905) seconds
	Test Suite 'All tests' passed at 2015-09-18 04:13:15.167.
		 Executed 1 test, with 0 failures (0 unexpected) in 17.902 (17.909) seconds`

	longOutputUITestStrFound = `Touch /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest
	    cd /Users/vagrant/git
	    export PATH="/Applications/Xcodes/Xcode-beta.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/usr/bin:/Applications/Xcodes/Xcode-beta.app/Contents/Developer/usr/bin:/Users/vagrant/.rvm/gems/ruby-2.1.5/bin:/Users/vagrant/.rvm/gems/ruby-2.1.5@global/bin:/Users/vagrant/.rvm/rubies/ruby-2.1.5/bin:/usr/local/bin:/usr/local/sbin:~/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/opt/go/libexec/bin:/Users/vagrant/go/bin:/Users/vagrant/bitrise/tools/cmd-bridge/bin/osx:/Users/vagrant/.rvm/bin:/Users/vagrant/.rvm/bin"
	    /usr/bin/touch -c /Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleTests.xctest
	2015-09-17 11:13:25.763 xcodebuild[915:4476] [MT] IDETestOperationsObserverDebug: (3C2D1A34-8263-40C0-9B3C-2CE4A642260A) Beginning test session with Xcode 7A192o on target  {
			SimDevice: SimDevice : iPhone 6 (058C71C4-46CB-4878-930E-6A0E8707B917) : state={ Booted } deviceType={ SimDeviceType : com.apple.CoreSimulator.SimDeviceType.iPhone-6 } runtime={ SimRuntime : 9.0 (13A4325c) - com.apple.CoreSimulator.SimRuntime.iOS-9-0 }
	} (9.0 (13A4325c))
	** TEST FAILED **
	Test Suite 'All tests' started at 2015-09-17 11:13:09.627
	Test Suite 'BitriseXcode7SampleTests.xctest' started at 2015-09-17 11:13:09.642
	Test Suite 'BitriseXcode7SampleTests' started at 2015-09-17 11:13:09.644
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testExample]' started.
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testExample]' passed (0.005 seconds).
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' started.
	/Users/vagrant/git/BitriseXcode7SampleTests/BitriseXcode7SampleTests.swift:33: Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' measured [Time, seconds] average: 0.000, relative standard deviation: 192.944%, values: [0.000009, 0.000001, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000, 0.000000], performanceMetricID:com.apple.XCTPerformanceMetric_WallClockTime, baselineName: "", baselineAverage: , maxPercentRegression: 10.000%, maxPercentRelativeStandardDeviation: 10.000%, maxRegression: 0.100, maxStandardDeviation: 0.100
	Test Case '-[BitriseXcode7SampleTests.BitriseXcode7SampleTests testPerformanceExample]' passed (0.449 seconds).
	Test Suite 'BitriseXcode7SampleTests' passed at 2015-09-17 11:13:10.104.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.454 (0.460) seconds
	Test Suite 'BitriseXcode7SampleTests.xctest' passed at 2015-09-17 11:13:10.105.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.454 (0.463) seconds
	Test Suite 'All tests' passed at 2015-09-17 11:13:10.106.
		 Executed 2 tests, with 0 failures (0 unexpected) in 0.454 (0.479) seconds
	2015-09-17 11:13:12.966 XCTRunner[1069:5910] Running tests...
	2015-09-17 11:13:12.969 XCTRunner[1069:5910] Looking for test bundles in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/8A14E1E4-C25E-410E-99E9-2202FFF0BC38/BitriseXcode7SampleUITests-Runner.app/PlugIns
	2015-09-17 11:13:12.981 XCTRunner[1069:5910] Found test bundle at /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/8A14E1E4-C25E-410E-99E9-2202FFF0BC38/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-17 11:13:12.984 XCTRunner[1069:5910] Looking for configurations in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/8A14E1E4-C25E-410E-99E9-2202FFF0BC38/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-17 11:13:12.989 XCTRunner[1069:5910] Found configuration
		                  testBundleURL:file:///Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
		              productModuleName:BitriseXcode7SampleUITests
		                    testsToSkip:(null)
		                     testsToRun:(null)
		             reportResultsToIDE:YES
		              sessionIdentifier:<__NSConcreteUUID 0x7fa411c26ea0> 8DFCCDA0-11F5-4F42-82AE-3664F28C3A3F
		     pathToXcodeReportingSocket:(null)
		      disablePerformanceMetrics:no
		treatMissingBaselinesAsFailures:no
		                baselineFileURL:(null)
		          targetApplicationPath:/Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7Sample.app
		      targetApplicationBundleID:com.bitrise.BitriseXcode7Sample
		               reportActivities:no
	Test Suite 'All tests' started at 2015-09-17 11:13:13.399
	Test Suite 'BitriseXcode7SampleUITests.xctest' started at 2015-09-17 11:13:13.401
	Test Suite 'BitriseXcode7SampleUITests' started at 2015-09-17 11:13:13.402
	Test Case '-[BitriseXcode7SampleUITests.BitriseXcode7SampleUITests testAddAnItemGoToDetailsThenDeleteIt]' started.
	    t =     0.00s     Start Test
	    t =     0.00s     Set Up
	    t =     0.02s         Launch com.bitrise.BitriseXcode7Sample
	    t =     2.57s             Assertion Failure: UI Testing Failure - Failed to receive completion for
	/Users/vagrant/git/BitriseXcode7SampleUITests/BitriseXcode7SampleUITests.swift:21: error: -[BitriseXcode7SampleUITests.BitriseXcode7SampleUITests testAddAnItemGoToDetailsThenDeleteIt] : UI Testing Failure - Failed to receive completion for
	2015-09-17 11:13:16.007 XCTRunner[1069:5910] *** Terminating app due to uncaught exception '_XCTestCaseInterruptionException', reason: 'Interrupting test'
	*** First throw call stack:
	(
		0   CoreFoundation                      0x000000010740c9b5 __exceptionPreprocess + 165
		1   libobjc.A.dylib                     0x0000000106e84deb objc_exception_throw + 48
		2   CoreFoundation                      0x000000010740c8ed +[NSException raise:format:] + 205
		3   XCTest                              0x00000001069478b9 -[XCTestCase _dequeueFailures] + 560
		4   XCTest                              0x0000000106947b95 -[XCTestCase _enqueueFailureWithDescription:inFile:atLine:expected:] + 576
		5   XCTest                              0x000000010694f340 _XCTFailureHandler + 1112
		6   XCTest                              0x0000000106946d39 _XCTFailInCurrentTest + 512
		7   XCTest                              0x0000000106964663 -[XCUIDevice _dispatchEventWithPage:usage:duration:] + 790
		8   XCTest                              0x00000001069557f8 -[XCAXClient_iOS init] + 153
		9   XCTest                              0x0000000106955756 __30+[XCAXClient_iOS sharedClient]_block_invoke + 24
		10  libdispatch.dylib                   0x00000001096464c7 _dispatch_client_callout + 8
		11  libdispatch.dylib                   0x0000000109633d2f dispatch_once_f + 76
		12  XCTest                              0x000000010695573c +[XCAXClient_iOS sharedClient] + 42
		13  XCTest                              0x0000000106965be6 __37-[XCUIApplication _launchUsingXcode:]_block_invoke + 50
		14  XCTest                              0x000000010694b443 -[XCTestCase startActivityWithTitle:block:] + 305
		15  XCTest                              0x0000000106965ba5 -[XCUIApplication _launchUsingXcode:] + 309
		16  BitriseXcode7SampleUITests          0x0000000112bf7dd9 _TFC26BitriseXcode7SampleUITests26BitriseXcode7SampleUITests5setUpfS0_FT_T_ + 153
		17  BitriseXcode7SampleUITests          0x0000000112bf7e62 _TToFC26BitriseXcode7SampleUITests26BitriseXcode7SampleUITests5setUpfS0_FT_T_ + 34
		18  XCTest                              0x000000010694b443 -[XCTestCase startActivityWithTitle:block:] + 305
		19  XCTest                              0x0000000106947def __24-[XCTestCase invokeTest]_block_invoke_2 + 118
		20  XCTest                              0x00000001069774a0 -[XCTestContext performInScope:] + 184
		21  XCTest                              0x0000000106947d68 -[XCTestCase invokeTest] + 169
		22  XCTest                              0x0000000106948203 -[XCTestCase performTest:] + 443
		23  XCTest                              0x0000000106945ec9 -[XCTestSuite performTest:] + 377
		24  XCTest                              0x0000000106945ec9 -[XCTestSuite performTest:] + 377
		25  XCTest                              0x0000000106945ec9 -[XCTestSuite performTest:] + 377
		26  XCTest                              0x00000001069336d2 __25-[XCTestDriver _runSuite]_block_invoke + 51
		27  XCTest                              0x00000001069538db -[XCTestObservationCenter _observeTestExecutionForBlock:] + 615
		28  XCTest                              0x000000010693361e -[XCTestDriver _runSuite] + 408
		29  XCTest                              0x000000010693437d -[XCTestDriver _checkForTestManager] + 263
		30  XCTest                              0x0000000106978801 _XCTestMain + 628
		31  libdispatch.dylib                   0x000000010962aea9 _dispatch_call_block_and_release + 12
		32  libdispatch.dylib                   0x00000001096464c7 _dispatch_client_callout + 8
		33  libdispatch.dylib                   0x000000010963107d _dispatch_main_queue_callback_4CF + 714
		34  CoreFoundation                      0x000000010736ce69 __CFRUNLOOP_IS_SERVICING_THE_MAIN_DISPATCH_QUEUE__ + 9
		35  CoreFoundation                      0x000000010732e3b9 __CFRunLoopRun + 2073
		36  CoreFoundation                      0x000000010732d918 CFRunLoopRunSpecific + 488
		37  GraphicsServices                    0x0000000108e70ad2 GSEventRunModal + 161
		38  UIKit                               0x00000001077ba99e UIApplicationMain + 171
		39  XCTRunner                           0x00000001068be6bf XCTRunner + 5823
		40  libdyld.dylib                       0x000000010967692d start + 1
		41  ???                                 0x0000000000000005 0x0 + 5
	)
	libc++abi.dylib: terminating with uncaught exception of type _XCTestCaseInterruptionException
	2015-09-17 11:13:28.121 XCTRunner[1074:6376] Running tests...
	2015-09-17 11:13:28.124 XCTRunner[1074:6376] Looking for test bundles in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/F66EFE75-B030-4E18-B370-F0A304CAA31B/BitriseXcode7SampleUITests-Runner.app/PlugIns
	2015-09-17 11:13:28.125 XCTRunner[1074:6376] Found test bundle at /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/F66EFE75-B030-4E18-B370-F0A304CAA31B/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-17 11:13:28.126 XCTRunner[1074:6376] Looking for configurations in /Users/vagrant/Library/Developer/CoreSimulator/Devices/058C71C4-46CB-4878-930E-6A0E8707B917/data/Containers/Bundle/Application/F66EFE75-B030-4E18-B370-F0A304CAA31B/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest
	2015-09-17 11:13:28.128 XCTRunner[1074:6376] Found configuration
		                  testBundleURL:file:///Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7SampleUITests-Runner.app/PlugIns/BitriseXcode7SampleUITests.xctest/
		              productModuleName:BitriseXcode7SampleUITests
		                    testsToSkip:BitriseXcode7SampleUITests/testAddAnItemGoToDetailsThenDeleteIt()
		                     testsToRun:(null)
		             reportResultsToIDE:YES
		              sessionIdentifier:<__NSConcreteUUID 0x7fece159f3b0> 3C2D1A34-8263-40C0-9B3C-2CE4A642260A
		     pathToXcodeReportingSocket:(null)
		      disablePerformanceMetrics:no
		treatMissingBaselinesAsFailures:no
		                baselineFileURL:(null)
		          targetApplicationPath:/Users/vagrant/Library/Developer/Xcode/DerivedData/BitriseXcode7Sample-adkzkiqwlfliqgcggdmlvbmsomkl/Build/Products/Debug-iphonesimulator/BitriseXcode7Sample.app
		      targetApplicationBundleID:com.bitrise.BitriseXcode7Sample
		               reportActivities:no
	Test Suite 'Selected tests' started at 2015-09-17 11:13:28.526
	Test Suite 'BitriseXcode7SampleUITests.xctest' started at 2015-09-17 11:13:28.532
	Test Suite 'BitriseXcode7SampleUITests.xctest' passed at 2015-09-17 11:13:28.535.
		 Executed 0 tests, with 0 failures (0 unexpected) in 0.000 (0.003) seconds`

	longOutputIPhoneSimulatorStrNotFound = `
    export SEPARATE_SYMBOL_EDIT=NO
    export SET_DIR_MODE_OWNER_GROUP=YES
    export SET_FILE_MODE_OWNER_GROUP=NO
    export SHALLOW_BUNDLE=YES
    export SHARED_DERIVED_FILE_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/DerivedSources
    export SHARED_FRAMEWORKS_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/SharedFrameworks
    export SHARED_PRECOMPS_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/PrecompiledHeaders
    export SHARED_SUPPORT_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/SharedSupport
    export SKIP_INSTALL=YES
    export SOURCE_ROOT=/Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export SRCROOT=/Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export STRINGS_FILE_OUTPUT_ENCODING=binary
    export STRIP_INSTALLED_PRODUCT=YES
    export STRIP_STYLE=non-global
    export SUPPORTED_DEVICE_FAMILIES="1 2"
    export SUPPORTED_PLATFORMS="iphonesimulator iphoneos"
    export SWIFT_OPTIMIZATION_LEVEL=-Onone
    export SYMROOT=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products
    export SYSTEM_ADMIN_APPS_DIR=/Applications/Utilities
    export SYSTEM_APPS_DIR=/Applications
    export SYSTEM_CORE_SERVICES_DIR=/System/Library/CoreServices
    export SYSTEM_DEMOS_DIR=/Applications/Extras
    export SYSTEM_DEVELOPER_APPS_DIR=/Applications/Xcode.app/Contents/Developer/Applications
    export SYSTEM_DEVELOPER_BIN_DIR=/Applications/Xcode.app/Contents/Developer/usr/bin
    export SYSTEM_DEVELOPER_DEMOS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Utilities/Built Examples"
    export SYSTEM_DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
    export SYSTEM_DEVELOPER_DOC_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library"
    export SYSTEM_DEVELOPER_GRAPHICS_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Graphics Tools"
    export SYSTEM_DEVELOPER_JAVA_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Java Tools"
    export SYSTEM_DEVELOPER_PERFORMANCE_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Performance Tools"
    export SYSTEM_DEVELOPER_RELEASENOTES_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/releasenotes"
    export SYSTEM_DEVELOPER_TOOLS=/Applications/Xcode.app/Contents/Developer/Tools
    export SYSTEM_DEVELOPER_TOOLS_DOC_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/documentation/DeveloperTools"
    export SYSTEM_DEVELOPER_TOOLS_RELEASENOTES_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/releasenotes/DeveloperTools"
    export SYSTEM_DEVELOPER_USR_DIR=/Applications/Xcode.app/Contents/Developer/usr
    export SYSTEM_DEVELOPER_UTILITIES_DIR=/Applications/Xcode.app/Contents/Developer/Applications/Utilities
    export SYSTEM_DOCUMENTATION_DIR=/Library/Documentation
    export SYSTEM_KEXT_INSTALL_PATH=/System/Library/Extensions
    export SYSTEM_LIBRARY_DIR=/System/Library
    export TARGETED_DEVICE_FAMILY=1,2
    export TARGETNAME=BitriseSampleWithYMLTests
    export TARGET_BUILD_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator
    export TARGET_NAME=BitriseSampleWithYMLTests
    export TARGET_TEMP_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_FILES_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_FILE_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_ROOT=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates
    export TEST_FRAMEWORK_SEARCH_PATHS=" /Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/Library/Frameworks /Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/SDKs/iPhoneSimulator8.4.sdk/Developer/Library/Frameworks"
    export TEST_HOST=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYML.app/BitriseSampleWithYML
    export TOOLCHAINS=com.apple.dt.toolchain.iOS8_4
    export TREAT_MISSING_BASELINES_AS_TEST_FAILURES=NO
    export UID=501
    export UNLOCALIZED_RESOURCES_FOLDER_PATH=BitriseSampleWithYMLTests.xctest
    export UNSTRIPPED_PRODUCT=NO
    export USER=awesome-bitrise-user
    export USER_APPS_DIR=/Users/awesome-bitrise-user/Applications
    export USER_LIBRARY_DIR=/Users/awesome-bitrise-user/Library
    export USE_DYNAMIC_NO_PIC=YES
    export USE_HEADERMAP=YES
    export USE_HEADER_SYMLINKS=NO
    export VALIDATE_PRODUCT=NO
    export VALID_ARCHS="i386 x86_64"
    export VERBOSE_PBXCP=NO
    export VERSIONPLIST_PATH=BitriseSampleWithYMLTests.xctest/version.plist
    export VERSION_INFO_BUILDER=awesome-bitrise-user
    export VERSION_INFO_FILE=BitriseSampleWithYMLTests_vers.c
    export VERSION_INFO_STRING="\"@(#)PROGRAM:BitriseSampleWithYMLTests  PROJECT:BitriseSampleWithYML-\""
    export WRAPPER_EXTENSION=xctest
    export WRAPPER_NAME=BitriseSampleWithYMLTests.xctest
    export WRAPPER_SUFFIX=.xctest
    export XCODE_APP_SUPPORT_DIR=/Applications/Xcode.app/Contents/Developer/Library/Xcode
    export XCODE_PRODUCT_BUILD_VERSION=6E35b
    export XCODE_VERSION_ACTUAL=0640
    export XCODE_VERSION_MAJOR=0600
    export XCODE_VERSION_MINOR=0640
    export XPCSERVICES_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/XPCServices
    export YACC=yacc
    export arch=x86_64
    export variant=normal
    /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/swift-stdlib-tool --verbose --copy
Copying libswiftCore.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftCoreGraphics.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftFoundation.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftSecurity.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftUIKit.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftXCTest.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftDispatch.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftObjectiveC.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftCoreImage.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftDarwin.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks

Touch /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest
    cd /Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export PATH="/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/usr/bin:/Applications/Xcode.app/Contents/Developer/usr/bin:/Users/awesome-bitrise-user/.rbenv/shims:/usr/local/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/awesome-bitrise-user/develop/go/bin"
    /usr/bin/touch -c /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest

** TEST SUCCEEDED **`

	// full test - FOUND
	longOutputIPhoneSimulatorStrDidFind = `
    export SEPARATE_SYMBOL_EDIT=NO
    export SET_DIR_MODE_OWNER_GROUP=YES
    export SET_FILE_MODE_OWNER_GROUP=NO
    export SHALLOW_BUNDLE=YES
    export SHARED_DERIVED_FILE_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/DerivedSources
    export SHARED_FRAMEWORKS_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/SharedFrameworks
    export SHARED_PRECOMPS_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/PrecompiledHeaders
    export SHARED_SUPPORT_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/SharedSupport
    export SKIP_INSTALL=YES
    export SOURCE_ROOT=/Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export SRCROOT=/Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export STRINGS_FILE_OUTPUT_ENCODING=binary
    export STRIP_INSTALLED_PRODUCT=YES
    export STRIP_STYLE=non-global
    export SUPPORTED_DEVICE_FAMILIES="1 2"
    export SUPPORTED_PLATFORMS="iphonesimulator iphoneos"
    export SWIFT_OPTIMIZATION_LEVEL=-Onone
    export SYMROOT=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products
    export SYSTEM_ADMIN_APPS_DIR=/Applications/Utilities
    export SYSTEM_APPS_DIR=/Applications
    export SYSTEM_CORE_SERVICES_DIR=/System/Library/CoreServices
    export SYSTEM_DEMOS_DIR=/Applications/Extras
    export SYSTEM_DEVELOPER_APPS_DIR=/Applications/Xcode.app/Contents/Developer/Applications
    export SYSTEM_DEVELOPER_BIN_DIR=/Applications/Xcode.app/Contents/Developer/usr/bin
    export SYSTEM_DEVELOPER_DEMOS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Utilities/Built Examples"
    export SYSTEM_DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
    export SYSTEM_DEVELOPER_DOC_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library"
    export SYSTEM_DEVELOPER_GRAPHICS_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Graphics Tools"
    export SYSTEM_DEVELOPER_JAVA_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Java Tools"
    export SYSTEM_DEVELOPER_PERFORMANCE_TOOLS_DIR="/Applications/Xcode.app/Contents/Developer/Applications/Performance Tools"
    export SYSTEM_DEVELOPER_RELEASENOTES_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/releasenotes"
    export SYSTEM_DEVELOPER_TOOLS=/Applications/Xcode.app/Contents/Developer/Tools
    export SYSTEM_DEVELOPER_TOOLS_DOC_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/documentation/DeveloperTools"
    export SYSTEM_DEVELOPER_TOOLS_RELEASENOTES_DIR="/Applications/Xcode.app/Contents/Developer/ADC Reference Library/releasenotes/DeveloperTools"
    export SYSTEM_DEVELOPER_USR_DIR=/Applications/Xcode.app/Contents/Developer/usr
    export SYSTEM_DEVELOPER_UTILITIES_DIR=/Applications/Xcode.app/Contents/Developer/Applications/Utilities
    export SYSTEM_DOCUMENTATION_DIR=/Library/Documentation
    export SYSTEM_KEXT_INSTALL_PATH=/System/Library/Extensions
    export SYSTEM_LIBRARY_DIR=/System/Library
    export TARGETED_DEVICE_FAMILY=1,2
    export TARGETNAME=BitriseSampleWithYMLTests
    export TARGET_BUILD_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator
    export TARGET_NAME=BitriseSampleWithYMLTests
    export TARGET_TEMP_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_FILES_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_FILE_DIR=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates/BitriseSampleWithYML.build/Debug-iphonesimulator/BitriseSampleWithYMLTests.build
    export TEMP_ROOT=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Intermediates
    export TEST_FRAMEWORK_SEARCH_PATHS=" /Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/Library/Frameworks /Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/SDKs/iPhoneSimulator8.4.sdk/Developer/Library/Frameworks"
    export TEST_HOST=/Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYML.app/BitriseSampleWithYML
    export TOOLCHAINS=com.apple.dt.toolchain.iOS8_4
    export TREAT_MISSING_BASELINES_AS_TEST_FAILURES=NO
    export UID=501
    export UNLOCALIZED_RESOURCES_FOLDER_PATH=BitriseSampleWithYMLTests.xctest
    export UNSTRIPPED_PRODUCT=NO
    export USER=awesome-bitrise-user
    export USER_APPS_DIR=/Users/awesome-bitrise-user/Applications
    export USER_LIBRARY_DIR=/Users/awesome-bitrise-user/Library
    export USE_DYNAMIC_NO_PIC=YES
    export USE_HEADERMAP=YES
    export USE_HEADER_SYMLINKS=NO
    export VALIDATE_PRODUCT=NO
    export VALID_ARCHS="i386 x86_64"
    export VERBOSE_PBXCP=NO
    export VERSIONPLIST_PATH=BitriseSampleWithYMLTests.xctest/version.plist
    export VERSION_INFO_BUILDER=awesome-bitrise-user
    export VERSION_INFO_FILE=BitriseSampleWithYMLTests_vers.c
    export VERSION_INFO_STRING="\"@(#)PROGRAM:BitriseSampleWithYMLTests  PROJECT:BitriseSampleWithYML-\""
    export WRAPPER_EXTENSION=xctest
    export WRAPPER_NAME=BitriseSampleWithYMLTests.xctest
    export WRAPPER_SUFFIX=.xctest
    export XCODE_APP_SUPPORT_DIR=/Applications/Xcode.app/Contents/Developer/Library/Xcode
    export XCODE_PRODUCT_BUILD_VERSION=6E35b
    export XCODE_VERSION_ACTUAL=0640
    export XCODE_VERSION_MAJOR=0600
    export XCODE_VERSION_MINOR=0640
    export XPCSERVICES_FOLDER_PATH=BitriseSampleWithYMLTests.xctest/XPCServices
    export YACC=yacc
    export arch=x86_64

    xxxxxxxxxxxxx iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.


    export variant=normal
    /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/swift-stdlib-tool --verbose --copy
Copying libswiftCore.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftCoreGraphics.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftFoundation.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftSecurity.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftUIKit.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftXCTest.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftDispatch.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftObjectiveC.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftCoreImage.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks
Copying libswiftDarwin.dylib from /Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/iphonesimulator to /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest/Frameworks

Touch /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest
    cd /Users/awesome-bitrise-user/develop/bitrise/bitrise-yml-converter-test
    export PATH="/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer/usr/bin:/Applications/Xcode.app/Contents/Developer/usr/bin:/Users/awesome-bitrise-user/.rbenv/shims:/usr/local/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/awesome-bitrise-user/develop/go/bin"
    /usr/bin/touch -c /Users/awesome-bitrise-user/Library/Developer/Xcode/DerivedData/BitriseSampleWithYML-brllnldkzzfqyjeofwhnmfsdyicc/Build/Products/Debug-iphonesimulator/BitriseSampleWithYMLTests.xctest`
)
