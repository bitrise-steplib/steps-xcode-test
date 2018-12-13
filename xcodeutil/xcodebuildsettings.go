package xcodeutil

import (
	"fmt"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-tools/xcode-project/serialized"
	"github.com/bitrise-tools/xcode-project/xcworkspace"
)

func parseShowBuildSettingsOutput(out string) (serialized.Object, error) {
	settings := serialized.Object{}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Build settings") {
			continue
		}

		if strings.HasPrefix(line, "User defaults from command line") {
			continue
		}

		if line == "" {
			continue
		}

		split := strings.Split(line, " = ")

		if len(split) < 2 {
			return nil, fmt.Errorf("unknown build settings: %s", line)
		}

		key := strings.TrimSpace(split[0])
		value := strings.TrimSpace(strings.Join(split[1:], " = "))

		settings[key] = value
	}

	return settings, nil
}

// ShowBuildSettingsForMultipleTargets gets the build settings for a scheme (multiple targets),
// returns the last outputted value for any key
func ShowBuildSettingsForMultipleTargets(projectPath, scheme, configuration string, customOptions ...string) (serialized.Object, error) {
	var args []string
	if xcworkspace.IsWorkspace(projectPath) {
		args = append(args, "-workspace", projectPath)
	} else {
		args = append(args, "-project", projectPath)
	}
	args = append(args, "-scheme", scheme)
	if configuration != "" {
		args = append(args, "-configuration", configuration)
	}

	args = append(args, "-showBuildSettings")
	args = append(args, customOptions...)

	cmd := command.New("xcodebuild", args...)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %s", cmd.PrintableCommandArgs(), err)
	}

	return parseShowBuildSettingsOutput(out)
}
