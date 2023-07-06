package xcodebuild

import "regexp"

func isStringFoundInOutput(searchStr, outputToSearchIn string) bool {
	r := regexp.MustCompile("(?i)" + searchStr)
	return r.MatchString(outputToSearchIn)
}
