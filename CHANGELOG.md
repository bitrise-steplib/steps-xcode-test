## Changelog (Current version: 1.18.3)

-----------------

### 1.18.3 (2017 Apr 13)

* [0c1c65e] Prepare for 1.18.3
* [b7ef8d0] Added retry pattern (#72)

### 1.18.2 (2017 Apr 04)

* [de72c93] Prepare for 1.18.2
* [92551b8] New retry pattern added (#71)

### 1.18.1 (2016 Nov 29)

* [8160d64] prepare for 1.18.1
* [583b25d] app accessibility is not loaded retry pattern (#69)

### 1.18.0 (2016 Nov 18)

* [fc01cfd] prepare for 1.18.0
* [da74fd4] Xcpretty fix (#68)
* [77f3e9f] CHANGELOG fix

### 1.17.2 (2016 Nov 15)

* [b923f4c] v1.17.2
* [8c6fabf] prep for v1.17.2
* [f297b54] Feature/delete xcpretty output (#67)
* [7622df6] Merge pull request #66 from bitrise-io/feature/retry-pattern-for-app-state-not-running
* [bb28f78] new retry pattern for "app state is still not running"

### 1.17.1 (2016 Oct 11)

* [5b076ca] prepare for 1.17.1
* [f562a05] step.yml update (#65)

### 1.17.0 (2016 Sep 29)

* [fba03e9] prepare for 1.17.0
* [e781ab7] xcpretty test additional options (#64)

### 1.16.1 (2016 Sep 28)

* [b848fde] prepare for 1.16.1
* [2e6d684] new retry patterns (#62)

### 1.16.0 (2016 Sep 23)

* [7483ba9] prepare for 1.16.0
* [f2ac5fd] automatic retry, typo fix (#61)
* [ef9f1c1] Xcodebuild output (#60)

### 1.15.0 (2016 Sep 19)

* [8bf5469] prepare for 1.15.0
* [0927fbf] Retry (#59)
* [f13e07a] toolkit support (#58)

### 1.14.1 (2016 Sep 16)

* [3ad45ff] prepare for 1.14.1
* [11d8908] Offer opt-out for redundant build before performing test (#56)

### 1.14.0 (2016 Jul 26)

* [ec8fde5] prep for v1.14.0
* [182aff8] Merge pull request #49 from bitrise-io/feature/multi-xcode-bug-fix
* [2a4723e] BootSimulator - multi xcode bug fix
* [c1c8e2f] run xcodebuild with "NSUnbufferedIO=YES" env
* [8cba6c9] GOPATH for test workflow
* [a39ef2f] bitrise.yml - releaseman revision
* [bdbcdeb] step.yml revision

### 1.13.8 (2016 Jul 12)

* [929a4de] prepare for 1.13.8
* [2372437] Merge pull request #48 from bitrise-io/viktorbenei-patch-1
* [165f06f] typo

### 1.13.7 (2016 Apr 12)

* [5ee62fb] prepare for release
* [4ee45ae] Merge pull request #46 from tiagomartinho/master
* [2712e31] remove duplicated and deprecated dependencies step

### 1.13.6 (2016 Apr 08)

* [b18f937] prepare for relase
* [329002c] Merge pull request #44 from bitrise-io/step_yml_updates
* [5f065f2] single build
* [d18ad7d] step.yml updates
* [440e737] bump version

### 1.13.5 (2016 Apr 04)

* [714434f] Merge pull request #43 from bitrise-io/derived_data_fix
* [557d48f] filepath instead of path package, renamed attachments
* [c03c9db] removed changing derived data dir

### 1.13.4 (2016 Mar 31)

* [0b1b778] prepare for release
* [0093472] Merge pull request #42 from bitrise-io/default_device
* [b9b6e5d] change default device

### 1.13.3 (2016 Mar 21)

* [3e3b5a7] prepare for release
* [be2a15a] Merge pull request #38 from bitrise-io/latest_version_fix
* [f9f845c] latest os version fix

### 1.13.2 (2016 Mar 18)

* [f5a96cd] prepare fro new version
* [2c80839] Merge pull request #37 from bitrise-io/sim_boot_fix
* [336efce] simulator boot fix

### 1.13.1 (2016 Mar 17)

* [2bcef31] version bump
* [d9bff12] Merge pull request #36 from bitrise-io/deprecated_ipad
* [385d31d] GOPATH fix
* [5d5dc26] deprecated iPad device handling

### 1.13.0 (2016 Mar 16)

* [de50ce2] version bump
* [555a6b5] Merge pull request #35 from bitrise-io/start_simulator
* [80acbd6] ensure clean git
* [936bde2] go test fix
* [9789fd3] review
* [8d0f5f4] boot simulator before run test
* [b654793] xcodebuild additional options

### 1.12.1 (2016 Mar 02)

* [a98ffd1] release config added
* [1c4afc0] Merge pull request #33 from bitrise-io/develop
* [2d960eb] bitrise.yml updates
* [590db52] build use the same derived data dir

### 1.12.0 (2016 Feb 12)

* [c3e6b5e] Merge pull request #31 from godrei/screenshots
* [b6348ba] export_uitest_artifacts description fix
* [00567e6] save attchments, export raw log as zip if build fails

### 1.11.0 (2016 Jan 25)

* [ca4c8d9] is_clean_build is now set to "no" by default
* [bcc2548] share: v1.10.0

### 1.10.0 (2016 Jan 09)

* [1bbccbe] new output : saves the raw output into a temp file, for other steps/tools to use (https://github.com/bitrise-io/steps-xcode-test/issues/29)

### 1.9.0 (2015 Dec 30)

* [0d088ba] Sharing : STEP_GIT_VERION_TAG_TO_SHARE: 1.9.0
* [5677964] README - removed the known issues section
* [193ec17] step.yml revisions : is_expand:true removed as it's unnecessary; platform and device are marked as required
* [cab47e8] NEW OPTION: simulator platform - can now be set to tvOS, to run tvOS app unit tests
* [7f05479] Merge pull request #27 from godrei/log_improvements
* [bfe594c] pretty log fix
* [02fe105] Merge pull request #26 from godrei/typo_fix
* [62d1ee6] typo fix

### 1.8.0 (2015 Dec 05)

* [02a457a] STEP_GIT_VERION_TAG_TO_SHARE: 1.8.0
* [25b024f] output, piping & buffers revision + an important #fix : always return the raw xcodebuild output for processing, for other functions (ex: searching for timeout error in the log)!!
* [b0d17b2] var name fix
* [f68ec99] testing and newline after config prints
* [d03e9e1] config printing log revision : to make it similar to Xcode Archive & Xcode Analyze's log
* [e276191] cleanup : removed the previous "build summary" related codes; this is now replaced with `xcpretty`'s HTML report
* [dcf1abe] Merge branch 'master' of github.com:bitrise-io/steps-xcode-test
* [731bfe1] bitrise.yml
* [0753c4b] Merge pull request #25 from godrei/xcpretty
* [7186e89] print config
* [673b4a2] Merge pull request #24 from godrei/xcpretty
* [c30072a] missing xcpretty hint
* [a829f37] xcpretty

### 1.7.0 (2015 Nov 09)

* [5853f3d] `share-this-step` workflow added
* [4c382d1] output `BITRISE_XCODE_TEST_FULL_RESULTS_TEXT` replaced with saving the Summary into a file instead, as the Test Summary might be larger than what common tools can process as an Environment Variable

### 1.6.0 (2015 Oct 28)

* [d1d5f53] bitrise.yml to test generate_code_coverate_files
* [a21756d] Merge pull request #21 from bitrise-io/feature/code_coverage_support
* [83ecd43] code coverage support
* [5129c48] Merge pull request #19 from gkiki90/deps
* [b6087eb] new deps

### 1.5.0 (2015 Sep 30)

* [62f5aab] Merge pull request #18 from gkiki90/master
* [784dd4a] combined output
* [3265b87] code cleaning
* [17ba418] Merge pull request #17 from gkiki90/master
* [8150283] removed -sdk iphonesimulator flag
* [7f7fb2a] Merge pull request #16 from gkiki90/master
* [b30c3b6] fixes
* [30f4960] refactor
* [88eb465] PR fix
* [8768be5] major version
* [7b144b2] step fix

### 1.4.2 (2015 Sep 29)

* [73f2e16] test summary delimiter search revision: try to find the first occurrence of any of the listed delimiters, as these might occur in varying order in Xcode's output

### 1.4.1 (2015 Sep 29)

* [fa9a949] added code comment as well about the 'build' arg
* [73bc841] better formatted "Full command" in the logs, which can be copy pasted into Terminal to run it
* [578d2dc] Merge branch 'master' of github.com:bitrise-io/steps-xcode-test
* [d495ef0] default simulator device: iPhone 6, to better match the default tests run from Xcode
* [2f52374] Merge pull request #15 from bazscsa/patch-1

### 1.4.0 (2015 Sep 28)

* [fa85652] new fix for `iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.` - add "build" command before "test". Without this the compilation time is added to the simulator boot time, so, for a larger project where the compilation itself takes about or more than 120 seconds it always triggers a simulator timeout!
* [d5ac9bf] clean build : is now a bool parameter & a retry is always started without a clean build
* [f95d07b] Update step.yml
* [2faf735] bug fix : if retry fails the returned error should be the one generated by `cmd.Run()`, NOT the one generated by `isStringFoundInOutput` !

### 1.3.0 (2015 Sep 21)

* [2d2a59b] Merge pull request #11 from bitrise-io/only-summary-output
* [c12eb5d] delim logging fix
* [15109fe] searching for an exporting test result summary (also exporting a success/error env) & testing revision
* [04f9ed0] declared the two new output & a new input (is_full_output) + debug/test printing in `bitrise.yml`
* [5a2984b] build log samples moved into files
* [bf80330] printing the version of `xcodebuild` & a bit of note in `bitrise.yml` why we change the workdir

### 1.2.0 (2015 Sep 18)

* [1b94c32] Merge pull request #9 from bitrise-io/xcode-7-ui-test-timout
* [2e43971] better, restructured tests, code comments, and removed the previous `isTimeOutError` function, in favor of the new generic `isStringFoundInOutput`
* [2c75ca4] first attempt to fix with a retry

### 1.1.2 (2015 Sep 16)

* [aa054d7] Merge pull request #6 from gkiki90/fix
* [f8fbc4f] retry test fix

### 1.1.1 (2015 Sep 15)

* [f477acc] added `go` dependency to `step.yml`

### 1.1.0 (2015 Sep 15)

* [d4dc102] Merge pull request #5 from gkiki90/fix
* [cb497e8] timeout fix

### 1.0.0 (2015 Sep 09)

* [49ca8fc] Merge pull request #3 from gkiki90/project_path
* [6872dd7] fix
* [33f1d2f] Update README.md
* [1a949b9] Merge pull request #2 from gkiki90/master
* [9c3b8e9] website fix
* [dfdc4d1] readme update

### 0.9.3 (2015 Aug 18)

* [ad299e7] is_clean_build input, to enable/disable full clean build before running the tests

### 0.9.2 (2015 Aug 17)

* [0d9ddce] updated to proper V2 step.yml format

-----------------

Updated: 2017 Apr 13