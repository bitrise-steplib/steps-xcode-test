package command

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// RunCommandReturnCombinedStdoutAndStderr ...
func RunCommandReturnCombinedStdoutAndStderr(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	return outStr, err
}

// ExportEnvironmentWithEnvman ...
func ExportEnvironmentWithEnvman(keyStr, valueStr string) error {
	envman := exec.Command("envman", "add", "--key", keyStr)
	envman.Stdin = strings.NewReader(valueStr)
	envman.Stdout = os.Stdout
	envman.Stderr = os.Stderr
	return envman.Run()
}

// PrintableCommandArgs ...
func PrintableCommandArgs(fullCommandArgs []string) string {
	cmdArgsDecorated := []string{}
	for idx, anArg := range fullCommandArgs {
		quotedArg := strconv.Quote(anArg)
		if idx == 0 {
			quotedArg = anArg
		}
		cmdArgsDecorated = append(cmdArgsDecorated, quotedArg)
	}

	return strings.Join(cmdArgsDecorated, " ")
}

// CreateXcodebuildCmd ...
func CreateXcodebuildCmd(xcodebuildArgs ...string) *exec.Cmd {
	return exec.Command("xcodebuild", xcodebuildArgs...)
}

// CreateXcprettyCmd ...
func CreateXcprettyCmd(testResultsFilePath string) *exec.Cmd {
	prettyArgs := []string{"--color"}
	if testResultsFilePath != "" {
		prettyArgs = append(prettyArgs, "--report", "html", "--output", testResultsFilePath)
	}
	return exec.Command("xcpretty", prettyArgs...)
}

// CreateBufferedWriter ...
func CreateBufferedWriter(buff *bytes.Buffer, writers ...io.Writer) io.Writer {
	if len(writers) > 0 {
		allWriters := append([]io.Writer{buff}, writers...)
		return io.MultiWriter(allWriters...)
	}
	return io.Writer(buff)
}
