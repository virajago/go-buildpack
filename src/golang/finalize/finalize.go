package finalize

import (
	"errors"
	"fmt"
	"golang/common"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/cloudfoundry/libbuildpack"
)

type Finalizer struct {
	Compiler         *libbuildpack.Compiler
	DepDir           string
	VendorTool       string
	GoVersion        string
	MainPackageName  string
	GoPath           string
	PackageList      []string
	BuildFlags       []string
	Godep            common.Godep
	VendorExperiment bool
}

func Run(gf *Finalizer) error {
	var err error

	err = gf.Compiler.LoadSuppliedDeps()
	if err != nil {
		gf.Compiler.Log.Error("Unable to setup environment: %s", err.Error())
		return err
	}

	gf.VendorTool, err = common.SelectVendorTool(gf.Compiler, &gf.Godep)
	if err != nil {
		gf.Compiler.Log.Error("Unable to select Go vendor tool: %s", err.Error())
		return err
	}

	if err := gf.SetMainPackageName(); err != nil {
		gf.Compiler.Log.Error("Unable to determine import path: %s", err.Error())
		return err
	}

	if err := os.MkdirAll(filepath.Join(gf.Compiler.BuildDir, "bin"), 0755); err != nil {
		gf.Compiler.Log.Error("Unable to create <build-dir>/bin: %s", err.Error())
		return err
	}

	if err := gf.SetupGoPath(); err != nil {
		gf.Compiler.Log.Error("Unable to setup Go path: %s", err.Error())
		return err
	}

	if err := gf.HandleVendorExperiment(); err != nil {
		gf.Compiler.Log.Error("Invalid vendor config: %s", err.Error())
		return err
	}

	if gf.VendorTool == "glide" {
		if err := gf.RunGlideInstall(); err != nil {
			gf.Compiler.Log.Error("Error running 'glide install': %s", err.Error())
			return err
		}
	}

	gf.SetBuildFlags()
	if err = gf.SetInstallPackages(); err != nil {
		gf.Compiler.Log.Error("Unable to determine packages to install: %s", err.Error())
		return err
	}

	if err := gf.CompileApp(); err != nil {
		gf.Compiler.Log.Error("Unable to compile application: %s", err.Error())
		return err
	}

	if err := gf.CreateStartupEnvironment("/tmp"); err != nil {
		gf.Compiler.Log.Error("Unable to create startup scripts: %s", err.Error())
		return err
	}

	return nil
}

func (gf *Finalizer) SetMainPackageName() error {
	switch gf.VendorTool {
	case "godep":
		gf.MainPackageName = gf.Godep.ImportPath

	case "glide":
		gf.Compiler.Command.SetDir(gf.Compiler.BuildDir)
		defer gf.Compiler.Command.SetDir("")

		stdout, err := gf.Compiler.Command.CaptureStdout("glide", "name")
		if err != nil {
			return err
		}
		gf.MainPackageName = strings.TrimSpace(stdout)

	case "go_nativevendoring":
		gf.MainPackageName = os.Getenv("GOPACKAGENAME")
		if gf.MainPackageName == "" {
			gf.Compiler.Log.Error(common.NoGOPACKAGENAMEerror())
			return errors.New("GOPACKAGENAME unset")
		}

	default:
		return errors.New("invalid vendor tool")
	}
	return nil
}

