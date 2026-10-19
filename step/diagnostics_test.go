package step

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_xcodebuildDiagnosticsOverride(t *testing.T) {
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
			// project_setting leaves the decision to the test plan, so nothing is passed.
			name:              "Xcode 27, project_setting - option not passed",
			condition:         projectSetting,
			xcodeMajorVersion: 27,
			want:              "",
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
			additionalOptions: []string{"-collect-test-diagnostics", "on-failure"},
			want:              "",
		},
		{
			// xcodebuild silently drops the -option=value form, so it must not count as an override.
			name:              "user wrote the option with = syntax - Step still passes its own",
			condition:         never,
			xcodeMajorVersion: 27,
			additionalOptions: []string{"-collect-test-diagnostics=on-failure"},
			want:              "never",
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
			got := xcodebuildDiagnosticsOverride(tt.condition, tt.xcodeMajorVersion, tt.additionalOptions)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_xcodebuildCollectsDiagnostics(t *testing.T) {
	tests := []struct {
		name              string
		condition         exportCondition
		testFailed        bool
		xcodeMajorVersion int64
		additionalOptions []string
		want              bool
	}{
		{name: "Xcode 27, on_failure, failed", condition: onFailure, testFailed: true, xcodeMajorVersion: 27, want: true},
		{name: "Xcode 27, always, failed", condition: always, testFailed: true, xcodeMajorVersion: 27, want: true},
		{name: "Xcode 27, project_setting, failed", condition: projectSetting, testFailed: true, xcodeMajorVersion: 27, want: true},
		{name: "Xcode 27, never, failed", condition: never, testFailed: true, xcodeMajorVersion: 27, want: false},
		{name: "Xcode 27, on_failure, passed", condition: onFailure, testFailed: false, xcodeMajorVersion: 27, want: false},
		// xcodebuild has no collection of its own before Xcode 26.
		{name: "Xcode 16, on_failure, failed", condition: onFailure, testFailed: true, xcodeMajorVersion: 16, want: false},
		{name: "Xcode 16, project_setting, failed", condition: projectSetting, testFailed: true, xcodeMajorVersion: 16, want: false},
		// A -collect-test-diagnostics option in xcodebuild_options wins over the input.
		{name: "Xcode 27, on_failure, failed, user never", condition: onFailure, testFailed: true, xcodeMajorVersion: 27, additionalOptions: []string{"-collect-test-diagnostics", "never"}, want: false},
		{name: "Xcode 27, never, failed, user on-failure", condition: never, testFailed: true, xcodeMajorVersion: 27, additionalOptions: []string{"-verbose", "-collect-test-diagnostics", "on-failure"}, want: true},
		{name: "Xcode 27, never, passed, user on-failure", condition: never, testFailed: false, xcodeMajorVersion: 27, additionalOptions: []string{"-collect-test-diagnostics", "on-failure"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xcodebuildCollectsDiagnostics(tt.condition, tt.testFailed, tt.xcodeMajorVersion, tt.additionalOptions)
			require.Equal(t, tt.want, got)
		})
	}
}
