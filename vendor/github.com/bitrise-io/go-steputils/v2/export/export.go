package export

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
)

const (
	filesType              = "files"
	foldersType            = "folders"
	mixedFileAndFolderType = "mixed"
)

// Exporter ...
type Exporter struct {
	cmdFactory  command.Factory
	fileManager FileManager
}

// NewExporter ...
func NewExporter(cmdFactory command.Factory) Exporter {
	return Exporter{
		cmdFactory:  cmdFactory,
		fileManager: NewFileManager(),
	}
}

// ExportOutput is used for exposing values for other steps.
// Regular env vars are isolated between steps, so instead of calling `os.Setenv()`, use this to explicitly expose
// a value for subsequent steps.
func (e *Exporter) ExportOutput(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value}, nil)
	return runExport(cmd)
}

// ExportOutputNoExpand works like ExportOutput but does not expand environment variables in the value.
// This can be used when the value is unstrusted or is beyond the control of the step.
func (e *Exporter) ExportOutputNoExpand(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value, "--no-expand"}, nil)
	return runExport(cmd)
}

// ExportSecretOutput is used for exposing secret values for other steps.
// Regular env vars are isolated between steps, so instead of calling `os.Setenv()`, use this to explicitly expose
// a secret value for subsequent steps.
func (e *Exporter) ExportSecretOutput(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value, "--sensitive"}, nil)
	return runExport(cmd)
}

// ExportOutputFile is a convenience method for copying sourcePath to destinationPath and then exporting the
// absolute destination path with ExportOutput()
func (e *Exporter) ExportOutputFile(key, sourcePath, destinationPath string) error {
	pathModifier := pathutil.NewPathModifier()
	absSourcePath, err := pathModifier.AbsPath(sourcePath)
	if err != nil {
		return err
	}
	absDestinationPath, err := pathModifier.AbsPath(destinationPath)
	if err != nil {
		return err
	}

	if absSourcePath != absDestinationPath {
		if err = e.fileManager.CopyFile(absSourcePath, absDestinationPath); err != nil {
			return err
		}
	}

	return e.ExportOutput(key, absDestinationPath)
}

// ExportOutputFilesZip is a convenience method for creating a ZIP archive from sourcePaths at zipPath and then
// exporting the absolute path of the ZIP with ExportOutput()
func (e *Exporter) ExportOutputFilesZip(key string, sourcePaths []string, zipPath string) error {
	tempZipPath, err := zipFilePath()
	if err != nil {
		return err
	}

	// We have separate zip functions for files and folders and that is the main reason we cannot have mixed
	// paths (files and also folders) in the input. It has to be either folders or files. Everything
	// else leads to an error.
	inputType, err := getInputType(sourcePaths)
	if err != nil {
		return err
	}
	switch inputType {
	case filesType:
		err = ziputil.ZipFiles(sourcePaths, tempZipPath)
	case foldersType:
		err = ziputil.ZipDirs(sourcePaths, tempZipPath)
	case mixedFileAndFolderType:
		return fmt.Errorf("source path list (%s) contains a mix of files and folders", sourcePaths)
	default:
		return fmt.Errorf("source path list (%s) is empty", sourcePaths)
	}

	if err != nil {
		return err
	}

	return e.ExportOutputFile(key, tempZipPath, zipPath)
}

// ExportOutputDir is a convenience method for copying sourceDir to destinationDir and then exporting the
// absolute destination dir with ExportOutput()
// Note: symlinks are preserved during the copy operation
func (e *Exporter) ExportOutputDir(envKey, srcDir, dstDir string) error {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	srcInfo, err := e.fileManager.Lstat(srcDir)
	if err != nil {
		return fmt.Errorf("stat src root: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("src is not a directory: %s", srcDir)
	}

	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		return err
	}

	if srcDir == dstDir {
		return e.ExportOutput(envKey, dstDir)
	}

	if err := e.fileManager.CopyDir(srcDir, dstDir); err != nil {
		return err
	}

	return e.ExportOutput(envKey, dstDir)
}

// ExportStringToFileOutput is a convenience method for writing content to dst and then exporting the
// absolute dst path with ExportOutputFile()
func (e *Exporter) ExportStringToFileOutput(envKey, content, dst string) error {
	if err := e.fileManager.WriteBytes(dst, []byte(content)); err != nil {
		return err
	}

	return e.ExportOutputFile(envKey, dst, dst)
}

// ExportStringToFileOutputAndReturnLastNLines is similar to ExportStringToFileOutput but it also returns the
// last N lines of the content.
func (e *Exporter) ExportStringToFileOutputAndReturnLastNLines(envKey, content, dst string, lines int) (string, error) {
	if err := e.ExportStringToFileOutput(envKey, content, dst); err != nil {
		return "", err
	}

	return e.fileManager.LastNLines(content, lines), nil
}

func zipFilePath() (string, error) {
	tmpDir, err := pathutil.NewPathProvider().CreateTempDir("__export_tmp_dir__")
	if err != nil {
		return "", err
	}

	return filepath.Join(tmpDir, "temp-zip-file.zip"), nil
}

func getInputType(sourcePths []string) (string, error) {
	var folderCount, fileCount int
	pathChecker := pathutil.NewPathChecker()

	for _, path := range sourcePths {
		exist, err := pathChecker.IsDirExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			folderCount++
			continue
		}

		exist, err = pathChecker.IsPathExists(path)
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

func runExport(cmd command.Command) error {
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("exporting output with envman failed: %s, output: %s", err, out)
	}
	return nil
}