func (gf *Finalizer) SetupGoPath() error {
	var skipMoveFile = map[string]bool{
		"Procfile": true,
		".profile": true,
		"src":      true,
	}

	var goPath string
	goPathInImage := os.Getenv("GO_SETUP_GOPATH_IN_IMAGE") == "true"

	if goPathInImage {
		goPath = gf.Compiler.BuildDir
	} else {
		tmpDir, err := ioutil.TempDir("", "gobuildpack.gopath")
		if err != nil {
			return err
		}
		goPath = filepath.Join(tmpDir, ".go")
	}

	err := os.Setenv("GOPATH", goPath)
	if err != nil {
		return err
	}
	gf.GoPath = goPath

	binDir := filepath.Join(gf.Compiler.BuildDir, "bin")
	err = os.MkdirAll(binDir, 0755)
	if err != nil {
		return err
	}

	packageDir := gf.mainPackagePath()
	err = os.MkdirAll(packageDir, 0755)
	if err != nil {
		return err
	}

	if goPathInImage {
		files, err := ioutil.ReadDir(gf.Compiler.BuildDir)
		if err != nil {
			return err
		}
		for _, f := range files {
			if !skipMoveFile[f.Name()] {
				src := filepath.Join(gf.Compiler.BuildDir, f.Name())
				dest := filepath.Join(packageDir, f.Name())

				err = os.Rename(src, dest)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = os.Setenv("GOBIN", binDir)
		if err != nil {
			return err
		}

		err = libbuildpack.CopyDirectory(gf.Compiler.BuildDir, packageDir)
		if err != nil {
			return err
		}
	}

	// unset git dir or it will mess with go install
	return os.Unsetenv("GIT_DIR")
}

func (gf *Finalizer) SetBuildFlags() {
	flags := []string{"-tags", "cloudfoundry", "-buildmode", "pie"}

	if os.Getenv("GO_LINKER_SYMBOL") != "" && os.Getenv("GO_LINKER_VALUE") != "" {
		ld_flags := []string{"-ldflags", fmt.Sprintf("-X %s=%s", os.Getenv("GO_LINKER_SYMBOL"), os.Getenv("GO_LINKER_VALUE"))}

		flags = append(flags, ld_flags...)
	}

	gf.BuildFlags = flags
	return
}

func (gf *Finalizer) RunGlideInstall() error {
	if gf.VendorTool != "glide" {
		return nil
	}

	vendorDirExists, err := libbuildpack.FileExists(filepath.Join(gf.mainPackagePath(), "vendor"))
	if err != nil {
		return err
	}
	runGlideInstall := true

	if vendorDirExists {
		numSubDirs := 0
		files, err := ioutil.ReadDir(filepath.Join(gf.mainPackagePath(), "vendor"))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				numSubDirs++
			}
		}

		if numSubDirs > 0 {
			runGlideInstall = false
		}
	}

	if runGlideInstall {
		gf.Compiler.Log.BeginStep("Fetching any unsaved dependencies (glide install)")
		gf.Compiler.Command.SetDir(gf.mainPackagePath())
		defer gf.Compiler.Command.SetDir("")

		err := gf.Compiler.Command.Run("glide", "install")
		if err != nil {
			return err
		}
	} else {
		gf.Compiler.Log.Info("Note: skipping (glide install) due to non-empty vendor directory.")
	}

	return nil
}

func (gf *Finalizer) HandleVendorExperiment() error {
	gf.VendorExperiment = true

	if os.Getenv("GO15VENDOREXPERIMENT") == "" {
		return nil
	}

	ver, err := semver.NewVersion(gf.GoVersion)
	if err != nil {
		return err
	}

	go16 := ver.Major() == 1 && ver.Minor() == 6
	if !go16 {
		gf.Compiler.Log.Error(common.UnsupportedGO15VENDOREXPERIMENTerror())
		return errors.New("unsupported GO15VENDOREXPERIMENT")
	}

	if os.Getenv("GO15VENDOREXPERIMENT") == "0" {
		gf.VendorExperiment = false
	}

	return nil
}

func (gf *Finalizer) SetInstallPackages() error {
	var packages []string
	vendorDirExists, err := libbuildpack.FileExists(filepath.Join(gf.mainPackagePath(), "vendor"))
	if err != nil {
		return err
	}

	if os.Getenv("GO_INSTALL_PACKAGE_SPEC") != "" {
		packages = append(packages, strings.Split(os.Getenv("GO_INSTALL_PACKAGE_SPEC"), " ")...)
	}

	if gf.VendorTool == "godep" {
		useVendorDir := gf.VendorExperiment && !gf.Godep.WorkspaceExists

		if gf.Godep.WorkspaceExists && vendorDirExists {
			gf.Compiler.Log.Warning(common.GodepsWorkspaceWarning())
		}

		if useVendorDir && !vendorDirExists {
			gf.Compiler.Log.Warning("vendor/ directory does not exist.")
		}

		if len(packages) != 0 {
			gf.Compiler.Log.Warning(common.PackageSpecOverride(packages))
		} else if len(gf.Godep.Packages) != 0 {
			packages = gf.Godep.Packages
		} else {
			gf.Compiler.Log.Warning("Installing package '.' (default)")
			packages = append(packages, ".")
		}

		if useVendorDir {
			packages = gf.updatePackagesForVendor(packages)
		}
	} else {
		if !gf.VendorExperiment && gf.VendorTool == "go_nativevendoring" {
			gf.Compiler.Log.Error(common.MustUseVendorError())
			return errors.New("must use vendor/ for go native vendoring")
		}

		if len(packages) == 0 {
			packages = append(packages, ".")
			gf.Compiler.Log.Warning("Installing package '.' (default)")
		}

		packages = gf.updatePackagesForVendor(packages)
	}

	gf.PackageList = packages
	return nil
}

