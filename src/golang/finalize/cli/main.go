package main

import (
	"golang"
	"golang/finalize"
	"os"

	"github.com/cloudfoundry/libbuildpack"
)

func main() {
	stager, err := libbuildpack.NewStager(os.Args[1:], libbuildpack.NewLogger())
	err = stager.CheckBuildpackValid()
	if err != nil {
		os.Exit(10)
	}

	err = libbuildpack.SetStagingEnvironment(stager.DepsDir)
	if err != nil {
		stager.Log.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(11)
	}

	var godep golang.Godep

	vendorTool, err := golang.SelectVendorTool(stager, &godep)
	if err != nil {
		stager.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		os.Exit(12)
	}

	goVersion, err := golang.SelectGoVersion(stager, vendorTool, godep)
	if err != nil {
		stager.Log.Error("Unable to select Go version: %s", err.Error())
		os.Exit(13)
	}

	gf := finalize.Finalizer{
		Stager:     stager,
		Godep:      godep,
		GoVersion:  goVersion,
		VendorTool: vendorTool,
	}

	err = finalize.Run(&gf)
	if err != nil {
		os.Exit(14)
	}

	err = libbuildpack.SetLaunchEnvironment(stager.DepsDir, stager.BuildDir)
	if err != nil {
		stager.Log.Error("Unable to setup launch environment: %s", err.Error())
		os.Exit(15)
	}

	err = libbuildpack.RunAfterCompile(stager)
	if err != nil {
		stager.Log.Error("After Compile: %s", err.Error())
		os.Exit(16)
	}

	stager.StagingComplete()
}
