package models

import "fmt"

//=======================================
// Models
//=======================================

// XcodeBuildParamsModel ...
type XcodeBuildParamsModel struct {
	Action            string
	ProjectPath       string
	Scheme            string
	DeviceDestination string
	CleanBuild        bool
}

// XcodeBuildTestParamsModel ...
type XcodeBuildTestParamsModel struct {
	BuildParams XcodeBuildParamsModel

	CleanBuild           bool
	BuildBeforeTest      bool
	GenerateCodeCoverage bool
	AdditionalOptions    string
}

// XcodebuildVersionModel ...
type XcodebuildVersionModel struct {
	Version      string
	BuildVersion string
	MajorVersion int64
}

// SimInfoModel ...
type SimInfoModel struct {
	Name        string
	SimID       string
	Status      string
	StatusOther string
}

// OSVersionSimInfoPairModel ...
type OSVersionSimInfoPairModel struct {
	OSVersion     string
	SimulatorInfo SimInfoModel
}

// SimulatorsGroupedByIOSVersionsModel ...
type SimulatorsGroupedByIOSVersionsModel map[string][]SimInfoModel

//=======================================
// Model methods
//=======================================

func (simsGrouped *SimulatorsGroupedByIOSVersionsModel) flatList() []OSVersionSimInfoPairModel {
	osVersionSimInfoPairs := []OSVersionSimInfoPairModel{}

	for osVer, simulatorInfos := range *simsGrouped {
		for _, aSimInfo := range simulatorInfos {
			osVersionSimInfoPairs = append(osVersionSimInfoPairs, OSVersionSimInfoPairModel{
				OSVersion:     osVer,
				SimulatorInfo: aSimInfo,
			})
		}
	}

	return osVersionSimInfoPairs
}

func (simsGrouped *SimulatorsGroupedByIOSVersionsModel) duplicates() []OSVersionSimInfoPairModel {
	duplicates := []OSVersionSimInfoPairModel{}
	for osVer, simulatorInfos := range *simsGrouped {
		simNameCache := map[string]bool{}
		for _, aSimInfo := range simulatorInfos {
			if _, isFound := simNameCache[aSimInfo.Name]; isFound {
				duplicates = append(duplicates, OSVersionSimInfoPairModel{
					OSVersion:     osVer,
					SimulatorInfo: aSimInfo,
				})
			}
			simNameCache[aSimInfo.Name] = true
		}
	}
	return duplicates
}

func (osVerSimInfoPair *OSVersionSimInfoPairModel) String() string {
	return fmt.Sprintf("[OS: %s] %#v", osVerSimInfoPair.OSVersion, osVerSimInfoPair.SimulatorInfo)
}
