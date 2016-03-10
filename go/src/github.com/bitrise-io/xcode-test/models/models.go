package models

import "fmt"

/*
func runBuild(
	outputTool,
	action,
	projectPath,
	scheme string,
	cleanBuild bool,
	deviceDestination,
	derivedDataDir string) (string, int, error) {
*/

// XcodeBuildParamsModel ...
type XcodeBuildParamsModel struct {
	Action            string
	ProjectPath       string
	Scheme            string
	DeviceDestination string
	DerivedDataDir    string
	CleanBuild        bool
}

// XcodeBuildTestParamsModel ...
type XcodeBuildTestParamsModel struct {
	BuildParams XcodeBuildParamsModel

	GenerateCodeCoverage bool
	AdditionalOptions    string
}

// XcodebuildVersionModel ...
type XcodebuildVersionModel struct {
	Version      string
	BuildVersion string
	MajorVersion int64
}

// SimInfo ...
type SimInfo struct {
	Name        string
	SimID       string
	Status      string
	StatusOther string
}

// OSVersionSimInfoPair ...
type OSVersionSimInfoPair struct {
	OSVersion     string
	SimulatorInfo SimInfo
}

// SimulatorsGroupedByIOSVersions ...
type SimulatorsGroupedByIOSVersions map[string][]SimInfo

func (simsGrouped *SimulatorsGroupedByIOSVersions) flatList() []OSVersionSimInfoPair {
	osVersionSimInfoPairs := []OSVersionSimInfoPair{}

	for osVer, simulatorInfos := range *simsGrouped {
		for _, aSimInfo := range simulatorInfos {
			osVersionSimInfoPairs = append(osVersionSimInfoPairs, OSVersionSimInfoPair{
				OSVersion:     osVer,
				SimulatorInfo: aSimInfo,
			})
		}
	}

	return osVersionSimInfoPairs
}

func (simsGrouped *SimulatorsGroupedByIOSVersions) duplicates() []OSVersionSimInfoPair {
	duplicates := []OSVersionSimInfoPair{}
	for osVer, simulatorInfos := range *simsGrouped {
		simNameCache := map[string]bool{}
		for _, aSimInfo := range simulatorInfos {
			if _, isFound := simNameCache[aSimInfo.Name]; isFound {
				duplicates = append(duplicates, OSVersionSimInfoPair{
					OSVersion:     osVer,
					SimulatorInfo: aSimInfo,
				})
			}
			simNameCache[aSimInfo.Name] = true
		}
	}
	return duplicates
}

func (osVerSimInfoPair *OSVersionSimInfoPair) String() string {
	return fmt.Sprintf("[OS: %s] %#v", osVerSimInfoPair.OSVersion, osVerSimInfoPair.SimulatorInfo)
}
