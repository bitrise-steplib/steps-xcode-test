package models

// XcodebuildParams ...
type XcodebuildParams struct {
	Action                    string
	ProjectPath               string
	Scheme                    string
	DeviceDestination         string
	CleanBuild                bool
	DisableIndexWhileBuilding bool
}

// XcodebuildTestParams ...
type XcodebuildTestParams struct {
	BuildParams                    XcodebuildParams
	TestPlan                       string
	TestOutputDir                  string
	TestRepetitionMode             string
	MaximumTestRepetitions         int
	RelaunchTestsForEachRepetition bool
	CleanBuild                     bool
	BuildBeforeTest                bool
	GenerateCodeCoverage           bool
	RetryTestsOnFailure            bool
	AdditionalOptions              string
}
