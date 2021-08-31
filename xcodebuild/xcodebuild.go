package xcodebuild

// Xcodebuild ....
type Xcodebuild interface {
	RunBuild(buildParams Params, outputTool string) (string, int, error)
	RunTest(params TestRunParams) (string, int, error)
}

type xcodebuild struct {
}

// New ...
func New() Xcodebuild {
	return &xcodebuild{}
}

// Params ...
type Params struct {
	Action                    string
	ProjectPath               string
	Scheme                    string
	DeviceDestination         string
	CleanBuild                bool
	DisableIndexWhileBuilding bool
}

// RunBuild ...
func (b *xcodebuild) RunBuild(buildParams Params, outputTool string) (string, int, error) {
	return runBuild(buildParams, outputTool)
}

// RunTest ...
func (b *xcodebuild) RunTest(params TestRunParams) (string, int, error) {
	return runTest(params)
}
