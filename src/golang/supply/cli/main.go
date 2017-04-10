package main

import (
	"golang"
	"golang/supply"
	"os"

	"github.com/cloudfoundry/libbuildpack"
)

func main() {
	stager, err := libbuildpack.NewStager(os.Args[1:], libbuildpack.NewLogger())
	err = stager.CheckBuildpackValid()
	if err != nil {
		os.Exit(10)
	}

	err = libbuildpack.RunBeforeCompile(stager)
	if err != nil {
		stager.Log.Error("Before Compile: %s", err.Error())
		os.Exit(12)
	}

	err = libbuildpack.SetStagingEnvironment(stager.DepsDir)
	if err != nil {
		stager.Log.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(13)
	}

	var godep golang.Godep

	vendorTool, err := golang.SelectVendorTool(stager, &godep)
	if err != nil {
		stager.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		os.Exit(14)
	}

	goVersion, err := golang.SelectGoVersion(stager, vendorTool, godep)
	if err != nil {
		stager.Log.Error("Unable to select Go version: %s", err.Error())
		os.Exit(15)
	}

	gs := supply.Supplier{
		Stager:     stager,
		GoVersion:  goVersion,
		VendorTool: vendorTool,
	}

	err = supply.Run(&gs)
	if err != nil {
		os.Exit(16)
	}
}
