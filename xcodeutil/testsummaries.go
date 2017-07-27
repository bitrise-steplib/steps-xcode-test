package xcodeutil

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bitrise-tools/go-xcode/plistutil"
)

func castToMapStringInterfaceArray(obj interface{}) ([]map[string]interface{}, error) {
	array, ok := obj.([]interface{})
	if !ok {
		return nil, errors.New("failed to cast to []interface{}")
	}

	casted := []map[string]interface{}{}
	for _, item := range array {
		mapStringInterface, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.New("failed to cast to map[string]interface{}")
		}

		casted = append(casted, mapStringInterface)
	}

	return casted, nil
}

func getValueAsMapStringInterfaceArray(obj map[string]interface{}, key string) ([]map[string]interface{}, error) {
	value, found := obj[key]
	if !found {
		return nil, fmt.Errorf("no value found for: %s", key)
	}

	return castToMapStringInterfaceArray(value)
}

func collectLastSubtests(testsItem map[string]interface{}) ([]map[string]interface{}, error) {
	var walk func(map[string]interface{}) []map[string]interface{}
	walk = func(item map[string]interface{}) []map[string]interface{} {
		subtests, err := getValueAsMapStringInterfaceArray(item, "Subtests")
		if err != nil {
			return []map[string]interface{}{item}
		}
		lastSubtests := []map[string]interface{}{}
		for _, subtest := range subtests {
			last := walk(subtest)
			lastSubtests = append(lastSubtests, last...)
		}
		return lastSubtests
	}

	return walk(testsItem), nil
}

func collectSubActivitiesWithScreenshots(activitySummaries []map[string]interface{}) ([]map[string]interface{}, error) {
	var walk func(map[string]interface{}) []map[string]interface{}
	walk = func(item map[string]interface{}) []map[string]interface{} {
		itemWithScreenshot := []map[string]interface{}{}

		value, found := item["HasScreenshotData"]
		if found {
			hasScreenshot, casted := value.(bool)
			if casted && hasScreenshot {
				itemWithScreenshot = append(itemWithScreenshot, item)
			}
		}

		subActivies, err := getValueAsMapStringInterfaceArray(item, "SubActivities")
		if err == nil {
			for _, subActivity := range subActivies {
				subActivityWithScreenshots := walk(subActivity)
				itemWithScreenshot = append(itemWithScreenshot, subActivityWithScreenshots...)
			}
		}

		return itemWithScreenshot
	}

	summaries := []map[string]interface{}{}
	for _, summary := range activitySummaries {
		summaries = append(summaries, walk(summary)...)
	}

	return summaries, nil
}

// CollectTestItemsWithScreenshot ...
func CollectTestItemsWithScreenshot(testSummariesContent string) ([]map[string]interface{}, error) {
	testSummariesPlistData, err := plistutil.NewPlistDataFromContent(testSummariesContent)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	testableSummaries, err := getValueAsMapStringInterfaceArray(testSummariesPlistData, "TestableSummaries")
	if err != nil {
		return []map[string]interface{}{}, err
	}

	subActivitiesWithScreenshot := []map[string]interface{}{}
	for _, testableSummariesItem := range testableSummaries {
		tests, err := getValueAsMapStringInterfaceArray(testableSummariesItem, "Tests")
		if err != nil {
			return []map[string]interface{}{}, err
		}

		for _, testsItem := range tests {
			lastSubtests, err := collectLastSubtests(testsItem)
			if err != nil {
				return []map[string]interface{}{}, err
			}

			for _, lastSubtest := range lastSubtests {
				activitySummaries, err := getValueAsMapStringInterfaceArray(lastSubtest, "ActivitySummaries")
				if err != nil {
					continue
				}

				subActivities, err := collectSubActivitiesWithScreenshots(activitySummaries)
				if err != nil {
					return []map[string]interface{}{}, err
				}
				subActivitiesWithScreenshot = append(subActivitiesWithScreenshot, subActivities...)
			}
		}
	}

	return subActivitiesWithScreenshot, nil
}

// TimestampStrToTime ...
func TimestampStrToTime(timestampStr string) (time.Time, error) {
	timestamp, err := strconv.ParseFloat(timestampStr, 64)
	if err != nil {
		return time.Time{}, err
	}

	return TimestampToTime(timestamp)
}

// TimestampToTime ...
func TimestampToTime(timestamp float64) (time.Time, error) {
	timestampInNanosec := int64(timestamp * float64(time.Second))
	referenceDate := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	return referenceDate.Add(time.Duration(timestampInNanosec)), nil
}
