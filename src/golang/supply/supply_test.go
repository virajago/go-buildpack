package supply_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"bytes"

	"golang/supply"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=../vendor/github.com/cloudfoundry/libbuildpack/manifest.go --destination=mocks_manifest_test.go --package=supply_test --imports=.=github.com/cloudfoundry/libbuildpack

var _ = Describe("Supply", func() {
	var (
		depsDir      string
		depsIdx      string
		gs           *supply.Supplier
		logger       libbuildpack.Logger
		buffer       *bytes.Buffer
		err          error
		mockCtrl     *gomock.Controller
		mockManifest *MockManifest
		goVersion    string
	)

	BeforeEach(func() {
		depsDir, err = ioutil.TempDir("", "go-buildpack.deps.")
		Expect(err).To(BeNil())

		depsIdx = "04"

		err = os.MkdirAll(filepath.Join(depsDir, depsIdx), 0755)
		Expect(err).To(BeNil())

		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger()
		logger.SetOutput(ansicleaner.New(buffer))

		mockCtrl = gomock.NewController(GinkgoT())
		mockManifest = NewMockManifest(mockCtrl)
		mockCommandRunner = NewMockCommandRunner(mockCtrl)
	})

	JustBeforeEach(func() {
		bps := &libbuildpack.Stager{
			DepsDir:  depsDir,
			DepsIdx:  depsIdx,
			Manifest: mockManifest,
			Log:      logger,
		}

		gs = &supply.Supplier{
			Stager:    bps,
			GoVersion: goVersion,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
	})

	Describe("InstallVendorTools", func() {
		It("installs godep + glide to the depDir, creating a symlink in <depDir>/bin", func() {
			godepInstallDir := filepath.Join(tempDir, "godep")
			glideInstallDir := filepath.Join(tempDir, "glide")

			mockManifest.EXPECT().InstallOnlyVersion("godep", godepInstallDir).Return(nil)
			mockManifest.EXPECT().InstallOnlyVersion("glide", glideInstallDir).Return(nil)

			err = gc.InstallVendorTool(tempDir)
			Expect(err).To(BeNil())

			newPath := os.Getenv("PATH")
			Expect(newPath).To(Equal(fmt.Sprintf("%s:%s", filepath.Join(installDir, "bin"), oldPath)))
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
			goVersion = "1.3.4"
			oldPath = os.Getenv("PATH")
			oldPath = os.Getenv("GOROOT")
			goInstallDir = filepath.Join(cacheDir, "go1.3.4")
			dep = libbuildpack.Dependency{Name: "go", Version: "1.3.4"}
		})

		AfterEach(func() {
			err = os.Setenv("PATH", oldPath)
			Expect(err).To(BeNil())

			err = os.Setenv("GOROOT", oldGoRoot)
			Expect(err).To(BeNil())
		})

		Context("go is already cached", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(goInstallDir, "go"), 0755)
				Expect(err).To(BeNil())
			})

			It("uses the cached version", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(buffer.String()).To(ContainSubstring("-----> Using go 1.3.4"))
			})

			It("Creates a bin directory", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(filepath.Join(buildDir, "bin")).To(BeADirectory())
			})

			It("Sets up GOROOT", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(os.Getenv("GOROOT")).To(Equal(filepath.Join(goInstallDir, "go")))
			})

			It("adds go to the PATH", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				newPath := fmt.Sprintf("%s:%s", filepath.Join(goInstallDir, "go", "bin"), oldPath)
				Expect(os.Getenv("PATH")).To(Equal(newPath))
			})
		})

		Context("go is not already cached", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(cacheDir, "go4.3.2", "go"), 0755)
				Expect(err).To(BeNil())
				mockManifest.EXPECT().InstallDependency(dep, goInstallDir).Return(nil)
			})

			It("Creates a bin directory", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(filepath.Join(buildDir, "bin")).To(BeADirectory())
			})

			It("Sets up GOROOT", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(os.Getenv("GOROOT")).To(Equal(filepath.Join(goInstallDir, "go")))
			})

			It("adds go to the PATH", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				newPath := fmt.Sprintf("%s:%s", filepath.Join(goInstallDir, "go", "bin"), oldPath)
				Expect(os.Getenv("PATH")).To(Equal(newPath))
			})

			It("installs go", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())
			})

			It("clears the cache", func() {
				err = gc.InstallGo()
				Expect(err).To(BeNil())

				Expect(filepath.Join(cacheDir, "go4.3.2", "go")).NotTo(BeADirectory())
			})
		})
	})
})
