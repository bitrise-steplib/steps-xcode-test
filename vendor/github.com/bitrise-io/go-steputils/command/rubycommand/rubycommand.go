package rubycommand

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/bitrise-io/go-utils/env"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/pathutil"
)

const (
	systemRubyPth  = "/usr/bin/ruby"
	brewRubyPth    = "/usr/local/bin/ruby"
	brewRubyPthAlt = "/usr/local/opt/ruby/bin/ruby"
)

// InstallType ...
type InstallType int8

const (
	// Unkown ...
	Unkown InstallType = iota
	// SystemRuby ...
	SystemRuby
	// BrewRuby ...
	BrewRuby
	// RVMRuby ...
	RVMRuby
	// RbenvRuby ...
	RbenvRuby
)

// TODO remove
var temporaryFactory = command.NewFactory(env.NewRepository())

// TODO remove
func newWithParams(args ...string) (command.Command, error) {
	if len(args) == 0 {
		return nil, errors.New("no command provided")
	} else if len(args) == 1 {
		return temporaryFactory.Create(args[0], nil, nil), nil
	}

	return temporaryFactory.Create(args[0], args[1:], nil), nil
}

func cmdExist(slice ...string) bool {
	if len(slice) == 0 {
		return false
	}

	cmd, err := newWithParams(slice...)
	if err != nil {
		return false
	}

	return cmd.Run() == nil
}

// RubyInstallType returns which version manager was used for the ruby install
func RubyInstallType() InstallType {
	whichRuby, err := temporaryFactory.Create("which", []string{"ruby"}, nil).RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return Unkown
	}

	installType := Unkown
	if whichRuby == systemRubyPth {
		installType = SystemRuby
	} else if whichRuby == brewRubyPth {
		installType = BrewRuby
	} else if whichRuby == brewRubyPthAlt {
		installType = BrewRuby
	} else if cmdExist("rvm", "-v") {
		installType = RVMRuby
	} else if cmdExist("rbenv", "-v") {
		installType = RbenvRuby
	}

	return installType
}

func sudoNeeded(installType InstallType, slice ...string) bool {
	if installType != SystemRuby {
		return false
	}

	if len(slice) < 2 {
		return false
	}

	name := slice[0]
	if name == "bundle" {
		cmd := slice[1]
		/*
			bundle command can contain version:
			`bundle _2.0.1_ install`
		*/
		const bundleVersionMarker = "_"
		if strings.HasPrefix(slice[1], bundleVersionMarker) && strings.HasSuffix(slice[1], bundleVersionMarker) {
			if len(slice) < 3 {
				return false
			}
			cmd = slice[2]
		}

		return cmd == "install" || cmd == "update"
	} else if name == "gem" {
		cmd := slice[1]
		return cmd == "install" || cmd == "uninstall"
	}

	return false
}

// NewWithParams ...
func NewWithParams(params ...string) (command.Command, error) {
	rubyInstallType := RubyInstallType()
	if rubyInstallType == Unkown {
		return nil, errors.New("unknown ruby installation type")
	}

	if sudoNeeded(rubyInstallType, params...) {
		params = append([]string{"sudo"}, params...)
	}

	return newWithParams(params...)
}

// NewFromSlice ...
func NewFromSlice(slice []string) (command.Command, error) {
	return NewWithParams(slice...)
}

// New ...
func New(name string, args ...string) (command.Command, error) {
	slice := append([]string{name}, args...)
	return NewWithParams(slice...)
}

// GemUpdate ...
func GemUpdate(gem string) ([]command.Command, error) {
	var cmds []command.Command

	cmd, err := New("gem", "update", gem, "--no-document")
	if err != nil {
		return []command.Command{}, err
	}

	cmds = append(cmds, cmd)

	rubyInstallType := RubyInstallType()
	if rubyInstallType == RbenvRuby {
		cmd, err := New("rbenv", "rehash")
		if err != nil {
			return []command.Command{}, err
		}

		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

func gemInstallCommand(gem, version string, enablePrerelease bool) []string {
	slice := []string{"gem", "install", gem, "--no-document"}
	if enablePrerelease {
		slice = append(slice, "--prerelease")
	}
	if version != "" {
		slice = append(slice, "-v", version)
	}

	return slice
}

// GemInstall ...
func GemInstall(gem, version string, enablePrerelease bool) ([]command.Command, error) {
	cmd, err := NewFromSlice(gemInstallCommand(gem, version, enablePrerelease))
	if err != nil {
		return []command.Command{}, err
	}

	cmds := []command.Command{cmd}

	rubyInstallType := RubyInstallType()
	if rubyInstallType == RbenvRuby {
		cmd, err := New("rbenv", "rehash")
		if err != nil {
			return []command.Command{}, err
		}

		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

func findGemInList(gemList, gem, version string) (bool, error) {
	// minitest (5.10.1, 5.9.1, 5.9.0, 5.8.3, 4.7.5)
	pattern := fmt.Sprintf(`^%s \(.*%s.*\)`, gem, version)
	re := regexp.MustCompile(pattern)

	reader := bytes.NewReader([]byte(gemList))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindString(line)
		if match != "" {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// IsGemInstalled ...
func IsGemInstalled(gem, version string) (bool, error) {
	cmd, err := New("gem", "list")
	if err != nil {
		return false, err
	}

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s: error: %s", out, err)
	}

	return findGemInList(out, gem, version)
}

func isSpecifiedRbenvRubyInstalled(message string) (bool, string, error) {
	//
	// Not installed
	reg, err := regexp.Compile("rbenv: version \x60.*' is not installed") // \x60 == ` (The go linter suggested to use the hex code instead)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", "rbenv: version \x60.*' is not installed", err) // \x60 == ` (The go linter suggested to use the hex code instead)
	}

	var version string
	if reg.MatchString(message) {
		message := reg.FindString(message)
		version = strings.Split(strings.Split(message, "`")[1], "'")[0]
		return false, version, nil
	}

	//
	// Installed
	reg, err = regexp.Compile(".* \\(set by")
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", ".* \\(set by", err)
	}

	if reg.MatchString(message) {
		s := reg.FindString(message)
		version = strings.Split(s, " (set by")[0]
		return true, version, nil
	}
	return false, version, nil
}

// IsSpecifiedRbenvRubyInstalled checks if the selected ruby version is installed via rbenv.
// Ruby version is set by
// 1. The RBENV_VERSION environment variable
// 2. The first .ruby-version file found by searching the directory of the script you are executing and each of its
// parent directories until reaching the root of your filesystem.
// 3.The first .ruby-version file found by searching the current working directory and each of its parent directories
// until reaching the root of your filesystem.
// 4. The global ~/.rbenv/version file. You can modify this file using the rbenv global command.
// src: https://github.com/rbenv/rbenv#choosing-the-ruby-version
func IsSpecifiedRbenvRubyInstalled(workdir string) (bool, string, error) {
	absWorkdir, err := pathutil.AbsPath(workdir)
	if err != nil {
		return false, "", fmt.Errorf("failed to get absolute path for ( %s ), error: %s", workdir, err)
	}

	cmd := temporaryFactory.Create("rbenv", []string{"version"}, &command.Opts{Dir: absWorkdir})
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("failed to check installed ruby version, %s error: %s", out, err)
	}
	return isSpecifiedRbenvRubyInstalled(out)
}
