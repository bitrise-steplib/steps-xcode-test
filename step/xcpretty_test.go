package step

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GivenXcprettyInstallationCheckError_WhenTheErrorIsHandled_ThenExpectAnEmptyOutputToolAndErrorToBeReturned(t *testing.T) {
	// Given
	givenError := newXcprettyInstallationCheckError("an error occurred")

	// When
	outputTool, err := handleXcprettyInstallError(givenError)

	// Then
	assert.Equal(t, "", outputTool)
	assert.Equal(t, givenError, err)
}

func Test_GivenXcprettyDetermineVersionError_WhenTheErrorIsHandled_ThenExpectTheXcodeBuildOutputToolToBeReturned(t *testing.T) {
	// Given
	givenError := errors.New("determineVersionError")

	// When
	outputTool, err := handleXcprettyInstallError(givenError)

	// Then
	assert.Equal(t, XcodebuildTool, outputTool)
	assert.NoError(t, err)
}
