package main

import (
	"golang/common"
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

	stager, err := libbuildpack.NewStager([]string{buildDir, cacheDir, depsDir, depsIdx}, logger)
	err = stager.CheckBuildpackValid()
	if err != nil {
		os.Exit(11)
	}

	err = libbuildpack.RunBeforeCompile(stager)
	if err != nil {
		stager.Log.Error("Before Compile: %s", err.Error())
		os.Exit(12)
	}

	err = libbuildpack.SetEnvironmentFromSupply(stager.DepsDir)
	if err != nil {
		stager.Log.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(11)
	}

	var godep common.Godep

	vendorTool, err := common.SelectVendorTool(stager, &godep)
	if err != nil {
		stager.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		os.Exit(14)
	}

	goVersion, err := common.SelectGoVersion(stager, vendorTool, godep)
	if err != nil {
		stager.Log.Error("Unable to select Go version: %s", err.Error())
		os.Exit(15)
	}

	gs := supply.Supplier{
		Stager:     stager,
		Godep:      godep,
		GoVersion:  goVersion,
		VendorTool: vendorTool,
	}

	err = supply.Run(&gs)
	if err != nil {
		os.Exit(14)
	}

	err = libbuildpack.SetEnvironmentFromSupply(stager.DepsDir)
	if err != nil {
		stager.Log.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(15)
	}

	err = libbuildpack.WriteProfileDFromSupply(stager.DepsDir, stager.BuildDir)
	if err != nil {
		stager.Log.Error("Unable to write .profile.d supply script: %s", err.Error())
		os.Exit(16)
	}

	gf := finalize.Finalizer{
		Stager:     stager,
		Godep:      godep,
		GoVersion:  goVersion,
		VendorTool: vendorTool,
	}

	err = finalize.Run(&gf)
	if err != nil {
		os.Exit(17)
	}

	err = libbuildpack.RunAfterCompile(stager)
	if err != nil {
		stager.Log.Error("After Compile: %s", err.Error())
		os.Exit(18)
	}

	stager.StagingComplete()
}
