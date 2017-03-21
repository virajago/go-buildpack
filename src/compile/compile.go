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
	gc.Compiler.Log.Info("It works!")

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

	//err = gc.DetermineVendorTool()
	if err != nil {

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

func addToPath(newPaths string) error {
	oldPath := os.Getenv("PATH")
	return os.Setenv("PATH", fmt.Sprintf("%s:%s", newPaths, oldPath))
}
