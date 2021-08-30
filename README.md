# Xcode Test for iOS

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-xcode-test?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-xcode-test/releases)

Runs your project's pre-defined Xcode tests on every build.

<details>
<summary>Description</summary>

This Steps runs all those Xcode tests that are included in your project. 
The Step will work out of the box if your project has test targets and your Workflow has the **Deploy to Bitrise.io** Step which exports the test results and (code coverage files if needed) to the Test Reports page. 
This Step does not need any code signing files since the Step deploys only the test results to [bitrise.io](https://www.bitrise.io).

### Configuring the Step
If you click into the Step, there are some required input fields whose input must be set in accordance with the Xcode configuration of the project.  
The **Scheme name** input field must be marked as Shared in Xcode. 
The **Device**, **OS version**, **Platform** input fields must be set to the value that's shown in Xcode’s device selection dropdown menu.
If you wish to export code coverage files as well, set the "Generate code coverage files?" to `yes`.

### Troubleshooting
If the **Deploy to Bitrise.io** Step is missing from your Workflow, then the **Xcode Test for iOS** Step will not be able to export the test results on the Test Reports page and you won't be able to view them either.
The xcpretty output tool does not support parallel tests. 
If parallel tests are enabled in your project, go to the Step’s Debug section and set the **Output tool** input’s value to xcodebuild.
If the Xcode test fails with the error `Unable to find a destination matching the provided destination specifier`, then check our [system reports](https://github.com/bitrise-io/bitrise.io/tree/master/system_reports) to see if the requested simulator is on the stack or not. 
If it is not, then pick a simulator that is on the stack.

### Useful links
- [About Test Reports](https://devcenter.bitrise.io/testing/test-reports/)
- [Running Xcode Tests for iOS](https://devcenter.bitrise.io/testing/running-xcode-tests/)

### Related Steps
- [Deploy to Bitrise.io](https://www.bitrise.io/integrations/steps/deploy-to-bitrise-io)
- [iOS Device Testing](https://www.bitrise.io/integrations/steps/virtual-device-testing-for-ios)
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `project_path` | A `.xcodeproj` or `.xcworkspace` path. | required | `$BITRISE_PROJECT_PATH` |
| `scheme` | The Scheme to use. | required | `$BITRISE_SCHEME` |
| `test_plan` | Run tests in a specific Test Plan associated with the Scheme.  Leave this input empty to run the default Test Plan or Test Targets associated with the Scheme. |  |  |
| `simulator_device` | Set it as it is shown in Xcode's device selection dropdown UI.  A couple of examples (the actual available options depend on which versions are installed):  * iPhone 8 Plus * iPhone Xs Max * iPad Air (3rd generation) * iPad Pro (12.9-inch) (3rd generation) * Apple TV 4K (don't forget to set the platform to `tvOS Simulator` to use this option!) | required | `iPhone 8 Plus` |
| `simulator_os_version` | Set it as it is shown in Xcode's device selection dropdown UI.  A couple of format examples (the actual available options depend on which versions are installed):  * 8.4 * latest | required | `latest` |
| `simulator_platform` | Set it as it is shown in Xcode's device selection dropdown UI.  A couple of examples (the actual available options depend on which versions are installed):  * iOS Simulator * tvOS Simulator | required | `iOS Simulator` |
| `export_uitest_artifacts` | If enabled, the attachments of the UITest will be exported into the `BITRISE_DEPLOY_DIR`, as a compressed ZIP file. Attachments include screenshots taken during the UI test, and other artifacts.  __NOTE:__ works only with Xcode version < 11. |  | `false` |
| `generate_code_coverage_files` | In case of `generate_code_coverage_files: "yes"` `xcodebuild` gets two additional flags:  * GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES * GCC_GENERATE_TEST_COVERAGE_FILES=YES | required | `no` |
| `disable_index_while_building` | Could make the build faster by adding `COMPILER_INDEX_STORE_ENABLE=NO` flag to the `xcodebuild` command which will disable the indexing during the build.  Indexing is needed for  * Autocomplete * Ability to quickly jump to definition * Get class and method help by alt clicking.  Which are not needed in CI environment.  **Note:** In Xcode you can turn off the `Index-WhileBuilding` feature  by disabling the `Enable Index-WhileBuilding Functionality` in the `Build Settings`.<br/> In CI environment you can disable it by adding `COMPILER_INDEX_STORE_ENABLE=NO` flag to the `xcodebuild` command. | required | `yes` |
| `test_repetition_mode` | Determines how the tests will repeat.  Available options: - `none`: Tests will never repeat. - `until_failure`: Tests will repeat until failure or up to maximum repetitions. - `retry_on_failure`: Only failed tests will repeat up to maximum repetitions. - `up_until_maximum_repetitions`: Tests will repeat up until maximum repetitions. |  | `none` |
| `maximum_test_repetitions` | The maximum number of times a test will repeat based on Test Repetition Mode.  Should be more than 1 if the Test Repetition Mode (`test_repetition_mode`) is other than `none`. | required | `3` |
| `relaunch_tests_for_each_repetition` | If enabled, tests will launch in a new process for each repetition.  By default, tests launch in the same process for each repetition. |  | `no` |
| `verbose` | You can enable the verbose log for easier debugging. |  | `no` |
| `headless_mode` | If you run your tests in headless mode the xcodebuild will start a simulator in a background. In headless mode the simulator will not be visible but your tests (even the screenshots) will run just like if you run a simulator in foreground.  **NOTE:** Headless mode is available with Xcode 9.x or newer. |  | `yes` |
| `is_clean_build` |  | required | `no` |
| `output_tool` | If set to `xcpretty`, the xcodebuild output will be prettified by xcpretty.   If set to `xcodebuild`, only the last 20 lines of raw xcodebuild output will be visible in the build log. The build log will always be added as an artifact. | required | `xcpretty` |
| `xcodebuild_test_options` | Options added to the end of the `xcodebuild build test` call.  If you leave empty this input, xcodebuild will be called as:  `xcodebuild   -project\-workspace PROJECT.xcodeproj\WORKSPACE.xcworkspace   -scheme SCHEME   build test   -destination platform=PLATFORM Simulator,name=NAME,OS=VERSION`  In case of `generate_code_coverage_files: "yes"` `xcodebuild` gets two additional flags:  * GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES * GCC_GENERATE_TEST_COVERAGE_FILES=YES  If you want to add more options, list that separated by space character. Example: `-xcconfig PATH -verbose` |  |  |
| `single_build` | If `single_build` is set to false, the Step runs `xcodebuild OPTIONS build OPTIONS` before the test to generate the project derived data. This is followed by `xcodebuild OPTIONS build test OPTIONS`. This command's log is presented in the Step's log.  If `single_build` is set to true, then the Step calls only `xcodebuild OPTIONS build test OPTIONS`. |  | `true` |
| `should_build_before_test` | Previous Xcode versions and configurations may throw the error `iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.` when the compilation before performing the tests takes too long.  This is fixed by running `xcodebuild OPTIONS build test OPTIONS` instead of `xcodebuild OPTIONS test OPTIONS`. Calling an explicit build before the test results in the code being compiled twice, thus creating an overhead.  Unless you are sure that your configuration is not prone to this error, it is recommended to leave this option turned on. | required | `yes` |
| `should_retry_test_on_fail` | If you set this input to `yes`, the Step will rerun ALL your tests once in the case of failed test/s. Note that ALL your tests will be rerun, not just the ones that failed.  This input is not available if you are using Xcode 13+. In that case, we recommend using the `retry_on_failure` Test Repetition Mode (test_repetition_mode). | required | `no` |
| `xcpretty_test_options` | Options added to the end of the `xcpretty` test call.  If you leave empty this input, xcpretty will be called as:  `set -o pipefail && XCODEBUILD_TEST_COMMAND \| xcpretty`  In case of leaving this input on default value:  `set -o pipefail && XCODEBUILD_TEST_COMMAND \| xcpretty --color --report html --output "${BITRISE_DEPLOY_DIR}/xcode-test-results-${BITRISE_SCHEME}.html"  If you want to add more options, list that separated by space character. |  | `--color --report html --output "${BITRISE_DEPLOY_DIR}/xcode-test-results-${BITRISE_SCHEME}.html"` |
| `cache_level` | Available options: - `none` : Disable caching - `swift_packages` : Cache Swift PM packages added to the Xcode project |  | `swift_packages` |
| `collect_simulator_diagnostics` |  |  | `never` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_XCODE_TEST_RESULT` |  |
| `BITRISE_XCRESULT_PATH` | The path of the generated `.xcresult`. |
| `BITRISE_XCRESULT_ZIP_PATH` | The path of the zipped `.xcresult`. |
| `BITRISE_XCODE_TEST_ATTACHMENTS_PATH` | This is the path of the test attachments zip. |
| `BITRISE_XCODEBUILD_BUILD_LOG_PATH` | If `single_build` is set to false, the step runs `xcodebuild build` before the test,   and exports the raw xcodebuild log. |
| `BITRISE_XCODEBUILD_TEST_LOG_PATH` | The step exports the `xcodebuild test` command output log. |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-xcode-test/pulls) and [issues](https://github.com/bitrise-steplib/steps-xcode-test/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
