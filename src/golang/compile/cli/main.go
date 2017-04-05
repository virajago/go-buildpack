package main

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

func main() {
	buildDir := os.Args[1]
	cacheDir := os.Args[2]
	depsDir := os.Args[3]
	depsIdx := os.Args[4]

	compiler, err := libbuildpack.NewCompiler([]string{buildDir, cacheDir, "", depsDir}, libbuildpack.NewLogger())
	err = compiler.CheckBuildpackValid()
	if err != nil {
		os.Exit(10)
	}

	// err = libbuildpack.RunBeforeCompile(compiler)
	// if err != nil {
	// 	compiler.Log.Error("Before Compile: %s", err.Error())
	// 	os.Exit(12)
	// }

	gc := Supplier{
		Compiler: compiler,
		DepDir:   filepath.Join(depsDir, depsIdx),
	}

	err = Run(&gc)
	if err != nil {
		os.Exit(13)
	}

	err = libbuildpack.RunAfterCompile(compiler)
	if err != nil {
		compiler.Log.Error("After Compile: %s", err.Error())
		os.Exit(14)
	}

	compiler.StagingComplete()
}

func Run(gc *Supplier) error {
	var err error

	if err := gc.InstallVendorTools(); err != nil {
		gc.Compiler.Log.Error("Unable to install %s: %s", gc.VendorTool, err.Error())
		return err
	}

	if err := gc.SelectVendorTool(); err != nil {
		gc.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	if err := gc.SelectGoVersion(); err != nil {
		gc.Compiler.Log.Error("Unable to determine Go version to install: %s", err.Error())
		return err
	}

	if err := gc.InstallGo(); err != nil {
		gc.Compiler.Log.Error("Error installing Go: %s", err.Error())
	}

	return nil
}
