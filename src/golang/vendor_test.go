package golang_test

import (
	"golang"
	"io/ioutil"
	"os"
	"path/filepath"

	"bytes"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vendor", func() {
	var (
		buildDir   string
		logger     libbuildpack.Logger
		buffer     *bytes.Buffer
		err        error
		vendorTool string
		godep      golang.Godep
		stager     *libbuildpack.Stager
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "go-buildpack.build")
		Expect(err).To(BeNil())

		godep = golang.Godep{}

		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger()
		logger.SetOutput(ansicleaner.New(buffer))

		stager = &libbuildpack.Stager{
			BuildDir: buildDir,
			Log:      logger,
		}
	})

	AfterEach(func() {
		err = os.RemoveAll(buildDir)
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

			Context("the json is valid", func() {
				BeforeEach(func() {
					godepsJsonContents = `
{
	"ImportPath": "go-online",
	"GoVersion": "go1.6",
	"Deps": []
}
`
				})
				It("sets the tool to godep", func() {
					vendorTool, err = golang.SelectVendorTool(stager, &godep)
					Expect(err).To(BeNil())

					Expect(vendorTool).To(Equal("godep"))
				})
				It("logs that it is checking the Godeps.json file", func() {
					_, err = golang.SelectVendorTool(stager, &godep)
					Expect(err).To(BeNil())

					Expect(buffer.String()).To(ContainSubstring("-----> Checking Godeps/Godeps.json file"))
				})
				It("stores the Godep info in the GoCompiler struct", func() {
					_, err = golang.SelectVendorTool(stager, &godep)
					Expect(err).To(BeNil())

					Expect(godep.ImportPath).To(Equal("go-online"))
					Expect(godep.GoVersion).To(Equal("go1.6"))

					var empty []string
					Expect(godep.Packages).To(Equal(empty))
				})

				Context("godeps workspace exists", func() {
					BeforeEach(func() {
						err = os.MkdirAll(filepath.Join(buildDir, "Godeps", "_workspace", "src"), 0755)
						Expect(err).To(BeNil())
					})

					It("sets Godep.WorkspaceExists to true", func() {
						_, err = golang.SelectVendorTool(stager, &godep)
						Expect(err).To(BeNil())

						Expect(godep.WorkspaceExists).To(BeTrue())
					})
				})

				Context("godeps workspace does not exist", func() {
					It("sets Godep.WorkspaceExists to false", func() {
						_, err = golang.SelectVendorTool(stager, &godep)
						Expect(err).To(BeNil())

						Expect(godep.WorkspaceExists).To(BeFalse())
					})
				})
			})

			Context("bad Godeps.json file", func() {
				BeforeEach(func() {
					godepsJsonContents = "not actually JSON"
				})

				It("logs that the Godeps.json file is invalid and returns an error", func() {
					_, err = golang.SelectVendorTool(stager, &godep)
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
				_, err = golang.SelectVendorTool(stager, &godep)
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** Deprecated, .godir file found! Please update to supported Godep or Glide dependency managers."))
				Expect(buffer.String()).To(ContainSubstring("See https://github.com/tools/godep or https://github.com/Masterminds/glide for usage information."))
			})
		})

		Context("there is a glide.yaml file", func() {
			BeforeEach(func() {
				err = ioutil.WriteFile(filepath.Join(buildDir, "glide.yaml"), []byte("xxx"), 0644)
				Expect(err).To(BeNil())
			})

			It("sets the tool to glide", func() {
				vendorTool, err = golang.SelectVendorTool(stager, &godep)
				Expect(err).To(BeNil())

				Expect(vendorTool).To(Equal("glide"))
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
				_, err = golang.SelectVendorTool(stager, &godep)
				Expect(err).NotTo(BeNil())

				Expect(buffer.String()).To(ContainSubstring("**ERROR** Cloud Foundry does not support the GB package manager."))
				Expect(buffer.String()).To(ContainSubstring("We currently only support the Godep and Glide package managers for go apps"))
				Expect(buffer.String()).To(ContainSubstring("For support please file an issue: https://github.com/cloudfoundry/go-buildpack/issues"))

			})
		})

		Context("none of the above", func() {
			It("sets the tool to go_nativevendoring", func() {
				vendorTool, err = golang.SelectVendorTool(stager, &godep)
				Expect(err).To(BeNil())

				Expect(vendorTool).To(Equal("go_nativevendoring"))
			})
		})
	})
})
