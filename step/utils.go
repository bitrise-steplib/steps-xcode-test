package step

import (
	"fmt"

	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/stringutil"
)

func printLastLinesOfXcodebuildTestLog(rawXcodebuildOutput string, isRunSuccess bool) {
	const lastLines = "\nLast lines of the build log:"
	if !isRunSuccess {
		log.Errorf(lastLines)
	} else {
		log.Infof(lastLines)
	}

	fmt.Println(stringutil.LastNLines(rawXcodebuildOutput, 20))

	if !isRunSuccess {
		log.Warnf("If you can't find the reason of the error in the log, please check the xcodebuild_test.log.")
	}

	log.Infof(colorstring.Magenta(`
The log file is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_XCODEBUILD_TEST_LOG_PATH environment variable.

If you have the Deploy to Bitrise.io step (after this step),
that will attach the file to your build as an artifact!`))
}
