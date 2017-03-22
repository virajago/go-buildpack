package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type GoCompiler struct {
	Compiler *libbuildpack.Compiler
}

func main() {
	compiler, err := libbuildpack.NewCompiler(os.Args[1:], libbuildpack.NewLogger())
	err = compiler.CheckBuildpackValid()
	if err != nil {
		panic(err)
	}

	err = compiler.LoadSuppliedDeps()
	if err != nil {
		panic(err)
	}

	gc := GoCompiler{Compiler: compiler}
	err = gc.Compile()
	if err != nil {
		panic(err)
	}

	compiler.StagingComplete()
}

func (gc *GoCompiler) Compile() error {
	err := gc.InstallGodep("/tmp/godep")
	if err != nil {
		gc.Compiler.Log.Error("Unable to install godep: %s", err.Error())
		return err
	}

	err = gc.InstallGlide("/tmp/glide")
	if err != nil {
		gc.Compiler.Log.Error("Unable to install glide: %s", err.Error())
		return err
	}

	_, _, _, err = gc.SelectVendorTool()
	if err != nil {
		gc.Compiler.Log.Error("Unable to select go vendor tool: %s", err.Error())
		return err
	}

	return nil
}

func (gc *GoCompiler) InstallGodep(installDir string) error {
	gc.Compiler.Log.BeginStep("Installing godep")

	godep, err := gc.Compiler.Manifest.DefaultVersion("godep")
	if err != nil {
		return err
	}
	gc.Compiler.Log.Info("godep version: %s", godep.Version)

	err = os.MkdirAll(installDir, 0755)
	if err != nil {
		return err
	}

	err = gc.Compiler.Manifest.InstallDependency(godep, installDir)
	if err != nil {
		return err
	}

	return addToPath(filepath.Join(installDir, "bin"))
}

func (gc *GoCompiler) InstallGlide(installDir string) error {
	gc.Compiler.Log.BeginStep("Installing glide")

	glide, err := gc.Compiler.Manifest.DefaultVersion("glide")
	if err != nil {
		return err
	}
	gc.Compiler.Log.Info("glide version: %s", glide.Version)

	err = os.MkdirAll(installDir, 0755)
	if err != nil {
		return err
	}

	err = gc.Compiler.Manifest.InstallDependency(glide, installDir)
	if err != nil {
		return err
	}

	return addToPath(filepath.Join(installDir, "bin"))
}

type godepsJSON struct {
	ImportPath string `json:"ImportPath"`
	GoVersion  string `json:"GoVersion"`
}

func (gc *GoCompiler) SelectVendorTool() (vendorTool, goVersion, goPackageName string, err error) {
	godepsJSONFile := filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json")
	isGodep, err := libbuildpack.FileExists(godepsJSONFile)
	if err != nil {
		return "", "", "", err
	}
	if isGodep {
		gc.Compiler.Log.BeginStep("Checking Godeps/Godeps.json file")
		var godeps godepsJSON
		err = libbuildpack.NewJSON().Load(godepsJSONFile, &godeps)
		if err != nil {
			gc.Compiler.Log.Error("Bad Godeps/Godeps.json file")
			return "", "", "", err
		}

		envGoVersion := os.Getenv("GOVERSION")
		if envGoVersion != "" {
			goVersion = envGoVersion
			gc.Compiler.Log.Warning(goVersionOverride(envGoVersion))
		} else {
			goVersion = godeps.GoVersion
		}

		return "godep", goVersion, godeps.ImportPath, nil
	}
	godirFile := filepath.Join(gc.Compiler.BuildDir, ".godir")
	isGodir, err := libbuildpack.FileExists(godirFile)
	if err != nil {
		return "", "", "", err
	}

	if isGodir {
		gc.Compiler.Log.Error(godirError())
		return "", "", "", fmt.Errorf(".godir deprecated")
	}

	return "", "", "", nil
}

func addToPath(newPaths string) error {
	oldPath := os.Getenv("PATH")
	return os.Setenv("PATH", fmt.Sprintf("%s:%s", newPaths, oldPath))
}
