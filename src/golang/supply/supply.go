package supply

import (
	"fmt"
	"golang/common"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type Supplier struct {
	Stager     *libbuildpack.Stager
	VendorTool string
	GoVersion  string
	Godep      common.Godep
}

func Run(gs *Supplier) error {
	if err := os.MkdirAll(filepath.Join(gs.Stager.DepDir(), "bin"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(gs.Stager.DepDir(), "env"), 0755); err != nil {
		return err
	}

	if err := gs.InstallVendorTools(); err != nil {
		gs.Stager.Log.Error("Unable to install vendor tools", err.Error())
		return err
	}

	if err := gs.InstallGo(); err != nil {
		gs.Stager.Log.Error("Error installing Go: %s", err.Error())
	}

	return nil
}

func (gs *Supplier) InstallVendorTools() error {
	tools := []string{"godep", "glide"}

	for _, tool := range tools {
		installDir := filepath.Join(gs.Stager.DepDir(), tool)
		if err := gs.Stager.Manifest.InstallOnlyVersion(tool, installDir); err != nil {
			return err
		}

		if err := os.Symlink(filepath.Join(installDir, "bin", tool), filepath.Join(gs.Stager.DepDir(), "bin", tool)); err != nil {
			return err
		}

	}
	return nil
}

func (gs *Supplier) InstallGo() error {
	goInstallDir := filepath.Join(gs.Stager.DepDir(), "go"+gs.GoVersion)

	// goInstalled, err := libbuildpack.FileExists(filepath.Join(goInstallDir, "go"))
	// if err != nil {
	// 	return err
	// }

	// if goInstalled {
	// 	gc.Stager.Log.BeginStep("Using go %s", gc.GoVersion)
	// } else {
	err := gs.Stager.ClearCache()
	if err != nil {
		return fmt.Errorf("clearing cache: %s", err.Error())
	}

	dep := libbuildpack.Dependency{Name: "go", Version: gs.GoVersion}
	err = gs.Stager.Manifest.InstallDependency(dep, goInstallDir)
	if err != nil {
		return err
	}
	// }
	if err := os.Symlink(filepath.Join(goInstallDir, "go", "bin", "go"), filepath.Join(gs.Stager.DepDir(), "bin", "go")); err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(gs.Stager.DepDir(), "env", "GOROOT"), []byte(filepath.Join(goInstallDir, "go")), 0644)
}
