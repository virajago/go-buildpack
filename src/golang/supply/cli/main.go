package main

import (
	"golang/supply"
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

	gc := supply.Supplier{
		Compiler: compiler,
		DepDir:   filepath.Join(depsDir, depsIdx),
	}

	err = supply.Run(&gc)
	if err != nil {
		os.Exit(13)
	}
}
