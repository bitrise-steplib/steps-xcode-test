package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/pretty"
	"github.com/bitrise-steplib/steps-xcode-test/xcodeutil/testsummaries"
)

func Test_createRenamePlan(t *testing.T) {
	const attachmentDir = "/tmp/test"
	const testID = "project/testSuccess()"
	const activityTitle = "Start Test"
	const activityUUID = "CE23D189-E75A-437D-A4B5-B97F1658FC98"
	const fileName = "Screenshot of main screen (ID 1)_1_A07C26DB-8E1E-46ED-90F2-981438BE0BBA.png"
	timeStamp := time.Time{}
	type args struct {
		testResults   []testsummaries.TestResult
		attachmentDir string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Empty",
			args: args{
				testResults: []testsummaries.TestResult{
					{},
				},
				attachmentDir: attachmentDir,
			},
			want: map[string]string{},
		},
		{
			name: "Test result with screenshots",
			args: args{
				testResults: []testsummaries.TestResult{{
					ID:          testID,
					Status:      "Success",
					FailureInfo: nil,
					Activities: []testsummaries.Activity{{
						Title: activityTitle,
						UUID:  activityUUID,
						Screenshots: []testsummaries.Screenshot{{
							FileName:    fileName,
							TimeCreated: timeStamp,
						}},
					}},
				}},
				attachmentDir: attachmentDir,
			},
			want: map[string]string{
				filepath.Join(attachmentDir, fileName): filepath.Join(attachmentDir, "project-testSuccess()_0001-01-01_12-00-00_Start Test_CE23D189-E75A-437D-A4B5-B97F1658FC98.png"),
			},
		},
		{
			name: "Failing test result with screenshots",
			args: args{
				testResults: []testsummaries.TestResult{{
					ID:          testID,
					Status:      "Failure",
					FailureInfo: nil,
					Activities: []testsummaries.Activity{{
						Title: activityTitle,
						UUID:  activityUUID,
						Screenshots: []testsummaries.Screenshot{{
							FileName:    fileName,
							TimeCreated: timeStamp,
						}},
					}},
				}},
				attachmentDir: attachmentDir,
			},
			want: map[string]string{
				filepath.Join(attachmentDir, fileName): filepath.Join(attachmentDir, "Failures", "project-testSuccess()_0001-01-01_12-00-00_Start Test_CE23D189-E75A-437D-A4B5-B97F1658FC98.png"),
			},
		},
		{
			name: "Test result with subactivity screenshot",
			args: args{
				testResults: []testsummaries.TestResult{{
					ID:          testID,
					Status:      "Success",
					FailureInfo: nil,
					Activities: []testsummaries.Activity{{
						Title:       "Launch",
						UUID:        "uuid",
						Screenshots: nil,
						SubActivities: []testsummaries.Activity{{
							Title: activityTitle,
							UUID:  activityUUID,
							Screenshots: []testsummaries.Screenshot{{
								FileName:    fileName,
								TimeCreated: timeStamp,
							}},
							SubActivities: nil,
						}},
					}},
				}},
				attachmentDir: attachmentDir,
			},
			want: map[string]string{
				filepath.Join(attachmentDir, fileName): filepath.Join(attachmentDir, "project-testSuccess()_0001-01-01_12-00-00_Start Test_CE23D189-E75A-437D-A4B5-B97F1658FC98.png"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createRenamePlan(tt.args.testResults, tt.args.attachmentDir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createRenamePlan() = %v, want %v", pretty.Object(got), pretty.Object(tt.want))
			}
		})
	}
}
