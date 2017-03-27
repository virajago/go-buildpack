package main_test

import (
	c "compile"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"bytes"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=vendor/github.com/cloudfoundry/libbuildpack/manifest.go --destination=mocks_manifest_test.go --package=main_test --imports=.=github.com/cloudfoundry/libbuildpack
//go:generate mockgen -source=vendor/github.com/cloudfoundry/libbuildpack/command_runner.go --destination=mocks_command_runner_test.go --package=main_test

var _ = Describe("Compile", func() {
	var (
		buildDir          string
		cacheDir          string
		depsDir           string
		gc                *c.GoCompiler
		logger            libbuildpack.Logger
		buffer            *bytes.Buffer
		err               error
		mockCtrl          *gomock.Controller
		mockManifest      *MockManifest
		mockCommandRunner *MockCommandRunner
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "go-buildpack.build.")
		Expect(err).To(BeNil())

		cacheDir, err = ioutil.TempDir("", "go-buildpack.cache.")
		Expect(err).To(BeNil())

		depsDir = ""

		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger()
		logger.SetOutput(buffer)

		mockCtrl = gomock.NewController(GinkgoT())
		mockManifest = NewMockManifest(mockCtrl)
		mockCommandRunner = NewMockCommandRunner(mockCtrl)
	})

	JustBeforeEach(func() {
		bpc := &libbuildpack.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			DepsDir:  depsDir,
			Manifest: mockManifest,
			Log:      logger,
			Command:  mockCommandRunner,
		}

		gc = &c.GoCompiler{Compiler: bpc}
	})

	AfterEach(func() {
		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(cacheDir)
		Expect(err).To(BeNil())
	})

	Describe("SelectVendorTool", func() {
		Context("There is a Godeps.json", func() {
			var godepsJson string

			JustBeforeEach(func() {
				err = os.MkdirAll(filepath.Join(buildDir, "Godeps"), 0755)
				Expect(err).To(BeNil())

				godepsJson = filepath.Join(buildDir, "Godeps", "Godeps.json")
				err = ioutil.WriteFile(godepsJson, []byte("xxx"), 0644)
				Expect(err).To(BeNil())
			})

			It("returns godep", func() {
				tool, err := gc.SelectVendorTool()
				Expect(err).To(BeNil())

				Expect(tool).To(Equal("godep"))
			})
		})

		Context("there is a .godir file", func() {
			BeforeEach(func() {
				err = ioutil.WriteFile(filepath.Join(buildDir, ".godir"), []byte("xxx"), 0644)
			})

			It("logs that .godir is deprecated and returns an error", func() {
				_, err := gc.SelectVendorTool()
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** Deprecated, .godir file found! Please update to supported Godep or Glide dependency managers."))
				Expect(buffer.String()).To(ContainSubstring("See https://github.com/tools/godep or https://github.com/Masterminds/glide for usage information."))
			})
		})

		Context("there is a glide.yaml file", func() {
			BeforeEach(func() {
				err = ioutil.WriteFile(filepath.Join(buildDir, "glide.yaml"), []byte("xxx"), 0644)
				dep := libbuildpack.Dependency{Name: "go", Version: "1.14.3"}

				mockManifest.EXPECT().DefaultVersion("go").Return(dep, nil)
			})

			It("returns glide", func() {
				tool, err := gc.SelectVendorTool()
				Expect(err).To(BeNil())

				Expect(tool).To(Equal("glide"))
			})
		})

		Context("the app contains src/**/**/*.go", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(buildDir, "src", "package"), 0755)
				Expect(err).To(BeNil())

				err = ioutil.WriteFile(filepath.Join(buildDir, "src", "package", "thing.go"), []byte("xxx"), 0644)
				Expect(err).To(BeNil())
			})

			It("logs that gb is deprecated and returns an error", func() {
				_, err := gc.SelectVendorTool()
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** Cloud Foundry does not support the GB package manager."))
				Expect(buffer.String()).To(ContainSubstring("We currently only support the Godep and Glide package managers for go apps"))
				Expect(buffer.String()).To(ContainSubstring("For support please file an issue: https://github.com/cloudfoundry/go-buildpack/issues"))

			})
		})

		Context("none of the above", func() {
			BeforeEach(func() {
				dep := libbuildpack.Dependency{Name: "go", Version: "2.0.1"}
				mockManifest.EXPECT().DefaultVersion("go").Return(dep, nil)
			})

			It("returns go_nativevendoring", func() {
				tool, err := gc.SelectVendorTool()
				Expect(err).To(BeNil())

				Expect(tool).To(Equal("go_nativevendoring"))
			})
		})
	})

	Describe("Installing vendor tools", func() {
		var (
			oldPath string
			tempDir string
		)

		BeforeEach(func() {
			oldPath = os.Getenv("PATH")
			tempDir, err = ioutil.TempDir("", "go-buildpack.tmp")
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err = os.Setenv("PATH", oldPath)
			Expect(err).To(BeNil())
		})

		Context("the tool is godep", func() {
			It("installs godep to the requested dir, adding it to the PATH", func() {
				dep := libbuildpack.Dependency{Name: "godep", Version: "v1.2.3"}
				installDir := filepath.Join(tempDir, "godep")

				mockManifest.EXPECT().DefaultVersion("godep").Return(dep, nil)
				mockManifest.EXPECT().InstallDependency(dep, installDir).Return(nil)

				err = gc.InstallGodep(installDir)
				Expect(err).To(BeNil())

				Expect(installDir).To(BeADirectory())

				newPath := os.Getenv("PATH")
				Expect(newPath).To(Equal(fmt.Sprintf("%s:%s", filepath.Join(installDir, "bin"), oldPath)))

				Expect(buffer.String()).To(ContainSubstring("-----> Installing godep"))
				Expect(buffer.String()).To(ContainSubstring("       godep version: v1.2.3"))

			})
		})
		Describe("the tool is glide", func() {
			It("installs glide to the requested dir, adding it to the PATH", func() {
				dep := libbuildpack.Dependency{Name: "glide", Version: "v5.6.7"}
				installDir := filepath.Join(tempDir, "glide")

				mockManifest.EXPECT().DefaultVersion("glide").Return(dep, nil)
				mockManifest.EXPECT().InstallDependency(dep, installDir).Return(nil)

				err = gc.InstallGlide(installDir)
				Expect(err).To(BeNil())

				Expect(installDir).To(BeADirectory())

				newPath := os.Getenv("PATH")
				Expect(newPath).To(Equal(fmt.Sprintf("%s:%s", filepath.Join(installDir, "bin"), oldPath)))

				Expect(buffer.String()).To(ContainSubstring("-----> Installing glide"))
				Expect(buffer.String()).To(ContainSubstring("       glide version: v5.6.7"))

			})
		})
	})

	Describe("SelectGoVersion", func() {
		BeforeEach(func() {
			versions := []string{"1.8.0", "1.7.5", "1.7.4", "1.6.3", "1.6.4", "34.34.0", "1.14.3"}
			mockManifest.EXPECT().AllDependencyVersions("go").Return(versions)
		})
		Context("godep", func() {
			var (
				godepsJson         string
				godepsJsonContents string
			)

			JustBeforeEach(func() {
				err = os.MkdirAll(filepath.Join(buildDir, "Godeps"), 0755)
				Expect(err).To(BeNil())

				godepsJson = filepath.Join(buildDir, "Godeps", "Godeps.json")
				err = ioutil.WriteFile(godepsJson, []byte(godepsJsonContents), 0644)
				Expect(err).To(BeNil())
			})

			Context("it is valid json", func() {
				BeforeEach(func() {
					godepsJsonContents = `
{
	"ImportPath": "go-online",
	"GoVersion": "go1.6",
	"Deps": []
}					
`
				})

				It("logs that it found a Godeps.json file", func() {
					_, err := gc.SelectGoVersion("godep")
					Expect(err).To(BeNil())

					Expect(buffer.String()).To(ContainSubstring("-----> Checking Godeps/Godeps.json file"))
				})

				Context("GOVERSION not set", func() {
					It("returns the go version from Godeps.json", func() {
						goVersion, err := gc.SelectGoVersion("godep")
						Expect(err).To(BeNil())

						Expect(goVersion).To(Equal("1.6.4"))
					})
				})

				Context("GOVERSION is set", func() {
					var oldGOVERSION string

					BeforeEach(func() {
						oldGOVERSION = os.Getenv("GOVERSION")
						err = os.Setenv("GOVERSION", "go34.34")
						Expect(err).To(BeNil())
					})

					AfterEach(func() {
						err = os.Setenv("GOVERSION", oldGOVERSION)
						Expect(err).To(BeNil())
					})

					It("returns the go version from GOVERSION and logs a warning", func() {
						goVersion, err := gc.SelectGoVersion("godep")
						Expect(err).To(BeNil())

						Expect(goVersion).To(Equal("34.34.0"))
						Expect(buffer.String()).To(ContainSubstring("**WARNING** Using $GOVERSION override.\n"))
						Expect(buffer.String()).To(ContainSubstring("    $GOVERSION = go34.34\n"))
						Expect(buffer.String()).To(ContainSubstring("If this isn't what you want please run:\n"))
						Expect(buffer.String()).To(ContainSubstring("    cf unset-env <app> GOVERSION"))
					})
				})
			})

			Context("bad Godeps.json file", func() {
				BeforeEach(func() {
					godepsJsonContents = "not actually JSON"
				})

				It("logs that the Godeps.json file is invalid and returns an error", func() {
					_, err := gc.SelectGoVersion("godep")
					Expect(err).NotTo(BeNil())

					Expect(buffer.String()).To(ContainSubstring("**ERROR** Bad Godeps/Godeps.json file"))
				})
			})
		})
		Context("glide or go_nativevendoring", func() {
			BeforeEach(func() {
				dep := libbuildpack.Dependency{Name: "go", Version: "1.14.3"}

				mockManifest.EXPECT().DefaultVersion("go").Return(dep, nil)
			})

			Context("GOVERSION is notset", func() {
				It("returns the default go version from the manifest.yml", func() {
					goVersion, err := gc.SelectGoVersion("glide")
					Expect(err).To(BeNil())

					Expect(goVersion).To(Equal("1.14.3"))
				})
			})
			Context("GOVERSION is set", func() {
				var oldGOVERSION string

				BeforeEach(func() {
					oldGOVERSION = os.Getenv("GOVERSION")
					err = os.Setenv("GOVERSION", "go34.34")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GOVERSION", oldGOVERSION)
					Expect(err).To(BeNil())
				})

				It("returns the go version from GOVERSION", func() {
					goVersion, err := gc.SelectGoVersion("go_nativevendoring")
					Expect(err).To(BeNil())

					Expect(goVersion).To(Equal("34.34.0"))
				})
			})
		})
	})

	Describe("ParseGoVersion", func() {
		BeforeEach(func() {
			versions := []string{"1.8.0", "1.7.5", "1.7.4", "1.6.3", "1.6.4"}
			mockManifest.EXPECT().AllDependencyVersions("go").Return(versions)
		})

		Context("a fully specified version is passed in", func() {
			It("returns the same value", func() {
				ver, err := gc.ParseGoVersion("go1.7.4")
				Expect(err).To(BeNil())

				Expect(ver).To(Equal("1.7.4"))
			})
		})

		Context("a version line is passed in", func() {
			It("returns the latest version of that line", func() {
				ver, err := gc.ParseGoVersion("go1.6")
				Expect(err).To(BeNil())

				Expect(ver).To(Equal("1.6.4"))
			})
		})

	})
	Describe("CheckBinDirectory", func() {
		Context("no directory exists", func() {
			It("returns nil", func() {
				err = gc.CheckBinDirectory()
				Expect(err).To(BeNil())
			})
		})

		Context("a bin directory exists", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(buildDir, "bin"), 0755)
				Expect(err).To(BeNil())
			})

			It("returns nil", func() {
				err := gc.CheckBinDirectory()
				Expect(err).To(BeNil())
			})
		})

		Context("a bin file exists", func() {
			BeforeEach(func() {
				err = ioutil.WriteFile(filepath.Join(buildDir, "bin"), []byte("xxx"), 0644)
				Expect(err).To(BeNil())
			})

			It("returns and logs an error", func() {
				err := gc.CheckBinDirectory()
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** File bin exists and is not a directory."))
			})
		})
	})

	Describe("InstallGo", func() {
		var (
			oldGoRoot    string
			oldPath      string
			goInstallDir string
			dep          libbuildpack.Dependency
		)

		BeforeEach(func() {
			oldPath = os.Getenv("PATH")
			oldPath = os.Getenv("GOROOT")
			goInstallDir = filepath.Join(cacheDir, "go1.3.4")

			dep = libbuildpack.Dependency{Name: "go", Version: "1.3.4"}
			mockManifest.EXPECT().InstallDependency(dep, goInstallDir).Return(nil).Times(1)
		})

		AfterEach(func() {
			err = os.Setenv("PATH", oldPath)
			Expect(err).To(BeNil())

			err = os.Setenv("GOROOT", oldGoRoot)
			Expect(err).To(BeNil())
		})

		It("Creates a bin directory", func() {
			err = gc.InstallGo("1.3.4")
			Expect(err).To(BeNil())

			Expect(filepath.Join(buildDir, "bin")).To(BeADirectory())
		})

		It("Sets up GOROOT", func() {
			err = gc.InstallGo("1.3.4")
			Expect(err).To(BeNil())

			Expect(os.Getenv("GOROOT")).To(Equal(filepath.Join(goInstallDir, "go")))
		})

		It("adds go to the PATH", func() {
			err = gc.InstallGo("1.3.4")
			Expect(err).To(BeNil())

			newPath := fmt.Sprintf("%s:%s", oldPath, filepath.Join(goInstallDir, "go", "bin"))
			Expect(os.Getenv("PATH")).To(Equal(newPath))
		})

		Context("go is already cached", func() {
			BeforeEach(func() {
				mockManifest.EXPECT().InstallDependency(dep, goInstallDir).Times(0)
				err = os.MkdirAll(filepath.Join(goInstallDir, "go"), 0755)
				Expect(err).To(BeNil())
			})

			It("uses the cached version", func() {
				err = gc.InstallGo("1.3.4")
				Expect(err).To(BeNil())

				Expect(buffer.String()).To(ContainSubstring("-----> Using go1.3.4"))
			})
		})

		Context("go is not already cached", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(cacheDir, "go4.3.2", "go"), 0755)
				Expect(err).To(BeNil())
			})

			It("installs go", func() {
				err = gc.InstallGo("1.3.4")
				Expect(err).To(BeNil())

				Expect(buffer.String()).To(ContainSubstring("-----> Installing go1.3.4"))
			})

			It("clears the cache", func() {
				err = gc.InstallGo("1.3.4")
				Expect(err).To(BeNil())

				Expect(filepath.Join(cacheDir, "go4.3.2", "go")).NotTo(BeADirectory())
			})

			It("creates the install directory", func() {
				err = gc.InstallGo("1.3.4")
				Expect(err).To(BeNil())

				Expect(filepath.Join(cacheDir, "go1.3.4")).To(BeADirectory())
			})
		})
	})

	Describe("SetupBuildFlags", func() {
		Context("link environment variables not set", func() {
			It("contains the default flags", func() {
				flags := gc.SetupBuildFlags("5.5.5")
				Expect(flags).To(Equal([]string{"-tags", "cloudfoundry", "-buildmode", "pie"}))
			})
		})

		Context("link environment variables are set set", func() {
			var (
				oldGoLinkerSymbol string
				oldGoLinkerValue  string
			)

			BeforeEach(func() {
				oldGoLinkerSymbol = os.Getenv("GO_LINKER_SYMBOL")
				oldGoLinkerValue = os.Getenv("GO_LINKER_VALUE")

				err = os.Setenv("GO_LINKER_SYMBOL", "package.main.thing")
				Expect(err).To(BeNil())

				err = os.Setenv("GO_LINKER_VALUE", "some_string")
				Expect(err).To(BeNil())

			})

			AfterEach(func() {
				err = os.Setenv("GO_LINKER_SYMBOL", oldGoLinkerSymbol)
				Expect(err).To(BeNil())

				err = os.Setenv("GO_LINKER_VALUE", oldGoLinkerValue)
				Expect(err).To(BeNil())
			})

			It("contains the ldflags argument", func() {
				flags := gc.SetupBuildFlags("5.5.5")
				Expect(flags).To(Equal([]string{"-tags", "cloudfoundry", "-buildmode", "pie", `-ldflags "-X package.main.thing=some_string"`}))
			})
		})
	})

	Describe("SetupGoPath", func() {
		var (
			oldGoPath               string
			oldGoBin                string
			oldGoSetupGopathInImage string
		)

		BeforeEach(func() {
			oldGoPath = os.Getenv("GOPATH")
			oldGoBin = os.Getenv("GOBIN")
			oldGoSetupGopathInImage = os.Getenv("GO_SETUP_GOPATH_IN_IMAGE")

			err := ioutil.WriteFile(filepath.Join(buildDir, "main.go"), []byte("xx"), 0644)
			Expect(err).To(BeNil())

			err = os.MkdirAll(filepath.Join(buildDir, "vendor"), 0755)
			Expect(err).To(BeNil())

			err = ioutil.WriteFile(filepath.Join(buildDir, "vendor", "lib.go"), []byte("xx"), 0644)
			Expect(err).To(BeNil())

			err = ioutil.WriteFile(filepath.Join(buildDir, "Procfile"), []byte("xx"), 0644)
			Expect(err).To(BeNil())

			err = ioutil.WriteFile(filepath.Join(buildDir, ".profile"), []byte("xx"), 0644)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err = os.Setenv("GOPATH", oldGoPath)
			Expect(err).To(BeNil())

			err = os.Setenv("GOBIN", oldGoBin)
			Expect(err).To(BeNil())

			err = os.Setenv("GO_SETUP_GOPATH_IN_IMAGE", oldGoSetupGopathInImage)
			Expect(err).To(BeNil())
		})

		It("creates <buildDir>/bin", func() {
			_, err = gc.SetupGoPath("a/package/name")
			Expect(err).To(BeNil())

			Expect(filepath.Join(buildDir, "bin")).To(BeADirectory())
		})

		Context("GO_SETUP_GOPATH_IN_IMAGE != true", func() {
			It("sets  GOPATH to a temp directory", func() {
				_, err = gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				dirRegex := regexp.MustCompile(`\/.{3,}\/gobuildpack\.gopath[0-9]{8,}\/\.go`)
				Expect(dirRegex.Match([]byte(os.Getenv("GOPATH")))).To(BeTrue())
			})

			It("the package dir is <tempdir>/.go/src/<mainPackageName>", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				dirRegex := regexp.MustCompile(`\/.{3,}\/gobuildpack\.gopath[0-9]{8,}\/\.go\/src\/a\/package\/name`)
				Expect(dirRegex.Match([]byte(packageDir))).To(BeTrue())
			})

			It("copies the buildDir contents to <tempdir>/.go/src/<mainPackageName>", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(filepath.Join(packageDir, "main.go")).To(BeAnExistingFile())
				Expect(filepath.Join(packageDir, "vendor", "lib.go")).To(BeAnExistingFile())
			})

			It("sets GOBIN to <buildDir>/bin", func() {
				_, err = gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(os.Getenv("GOBIN")).To(Equal(filepath.Join(buildDir, "bin")))
			})
		})

		Context("GO_SETUP_GOPATH_IN_IMAGE = true", func() {
			BeforeEach(func() {
				err = os.Setenv("GO_SETUP_GOPATH_IN_IMAGE", "true")
			})

			It("sets GOPATH to the build directory", func() {
				_, err = gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(os.Getenv("GOPATH")).To(Equal(buildDir))
			})

			It("the package directory is <buildDir>/src/<mainPackageName>", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(packageDir).To(Equal(filepath.Join(buildDir, "src", "a/package/name")))
			})

			It("moves the buildDir contents to <buildDir>/src/<mainPackageName>", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(filepath.Join(packageDir, "main.go")).To(BeAnExistingFile())
				Expect(filepath.Join(packageDir, "vendor", "lib.go")).To(BeAnExistingFile())
				Expect(filepath.Join(packageDir, "src", "a/package/name")).NotTo(BeAnExistingFile())
			})

			It("does not move the Procfile", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(filepath.Join(packageDir, "Procfile")).NotTo(BeAnExistingFile())
				Expect(filepath.Join(buildDir, "Procfile")).To(BeAnExistingFile())
			})

			It("does not move the .profile script", func() {
				packageDir, err := gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(filepath.Join(packageDir, ".profile")).NotTo(BeAnExistingFile())
				Expect(filepath.Join(buildDir, ".profile")).To(BeAnExistingFile())
			})

			It("does not set GOBIN", func() {
				_, err = gc.SetupGoPath("a/package/name")
				Expect(err).To(BeNil())

				Expect(os.Getenv("GOBIN")).To(Equal(oldGoBin))
			})
		})
	})

	Describe("MainPackageName", func() {
		Context("the vendor tool is godep", func() {
			BeforeEach(func() {
				godepsJsonContents := `
{
	"ImportPath": "go-online",
	"GoVersion": "go1.6",
	"Deps": []
}					
`
				err = os.MkdirAll(filepath.Join(buildDir, "Godeps"), 0755)
				Expect(err).To(BeNil())

				godepsJson := filepath.Join(buildDir, "Godeps", "Godeps.json")
				err = ioutil.WriteFile(godepsJson, []byte(godepsJsonContents), 0644)
				Expect(err).To(BeNil())
			})

			It("returns the package name from Godeps.json", func() {
				goPackageName, err := gc.MainPackageName("godep")
				Expect(err).To(BeNil())

				Expect(goPackageName).To(Equal("go-online"))
			})
		})

		Context("the vendor tool is glide", func() {
			It("returns the value of 'glide name'", func() {

				gomock.InOrder(
					mockCommandRunner.EXPECT().SetDir(buildDir),
					mockCommandRunner.EXPECT().CaptureStdout("glide", "name").Return("go-package-name\n", nil),
					mockCommandRunner.EXPECT().SetDir(""),
				)

				goPackageName, err := gc.MainPackageName("glide")
				Expect(err).To(BeNil())
				Expect(goPackageName).To(Equal("go-package-name"))
			})
		})

		Context("the vendor tool is go_nativevendoring", func() {
			Context("GOPACKAGENAME is not set", func() {
				It("logs an error", func() {
					_, err := gc.MainPackageName("go_nativevendoring")
					Expect(err).NotTo(BeNil())

					Expect(buffer.String()).To(ContainSubstring("**ERROR** To use go native vendoring set the $GOPACKAGENAME"))
					Expect(buffer.String()).To(ContainSubstring("environment variable to your app's package name"))
				})
			})
			Context("GOPACKAGENAME is set", func() {
				var oldGOPACKAGENAME string

				BeforeEach(func() {
					oldGOPACKAGENAME = os.Getenv("GOPACKAGENAME")
					err = os.Setenv("GOPACKAGENAME", "my-go-app")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GOPACKAGENAME", oldGOPACKAGENAME)
					Expect(err).To(BeNil())
				})

				It("returns the package name from GOPACKAGENAME", func() {
					goPackageName, err := gc.MainPackageName("go_nativevendoring")
					Expect(err).To(BeNil())

					Expect(goPackageName).To(Equal("my-go-app"))
				})
			})
		})
	})

	Describe("InstallPackages", func() {
		var (
			packageDir      string
			vendorTool      string
			mainPackageName string
			tempDir         string
		)

		BeforeEach(func() {
			mainPackageName = "a/package/name"
			tempDir, err = ioutil.TempDir("", "go-buildpack.package")
			Expect(err).To(BeNil())

			packageDir = filepath.Join(tempDir, mainPackageName)

			err = os.MkdirAll(packageDir, 0755)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err = os.RemoveAll(tempDir)
			Expect(err).To(BeNil())
		})

		Context("the vendor tool is godep", func() {
			var godepsJsonContents string

			JustBeforeEach(func() {
				vendorTool = "godep"
				err = os.MkdirAll(filepath.Join(packageDir, "Godeps"), 0755)
				Expect(err).To(BeNil())

				godepsJson := filepath.Join(packageDir, "Godeps", "Godeps.json")
				err = ioutil.WriteFile(godepsJson, []byte(godepsJsonContents), 0644)
				Expect(err).To(BeNil())
			})

			Context("GO_INSTALL_PACKAGE_SPEC is set", func() {
				var oldGoInstallPackageSpec string

				BeforeEach(func() {
					oldGoInstallPackageSpec = os.Getenv("GO_INSTALL_PACKAGE_SPEC")
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", "a-package-name another-package")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", oldGoInstallPackageSpec)
					Expect(err).To(BeNil())
				})

				It("uses the packages from the env var", func() {
					packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(packages).To(Equal([]string{"a-package-name", "another-package"}))
				})

				It("logs a warning that it overrode the Godeps.json packages", func() {
					_, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(buffer.String()).To(ContainSubstring("**WARNING** Using $GO_INSTALL_PACKAGE_SPEC override."))
					Expect(buffer.String()).To(ContainSubstring("    $GO_INSTALL_PACKAGE_SPEC = a-package-name"))
					Expect(buffer.String()).To(ContainSubstring("If this isn't what you want please run:"))
					Expect(buffer.String()).To(ContainSubstring("    cf unset-env <app> GO_INSTALL_PACKAGE_SPEC"))
				})

			})

			Context("No packages in Godeps.json", func() {
				BeforeEach(func() {
					godepsJsonContents = `
{
	"ImportPath": "go-online",
	"GoVersion": "go1.6",
	"Deps": []
}					
`
				})

				It("returns default", func() {
					packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(packages).To(Equal([]string{"."}))
				})

				It("logs a warning that it is using the default", func() {
					_, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(buffer.String()).To(ContainSubstring("**WARNING** Installing package '.' (default)"))
				})
			})

			Context("GO_INSTALL_PACKAGE_SPEC is not set", func() {
				BeforeEach(func() {
					godepsJsonContents = `
{
	"ImportPath": "go-online",
	"GoVersion": "go1.6",
	"Deps": [],
	"Packages": ["foo", "bar"]
}					
`
				})

				Context("packages are vendored", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor", "foo"), 0755)
						Expect(err).To(BeNil())
					})

					It("handles the vendoring correctly", func() {
						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{filepath.Join(mainPackageName, "vendor", "foo"), "bar"}))
					})

					Context("packages are in the Godeps/_workspace", func() {
						BeforeEach(func() {
							err = os.MkdirAll(filepath.Join(packageDir, "Godeps", "_workspace", "src"), 0755)
							Expect(err).To(BeNil())
						})

						It("uses the packages from Godeps.json", func() {
							packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
							Expect(err).To(BeNil())

							Expect(packages).To(Equal([]string{"foo", "bar"}))
						})

						It("logs a warning about vendor and godeps both existing", func() {
							_, err := gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
							Expect(err).To(BeNil())

							Expect(buffer.String()).To(ContainSubstring("**WARNING** Godeps/_workspace/src and vendor/ exist"))
							Expect(buffer.String()).To(ContainSubstring("code may not compile. Please convert all deps to vendor/"))
						})

					})

					Context("go 1.6.x with GO15VENDOREXPERIMENT=0", func() {
						var oldGO15VENDOREXPERIMENT string

						BeforeEach(func() {
							oldGO15VENDOREXPERIMENT = os.Getenv("GO15VENDOREXPERIMENT")
							err = os.Setenv("GO15VENDOREXPERIMENT", "0")
							Expect(err).To(BeNil())
						})

						AfterEach(func() {
							err = os.Setenv("GO15VENDOREXPERIMENT", oldGO15VENDOREXPERIMENT)
							Expect(err).To(BeNil())
						})

						It("uses the packages from Godeps.json", func() {
							packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
							Expect(err).To(BeNil())

							Expect(packages).To(Equal([]string{"foo", "bar"}))
						})
					})
				})

				Context("packages are in the Godeps/_workspace", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "Godeps", "_workspace", "src"), 0755)
						Expect(err).To(BeNil())
					})

					It("uses the packages from Godeps.json", func() {
						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{"foo", "bar"}))
					})

					It("doesn't log any warnings", func() {
						_, err := gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(Equal(""))
					})
				})

				Context("go 1.7 or later with GO15VENDOREXPERIMENT set", func() {
					var oldGO15VENDOREXPERIMENT string

					BeforeEach(func() {
						oldGO15VENDOREXPERIMENT = os.Getenv("GO15VENDOREXPERIMENT")
						err = os.Setenv("GO15VENDOREXPERIMENT", "0")
						Expect(err).To(BeNil())
					})

					AfterEach(func() {
						err = os.Setenv("GO15VENDOREXPERIMENT", oldGO15VENDOREXPERIMENT)
						Expect(err).To(BeNil())
					})

					It("errors out with a warning messaage", func() {
						_, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).NotTo(BeNil())

						Expect(buffer.String()).To(ContainSubstring("**ERROR** GO15VENDOREXPERIMENT is set, but is not supported by go1.7 and later"))
						Expect(buffer.String()).To(ContainSubstring("Run 'cf unset-env <app> GO15VENDOREXPERIMENT' before pushing again"))
					})
				})

			})
		})
		Context("the vendor tool is go_nativevendoring", func() {
			BeforeEach(func() {
				vendorTool = "go_nativevendoring"
			})

			Context("GO_INSTALL_PACKAGE_SPEC is set", func() {
				var oldGoInstallPackageSpec string

				BeforeEach(func() {
					oldGoInstallPackageSpec = os.Getenv("GO_INSTALL_PACKAGE_SPEC")
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", "a-package-name another-package")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", oldGoInstallPackageSpec)
					Expect(err).To(BeNil())
				})

				Context("packages are vendored", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor", "another-package"), 0755)
						Expect(err).To(BeNil())
					})
					It("handles the vendoring correctly", func() {
						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{"a-package-name", filepath.Join(mainPackageName, "vendor", "another-package")}))
					})

				})
				Context("packages are not vendored", func() {
					It("returns the packages", func() {
						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{"a-package-name", "another-package"}))
					})
				})
			})
			Context("GO_INSTALL_PACKAGE_SPEC is not set", func() {
				It("returns default", func() {
					packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(packages).To(Equal([]string{"."}))
				})

				It("logs a warning that it is using the default", func() {
					_, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(buffer.String()).To(ContainSubstring("**WARNING** Installing package '.' (default)"))
				})
			})
			Context("GO15VENDOREXPERIMENT = 0", func() {
				var oldGO15VENDOREXPERIMENT string

				BeforeEach(func() {
					oldGO15VENDOREXPERIMENT = os.Getenv("GO15VENDOREXPERIMENT")
					err = os.Setenv("GO15VENDOREXPERIMENT", "0")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GO15VENDOREXPERIMENT", oldGO15VENDOREXPERIMENT)
					Expect(err).To(BeNil())
				})

				Context("go version is 1.6.x", func() {
					It("logs a error and returns an error", func() {
						_, err = gc.InstallPackages(mainPackageName, packageDir, "1.6.3", vendorTool)
						Expect(err).NotTo(BeNil())

						Expect(buffer.String()).To(ContainSubstring("**ERROR** $GO15VENDOREXPERIMENT=0. To vendor your packages in vendor/"))
						Expect(buffer.String()).To(ContainSubstring("with go 1.6 this environment variable must unset or set to 1."))
					})

				})
				Context("go version is not 1.6.x", func() {
					It("doesn't log a error", func() {
						_, err = gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())
						Expect(buffer.String()).NotTo(ContainSubstring("**ERROR**"))
					})
				})
			})
		})
		Context("the vendor tool is glide", func() {
			BeforeEach(func() {
				vendorTool = "glide"
			})

			Context("GO_INSTALL_PACKAGE_SPEC is set", func() {
				var oldGoInstallPackageSpec string

				BeforeEach(func() {
					oldGoInstallPackageSpec = os.Getenv("GO_INSTALL_PACKAGE_SPEC")
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", "a-package-name another-package")
					Expect(err).To(BeNil())
				})

				AfterEach(func() {
					err = os.Setenv("GO_INSTALL_PACKAGE_SPEC", oldGoInstallPackageSpec)
					Expect(err).To(BeNil())
				})

				It("returns the packages", func() {
					gomock.InOrder(
						mockCommandRunner.EXPECT().SetDir(packageDir),
						mockCommandRunner.EXPECT().Run("glide", "install").Return(nil),
						mockCommandRunner.EXPECT().SetDir(""),
					)

					packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())

					Expect(packages).To(Equal([]string{"a-package-name", "another-package"}))
				})

				Context("packages are already vendored", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor", "another-package"), 0755)
						Expect(err).To(BeNil())
					})
					It("handles the vendoring correctly", func() {
						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{"a-package-name", filepath.Join(mainPackageName, "vendor", "another-package")}))
					})
				})
				Context("packages are not already vendored", func() {
					It("uses glide to install the packages", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("glide", "install").Do(func(_, _ string) {
								os.MkdirAll(filepath.Join(packageDir, "vendor", "another-package"), 0755)

							}),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
						Expect(err).To(BeNil())

						Expect(packages).To(Equal([]string{"a-package-name", filepath.Join(mainPackageName, "vendor", "another-package")}))
					})
				})

			})
			Context("GO_INSTALL_PACKAGE_SPEC is not set", func() {
				It("returns default", func() {
					gomock.InOrder(
						mockCommandRunner.EXPECT().SetDir(packageDir),
						mockCommandRunner.EXPECT().Run("glide", "install").Return(nil),
						mockCommandRunner.EXPECT().SetDir(""),
					)

					packages, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(packages).To(Equal([]string{"."}))
				})

				It("logs a warning that it is using the default", func() {
					gomock.InOrder(
						mockCommandRunner.EXPECT().SetDir(packageDir),
						mockCommandRunner.EXPECT().Run("glide", "install").Return(nil),
						mockCommandRunner.EXPECT().SetDir(""),
					)

					_, err := gc.InstallPackages(mainPackageName, packageDir, "1.8.3", vendorTool)
					Expect(err).To(BeNil())
					Expect(buffer.String()).To(ContainSubstring("**WARNING** Installing package '.' (default)"))
				})
			})
		})
	})
	Describe("CompileApp", func() {
		var (
			packages   []string
			buildFlags []string
			packageDir string
		)

		BeforeEach(func() {
			packages = []string{"first", "second"}
			buildFlags = []string{"-a=1", "-b=2"}
			packageDir, err = ioutil.TempDir("", "go-buildpack.package")
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err = os.RemoveAll(packageDir)
			Expect(err).To(BeNil())
		})

		Context("the tool is godep", func() {
			Context("godeps workspace dir exists", func() {
				BeforeEach(func() {
					err = os.MkdirAll(filepath.Join(packageDir, "Godeps", "_workspace", "src"), 0755)
					Expect(err).To(BeNil())
				})

				Context("vendor dir exists", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor"), 0755)
						Expect(err).To(BeNil())
					})
					It("logs a warning that both a vendor dir + godeps workspace exist", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("godep", "go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						err = gc.CompileApp(packages, buildFlags, packageDir, "godep")
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(ContainSubstring("**WARNING** Godeps/_workspace/src and vendor/ exist"))
						Expect(buffer.String()).To(ContainSubstring("code may not compile. Please convert all deps to vendor/"))
					})
				})

				Context("vendor dir does not exist", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor"), 0755)
						Expect(err).To(BeNil())
					})
					It("logs and runs the install command it is going to run", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("godep", "go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						err = gc.CompileApp(packages, buildFlags, packageDir, "godep")
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(ContainSubstring("-----> Running: godep go install -v -a=1 -b=2 first second"))
					})
				})

			})
			Context("godeps workspace dir does not exist", func() {
				Context("vendor dir exists", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(packageDir, "vendor"), 0755)
						Expect(err).To(BeNil())
					})
					It("logs and runs the install command it is going to run", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						err = gc.CompileApp(packages, buildFlags, packageDir, "godep")
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(ContainSubstring("-----> Running: go install -v -a=1 -b=2 first second"))
					})
				})
				Context("vendor dir does not exist", func() {
					It("logs and runs the install command it is going to run", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						err = gc.CompileApp(packages, buildFlags, packageDir, "godep")
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(ContainSubstring("-----> Running: go install -v -a=1 -b=2 first second"))
					})

					It("logs a warning that the vendor dir is missing", func() {
						gomock.InOrder(
							mockCommandRunner.EXPECT().SetDir(packageDir),
							mockCommandRunner.EXPECT().Run("go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
							mockCommandRunner.EXPECT().SetDir(""),
						)

						err = gc.CompileApp(packages, buildFlags, packageDir, "godep")
						Expect(err).To(BeNil())

						Expect(buffer.String()).To(ContainSubstring("**WARNING** vendor/ directory does not exist."))
					})
				})
			})
		})
		Context("the tool is glide", func() {
			It("logs and runs the install command it is going to run", func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().SetDir(packageDir),
					mockCommandRunner.EXPECT().Run("go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
					mockCommandRunner.EXPECT().SetDir(""),
				)

				err = gc.CompileApp(packages, buildFlags, packageDir, "glide")
				Expect(err).To(BeNil())

				Expect(buffer.String()).To(ContainSubstring("-----> Running: go install -v -a=1 -b=2 first second"))
			})
		})
		Context("the tool is go_nativevendoring", func() {
			It("logs and runs the install command it is going to run", func() {
				gomock.InOrder(
					mockCommandRunner.EXPECT().SetDir(packageDir),
					mockCommandRunner.EXPECT().Run("go", "install", "-v", "-a=1", "-b=2", "first", "second").Return(nil),
					mockCommandRunner.EXPECT().SetDir(""),
				)

				err = gc.CompileApp(packages, buildFlags, packageDir, "go_nativevendoring")
				Expect(err).To(BeNil())

				Expect(buffer.String()).To(ContainSubstring("-----> Running: go install -v -a=1 -b=2 first second"))
			})
		})

	})
})
