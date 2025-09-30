package testquarantine

import "encoding/json"

// QuarantinedTest represents a test case that has been marked as quarantined.
type QuarantinedTest struct {
	TestCaseName  string   `json:"testCaseName"`
	TestSuiteName []string `json:"testSuiteName"`
	ClassName     string   `json:"className"`
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
