package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// Project is the part of a project.pbxproj the selection needs.
type Project struct {
	// Path is the absolute path of the .xcodeproj bundle.
	Path string
	// Dir is the directory source paths are relative to (projectDirPath applied).
	Dir  string
	Name string
	// Targets in declaration order.
	Targets []*Target
	// LocalPackages are absolute directories of XCLocalSwiftPackageReference entries.
	LocalPackages []string

	objects map[string]map[string]interface{}
	parents map[string]string
}

// Target is one PBXNativeTarget (or aggregate/legacy target).
type Target struct {
	ID          string
	Name        string
	ProductType string
	// ProductFile is the product reference's file name, e.g. "App.app" or "Core.framework".
	ProductFile string
	IsTest      bool
	// Files are absolute paths of the files in the target's build phases.
	Files []string
	// Dirs are absolute directories of file-system-synchronized groups.
	Dirs []string
	// DependencyIDs are the IDs of targets this one depends on (same or other project).
	DependencyIDs []string
	// LinkedProducts are product file names from the frameworks build phase ("Core.framework").
	LinkedProducts []string
	// PackageProducts are Swift package product names the target depends on.
	PackageProducts []string
	// TestHostProduct is the product name from TEST_HOST / BUNDLE_LOADER ("App").
	TestHostProduct string
	// TestTargetName is TEST_TARGET_NAME for UI test bundles.
	TestTargetName string
}

// ParseProject reads an .xcodeproj bundle.
func ParseProject(xcodeprojPath string) (*Project, error) {
	abs, err := filepath.Abs(xcodeprojPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "project.pbxproj"))
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", xcodeprojPath, err)
	}
	rawObjects, _ := root["objects"].(map[string]interface{})
	p := &Project{
		Path:    abs,
		Name:    strings.TrimSuffix(filepath.Base(abs), ".xcodeproj"),
		objects: map[string]map[string]interface{}{},
		parents: map[string]string{},
	}
	for id, o := range rawObjects {
		if m, ok := o.(map[string]interface{}); ok {
			p.objects[id] = m
		}
	}
	rootID, _ := root["rootObject"].(string)
	proj := p.objects[rootID]
	if proj == nil {
		return nil, fmt.Errorf("parse %s: rootObject %q missing", xcodeprojPath, rootID)
	}
	p.Dir = filepath.Join(filepath.Dir(abs), str(proj, "projectDirPath"))

	var folderRefs []string
	for id, o := range p.objects {
		switch str(o, "isa") {
		case "PBXGroup", "PBXVariantGroup", "XCVersionGroup", "PBXFileSystemSynchronizedRootGroup":
			for _, child := range list(o, "children") {
				p.parents[child] = id
			}
		case "XCLocalSwiftPackageReference":
			if rel := str(o, "relativePath"); rel != "" {
				p.LocalPackages = append(p.LocalPackages, filepath.Join(p.Dir, rel))
			}
		case "PBXFileReference":
			// Before Xcode 15 a local package was a plain folder reference dragged into the
			// project; Xcode writes it as "folder" or "wrapper" and it is still common.
			switch str(o, "lastKnownFileType") {
			case "folder", "wrapper":
				folderRefs = append(folderRefs, id)
			}
		}
	}
	for _, id := range folderRefs {
		pth, ok := p.resolvePath(id)
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(pth, "Package.swift")); err == nil {
			p.LocalPackages = append(p.LocalPackages, filepath.Clean(pth))
		}
	}

	for _, tid := range list(proj, "targets") {
		t := p.parseTarget(tid)
		if t != nil {
			p.Targets = append(p.Targets, t)
		}
	}
	return p, nil
}

