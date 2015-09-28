# Run Xcode Test step

The new Run Xcode Test step.

## How to use this Step

Can be run directly with the [bitrise CLI](https://github.com/bitrise-io/bitrise),
just `git clone` this repository, `cd` into it's folder in your Terminal/Command Line
and call `bitrise run test`.

*Check the `bitrise.yml` file for required inputs which have to be
added to your `.bitrise.secrets.yml` file!*


## Known issues with running Xcode tests from Command Line / Terminal

* `iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.`
    * possible solutions: retrying the test, without a new clean build (if a clean build was set/enabled)
    * to include `build` in the command before `test`, as reported here: https://openradar.appspot.com/22413115 and demonstrated here: https://github.com/bitrise-io/simulator-launch-timeout-includes-build-time
