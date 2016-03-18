package xcodeutil

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	cmd "github.com/bitrise-io/xcode-test/command"
	log "github.com/bitrise-io/xcode-test/logutil"
	"github.com/bitrise-io/xcode-test/models"
)

var (
	osRegexp          = regexp.MustCompile(`-- (.+) (\d\.\d) --`)
	deviceStateRegexp = regexp.MustCompile(` *(.+) \(([A-Z0-9-]+)\) \((.+)\)`)
)

//=======================================
// Utility
//=======================================

// a simulator info line should look like this:
//  iPhone 5s (EA1C7E48-8137-428C-A0A5-B2C63FF276EB) (Shutdown)
// or
//  iPhone 4s (51B10EBD-C949-49F5-A38B-E658F41640FF) (Shutdown) (unavailable, runtime profile not found)
func getSimInfoFromLine(lineStr string) (models.SimInfoModel, error) {
	baseInfosExp := regexp.MustCompile(`(?P<deviceName>[a-zA-Z].*[a-zA-Z0-9 -]*) \((?P<simulatorID>[a-zA-Z0-9-]{36})\) \((?P<status>[a-zA-Z]*)\)`)
	baseInfosRes := baseInfosExp.FindStringSubmatch(lineStr)
	if baseInfosRes == nil {
		return models.SimInfoModel{}, fmt.Errorf("No match found")
	}

	simInfo := models.SimInfoModel{
		Name:   baseInfosRes[1],
		SimID:  baseInfosRes[2],
		Status: baseInfosRes[3],
	}

	// StatusOther
	restOfTheLine := lineStr[len(baseInfosRes[0]):]
	if len(restOfTheLine) > 0 {
		statusOtherExp := regexp.MustCompile(`\((?P<statusOther>[a-zA-Z ,]*)\)`)
		statusOtherRes := statusOtherExp.FindStringSubmatch(restOfTheLine)
		if statusOtherRes != nil {
			simInfo.StatusOther = statusOtherRes[1]
		}
	}
	return simInfo, nil
}

func collectAllSimIDs(simctlListOutputToScan string) models.SimulatorsGroupedByIOSVersionsModel {
	simulatorsByIOSVersions := models.SimulatorsGroupedByIOSVersionsModel{}
	currIOSVersion := ""

	fscanner := bufio.NewScanner(strings.NewReader(simctlListOutputToScan))
	isDevicesSectionFound := false
	for fscanner.Scan() {
		aLine := fscanner.Text()

		if aLine == "== Devices ==" {
			isDevicesSectionFound = true
			continue
		}

		if !isDevicesSectionFound {
			continue
		}
		if strings.HasPrefix(aLine, "==") {
			isDevicesSectionFound = false
			continue
		}
		if strings.HasPrefix(aLine, "--") {
			iosVersionSectionExp := regexp.MustCompile(`-- (?P<iosVersionSection>.*) --`)
			iosVersionSectionRes := iosVersionSectionExp.FindStringSubmatch(aLine)
			if iosVersionSectionRes != nil {
				currIOSVersion = iosVersionSectionRes[1]
			}
			continue
		}

		// fmt.Println("-> ", aLine)
		simInfo, err := getSimInfoFromLine(aLine)
		if err != nil {
			fmt.Println(" [!] Error scanning the line for Simulator info: ", err)
		}

		currIOSVersionSimList := simulatorsByIOSVersions[currIOSVersion]
		currIOSVersionSimList = append(currIOSVersionSimList, simInfo)
		simulatorsByIOSVersions[currIOSVersion] = currIOSVersionSimList
	}

	return simulatorsByIOSVersions
}

// Compares sematic versions with 2 components (9.1, 9.2, ...)
// Return true if first version is greater then second
func isOsVersionGreater(osVersion, otherOsVersion string) (bool, error) {
	versionsComponents := [][]int64{}
	for _, osVer := range []string{osVersion, otherOsVersion} {
		osVersionSplit := strings.Split(osVer, " ")
		if len(osVersionSplit) != 2 {
			return false, fmt.Errorf("failed to parse version: %s", osVer)
		}

		versionSplit := strings.Split(osVersionSplit[1], ".")
		if len(versionSplit) != 2 {
			return false, fmt.Errorf("failed to parse version: %s", osVer)
		}

		versionComponents := []int64{}
		for _, versionComponentStr := range versionSplit {
			versionComponent, err := strconv.ParseInt(versionComponentStr, 10, 32)
			if err != nil {
				return false, fmt.Errorf("failed to parse version: %s", osVer)
			}
			versionComponents = append(versionComponents, versionComponent)
		}

		versionsComponents = append(versionsComponents, versionComponents)
	}

	for i := 0; i < len(versionsComponents[0]); i++ {
		versionPart := versionsComponents[0][i]
		otherVersionPart := versionsComponents[1][i]

		if otherVersionPart > versionPart {
			return true, nil
		}
	}

	return false, nil
}

