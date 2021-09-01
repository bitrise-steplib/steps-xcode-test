package xcodebuild

import (
	"errors"
	"testing"

	mockcommand "github.com/bitrise-io/go-utils/command/mocks"
	mocklog "github.com/bitrise-io/go-utils/log/mocks"
	mockpathutil "github.com/bitrise-io/go-utils/pathutil/mocks"
	mockfileremover "github.com/bitrise-steplib/steps-xcode-test/fileremover/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_WhenXcodebuildFails_ThenExitCodeGetsReturned(t *testing.T) {
	logger := new(mocklog.Logger)
	logger.On("Infof", mock.Anything, mock.Anything).Return()
	logger.On("Printf", mock.Anything, mock.Anything).Return()
	logger.On("Println").Return()

	cmd := new(mockcommand.Command)
	cmd.On("PrintableCommandArgs").Return("")
	cmd.On("RunAndReturnExitCode").Return(5, errors.New("exist status: 5"))

	commandFactory := new(mockcommand.Factory)
	commandFactory.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	pathChecker := new(mockpathutil.PathChecker)
	fileremover := new(mockfileremover.FileRemover)

	xcodebuild := New(logger, commandFactory, pathChecker, fileremover)

	params := Params{
		Action:                    "-project",
		ProjectPath:               "project.xcproj",
		Scheme:                    "scheme",
		DeviceDestination:         "simulator",
		CleanBuild:                false,
		DisableIndexWhileBuilding: false,
	}
	_, exitCode, err := xcodebuild.RunBuild(params, "xcodebuild")
	require.Error(t, err)
	require.Equal(t, exitCode, 5)
}
