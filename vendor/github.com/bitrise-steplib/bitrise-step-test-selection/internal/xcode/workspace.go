package xcode

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// Workspace lists what an .xcworkspace refers to.
type Workspace struct {
	Path     string
	Projects []string
	// PackageDirs are referenced directories that contain a Package.swift.
	PackageDirs []string
}

type wsGroup struct {
	Location string    `xml:"location,attr"`
	FileRefs []wsRef   `xml:"FileRef"`
	Groups   []wsGroup `xml:"Group"`
}

type wsRef struct {
	Location string `xml:"location,attr"`
}

// ParseWorkspace reads contents.xcworkspacedata.
func ParseWorkspace(workspacePath string) (*Workspace, error) {
	abs, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "contents.xcworkspacedata"))
	if err != nil {
		return nil, err
	}
	var root wsGroup
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	ws := &Workspace{Path: abs}
	wsDir := filepath.Dir(abs)
	ws.collect(root, wsDir, wsDir)
	return ws, nil
}

func (ws *Workspace) collect(g wsGroup, wsDir, base string) {
	for _, ref := range g.FileRefs {
		pth := resolveLocation(ref.Location, wsDir, base)
		if pth == "" {
			continue
		}
		switch {
		case strings.HasSuffix(pth, ".xcodeproj"):
			ws.Projects = append(ws.Projects, pth)
		default:
			if _, err := os.Stat(filepath.Join(pth, "Package.swift")); err == nil {
				ws.PackageDirs = append(ws.PackageDirs, pth)
			}
		}
	}
	for _, sub := range g.Groups {
		subBase := base
		if loc := resolveLocation(sub.Location, wsDir, base); loc != "" {
			subBase = loc
		}
		ws.collect(sub, wsDir, subBase)
	}
}

// resolveLocation turns a workspace location ("group:App.xcodeproj", "container:x",
// "self:", "absolute:/x") into an absolute path.
func resolveLocation(loc, wsDir, base string) string {
	scheme, rest, ok := strings.Cut(loc, ":")
	if !ok {
		return ""
	}
	switch scheme {
	case "group":
		return filepath.Join(base, rest)
	case "container":
		return filepath.Join(wsDir, rest)
	case "self":
		// "self:" inside <proj>.xcodeproj/project.xcworkspace refers to the enclosing project.
		if strings.HasSuffix(wsDir, ".xcodeproj") {
			return wsDir
		}
		return filepath.Join(wsDir, rest)
	case "absolute":
		return rest
	}
	return ""
}
