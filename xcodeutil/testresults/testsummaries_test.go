package testresults

import (
	"reflect"
	"testing"

	"github.com/bitrise-io/steps-xcode-test/pretty"
	"github.com/bitrise-tools/go-xcode/plistutil"
	"github.com/stretchr/testify/require"
)

/*
func TestWalkXcodeTestSummaries(t *testing.T) {
	t.Log()
	{
		log, err := fileutil.ReadStringFromFile("../_samples/TestSummaries.plist")
		require.NoError(t, err)

		var testSummaries TestSummaries
		testSummaries.Content = log
		testSummaries, err = testSummaries.collectTestItemsWithScreenshotAndSetType()
		require.NoError(t, err)
		require.Equal(t, 2, len(testSummaries.TestItemsWithScreenshots))
	}

	t.Log()
	{
		log, err := fileutil.ReadStringFromFile("../_samples/TestSummaries2.plist")
		require.NoError(t, err)

		var testSummaries TestSummaries
		testSummaries.Content = log
		testSummaries, err = testSummaries.collectTestItemsWithScreenshotAndSetType()
		require.NoError(t, err)
		require.Equal(t, 2, len(testSummaries.TestItemsWithScreenshots))
	}
}
*/

func TestTimestampToTime(t *testing.T) {
	time, err := TimestampStrToTime("522675441.31045401")
	require.NoError(t, err)

	require.Equal(t, 2017, time.Year())
	require.Equal(t, 7, int(time.Month()))
	require.Equal(t, 25, time.Day())
	require.Equal(t, 11, time.Hour())
	require.Equal(t, 37, time.Minute())
	require.Equal(t, 21, time.Second())
}

func Test_parseTestSummaries(t *testing.T) {
	const testSummariesPth = "action_TestSummaries.plist"
	testSummariesPlistData, err := plistutil.NewPlistDataFromFile(testSummariesPth)
	if err != nil {
		t.Errorf("failed to parse TestSummaries file: %s, error: %s", testSummariesPth, err)
	}

	type args struct {
		testSummariesContent plistutil.PlistData
	}
	tests := []struct {
		name    string
		args    args
		want    *[]TestResult
		wantErr bool
	}{
		{
			name: "Smoke",
			args: args{
				testSummariesContent: testSummariesPlistData,
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestSummaries(tt.args.testSummariesContent)
			t.Logf(pretty.Object(got))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTestSummaries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTestSummaries() = %v, want %v", got, tt.want)
			}
		})
	}
}
