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

	Describe("InstallGodep", func() {
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
	Describe("InstallGlide", func() {
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

		It("installs godep to the requested dir, adding it to the PATH", func() {
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
