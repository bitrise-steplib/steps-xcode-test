package command

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PrintableCommandArgs ...
func PrintableCommandArgs(fullCommandArgs []string) string {
	return PrintableCommandArgsWithEnvs(fullCommandArgs, []string{})
}

// PrintableCommandArgsWithEnvs ...
func PrintableCommandArgsWithEnvs(fullCommandArgs []string, envs []string) string {
	cmdArgsDecorated := []string{}
	for idx, anArg := range fullCommandArgs {
		quotedArg := strconv.Quote(anArg)
		if idx == 0 {
			quotedArg = anArg
		}
		cmdArgsDecorated = append(cmdArgsDecorated, quotedArg)
	}

	fullCmdArgs := cmdArgsDecorated
	if len(envs) > 0 {
		fullCmdArgs = []string{"env"}
		for _, anArg := range envs {
			quotedArg := strconv.Quote(anArg)
			fullCmdArgs = append(fullCmdArgs, quotedArg)
		}
		fullCmdArgs = append(fullCmdArgs, cmdArgsDecorated...)
	}

	return strings.Join(fullCmdArgs, " ")
}

// CreateBufferedWriter ...
func CreateBufferedWriter(buff *bytes.Buffer, writers ...io.Writer) io.Writer {
	if len(writers) > 0 {
		allWriters := append([]io.Writer{buff}, writers...)
		return io.MultiWriter(allWriters...)
	}
	return io.Writer(buff)
}

// ExportEnvironmentWithEnvman ...
func ExportEnvironmentWithEnvman(keyStr, valueStr string) error {
	envman := exec.Command("envman", "add", "--key", keyStr)
	envman.Stdin = strings.NewReader(valueStr)
	envman.Stdout = os.Stdout
	envman.Stderr = os.Stderr
	return envman.Run()
}

// GetXcprettyVersion ...
func GetXcprettyVersion() (string, error) {
	cmd := exec.Command("xcpretty", "-version")
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	if strings.HasSuffix(outStr, "\n") {
		outStr = strings.TrimSuffix(outStr, "\n")
	}

	if err != nil {
		return "", fmt.Errorf("xcpretty -version failed, err: %s, details: %s", err, outStr)
	}

	return outStr, nil
}

// CreateXcodebuildCmd ...
func CreateXcodebuildCmd(xcodebuildArgs ...string) *exec.Cmd {
	return exec.Command("xcodebuild", xcodebuildArgs...)
}

// CreateXcprettyCmd ...
func CreateXcprettyCmd(xcprettydArgs ...string) *exec.Cmd {
	return exec.Command("xcpretty", xcprettydArgs...)
}

// Zip ...
func Zip(targetDir, targetRelPathToZip, zipPath string) error {
	zipCmd := exec.Command("/usr/bin/zip", "-rTy", zipPath, targetRelPathToZip)
	zipCmd.Dir = targetDir
	if out, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Zip failed, out: %s, err: %#v", out, err)
	}
	return nil
}
