package main

import (
	"golang/finalize"
	"golang/supply"

	"os"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

func main() {
	buildDir := os.Args[1]
	cacheDir := os.Args[2]

	logger := libbuildpack.NewLogger()
	depsDir := filepath.Join(buildDir, ".cloudfoundry")
	depsIdx := "0"

	if err := os.MkdirAll(filepath.Join(depsDir, depsIdx), 0755); err != nil {
		logger.Error(err.Error())
		os.Exit(10)
	}

	compiler, err := libbuildpack.NewCompiler([]string{buildDir, cacheDir, "", depsDir}, logger)
	err = compiler.CheckBuildpackValid()
	if err != nil {
		os.Exit(11)
	}

	// err = libbuildpack.RunBeforeCompile(compiler)
	// if err != nil {
	// 	compiler.Log.Error("Before Compile: %s", err.Error())
	// 	os.Exit(12)
	// }

	gs := supply.Supplier{
		Compiler: compiler,
		DepDir:   filepath.Join(depsDir, depsIdx),
	}

	err = supply.Run(&gs)
	if err != nil {
		os.Exit(12)
	}

	gf := finalize.Finalizer{
		Compiler: compiler,
		DepDir:   filepath.Join(depsDir, depsIdx),
	}

	err = finalize.Run(&gf)
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
