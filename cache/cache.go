package cache

import (
	xcodecache "github.com/bitrise-io/go-xcode/xcodecache"
)

// SwiftPackageCache ...
type SwiftPackageCache interface {
	SwiftPackagesPath(projectPth string) (string, error)
	CollectSwiftPackages(projectPath string) error
}

type cache struct {
}

// NewSwiftPackageCache ...
func NewSwiftPackageCache() SwiftPackageCache {
	return &cache{}
}

func (c cache) SwiftPackagesPath(projectPth string) (string, error) {
	return xcodecache.SwiftPackagesPath(projectPth)
}

func (c cache) CollectSwiftPackages(projectPath string) error {
	return xcodecache.CollectSwiftPackages(projectPath)
}
