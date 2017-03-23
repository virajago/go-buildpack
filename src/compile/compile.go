package main

import (
	"bytes"
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

	vendorTool, err := gc.SelectVendorTool()
	if err != nil {
		gc.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	goVersion, err := gc.SelectGoVersion(vendorTool)
	if err != nil {
		gc.Compiler.Log.Error("Unable to determine Go version to install: %s", err.Error())
		return err
	}

	err = gc.InstallGo(goVersion)
	if err != nil {
		gc.Compiler.Log.Error("Error installing Go: %s", err.Error())
	}

	packageName, err := gc.PackageName(vendorTool)
	if err != nil {
		gc.Compiler.Log.Error("Unable to determine import path: %s", err.Error())
		return err
	}
	fmt.Println(packageName)

	err = gc.CheckBinDirectory()
	if err != nil {
		gc.Compiler.Log.Error("Error checking bin directory: %s", err.Error())
		return err
	}

	flags := gc.SetupBuildFlags(goVersion, vendorTool)

	fmt.Println(flags)

	return nil

}

func (gc *GoCompiler) PackageName(vendorTool string) (string, error) {
	var packageName string

	switch vendorTool {
	case "godep":
		godepsJSONFile := filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json")
		var godeps godepsJSON
		err := libbuildpack.NewJSON().Load(godepsJSONFile, &godeps)
		if err != nil {
			gc.Compiler.Log.Error("Bad Godeps/Godeps.json file")
			return "", err
		}
		packageName = godeps.ImportPath

	case "glide":
		outputBuffer := new(bytes.Buffer)

		gc.Compiler.Command.SetStdout(outputBuffer)
		gc.Compiler.Command.SetStderr(ioutil.Discard)
		gc.Compiler.Command.SetDir(gc.Compiler.BuildDir)

		err := gc.Compiler.Command.Run("glide", "name")
		if err != nil {
			return "", err
		}
		gc.Compiler.Command.Reset()

		packageName = outputBuffer.String()
	case "go_nativevendoring":
		packageName = os.Getenv("GOPACKAGENAME")
		if packageName == "" {
			gc.Compiler.Log.Error(noGOPACKAGENAMEerror())
			return "", errors.New("GOPACKAGENAME unset")
		}

	default:
		return "", errors.New("invalid vendor tool")
	}
	return packageName, nil
}

func (gc *GoCompiler) SelectGoVersion(vendorTool string) (string, error) {
	var err error
	var goVersion string

	switch vendorTool {
	case "godep":
		gc.Compiler.Log.BeginStep("Checking Godeps/Godeps.json file")
		var godeps godepsJSON
		err = libbuildpack.NewJSON().Load(filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json"), &godeps)
		if err != nil {
			gc.Compiler.Log.Error("Bad Godeps/Godeps.json file")
			return "", err
		}

		envGoVersion := os.Getenv("GOVERSION")
		if envGoVersion != "" {
			goVersion = envGoVersion
			gc.Compiler.Log.Warning(goVersionOverride(envGoVersion))
		} else {
			goVersion = godeps.GoVersion
		}
	case "glide":
		fallthrough
	case "go_nativevendoring":
		envGoVersion := os.Getenv("GOVERSION")
		if envGoVersion != "" {
			goVersion = envGoVersion
		} else {
			defaultGo, err := gc.Compiler.Manifest.DefaultVersion("go")
			if err != nil {
				return "", err
			}
			goVersion = fmt.Sprintf("go%s", defaultGo.Version)
		}
	default:
		return "", errors.New("invalid vendor tool")
	}

	return gc.ParseGoVersion(goVersion)
}

func (gc *GoCompiler) SetupGoPath(packageName string) (string, error) {
	var goPath string
	goPathInImage := os.Getenv("GO_SETUP_GOPATH_IN_IMAGE") == "true"

	if goPathInImage {
		goPath = gc.Compiler.BuildDir
	} else {
		tmpDir, err := ioutil.TempDir("", "gobuildpack.gopath")
		if err != nil {
			return "", err
		}
		goPath = filepath.Join(tmpDir, ".go")
	}

	packageDir := filepath.Join(goPath, "src", packageName)
	err := os.MkdirAll(packageDir, 0755)
	if err != nil {
		return "", err
	}

	err = os.Setenv("GOPATH", goPath)
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(gc.Compiler.BuildDir, "bin")
	err = os.MkdirAll(binDir, 0755)
	if err != nil {
		return "", err
	}

	if goPathInImage {
		files, err := ioutil.ReadDir(gc.Compiler.BuildDir)
		if err != nil {
			return "", err
		}
		for _, f := range files {
			if f.Name() != "src" {
				src := filepath.Join(gc.Compiler.BuildDir, f.Name())
				dest := filepath.Join(packageDir, f.Name())

				err = os.Rename(src, dest)
				if err != nil {
					return "", err
				}
			}
		}
	} else {
		err = os.Setenv("GOBIN", binDir)
		if err != nil {
			return "", err
		}

		err = libbuildpack.CopyDirectory(gc.Compiler.BuildDir, packageDir)
		if err != nil {
			return "", err
		}
	}

	return packageDir, nil
}

func (gc *GoCompiler) SetupBuildFlags(goVersion, tool string) []string {
	flags := []string{"-tags cloudfoundry"}

	if os.Getenv("GO_LINKER_SYMBOL") != "" && os.Getenv("GO_LINKER_VALUE") != "" {
		flags = append(flags, fmt.Sprintf("-ldflags \"-X %s=%s\"", os.Getenv("GO_LINKER_SYMBOL"), os.Getenv("GO_LINKER_VALUE")))
	}

	if strings.Split(goVersion, ".")[0] == "1" && strings.Split(goVersion, ".")[1] == "6" && tool == "godep" {
		flags = append(flags, "--buildmode=pie")
	}
	return flags
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

func (gc *GoCompiler) SelectVendorTool() (vendorTool string, err error) {
	godepsJSONFile := filepath.Join(gc.Compiler.BuildDir, "Godeps", "Godeps.json")
	isGodep, err := libbuildpack.FileExists(godepsJSONFile)
	if err != nil {
		return "", err
	}
	if isGodep {
		return "godep", nil
	}

	godirFile := filepath.Join(gc.Compiler.BuildDir, ".godir")
	isGodir, err := libbuildpack.FileExists(godirFile)
	if err != nil {
		return "", err
	}
	if isGodir {
		gc.Compiler.Log.Error(godirError())
		return "", errors.New(".godir deprecated")
	}

	glideFile := filepath.Join(gc.Compiler.BuildDir, "glide.yaml")
	isGlide, err := libbuildpack.FileExists(glideFile)
	if err != nil {
		return "", err
	}
	if isGlide {
		return "glide", nil
	}

	isGB, err := gc.isGB()
	if err != nil {
		return "", err
	}
	if isGB {
		gc.Compiler.Log.Error(gbError())
		return "", errors.New("gb unsupported")
	}

	return "go_nativevendoring", nil
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

func (gc *GoCompiler) InstallGo(goVersion string) error {
	err := os.MkdirAll(filepath.Join(gc.Compiler.BuildDir, "bin"), 0755)
	if err != nil {
		return err
	}

	goInstallDir := filepath.Join(gc.Compiler.CacheDir, "go"+goVersion)

	goInstalled, err := libbuildpack.FileExists(filepath.Join(goInstallDir, "go"))
	if err != nil {
		return err
	}

	if goInstalled {
		gc.Compiler.Log.BeginStep("Using go%s", goVersion)
	} else {
		gc.Compiler.Log.BeginStep("Installing go%s", goVersion)

		err = gc.clearCache()
		if err != nil {
			return fmt.Errorf("clearing cache: %s", err.Error())
		}

		err = os.MkdirAll(goInstallDir, 0755)
		if err != nil {
			return err
		}

		dep := libbuildpack.Dependency{Name: "go", Version: goVersion}
		err = gc.Compiler.Manifest.InstallDependency(dep, goInstallDir)
		if err != nil {
			return err
		}
	}

	err = os.Setenv("GOROOT", filepath.Join(goInstallDir, "go"))
	if err != nil {
		return err
	}

	return os.Setenv("PATH", fmt.Sprintf("%s:%s", os.Getenv("PATH"), filepath.Join(goInstallDir, "go", "bin")))
}

func (gc *GoCompiler) clearCache() error {
	files, err := ioutil.ReadDir(gc.Compiler.CacheDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.RemoveAll(filepath.Join(gc.Compiler.CacheDir, file.Name()))
		if err != nil {
			return err
		}
	}

	return nil
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
