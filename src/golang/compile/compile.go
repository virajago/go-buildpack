package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type Supplier struct {
	Compiler   *libbuildpack.Compiler
	DepDir     string
	VendorTool string
	GoVersion  string
	Godep      Godep
}

type Godep struct {
	ImportPath      string   `json:"ImportPath"`
	GoVersion       string   `json:"GoVersion"`
	Packages        []string `json:"Packages"`
	WorkspaceExists bool
}

func (gc *Supplier) SelectVendorTool() error {
	godepsJSONFile := filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json")

	godirFile := filepath.Join(gc.Compiler.BuildDir, ".godir")
	isGodir, err := libbuildpack.FileExists(godirFile)
	if err != nil {
		return err
	}
	if isGodir {
		gc.Compiler.Log.Error(godirError())
		return errors.New(".godir deprecated")
	}

	isGB, err := gc.isGB()
	if err != nil {
		return err
	}
	if isGB {
		gc.Compiler.Log.Error(gbError())
		return errors.New("gb unsupported")
	}

	isGodep, err := libbuildpack.FileExists(godepsJSONFile)
	if err != nil {
		return err
	}
	if isGodep {
		gc.Compiler.Log.BeginStep("Checking Godeps/Godeps.json file")

		err = libbuildpack.NewJSON().Load(filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json"), &gc.Godep)
		if err != nil {
			gc.Compiler.Log.Error("Bad Godeps/Godeps.json file")
			return err
		}

		gc.Godep.WorkspaceExists, err = libbuildpack.FileExists(filepath.Join(gc.Compiler.BuildDir, "Godeps", "_workspace", "src"))
		if err != nil {
			return err
		}

		gc.VendorTool = "godep"
		return nil
	}

	glideFile := filepath.Join(gc.Compiler.BuildDir, "glide.yaml")
	isGlide, err := libbuildpack.FileExists(glideFile)
	if err != nil {
		return err
	}
	if isGlide {
		gc.VendorTool = "glide"
		return nil
	}

	gc.VendorTool = "go_nativevendoring"
	return nil
}

func (gc *Supplier) InstallVendorTools(tmpDir string) error {
	if gc.VendorTool == "go_nativevendoring" {
		return nil
	}

	installDir := filepath.Join(tmpDir, gc.VendorTool)

	err := gc.Compiler.Manifest.InstallOnlyVersion(gc.VendorTool, installDir)
	if err != nil {
		return err
	}

	return addToPath(filepath.Join(installDir, "bin"))
}

func (gc *Supplier) SelectGoVersion() error {
	goVersion := os.Getenv("GOVERSION")

	if gc.VendorTool == "godep" {
		if goVersion != "" {
			gc.Compiler.Log.Warning(goVersionOverride(goVersion))
		} else {
			goVersion = gc.Godep.GoVersion
		}
	} else {
		if goVersion == "" {
			defaultGo, err := gc.Compiler.Manifest.DefaultVersion("go")
			if err != nil {
				return err
			}
			goVersion = fmt.Sprintf("go%s", defaultGo.Version)
		}
	}

	parsed, err := gc.ParseGoVersion(goVersion)
	if err != nil {
		return err
	}

	gc.GoVersion = parsed
	return nil
}

func (gc *Supplier) ParseGoVersion(partialGoVersion string) (string, error) {
	existingVersions := gc.Compiler.Manifest.AllDependencyVersions("go")

	if len(strings.Split(partialGoVersion, ".")) < 3 {
		partialGoVersion += ".x"
	}

	strippedGoVersion := strings.TrimLeft(partialGoVersion, "go")

	expandedVer, err := libbuildpack.FindMatchingVersion(strippedGoVersion, existingVersions)
	if err != nil {
		return "", err
	}

	return expandedVer, nil
}

func (gc *Supplier) InstallGo() error {
	err := os.MkdirAll(filepath.Join(gc.Compiler.BuildDir, "bin"), 0755)
	if err != nil {
		return err
	}

	goInstallDir := gc.goInstallLocation()

	goInstalled, err := libbuildpack.FileExists(filepath.Join(goInstallDir, "go"))
	if err != nil {
		return err
	}

	if goInstalled {
		gc.Compiler.Log.BeginStep("Using go %s", gc.GoVersion)
	} else {
		err = gc.Compiler.ClearCache()
		if err != nil {
			return fmt.Errorf("clearing cache: %s", err.Error())
		}

		dep := libbuildpack.Dependency{Name: "go", Version: gc.GoVersion}
		err = gc.Compiler.Manifest.InstallDependency(dep, goInstallDir)
		if err != nil {
			return err
		}
	}

	err = os.Setenv("GOROOT", filepath.Join(goInstallDir, "go"))
	if err != nil {
		return err
	}

	return addToPath(filepath.Join(goInstallDir, "go", "bin"))
}
