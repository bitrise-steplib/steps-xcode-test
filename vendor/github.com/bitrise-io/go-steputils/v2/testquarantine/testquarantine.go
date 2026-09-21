package testquarantine

import "encoding/json"

// QuarantinedTest represents a test case that has been marked as quarantined.
type QuarantinedTest struct {
	TestCaseName  string   `json:"testCaseName"`
	TestSuiteName []string `json:"testSuiteName"`
	ClassName     string   `json:"className"`
	// TestCaseIdentifier is the test framework's own identifier of the test case. Empty unless the
	// test report the quarantine entry was created from carried it. Prefer it over TestCaseName,
	// which can be a display name that no test framework accepts as a filter.
	TestCaseIdentifier string `json:"testCaseIdentifier"`
}

// ParseQuarantinedTests parses Bitrise quarantined tests JSON ($BITRISE_QUARANTINED_TESTS_JSON) into a slice of QuarantinedTest structs.
func ParseQuarantinedTests(jsonContent string) ([]QuarantinedTest, error) {
	if jsonContent == "" {
		return nil, nil
	}

	var quarantinedTests []QuarantinedTest
	err := json.Unmarshal([]byte(jsonContent), &quarantinedTests)
	if err != nil {
		return nil, err
	}
	return quarantinedTests, nil
}
