package common_test

import (
	"golang/common"
	"os"

	"bytes"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=../vendor/github.com/cloudfoundry/libbuildpack/manifest.go --destination=mocks_manifest_test.go --package=common_test --imports=.=github.com/cloudfoundry/libbuildpack

var _ = Describe("Vendor", func() {
	var (
		logger       libbuildpack.Logger
		buffer       *bytes.Buffer
		err          error
		vendorTool   string
		goVersion    string
		godep        common.Godep
		mockCtrl     *gomock.Controller
		mockManifest *MockManifest
		stager       *libbuildpack.Stager
	)

	BeforeEach(func() {
		godep = common.Godep{}
		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger()
		logger.SetOutput(ansicleaner.New(buffer))

		mockCtrl = gomock.NewController(GinkgoT())
		mockManifest = NewMockManifest(mockCtrl)

		stager = &libbuildpack.Stager{
			Log:      logger,
			Manifest: mockManifest,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("SelectGoVersion", func() {

		BeforeEach(func() {
			versions := []string{"1.8.0", "1.7.5", "1.7.4", "1.6.3", "1.6.4", "34.34.0", "1.14.3"}
			mockManifest.EXPECT().AllDependencyVersions("go").Return(versions)
		})

		Context("godep", func() {
			BeforeEach(func() {
				vendorTool = "godep"
				godep = common.Godep{ImportPath: "go-online", GoVersion: "go1.6"}
			})

			Context("GOVERSION not set", func() {
				It("sets the go version from Godeps.json", func() {
					goVersion, err = common.SelectGoVersion(stager, vendorTool, godep)
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

				It("sets the go version from GOVERSION and logs a warning", func() {
					goVersion, err = common.SelectGoVersion(stager, vendorTool, godep)
					Expect(err).To(BeNil())

					Expect(goVersion).To(Equal("34.34.0"))
					Expect(buffer.String()).To(ContainSubstring("**WARNING** Using $GOVERSION override.\n"))
					Expect(buffer.String()).To(ContainSubstring("    $GOVERSION = go34.34\n"))
					Expect(buffer.String()).To(ContainSubstring("If this isn't what you want please run:\n"))
					Expect(buffer.String()).To(ContainSubstring("    cf unset-env <app> GOVERSION"))
				})
			})
		})

		Context("glide or go_nativevendoring", func() {
			Context("GOVERSION is notset", func() {
				BeforeEach(func() {
					vendorTool = "glide"
					dep := libbuildpack.Dependency{Name: "go", Version: "1.14.3"}
					mockManifest.EXPECT().DefaultVersion("go").Return(dep, nil)
				})

				It("sets the go version to the default from the manifest.yml", func() {
					goVersion, err = common.SelectGoVersion(stager, vendorTool, godep)
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
					vendorTool = "go_nativevendoring"
				})

				AfterEach(func() {
					err = os.Setenv("GOVERSION", oldGOVERSION)
					Expect(err).To(BeNil())
				})

				It("sets the go version from GOVERSION", func() {
					goVersion, err = common.SelectGoVersion(stager, vendorTool, godep)
					Expect(err).To(BeNil())

					Expect(goVersion).To(Equal("34.34.0"))
				})
			})
		})
	})
})
