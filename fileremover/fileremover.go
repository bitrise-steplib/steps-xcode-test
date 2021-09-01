package fileremover

import "os"

// FileRemover ...
type FileRemover interface {
	Remove(name string) error
	RemoveAll(path string) error
}

type fileRemover struct{}

// NewFileRemover ...
func NewFileRemover() FileRemover {
	return fileRemover{}
}

func (r fileRemover) Remove(name string) error {
	return os.Remove(name)
}

func (r fileRemover) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
