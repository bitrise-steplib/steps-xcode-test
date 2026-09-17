package step

import (
	"testing"

	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/stretchr/testify/require"
)

func Test_collectTestDiagnosticsValue(t *testing.T) {
	tests := []struct {
		name              string
		condition         exportCondition
		xcodeMajorVersion int64
		additionalOptions []string
		want              string
	}{
		{
			name:              "Xcode 26, never - suppresses xcodebuild's own collection",
			condition:         never,
			xcodeMajorVersion: 26,
			want:              "never",
		},
		{
			name:              "Xcode 27, never - suppresses xcodebuild's own collection",
			condition:         never,
			xcodeMajorVersion: 27,
			want:              "never",
		},
		{
			name:              "Xcode 27, on_failure - keeps xcodebuild's collection",
			condition:         onFailure,
			xcodeMajorVersion: 27,
			want:              "on-failure",
		},
		{
			// xcodebuild has no "always"; its collection is failure-triggered regardless.
			name:              "Xcode 27, always - maps onto on-failure",
			condition:         always,
			xcodeMajorVersion: 27,
			want:              "on-failure",
		},
		{
			// The option does not exist before Xcode 26; passing it would be a usage error.
			name:              "Xcode 25, never - option not passed",
			condition:         never,
			xcodeMajorVersion: 25,
			want:              "",
		},
		{
			name:              "Xcode 15, always - option not passed",
			condition:         always,
			xcodeMajorVersion: 15,
			want:              "",
		},
		{
			// main.go treats an unreadable Xcode version as non-fatal and leaves Major at 0.
			name:              "unknown Xcode version - option not passed",
			condition:         never,
			xcodeMajorVersion: 0,
			want:              "",
		},
		{
			name:              "user set the option explicitly - theirs wins",
			condition:         never,
			xcodeMajorVersion: 27,
			additionalOptions: []string{xcodebuild.CollectTestDiagnosticsFlag, "on-failure"},
			want:              "",
		},
		{
			name:              "user set the option with = syntax - theirs wins",
			condition:         never,
			xcodeMajorVersion: 27,
			additionalOptions: []string{xcodebuild.CollectTestDiagnosticsFlag + "=on-failure"},
			want:              "",
		},
		{
			name:              "unrelated additional options are ignored",
			condition:         never,
			xcodeMajorVersion: 27,
			additionalOptions: []string{"-quiet", "-parallel-testing-enabled", "NO"},
			want:              "never",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectTestDiagnosticsValue(tt.condition, tt.xcodeMajorVersion, tt.additionalOptions)
			require.Equal(t, tt.want, got)
		})
	}
}
