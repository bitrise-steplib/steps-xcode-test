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
The **Scheme** input field must be marked as Shared in Xcode.

### Troubleshooting
If the **Deploy to Bitrise.io** Step is missing from your Workflow, then the **Xcode Test for iOS** Step will not be able to export the test results on the Test Reports page and you won't be able to view them either.
The xcpretty output tool does not support parallel tests.
If parallel tests are enabled in your project, go to the Step's **xcodebuild log formatting** section and set the **Log formatter** input's value to `xcodebuild` or `xcbeautify`.
If the Xcode test fails with the error `Unable to find a destination matching the provided destination specifier`, then check our [system reports](https://stacks.bitrise.io) to see if the requested simulator is on the stack or not.
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

### Examples

Run the default Test Plan or Test Targets associated with a Scheme:
```yaml
- xcode-test:
    inputs:
    - project_path: ./ios-sample/ios-sample.xcodeproj
    - scheme: ios-sample
```


Run tests in a specific Test Plan associated with a Scheme:
```yaml
- xcode-test:
    inputs:
    - project_path: ./ios-sample/ios-sample.xcodeproj
    - scheme: ios-sample
    - test_plan: UITests
```

Use xcbeautify to beautify xcodebuild logs:
```yaml
- xcode-test:
    inputs:
    - project_path: ./ios-sample/ios-sample.xcodeproj
    - scheme: ios-sample
    - log_formatter: xcbeautify
```

Run tests with custom xcconfig content:
```yaml
- xcode-test:
    inputs:
    - project_path: ./ios-sample/ios-sample.xcodeproj
    - scheme: ios-sample
    - xcconfig_content: |
        CODE_SIGN_IDENTITY = Apple Development
```

Run tests with custom xcconfig file path:
```yaml
- xcode-test:
    inputs:
    - project_path: ./ios-sample/ios-sample.xcodeproj
    - scheme: ios-sample
    - xcconfig_content: ./ios-sample/ios-sample/Configurations/Dev.xcconfig
```



## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `project_path` | Xcode Project (`.xcodeproj`) or Workspace (`.xcworkspace`) path. The input value sets xcodebuild's `-project` or `-workspace` option.  If this is a Swift package, this should be the path to the `Package.swift` file. | required | `$BITRISE_PROJECT_PATH` |
| `scheme` | Xcode Scheme name.  The input value sets xcodebuild's `-scheme` option. | required | `$BITRISE_SCHEME` |
| `destination` | Destination specifier describes the device to use as a destination.  The input value sets xcodebuild's `-destination` option.  In a CI environment, a Simulator device called `Bitrise iOS default` is already created. It is a compatible device with the selected Simulator runtime, pre-warmed for better performance.  If a device with this name is not found (e.g. in a local dev environment), the first matching device will be selected. | required | `platform=iOS Simulator,name=Bitrise iOS default,OS=latest` |
| `test_plan` | Run tests in a specific Test Plan associated with the Scheme.  Leave this input empty to run the default Test Plan or Test Targets associated with the Scheme.  The input value sets xcodebuild's `-testPlan` option. |  |  |
| `test_repetition_mode` | Determines how the tests will repeat.  Available options: - `none`: Tests will never repeat. - `until_failure`: Tests will repeat until failure or up to maximum repetitions. - `retry_on_failure`: Only failed tests will repeat up to maximum repetitions. - `up_until_maximum_repetitions`: Tests will repeat up until maximum repetitions.  The input value together with Maximum Test Repetitions (`maximum_test_repetitions`) input sets xcodebuild's `-run-tests-until-failure` / `-retry-tests-on-failure` or `-test-iterations` option. |  | `retry_on_failure` |
| `maximum_test_repetitions` | The maximum number of times a test repeats based on the Test Repetition Mode (`test_repetition_mode`).  Should be more than 1 if the Test Repetition Mode is other than `none`.  The input value sets xcodebuild's `-test-iterations` option. | required | `3` |
| `relaunch_tests_for_each_repetition` | If this input is set, tests will launch in a new process for each repetition.  By default, tests launch in the same process for each repetition.  The input value sets xcodebuild's `-test-repetition-relaunch-enabled` option. |  | `no` |
| `xcconfig_content` | Build settings to override the project's build settings, using xcodebuild's `-xcconfig` option.  You can't define `-xcconfig` option in `Additional options for the xcodebuild command` if this input is set.  If empty, no setting is changed. When set it can be either: 1.  Existing `.xcconfig` file path.      Example:      `./ios-sample/ios-sample/Configurations/Dev.xcconfig`  2.  The contents of a newly created temporary `.xcconfig` file. (This is the default.)      Build settings must be separated by newline character (`\n`).      Example:     ```     COMPILER_INDEX_STORE_ENABLE = NO     ONLY_ACTIVE_ARCH[config=Debug][sdk=*][arch=*] = YES     ``` |  | `COMPILER_INDEX_STORE_ENABLE = NO` |
| `perform_clean_action` | If this input is set, `clean` xcodebuild action will be performed besides the `test` action. | required | `no` |
| `xcodebuild_options` | Additional options to be added to the executed xcodebuild command.  Prefer using `Build settings (xcconfig)` input for specifying `-xcconfig` option. You can't use both. |  |  |
| `log_formatter` | Defines how xcodebuild command's log is formatted.  Available options: - `xcbeautify`: The xcodebuild command's output will be beautified by xcbeautify. - `xcodebuild`: Only the last 20 lines of raw xcodebuild output will be visible in the build log. - `xcpretty`: The xcodebuild command's output will be prettified by xcpretty.  The raw xcodebuild log will be exported in all cases. | required | `xcbeautify` |
| `xcbeautify_options` | Additional options to be added to the executed xcbeautify command. |  |  |
| `xcpretty_options` | Additional options to be added to the executed xcpretty command. |  | `--color --report html --output "${BITRISE_DEPLOY_DIR}/xcode-test-results-${BITRISE_SCHEME}.html"` |
| `cache_level` | Defines what cache content should be automatically collected. Use key-based caching instead for better performance.  Available options: - `none`: Disable collecting cache content. - `swift_packages`: Collect Swift PM packages added to the Xcode project.  With key-based caching, you only need the Restore SPM cache and the Save SPM cache Steps to cache your Swift packages. [See devcenter for more information.](https://devcenter.bitrise.io/en/dependencies-and-caching/managing-dependencies-for-ios-apps/managing-dependencies-with-spm.html#caching-swift-packages) |  | `none` |
| `verbose_log` | If this input is set, the Step will print additional logs for debugging. |  | `no` |
| `collect_simulator_diagnostics` | If this input is set, the simulator verbose logging will be enabled and the simulator diagnostics log will be exported. |  | `never` |
| `headless_mode` | In headless mode the simulator is not launched in the foreground.  If this input is set, the simulator will not be visible but tests (even the screenshots) will run just like if you run a simulator in foreground. |  | `yes` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_XCODE_TEST_RESULT` | Result of the tests. 'succeeded' or 'failed'. |
| `BITRISE_XCRESULT_PATH` | The path of the generated `.xcresult`. |
| `BITRISE_XCRESULT_ZIP_PATH` | The path of the zipped `.xcresult`. |
| `BITRISE_XCODE_TEST_ATTACHMENTS_PATH` | This is the path of the test attachments zip. |
| `BITRISE_XCODEBUILD_BUILD_LOG_PATH` | If `single_build` is set to false, the step runs `xcodebuild build` before the test, and exports the raw xcodebuild log. |
| `BITRISE_XCODEBUILD_TEST_LOG_PATH` | The step exports the `xcodebuild test` command output log. |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-xcode-test/pulls) and [issues](https://github.com/bitrise-steplib/steps-xcode-test/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
