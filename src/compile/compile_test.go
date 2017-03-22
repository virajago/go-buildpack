package main_test

import (
	c "compile"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"bytes"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=vendor/github.com/cloudfoundry/libbuildpack/manifest.go --destination=mocks_manifest_test.go --package=main_test --imports=.=github.com/cloudfoundry/libbuildpack

var _ = Describe("Compile", func() {
	var (
		buildDir     string
		cacheDir     string
		depsDir      string
		gc           *c.GoCompiler
		logger       libbuildpack.Logger
		buffer       *bytes.Buffer
		err          error
		mockCtrl     *gomock.Controller
		mockManifest *MockManifest
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
	})

	JustBeforeEach(func() {
		bpc := &libbuildpack.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			DepsDir:  depsDir,
			Manifest: mockManifest,
			Log:      logger}

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
					tool, _, _, err := gc.SelectVendorTool()
					Expect(err).To(BeNil())

					Expect(tool).To(Equal("godep"))
					Expect(buffer.String()).To(ContainSubstring("-----> Checking Godeps/Godeps.json file"))
				})

				Context("GOVERSION is not set", func() {
					It("returns the go version from Godeps.json", func() {
						_, goVersion, _, err := gc.SelectVendorTool()
						Expect(err).To(BeNil())

						Expect(goVersion).To(Equal("go1.6"))
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
						_, goVersion, _, err := gc.SelectVendorTool()
						Expect(err).To(BeNil())

						Expect(goVersion).To(Equal("go34.34"))
						Expect(buffer.String()).To(ContainSubstring("**WARNING** Using $GOVERSION override.\n"))
						Expect(buffer.String()).To(ContainSubstring("    $GOVERSION = go34.34\n"))
						Expect(buffer.String()).To(ContainSubstring("If this isn't what you want please run:\n"))
						Expect(buffer.String()).To(ContainSubstring("    cf unset-env <app> GOVERSION"))
					})
				})

				It("returns the package name from Godeps.json", func() {
					_, _, goPackageName, err := gc.SelectVendorTool()
					Expect(err).To(BeNil())

					Expect(goPackageName).To(Equal("go-online"))
				})
			})

			Context("invalid json", func() {
				BeforeEach(func() {
					godepsJsonContents = "not actually JSON"
				})

				It("logs that the Godeps.json file is invalid and returns an error", func() {
					_, _, _, err := gc.SelectVendorTool()
					Expect(err).NotTo(BeNil())

					Expect(buffer.String()).To(ContainSubstring("**ERROR** Bad Godeps/Godeps.json file"))
				})
			})
		})

		Context("there is a .godir file", func() {
			BeforeEach(func() {
				err = ioutil.WriteFile(filepath.Join(buildDir, ".godir"), []byte("xxx"), 0644)
			})

			It("logs that .godir is deprecated and returns an error", func() {
				_, _, _, err := gc.SelectVendorTool()
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** Deprecated, .godir file found! Please update to supported Godep or Glide dependency managers."))
				Expect(buffer.String()).To(ContainSubstring("See https://github.com/tools/godep or https://github.com/Masterminds/glide for usage information."))
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
})
