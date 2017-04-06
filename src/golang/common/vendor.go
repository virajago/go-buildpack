package common

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type Godep struct {
	ImportPath      string   `json:"ImportPath"`
	GoVersion       string   `json:"GoVersion"`
	Packages        []string `json:"Packages"`
	WorkspaceExists bool
}

func SelectVendorTool(s *libbuildpack.Stager, godep *Godep) (string, error) {
	godepsJSONFile := filepath.Join(s.BuildDir, "Godeps", "Godeps.json")

	godirFile := filepath.Join(s.BuildDir, ".godir")
	isGodir, err := libbuildpack.FileExists(godirFile)
	if err != nil {
		return "", err
	}
	if isGodir {
		s.Log.Error(GodirError())
		return "", errors.New(".godir deprecated")
	}

	isGoPath, err := isGoPath(s.BuildDir)
	if err != nil {
		return "", err
	}
	if isGoPath {
		s.Log.Error(GBError())
		return "", errors.New("gb unsupported")
	}

	isGodep, err := libbuildpack.FileExists(godepsJSONFile)
	if err != nil {
		return "", err
	}
	if isGodep {
		s.Log.BeginStep("Checking Godeps/Godeps.json file")

		err = libbuildpack.NewJSON().Load(filepath.Join(s.BuildDir, "Godeps", "Godeps.json"), godep)
		if err != nil {
			s.Log.Error("Bad Godeps/Godeps.json file")
			return "", err
		}

		godep.WorkspaceExists, err = libbuildpack.FileExists(filepath.Join(s.BuildDir, "Godeps", "_workspace", "src"))
		if err != nil {
			return "", err
		}

		return "godep", nil
	}

	glideFile := filepath.Join(s.BuildDir, "glide.yaml")
	isGlide, err := libbuildpack.FileExists(glideFile)
	if err != nil {
		return "", err
	}
	if isGlide {
		return "glide", nil
	}

	return "go_nativevendoring", nil
}

func isGoPath(buildDir string) (bool, error) {
	srcDir := filepath.Join(buildDir, "src")
	srcDirAtAppRoot, err := libbuildpack.FileExists(srcDir)
	if err != nil {
		return false, err
	}

	if !srcDirAtAppRoot {
		return false, nil
	}

	files, err := ioutil.ReadDir(filepath.Join(buildDir, "src"))
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.Mode().IsDir() {
			err = filepath.Walk(filepath.Join(srcDir, file.Name()), isGoFile)
			if err != nil {
				if err.Error() == "found Go file" {
					return true, nil
				}

				return false, err
			}
		}
	}

	return false, nil
}

func isGoFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if strings.HasSuffix(path, ".go") {
		return errors.New("found Go file")
	}

	return nil
}
