package main

import (
	"os"

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
	return nil
}
