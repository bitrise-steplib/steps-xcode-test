package testsummaries

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bitrise-steplib/steps-xcode-test/pretty"
	"github.com/bitrise-io/go-xcode/plistutil"
)

func TestTimestampToTime(t *testing.T) {
	time, err := TimestampStrToTime("522675441.31045401")
	wantErr := false
	if (err != nil) != wantErr {
		t.Errorf("TimestampStrToTime() wantErr: %v, got: %v", wantErr, err)
	}
	want := []int{
		2017,
		7,
		25,
		11,
		37,
		21,
	}
	got := []int{
		time.Year(),
		int(time.Month()),
		time.Day(),
		time.Hour(),
		time.Minute(),
		time.Second(),
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("TimestampStrToTime() want: %v, got: %v", want, got)
	}
}

func Test_parseTestSummaries(t *testing.T) {
	const testID = "ios_simple_objcTests/testExample"
	const testStatus = "Success"
	type args struct {
		testSummariesContent plistutil.PlistData
	}
	tests := []struct {
		name    string
		args    args
		want    []TestResult
		wantErr bool
	}{
		{
			name: "Simple, single test result",
			args: args{
				plistutil.PlistData{
					"TestableSummaries": []interface{}{
						map[string]interface{}{
							"Tests": []interface{}{
								map[string]interface{}{
									"Subtests": []interface{}{
										map[string]interface{}{
											"TestIdentifier": testID,
											"TestStatus":     testStatus,
										},
									},
								},
							},
						},
					},
				},
			},
			want: []TestResult{
				{
					ID:          testID,
					Status:      testStatus,
					FailureInfo: nil,
					Activities:  make([]Activity, 0),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestSummaries(tt.args.testSummariesContent)
			// t.Logf(pretty.Object(got))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTestSummaries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTestSummaries() = %v, want %v", pretty.Object(got), pretty.Object(tt.want))
			}
		})
	}
}

