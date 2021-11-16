package xcpretty

import (
	gocommand "github.com/bitrise-io/go-utils/command"
	mockcommand "github.com/bitrise-io/go-utils/command/mocks"
	mockxcpretty "github.com/bitrise-io/go-xcode/xcpretty/mocks"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testingMocks struct {
	command *mockcommand.Command
	xcpretty *mockxcpretty.Xcpretty
}

func Test_GivenNotInstalled_WhenInstall_ThenInstallsIt(t *testing.T) {
	// Given
	installer, version, mocks := createInstallerAndMocks(false)

	// When
	installedVersion, err := installer.Install()

	// Then
	assert.NoError(t, err)
	assert.Equal(t, version, installedVersion)
	mocks.xcpretty.AssertCalled(t, "IsInstalled")
	mocks.xcpretty.AssertCalled(t, "Install")
	mocks.xcpretty.AssertCalled(t, "Version")
	mocks.command.AssertCalled(t, "Run")
}

func Test_GivenInstalled_WhenInstall_OnlyReturnsVersion(t *testing.T) {
	// Given
	installer, version, mocks := createInstallerAndMocks(true)

	// When
	installedVersion, err := installer.Install()

	// Then
	assert.NoError(t, err)
	assert.Equal(t, version, installedVersion)
	mocks.xcpretty.AssertCalled(t, "IsInstalled")
	mocks.xcpretty.AssertNotCalled(t, "Install")
	mocks.xcpretty.AssertCalled(t, "Version")
	mocks.command.AssertNotCalled(t, "Run")
}

func createInstallerAndMocks(installed bool) (Installer, *version.Version, testingMocks) {
	command := new(mockcommand.Command)
	command.On("Run").Return(nil)

	version, _ := version.NewVersion("1.0.0")

	mockxcpretty := new(mockxcpretty.Xcpretty)
	mockxcpretty.On("IsInstalled").Return(installed, nil)
	mockxcpretty.On("Install").Return([]gocommand.Command{command}, nil)
	mockxcpretty.On("Version").Return(version, nil)

	installer := NewInstaller(mockxcpretty)

	return installer, version, testingMocks{
		command:  command,
		xcpretty: mockxcpretty,
	}
}