func (p *Project) parseTarget(id string) *Target {
	o := p.objects[id]
	if o == nil {
		return nil
	}
	t := &Target{ID: id, Name: str(o, "name"), ProductType: str(o, "productType")}
	if ref := p.objects[str(o, "productReference")]; ref != nil {
		t.ProductFile = filepath.Base(str(ref, "path"))
	}
	t.IsTest = strings.Contains(t.ProductType, "unit-test") || strings.Contains(t.ProductType, "ui-testing")

	for _, depID := range list(o, "dependencies") {
		dep := p.objects[depID]
		if dep == nil {
			continue
		}
		if target := str(dep, "target"); target != "" {
			t.DependencyIDs = append(t.DependencyIDs, target)
			continue
		}
		if proxy := p.objects[str(dep, "targetProxy")]; proxy != nil {
			if remote := str(proxy, "remoteGlobalIDString"); remote != "" {
				t.DependencyIDs = append(t.DependencyIDs, remote)
			}
		}
	}

	for _, phaseID := range list(o, "buildPhases") {
		phase := p.objects[phaseID]
		if phase == nil {
			continue
		}
		isa := str(phase, "isa")
		for _, bfID := range list(phase, "files") {
			bf := p.objects[bfID]
			if bf == nil {
				continue
			}
			if prod := p.objects[str(bf, "productRef")]; prod != nil {
				t.PackageProducts = append(t.PackageProducts, str(prod, "productName"))
				continue
			}
			refID := str(bf, "fileRef")
			ref := p.objects[refID]
			if ref == nil {
				continue
			}
			if isa == "PBXFrameworksBuildPhase" {
				t.LinkedProducts = append(t.LinkedProducts, filepath.Base(str(ref, "path")))
				continue
			}
			if pth, ok := p.resolvePath(refID); ok {
				// Asset catalogs, data models, localized folders and folder references
				// are single entries in the project but whole directories on disk.
				if st, err := os.Stat(pth); err == nil && st.IsDir() {
					t.Dirs = append(t.Dirs, pth)
				} else {
					t.Files = append(t.Files, pth)
				}
			}
		}
	}

	for _, prod := range list(o, "packageProductDependencies") {
		if pd := p.objects[prod]; pd != nil {
			t.PackageProducts = append(t.PackageProducts, str(pd, "productName"))
		}
	}

	for _, gid := range list(o, "fileSystemSynchronizedGroups") {
		if pth, ok := p.resolvePath(gid); ok {
			t.Dirs = append(t.Dirs, pth)
		}
	}

	settings := p.buildSettings(str(o, "buildConfigurationList"))
	// Files referenced only through build settings, not through a build phase.
	for _, key := range []string{"INFOPLIST_FILE", "CODE_SIGN_ENTITLEMENTS"} {
		v := strings.TrimPrefix(settings[key], "$(SRCROOT)/")
		if v != "" && !strings.Contains(v, "$(") {
			t.Files = append(t.Files, filepath.Join(p.Dir, v))
		}
	}
	t.TestHostProduct = hostProduct(settings["TEST_HOST"])
	if t.TestHostProduct == "" {
		t.TestHostProduct = hostProduct(settings["BUNDLE_LOADER"])
	}
	t.TestTargetName = settings["TEST_TARGET_NAME"]
	return t
}

// buildSettings merges the build settings of every configuration; the first
// configuration wins on conflicts, which is enough for TEST_HOST-style keys.
func (p *Project) buildSettings(listID string) map[string]string {
	out := map[string]string{}
	cl := p.objects[listID]
	if cl == nil {
		return out
	}
	for _, cid := range list(cl, "buildConfigurations") {
		conf := p.objects[cid]
		if conf == nil {
			continue
		}
		bs, _ := conf["buildSettings"].(map[string]interface{})
		for k, v := range bs {
			if s, ok := v.(string); ok {
				if _, exists := out[k]; !exists {
					out[k] = s
				}
			}
		}
	}
	return out
}

// resolvePath returns the absolute path of a file reference or group by walking the
// group tree. Build-product and SDK references are not source files and return false.
func (p *Project) resolvePath(id string) (string, bool) {
	o := p.objects[id]
	if o == nil {
		return "", false
	}
	pth := str(o, "path")
	switch str(o, "sourceTree") {
	case "<absolute>":
		return pth, true
	case "SOURCE_ROOT":
		return filepath.Join(p.Dir, pth), true
	case "<group>", "":
		base := p.Dir
		if parent, ok := p.parents[id]; ok {
			b, ok := p.resolvePath(parent)
			if !ok {
				return "", false
			}
			base = b
		}
		return filepath.Join(base, pth), true
	default:
		return "", false
	}
}

// hostProduct extracts the product name from a TEST_HOST-style value such as
// "$(BUILT_PRODUCTS_DIR)/App.app/App" or ".../App.app/Contents/MacOS/App".
func hostProduct(v string) string {
	for _, part := range strings.Split(v, "/") {
		if strings.HasSuffix(part, ".app") {
			return strings.TrimSuffix(part, ".app")
		}
	}
	return ""
}

func str(o map[string]interface{}, key string) string {
	s, _ := o[key].(string)
	return s
}

func list(o map[string]interface{}, key string) []string {
	raw, _ := o[key].([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
