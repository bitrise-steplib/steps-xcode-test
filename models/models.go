package models

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
	SPM                       bool
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
