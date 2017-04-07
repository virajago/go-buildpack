package supply_test

import (
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
			godepInstallDir := filepath.Join(depsDir, depsIdx, "godep")
			glideInstallDir := filepath.Join(depsDir, depsIdx, "glide")

			mockManifest.EXPECT().InstallOnlyVersion("godep", godepInstallDir).Return(nil)
			mockManifest.EXPECT().InstallOnlyVersion("glide", glideInstallDir).Return(nil)

			err = gs.InstallVendorTools()
			Expect(err).To(BeNil())

			link, err := os.Readlink(filepath.Join(depsDir, depsIdx, "bin", "godep"))
			Expect(err).To(BeNil())

			Expect(link).To(Equal("../godep/bin/godep"))

			link, err = os.Readlink(filepath.Join(depsDir, depsIdx, "bin", "glide"))
			Expect(err).To(BeNil())

			Expect(link).To(Equal("../glide/bin/glide"))
		})
	})

	Describe("InstallGo", func() {
		var (
			goInstallDir string
			dep          libbuildpack.Dependency
		)

		BeforeEach(func() {
			goVersion = "1.3.4"
			goInstallDir = filepath.Join(depsDir, depsIdx, "go1.3.4")
			dep = libbuildpack.Dependency{Name: "go", Version: "1.3.4"}
		})

		//Context("go is already cached", func() {
		//	BeforeEach(func() {
		//		err = os.MkdirAll(filepath.Join(goInstallDir, "go"), 0755)
		//		Expect(err).To(BeNil())
		//	})
		//
		//	It("uses the cached version", func() {
		//		err = gc.InstallGo()
		//		Expect(err).To(BeNil())
		//
		//		Expect(buffer.String()).To(ContainSubstring("-----> Using go 1.3.4"))
		//	})
		//
		//	It("Creates a bin directory", func() {
		//		err = gc.InstallGo()
		//		Expect(err).To(BeNil())
		//
		//		Expect(filepath.Join(buildDir, "bin")).To(BeADirectory())
		//	})
		//
		//	It("Sets up GOROOT", func() {
		//		err = gc.InstallGo()
		//		Expect(err).To(BeNil())
		//
		//		Expect(os.Getenv("GOROOT")).To(Equal(filepath.Join(goInstallDir, "go")))
		//	})
		//
		//	It("adds go to the PATH", func() {
		//		err = gc.InstallGo()
		//		Expect(err).To(BeNil())
		//
		//		newPath := fmt.Sprintf("%s:%s", filepath.Join(goInstallDir, "go", "bin"), oldPath)
		//		Expect(os.Getenv("PATH")).To(Equal(newPath))
		//	})
		//})

		// cached buildpack
		Context("go is not already in cache", func() {
			BeforeEach(func() {
				err = os.MkdirAll(filepath.Join(goInstallDir, "go"), 0755)
				Expect(err).To(BeNil())
				mockManifest.EXPECT().InstallDependency(dep, goInstallDir).Return(nil)
			})

			It("Write GOROOT to envfile", func() {
				err = gs.InstallGo()
				Expect(err).To(BeNil())

				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "env", "GOROOT"))
				Expect(err).To(BeNil())
				Expect(string(contents)).To(Equal(filepath.Join(goInstallDir, "go")))
			})

			It("installs go to the depDir, creating a symlink in <depDir>/bin", func() {
				err = gs.InstallGo()
				Expect(err).To(BeNil())

				link, err := os.Readlink(filepath.Join(depsDir, depsIdx, "bin", "go"))
				Expect(err).To(BeNil())

				Expect(link).To(Equal("../go1.3.4/go/bin/go"))

			})

			//It("clears the cache", func() {
			//	err = gs.InstallGo()
			//	Expect(err).To(BeNil())
			//
			//	Expect(filepath.Join(cacheDir, "go4.3.2", "go")).NotTo(BeADirectory())
			//})
		})
	})
})
