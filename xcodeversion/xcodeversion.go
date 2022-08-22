package xcodeversion

import (
	"github.com/bitrise-io/go-xcode/models"
	"github.com/bitrise-io/go-xcode/utility"
)

type Version models.XcodebuildVersionModel

type Reader interface {
	Version() (Version, error)
}

type reader struct{}

func NewXcodeVersionReader() Reader {
	return &reader{}
}

func (b *reader) Version() (Version, error) {
	version, err := utility.GetXcodeVersion()
	return Version(version), err
}
