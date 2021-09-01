package cache

import (
	xcodecache "github.com/bitrise-io/go-xcode/xcodecache"
)

// Cache ...
type Cache interface {
	SwiftPackagesPath(projectPth string) (string, error)
	CollectSwiftPackages(projectPath string) error
}

type cache struct {
}

// NewCache ...
func NewCache() Cache {
	return &cache{}
}

func (c cache) SwiftPackagesPath(projectPth string) (string, error) {
	return xcodecache.SwiftPackagesPath(projectPth)
}

func (c cache) CollectSwiftPackages(projectPath string) error {
	return xcodecache.CollectSwiftPackages(projectPath)
}
