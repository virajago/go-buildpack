package supply

import (
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type Supplier struct {
	Stager     *libbuildpack.Stager
	VendorTool string
	GoVersion  string
}

func Run(gs *Supplier) error {
	if err := gs.InstallVendorTools(); err != nil {
		gs.Stager.Log.Error("Unable to install vendor tools", err.Error())
		return err
	}

	if err := gs.InstallGo(); err != nil {
		gs.Stager.Log.Error("Error installing Go: %s", err.Error())
		return err
	}

	if err := gs.Stager.WriteConfigYml(); err != nil {
		gs.Stager.Log.Error("Error writing config.yml: %s", err.Error())
		return err
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

		if err := gs.Stager.AddBinDependencyLink(filepath.Join(installDir, "bin", tool), tool); err != nil {
			return err
		}

	}
	return nil
}

func (gs *Supplier) InstallGo() error {
	goInstallDir := filepath.Join(gs.Stager.DepDir(), "go"+gs.GoVersion)

	dep := libbuildpack.Dependency{Name: "go", Version: gs.GoVersion}
	if err := gs.Stager.Manifest.InstallDependency(dep, goInstallDir); err != nil {
		return err
	}

	if err := gs.Stager.AddBinDependencyLink(filepath.Join(goInstallDir, "go", "bin", "go"), "go"); err != nil {
		return err
	}

	return gs.Stager.WriteEnvFile("GOROOT", filepath.Join(goInstallDir, "go"))
}