func Test_parseFailureSummaries(t *testing.T) {
	type args struct {
		failureSummariesData []plistutil.PlistData
	}
	tests := []struct {
		name    string
		args    args
		want    []FailureSummary
		wantErr bool
	}{
		{
			name: "Ok case",
			args: args{[]plistutil.PlistData{{
				"FileName":           "/tmp/ios_simple_objcUITests.m",
				"LineNumber":         uint64(64),
				"Message":            "((NO) is true) failed",
				"PerformanceFailure": false,
			}}},
			want: []FailureSummary{{
				FileName:             "/tmp/ios_simple_objcUITests.m",
				LineNumber:           64,
				Message:              "((NO) is true) failed",
				IsPerformanceFailure: false,
			}},
			wantErr: false,
		},
		{
			name: "Key FileName not found",
			args: args{[]plistutil.PlistData{{
				"LineNumber":         uint64(64),
				"Message":            "((NO) is true) failed",
				"PerformanceFailure": false,
			}}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Key LineNumber not found",
			args: args{[]plistutil.PlistData{{
				"FileName":           "/tmp/ios_simple_objcUITests.m",
				"Message":            "((NO) is true) failed",
				"PerformanceFailure": false,
			}}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Key Message not found",
			args: args{[]plistutil.PlistData{{
				"FileName":           "/tmp/ios_simple_objcUITests.m",
				"LineNumber":         uint64(64),
				"PerformanceFailure": false,
			}}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Key PerformanceFailure not found",
			args: args{[]plistutil.PlistData{{
				"FileName":   "/tmp/ios_simple_objcUITests.m",
				"LineNumber": uint64(64),
				"Message":    "((NO) is true) failed",
			}}},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFailureSummaries(tt.args.failureSummariesData)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFailureSummaries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseFailureSummaries() = %v, want %v", pretty.Object(got), pretty.Object(tt.want))
			}
		})
	}
}

func Test_collectLastSubtests(t *testing.T) {
	type args struct {
		testsItem plistutil.PlistData
	}
	tests := []struct {
		name    string
		args    args
		want    []plistutil.PlistData
		wantErr bool
	}{
		{
			name: "Simple case",
			args: args{
				map[string]interface{}{
					"1": "",
					"Subtests": []interface{}{
						map[string]interface{}{
							"2": "",
							"Subtests": []interface{}{
								map[string]interface{}{
									"3": "",
								}},
						}},
				},
			},
			want: []plistutil.PlistData{map[string]interface{}{
				"3": "",
			}},
			wantErr: false,
		},
		{
			name: "Multiple levels",
			args: args{
				map[string]interface{}{
					"1": "",
					"Subtests": []interface{}{
						map[string]interface{}{
							"2": "",
							"Subtests": []interface{}{
								map[string]interface{}{
									"3": "",
								},
							},
						},
						map[string]interface{}{
							"4": "",
						},
					},
				},
			},
			want: []plistutil.PlistData{
				{
					"3": "",
				},
				{
					"4": "",
				}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectLastSubtests(tt.args.testsItem)
			if (err != nil) != tt.wantErr {
				t.Errorf("collectLastSubtests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectLastSubtests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseActivites(t *testing.T) {
	type args struct {
		activitySummariesData []plistutil.PlistData
	}
	tests := []struct {
		name    string
		args    args
		want    []Activity
		wantErr bool
	}{
		{
			name: "Simple case",
			args: args{[]plistutil.PlistData{{
				"Title":             "Start Test",
				"UUID":              "CE23D189-E75A-437D-A4B5-B97F1658FC98",
				"StartTimeInterval": 568123776.87169898,
			}}},
			want: []Activity{{
				Title:         "Start Test",
				UUID:          "CE23D189-E75A-437D-A4B5-B97F1658FC98",
				Screenshots:   nil,
				SubActivities: nil,
			}},
			wantErr: false,
		},
		{
			name: "Subactivty case",
			args: args{[]plistutil.PlistData{{
				"Title":             "Start Test",
				"UUID":              "CE23D189-E75A-437D-A4B5-B97F1658FC98",
				"StartTimeInterval": 568123776.87169898,
				"SubActivities": []interface{}{
					map[string]interface{}{
						"Title":             "Launch",
						"UUID":              "1D7E1C6A-D0A3-432F-819F-64BE07C30517",
						"StartTimeInterval": 568123780.54294205,
					},
				},
			}}},
			want: []Activity{{
				Title:       "Start Test",
				UUID:        "CE23D189-E75A-437D-A4B5-B97F1658FC98",
				Screenshots: nil,
				SubActivities: []Activity{{
					Title:         "Launch",
					UUID:          "1D7E1C6A-D0A3-432F-819F-64BE07C30517",
					Screenshots:   nil,
					SubActivities: nil,
				}},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseActivites(tt.args.activitySummariesData)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseActivites() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseActivites() = %v, want %v", pretty.Object(got), pretty.Object(tt.want))
			}
		})
	}
}

func Test_parseSceenshots(t *testing.T) {
	const fileName = "Screenshot of main screen (ID 1)_1_A07C26DB-8E1E-46ED-90F2-981438BE0BBA.png"
	const attachmentTimeStampFloat = 568123782.31287897
	const activityUUIDinLegacyScreenshotName = "C02EF626-0892-4B50-9B98-70D6F2C3EFE5"
	activityStartTimeForLegacyScreenshot := time.Now()
	type args struct {
		activitySummary   plistutil.PlistData
		activityUUID      string
		activityStartTime time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    []Screenshot
		wantErr bool
	}{
		{
			name: "Attachments (new)",
			args: args{
				activitySummary: plistutil.PlistData{
					"Title":             "Start Test",
					"UUID":              "CE23D189-E75A-437D-A4B5-B97F1658FC98",
					"StartTimeInterval": 568123776.87169898,
					"Attachments": []interface{}{
						map[string]interface{}{
							"Filename":  fileName,
							"Name":      "Screenshot of main screen (ID 1)",
							"Timestamp": attachmentTimeStampFloat,
						},
					},
				},
				activityUUID:      "C02EF626-0892-4B50-9B98-70D6F2C3EFE5",
				activityStartTime: time.Time{},
			},
			want: []Screenshot{{
				FileName:    fileName,
				TimeCreated: TimestampToTime(attachmentTimeStampFloat),
			}},
			wantErr: false,
		},
		{
			name: "Screenhot data (legacy)",
			args: args{
				activitySummary: plistutil.PlistData{
					"Title":             "Start Test",
					"UUID":              "CE23D189-E75A-437D-A4B5-B97F1658FC98",
					"StartTimeInterval": 568123776.87169898,
					"HasScreenshotData": true,
				},
				activityUUID:      activityUUIDinLegacyScreenshotName,
				activityStartTime: activityStartTimeForLegacyScreenshot,
			},
			want: []Screenshot{
				{
					FileName:    fmt.Sprintf("Screenshot_%s.%s", activityUUIDinLegacyScreenshotName, "png"),
					TimeCreated: activityStartTimeForLegacyScreenshot,
				},
				{
					FileName:    fmt.Sprintf("Screenshot_%s.%s", activityUUIDinLegacyScreenshotName, "jpg"),
					TimeCreated: activityStartTimeForLegacyScreenshot,
				}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSceenshots(tt.args.activitySummary, tt.args.activityUUID, tt.args.activityStartTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSceenshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSceenshots() = %v, want %v", pretty.Object(got), pretty.Object(tt.want))
			}
		})
	}
}
