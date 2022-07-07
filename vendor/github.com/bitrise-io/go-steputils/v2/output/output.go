package output

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
)

// ExportOutputFile ...
func exportOutputFile(sourcePth, destinationPth, envKey string) error {
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

// ZipAndExportOutput ...
func ZipAndExportOutput(sourcePths []string, destinationZipPth, envKey string) error {
	tmpZipFilePth, err := zipFilePath()
	if err != nil {
		return err
	}

	inputType, err := getInputType(sourcePths)
	if err != nil {
		return err
	}

	// We have separate zip functions for files and folders and that is the main reason we cannot have mixed
	// paths (files and also folders) in the input. It has to be either folders or files. Everything
	// else leads to an error.
	switch inputType {
	case filesType:
		err = ziputil.ZipFiles(sourcePths, tmpZipFilePth)
	case foldersType:
		err = ziputil.ZipDirs(sourcePths, tmpZipFilePth)
	case mixedFileAndFolderType:
		return fmt.Errorf("source path list (%s) contains a mix of files and folders", sourcePths)
	default:
		return fmt.Errorf("source path list (%s) is empty", sourcePths)
	}

	if err != nil {
		return err
	}

	return exportOutputFile(tmpZipFilePth, destinationZipPth, envKey)
}

func zipFilePath() (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export_tmp_dir__")
	if err != nil {
		return "", err
	}

	return filepath.Join(tmpDir, "temp-zip-file.zip"), nil
}

const (
	filesType              = "files"
	foldersType            = "folders"
	mixedFileAndFolderType = "mixed"
)

func getInputType(sourcePths []string) (string, error) {
	var folderCount, fileCount int

	for _, path := range sourcePths {
		exist, err := pathutil.IsDirExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			folderCount++
			continue
		}

		exist, err = pathutil.IsPathExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			fileCount++
		}
	}

	if fileCount == len(sourcePths) {
		return filesType, nil
	} else if folderCount == len(sourcePths) {
		return foldersType, nil
	} else if 0 < folderCount && 0 < fileCount {
		return mixedFileAndFolderType, nil
	}

	return "", nil
}
