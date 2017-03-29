package main

import (
	"compile/golang"
	"os"

	"github.com/cloudfoundry/libbuildpack"
)

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

	gc := golang.Compiler{Compiler: compiler}
	err = compile(&gc)
	if err != nil {
		panic(err)
	}

	compiler.StagingComplete()
}

func compile(gc *golang.Compiler) error {
	var err error

	if err := gc.SelectVendorTool(); err != nil {
		gc.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	if err := gc.InstallVendorTool("/tmp"); err != nil {
		gc.Compiler.Log.Error("Unable to install %s: %s", gc.VendorTool, err.Error())
		return err
	}

	if err := gc.SelectGoVersion(); err != nil {
		gc.Compiler.Log.Error("Unable to determine Go version to install: %s", err.Error())
		return err
	}

	if err := gc.InstallGo(); err != nil {
		gc.Compiler.Log.Error("Error installing Go: %s", err.Error())
	}

	if err := gc.SetMainPackageName(); err != nil {
		gc.Compiler.Log.Error("Unable to determine import path: %s", err.Error())
		return err
	}

	if err := gc.CheckBinDirectory(); err != nil {
		gc.Compiler.Log.Error("Error checking bin directory: %s", err.Error())
		return err
	}

	if err := gc.SetupGoPath(); err != nil {
		gc.Compiler.Log.Error("Unable to setup Go path: %s", err.Error())
		return err
	}

	gc.SetBuildFlags()
	if err = gc.SetInstallPackages(); err != nil {
		gc.Compiler.Log.Error("Unable to determine packages to install: %s", err.Error())
		return err
	}

	if err := gc.CompileApp(); err != nil {
		gc.Compiler.Log.Error("Unable to compile application: %s", err.Error())
		return err
	}

	if err := gc.CreateStartupEnvironment("/tmp"); err != nil {
		gc.Compiler.Log.Error("Unable to create startup scripts: %s", err.Error())
		return err
	}

	return nil
}
