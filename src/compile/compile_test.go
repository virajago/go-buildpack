package main_test

import (
	c "compile"
	"io/ioutil"
	"os"

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

	It("Works", func() {
		err = gc.Compile()
		Expect(err).To(BeNil())

		Expect(buffer.String()).To(Equal("       It works!\n"))
	})
})
