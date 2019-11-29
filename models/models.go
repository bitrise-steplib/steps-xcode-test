package models

//=======================================
// Models
//=======================================

// XcodeBuildParamsModel ...
type XcodeBuildParamsModel struct {
	Action                    string
	ProjectPath               string
	Scheme                    string
	DeviceDestinations        []string
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
}
