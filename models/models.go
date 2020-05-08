package models

import (
	"strings"
)

//=======================================
// Models
//=======================================

// XcodeBuildParamsModel ...
type XcodeBuildParamsModel struct {
	Action                    string
	ProjectPath               string
	Scheme                    string
	DeviceDestination         string
	CleanBuild                bool
	DisableIndexWhileBuilding bool
}

// XcodeBuildTestParamsModel ...
type XcodeBuildTestParamsModel struct {
	BuildParams XcodeBuildParamsModel

	TestOutputDir        string
	CleanBuild           bool
	BuildBeforeTest      bool
	GenerateCodeCoverage bool
	AdditionalOptions    string
	OnlyTestOptions      string
}

//=======================================
// xcresulttool models useful for Xcode
// versions greater than or equal to 11.
//=======================================

// XcodeActionsInvocationRecord ...
// based on Xcode Result Types Version: 3.24
type XcodeActionsInvocationRecord struct {
	Type   TypeStruct              `json:"_type"`
	Issues XcodeResultIssueSummary `json:"issues"`
}

// TestFailures returns a slice of the test paths that failed.
func (r *XcodeActionsInvocationRecord) TestFailures() []string {
	failures := []string{}

	for _, failure := range r.Issues.TestFailureSummaries.Values {
		failures = append(failures, failure.TestPath())
	}

	return failures
}

// XcodeResultIssueSummary ...
// based on Xcode Result Types Version: 3.24
type XcodeResultIssueSummary struct {
	Type                 TypeStruct                              `json:"_type"`
	ErrorSummaries       XcodeIssueSummaryArrayStruct            `json:"errorSummaries"`
	TestFailureSummaries XcodeTestFailureIssueSummaryArrayStruct `json:"testFailureSummaries"`
}

// XcodeIssueSummaryArrayStruct ...
// based on Xcode Result Types Version: 3.24
type XcodeIssueSummaryArrayStruct struct {
	Type   TypeStruct          `json:"_type"`
	Values []XcodeIssueSummary `json:"_values"`
}

// XcodeTestFailureIssueSummaryArrayStruct ...
// based on Xcode Result Types Version: 3.24
type XcodeTestFailureIssueSummaryArrayStruct struct {
	Type   TypeStruct                     `json:"_type"`
	Values []XcodeTestFailureIssueSummary `json:"_values"`
}

// XcodeIssueSummary ...
// based on Xcode Result Types Version: 3.24
type XcodeIssueSummary struct {
	Type                                TypeStruct            `json:"_type"`
	IssueType                           StringStruct          `json:"issueType"`
	Message                             StringStruct          `json:"message"`
	ProducingTarget                     StringStruct          `json:"producingTarget"`
	DocumentLocationInCreatingWorkspace XcodeDocumentLocation `json:"documentLocationInCreatingWorkspace"`
}

// XcodeTestFailureIssueSummary ...
// based on Xcode Result Types Version: 3.24
type XcodeTestFailureIssueSummary struct {
	Type                                TypeStruct            `json:"_type"`
	IssueType                           StringStruct          `json:"issueType"`
	Message                             StringStruct          `json:"message"`
	ProducingTarget                     StringStruct          `json:"producingTarget"`
	DocumentLocationInCreatingWorkspace XcodeDocumentLocation `json:"documentLocationInCreatingWorkspace"`
	TestCaseName                        StringStruct          `json:"testCaseName"`
}

// TestPath returns the string representation of the test path that failed.
func (s *XcodeTestFailureIssueSummary) TestPath() string {
	testCase := strings.ReplaceAll(s.TestCaseName.Value, ".", "/")
	testCase = strings.ReplaceAll(testCase, "(", "")
	testCase = strings.ReplaceAll(testCase, ")", "")

	return s.ProducingTarget.Value + "/" + testCase
}

// XcodeDocumentLocation ...
// based on Xcode Result Types Version: 3.24
type XcodeDocumentLocation struct {
	Type             TypeStruct   `json:"_type"`
	Url              StringStruct `json:"url"`
	ConcreteTypeName StringStruct `json:"concreteTypeName"`
}

// TypeStruct ...
// based on Xcode Result Types Version: 3.24
type TypeStruct struct {
	Name string `json:"_name"`
}

// StringStruct ...
// based on Xcode Result Types Version: 3.24
type StringStruct struct {
	Type  TypeStruct `json:"_type"`
	Value string     `json:"_value"`
}
