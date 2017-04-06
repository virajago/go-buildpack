package supply

import (
	"fmt"
	"golang/common"
	"io/ioutil"
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
	Godep      common.Godep
}

func Run(gs *Supplier) error {
	var err error

	if err := os.MkdirAll(filepath.Join(gs.DepDir, "bin"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(gs.DepDir, "env"), 0755); err != nil {
		return err
	}

	if err := gs.InstallVendorTools(); err != nil {
		gs.Compiler.Log.Error("Unable to install vendor tools", err.Error())
		return err
	}

	gs.VendorTool, err = common.SelectVendorTool(gs.Compiler, &gs.Godep)
	if err != nil {
		gs.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	if err := gs.SelectGoVersion(); err != nil {
		gs.Compiler.Log.Error("Unable to determine Go version to install: %s", err.Error())
		return err
	}

	if err := gs.InstallGo(); err != nil {
		gs.Compiler.Log.Error("Error installing Go: %s", err.Error())
	}

	return nil
}

func (gs *Supplier) InstallVendorTools() error {
	tools := []string{"godep", "glide"}

	for _, tool := range tools {
		installDir := filepath.Join(gs.DepDir, tool)
		if err := gs.Compiler.Manifest.InstallOnlyVersion(tool, installDir); err != nil {
			return err
		}

		if err := os.Symlink(filepath.Join(installDir, "bin", tool), filepath.Join(gs.DepDir, "bin", tool)); err != nil {
			return err
		}

	}
	return nil
}

func (gs *Supplier) SelectGoVersion() error {
	goVersion := os.Getenv("GOVERSION")

	if gs.VendorTool == "godep" {
		if goVersion != "" {
			gs.Compiler.Log.Warning(common.GoVersionOverride(goVersion))
		} else {
			goVersion = gs.Godep.GoVersion
		}
	} else {
		if goVersion == "" {
			defaultGo, err := gs.Compiler.Manifest.DefaultVersion("go")
			if err != nil {
				return err
			}
			goVersion = fmt.Sprintf("go%s", defaultGo.Version)
		}
	}

	parsed, err := gs.ParseGoVersion(goVersion)
	if err != nil {
		return err
	}

	gs.GoVersion = parsed
	return nil
}

func (gs *Supplier) ParseGoVersion(partialGoVersion string) (string, error) {
	existingVersions := gs.Compiler.Manifest.AllDependencyVersions("go")

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

func (gs *Supplier) InstallGo() error {
	goInstallDir := filepath.Join(gs.DepDir, "go"+gs.GoVersion)

	// goInstalled, err := libbuildpack.FileExists(filepath.Join(goInstallDir, "go"))
	// if err != nil {
	// 	return err
	// }

	// if goInstalled {
	// 	gc.Compiler.Log.BeginStep("Using go %s", gc.GoVersion)
	// } else {
	err := gs.Compiler.ClearCache()
	if err != nil {
		return fmt.Errorf("clearing cache: %s", err.Error())
	}

	dep := libbuildpack.Dependency{Name: "go", Version: gs.GoVersion}
	err = gs.Compiler.Manifest.InstallDependency(dep, goInstallDir)
	if err != nil {
		return err
	}
	// }
	if err := os.Symlink(filepath.Join(goInstallDir, "go", "bin", "go"), filepath.Join(gs.DepDir, "bin", "go")); err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(gs.DepDir, "env", "GOROOT"), []byte(filepath.Join(goInstallDir, "go")), 0644)
}
