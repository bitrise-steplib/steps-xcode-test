package output

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/stringutil"
	"github.com/bitrise-io/go-utils/ziputil"
)

// ExportOutputDir ...
func ExportOutputDir(sourceDir, destinationDir, envKey string) error {
	absSourceDir, err := pathutil.AbsPath(sourceDir)
	if err != nil {
		return err
	}

	absDestinationDir, err := pathutil.AbsPath(destinationDir)
	if err != nil {
		return err
	}

	if absSourceDir != absDestinationDir {
		if err := command.CopyDir(absSourceDir, absDestinationDir, true); err != nil {
			return err
		}
	}
	return tools.ExportEnvironmentWithEnvman(envKey, absDestinationDir)
}

// ExportOutputFile ...
func ExportOutputFile(sourcePth, destinationPth, envKey string) error {
	absSourcePth, err := pathutil.AbsPath(sourcePth)
	if err != nil {
		return err
	}

	absDestinationPth, err := pathutil.AbsPath(destinationPth)
	if err != nil {
		return err
	}

	if absSourcePth != absDestinationPth {
		if err := command.CopyFile(absSourcePth, absDestinationPth); err != nil {
			return err
		}
	}
	return tools.ExportEnvironmentWithEnvman(envKey, absDestinationPth)
}

// ExportOutputFileContent ...
func ExportOutputFileContent(content, destinationPth, envKey string) error {
	if err := fileutil.WriteStringToFile(destinationPth, content); err != nil {
		return err
	}

	return ExportOutputFile(destinationPth, destinationPth, envKey)
}

// ExportOutputFileContentAndReturnLastNLines ...
func ExportOutputFileContentAndReturnLastNLines(content, destinationPath, envKey string, lines int) (string, error) {
	if err := fileutil.WriteStringToFile(destinationPath, content); err != nil {
		return "", err
	}

	if err := ExportOutputFile(destinationPath, destinationPath, envKey); err != nil {
		return "", err
	}

	return stringutil.LastNLines(content, lines), nil
}

// ZipAndExportOutput ...
func ZipAndExportOutput(sourcePth, destinationZipPth, envKey string) error {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export_tmp_dir__")
	if err != nil {
		return err
	}

	base := filepath.Base(sourcePth)
	tmpZipFilePth := filepath.Join(tmpDir, base+".zip")

	if exist, err := pathutil.IsDirExists(sourcePth); err != nil {
		return err
	} else if exist {
		if err := ziputil.ZipDir(sourcePth, tmpZipFilePth, false); err != nil {
			return err
		}
	} else if exist, err := pathutil.IsPathExists(sourcePth); err != nil {
		return err
	} else if exist {
		if err := ziputil.ZipFile(sourcePth, tmpZipFilePth); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("source path (%s) not exists", sourcePth)
	}

	return ExportOutputFile(tmpZipFilePth, destinationZipPth, envKey)
}