//=======================================
// Main
//=======================================

// GetSimulator ...
func GetSimulator(simulatorPlatform, simulatorDevice, simulatorOsVersion string) (models.SimInfoModel, error) {
	cmd := exec.Command("xcrun", "simctl", "list")
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return models.SimInfoModel{}, err
	}

	simctlListOut := string(outBytes)

	allSimIDsGroupedBySimVersion := collectAllSimIDs(simctlListOut)

	//
	// map desired inputs
	simulatorPlatformSplit := strings.Split(simulatorPlatform, " Simulator")
	if len(simulatorPlatformSplit) == 0 {
		return models.SimInfoModel{}, fmt.Errorf("failed to parse simulator platform (%s)", simulatorPlatform)
	}

	if simulatorDevice == "iPad" {
		log.LogWarn("Given device (%s) is deprecated, using (iPad 2)...", simulatorDevice)
		simulatorDevice = "iPad 2"
	}

	desiredPlatform := simulatorPlatformSplit[0]
	desiredOsVersion := ""

	if simulatorOsVersion == "latest" {
		latestOsVersion := ""
		for osVersion := range allSimIDsGroupedBySimVersion {
			if !strings.HasPrefix(osVersion, desiredPlatform) {
				continue
			}

			if latestOsVersion == "" {
				latestOsVersion = osVersion
			} else {
				greater, err := isOsVersionGreater(latestOsVersion, osVersion)
				if err != nil {
					return models.SimInfoModel{}, err
				}

				if greater {
					latestOsVersion = osVersion
				}
			}
		}

		desiredOsVersion = latestOsVersion
	} else {
		desiredOsVersion = fmt.Sprintf("%s %s", desiredPlatform, simulatorOsVersion)
	}

	//
	// find desired simulator
	simInfoList, found := allSimIDsGroupedBySimVersion[desiredOsVersion]
	if !found {
		return models.SimInfoModel{}, fmt.Errorf("no simulator found for desired os: %s", desiredOsVersion)
	}

	for _, simInfo := range simInfoList {
		if simInfo.Name == simulatorDevice {
			return simInfo, nil
		}
	}

	return models.SimInfoModel{}, fmt.Errorf("%s - %s - %s not found", simulatorPlatform, simulatorDevice, simulatorOsVersion)
}

// BootSimulator ...
func BootSimulator(simulator models.SimInfoModel, xcodebuildVersion models.XcodebuildVersionModel) error {
	simulatorApp := "Simulator"
	if xcodebuildVersion.MajorVersion == 6 {
		simulatorApp = "iOS Simulator"
	}

	openCmd := exec.Command("open", "-a", simulatorApp, "--args", "-CurrentDeviceUDID", simulator.SimID)

	log.LogDetails("$ %s", cmd.PrintableCommandArgs(openCmd.Args))

	out, err := openCmd.CombinedOutput()
	outStr := string(out)
	if err != nil {
		return fmt.Errorf("failed to start simulators (%s), output: %s, error: %s", simulator.SimID, outStr, err)
	}

	return nil
}

// GetXcodeVersion ...
func GetXcodeVersion() (models.XcodebuildVersionModel, error) {
	cmd := exec.Command("xcodebuild", "-version")
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	if err != nil {
		return models.XcodebuildVersionModel{}, fmt.Errorf("xcodebuild -version failed, err: %s, details: %s", err, outStr)
	}

	split := strings.Split(outStr, "\n")
	if len(split) == 0 {
		return models.XcodebuildVersionModel{}, fmt.Errorf("failed to parse xcodebuild version output (%s)", outStr)
	}

	xcodebuildVersion := split[0]
	buildVersion := split[1]

	split = strings.Split(xcodebuildVersion, " ")
	if len(split) != 2 {
		return models.XcodebuildVersionModel{}, fmt.Errorf("failed to parse xcodebuild version output (%s)", outStr)
	}

	version := split[1]

	split = strings.Split(version, ".")
	majorVersionStr := split[0]

	majorVersion, err := strconv.ParseInt(majorVersionStr, 10, 32)
	if err != nil {
		return models.XcodebuildVersionModel{}, fmt.Errorf("failed to parse xcodebuild version output (%s), error: %s", outStr, err)
	}

	return models.XcodebuildVersionModel{
		Version:      xcodebuildVersion,
		BuildVersion: buildVersion,
		MajorVersion: majorVersion,
	}, nil
}
