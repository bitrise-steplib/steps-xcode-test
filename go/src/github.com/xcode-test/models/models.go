package models

import "fmt"

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