func (gf *Finalizer) CompileApp() error {
	cmd := "go"
	args := []string{"install"}
	args = append(args, gf.BuildFlags...)
	args = append(args, gf.PackageList...)

	if gf.VendorTool == "godep" && (gf.Godep.WorkspaceExists || !gf.VendorExperiment) {
		args = append([]string{"go"}, args...)
		cmd = "godep"
	}

	gf.Compiler.Log.BeginStep(fmt.Sprintf("Running: %s %s", cmd, strings.Join(args, " ")))

	gf.Compiler.Command.SetDir(gf.mainPackagePath())
	defer gf.Compiler.Command.SetDir("")

	err := gf.Compiler.Command.Run(cmd, args...)
	if err != nil {
		return err
	}
	return nil
}

func (gf *Finalizer) CreateStartupEnvironment(tempDir string) error {
	err := ioutil.WriteFile(filepath.Join(tempDir, "buildpack-release-step.yml"), []byte(common.ReleaseYAML(gf.MainPackageName)), 0644)
	if err != nil {
		gf.Compiler.Log.Error("Unable to write relase yml: %s", err.Error())
		return err
	}

	if os.Getenv("GO_INSTALL_TOOLS_IN_IMAGE") == "true" {
		gf.Compiler.Log.BeginStep("Copying go tool chain to $GOROOT=$HOME/.cloudfoundry/go")

		imageDir := filepath.Join(gf.Compiler.BuildDir, ".cloudfoundry")
		err = os.MkdirAll(imageDir, 0755)
		if err != nil {
			return err
		}
		err = libbuildpack.CopyDirectory(gf.goInstallLocation(), imageDir)
		if err != nil {
			return err
		}

		err = libbuildpack.WriteProfileD(gf.Compiler.BuildDir, "goroot.sh", common.GoRootScript())
		if err != nil {
			return err
		}
	}

	if os.Getenv("GO_SETUP_GOPATH_IN_IMAGE") == "true" {
		gf.Compiler.Log.BeginStep("Cleaning up $GOPATH/pkg")
		err = os.RemoveAll(filepath.Join(gf.GoPath, "pkg"))
		if err != nil {
			return err
		}

		err = libbuildpack.WriteProfileD(gf.Compiler.BuildDir, "zzgopath.sh", common.ZZGoPathScript(gf.MainPackageName))
		if err != nil {
			return err
		}
	}

	return libbuildpack.WriteProfileD(gf.Compiler.BuildDir, "go.sh", common.GoScript())
}

func (gf *Finalizer) mainPackagePath() string {
	return filepath.Join(gf.GoPath, "src", gf.MainPackageName)
}

func (gf *Finalizer) goInstallLocation() string {
	return filepath.Join(gf.Compiler.CacheDir, "go"+gf.GoVersion)
}

func (gf *Finalizer) updatePackagesForVendor(packages []string) []string {
	var newPackages []string

	for _, pkg := range packages {
		vendored, _ := libbuildpack.FileExists(filepath.Join(gf.mainPackagePath(), "vendor", pkg))
		if pkg == "." || !vendored {
			newPackages = append(newPackages, pkg)
		} else {
			newPackages = append(newPackages, filepath.Join(gf.MainPackageName, "vendor", pkg))
		}
	}

	return newPackages
}

func (gf *Finalizer) isGB() (bool, error) {
	srcDir := filepath.Join(gf.Compiler.BuildDir, "src")
	srcDirAtAppRoot, err := libbuildpack.FileExists(srcDir)
	if err != nil {
		return false, err
	}

	if !srcDirAtAppRoot {
		return false, nil
	}

	files, err := ioutil.ReadDir(filepath.Join(gf.Compiler.BuildDir, "src"))
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
