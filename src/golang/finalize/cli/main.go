package main

import (
	"golang/finalize"
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

	gf := finalize.Finalizer{
		Compiler: compiler,
		DepDir:   filepath.Join(depsDir, depsIdx),
	}
	err = finalize.Run(&gf)
	if err != nil {
		os.Exit(13)
	}
}
