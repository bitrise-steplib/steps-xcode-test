package export

import "github.com/bitrise-io/go-utils/v2/fileutil"

// CopyOptions configures a [FileManager.CopyFile] operation.
// A nil pointer means default behavior.
type CopyOptions = fileutil.CopyOptions

// FileManager defines file management operations.
type FileManager = fileutil.FileManager

// NewFileManager creates a new FileManager instance.
func NewFileManager() FileManager {
	return fileutil.NewFileManager()
}
