package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type GoCompiler struct {
	Compiler *libbuildpack.Compiler
}

type godepsJSON struct {
	ImportPath string `json:"ImportPath"`
	GoVersion  string `json:"GoVersion"`
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

	_, goVersion, _, err := gc.SelectVendorTool()
	if err != nil {
		gc.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	parsedGoVersion, err := gc.ParseGoVersion(goVersion)
	if err != nil {
		gc.Compiler.Log.Error("Unable to expand Go version %s: %s", goVersion, err.Error())
		return err
	}

	err = gc.CheckBinDirectory()
	if err != nil {
		gc.Compiler.Log.Error("Error checking bin directory: %s", err.Error())
		return err
	}

	err = gc.InstallGo(parsedGoVersion)
	if err != nil {
		gc.Compiler.Log.Error("Error installing Go: %s", err.Error())
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

func (gc *GoCompiler) SelectVendorTool() (vendorTool, goVersion, goPackageName string, err error) {
	godepsJSONFile := filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json")
	isGodep, err := libbuildpack.FileExists(godepsJSONFile)
	if err != nil {
		return "", "", "", err
	}
	if isGodep {
		gc.Compiler.Log.BeginStep("Checking Godeps/Godeps.json file")
		var godeps godepsJSON
		err = libbuildpack.NewJSON().Load(godepsJSONFile, &godeps)
		if err != nil {
			gc.Compiler.Log.Error("Bad Godeps/Godeps.json file")
			return "", "", "", err
		}

		envGoVersion := os.Getenv("GOVERSION")
		if envGoVersion != "" {
			goVersion = envGoVersion
			gc.Compiler.Log.Warning(goVersionOverride(envGoVersion))
		} else {
			goVersion = godeps.GoVersion
		}

		return "godep", goVersion, godeps.ImportPath, nil
	}
	godirFile := filepath.Join(gc.Compiler.BuildDir, ".godir")
	isGodir, err := libbuildpack.FileExists(godirFile)
	if err != nil {
		return "", "", "", err
	}

	if isGodir {
		gc.Compiler.Log.Error(godirError())
		return "", "", "", errors.New(".godir deprecated")
	}

	glideFile := filepath.Join(gc.Compiler.BuildDir, "glide.yaml")
	isGlide, err := libbuildpack.FileExists(glideFile)
	if err != nil {
		return "", "", "", err
	}

	if isGlide {
		envGoVersion := os.Getenv("GOVERSION")
		if envGoVersion != "" {
			goVersion = envGoVersion
		} else {
			defaultGo, err := gc.Compiler.Manifest.DefaultVersion("go")
			if err != nil {
				return "", "", "", err
			}
			goVersion = fmt.Sprintf("go%s", defaultGo.Version)
		}
		return "glide", goVersion, "", nil
	}

	isGB, err := gc.isGB()
	if err != nil {
		return "", "", "", err
	}

	if isGB {
		gc.Compiler.Log.Error(gbError())
		return "", "", "", errors.New("gb unsupported")
	}

	envPackageName := os.Getenv("GOPACKAGENAME")
	if envPackageName == "" {
		gc.Compiler.Log.Error(noGOPACKAGENAMEerror())
		return "", "", "", errors.New("GOPACKAGENAME unset")

	}

	envGoVersion := os.Getenv("GOVERSION")
	if envGoVersion != "" {
		goVersion = envGoVersion
	} else {
		defaultGo, err := gc.Compiler.Manifest.DefaultVersion("go")
		if err != nil {
			return "", "", "", err
		}
		goVersion = fmt.Sprintf("go%s", defaultGo.Version)
	}

	return "go_nativevendoring", goVersion, envPackageName, nil
}

func (gc *GoCompiler) ParseGoVersion(partialGoVersion string) (string, error) {
	existingVersions := gc.Compiler.Manifest.AllDependencyVersions("go")

	if len(strings.Split(partialGoVersion, ".")) == 2 {
		partialGoVersion += ".x"
	}

	strippedGoVersion := strings.TrimLeft(partialGoVersion, "go")

	expandedVer, err := libbuildpack.FindMatchingVersion(strippedGoVersion, existingVersions)
	if err != nil {
		return "", err
	}

	return expandedVer, nil
}

func (gc *GoCompiler) InstallGo(goVersion string) error {
	return nil
}

func (gc *GoCompiler) CheckBinDirectory() error {
	fi, err := os.Stat(filepath.Join(gc.Compiler.BuildDir, "bin"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if fi.Mode().IsDir() {
		return nil
	}

	gc.Compiler.Log.Error("File bin exists and is not a directory.")
	return errors.New("invalid bin")
}

func (gc *GoCompiler) isGB() (bool, error) {
	srcDir := filepath.Join(gc.Compiler.BuildDir, "src")
	srcDirAtAppRoot, err := libbuildpack.FileExists(srcDir)
	if err != nil {
		return false, err
	}

	if !srcDirAtAppRoot {
		return false, nil
	}

	files, err := ioutil.ReadDir(filepath.Join(gc.Compiler.BuildDir, "src"))
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.Mode().IsDir() {
			err = filepath.Walk(filepath.Join(srcDir, file.Name()), isGoFile)
			if err != nil {
				if err.Error() == "found Go file" {
					return true, nil
				}

				return false, err
			}
		}
	}

	return false, nil
}

func isGoFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if strings.HasSuffix(path, ".go") {
		return errors.New("found Go file")
	}

	return nil
}

func addToPath(newPaths string) error {
	oldPath := os.Getenv("PATH")
	return os.Setenv("PATH", fmt.Sprintf("%s:%s", newPaths, oldPath))
}
