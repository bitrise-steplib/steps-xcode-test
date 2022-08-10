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