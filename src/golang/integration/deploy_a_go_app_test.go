package integration_test

import (
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack/cutlass"
	"github.com/cloudfoundry/libbuildpack/packager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CF Go Buildpack", func() {
	var app *cutlass.App
	var resource_url string

	AfterEach(func() {
		if app != nil {
			app.Destroy()
		}
		app = nil
	})

	Context("with cached buildpack dependencies", func() {
		Context("that are unvendored", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "with_dependencies", "src", "with_dependencies"))
			})

			It("", func() {
				bpFile, err := packager.Package(bpDir, packager.CacheDir, fmt.Sprintf("%s.%s", buildpackVersion, "notraffic1"), cutlass.Cached)

				PushAppAndConfirm(app)
				Expect(app.Stdout.String()).To(MatchRegexp("Hello from foo!"))
				Expect(app.GetBody("/")).To(ContainSubstring("hello, world"))

				traffic, err := cutlass.InternetTraffic(
					bpDir,
					"fixtures/with_yarn_vendored",
					bpFile,
					[]string{},
				)
				Expect(err).To(BeNil())
				Expect(traffic).To(HaveLen(0))
			})

			Context("app uses go1.6 and godep with GO15VENDOREXPERIMENT=0", func() {
				BeforeEach(func() {
					app = cutlass.New(filepath.Join(bpDir, "fixtures", "go16_dependencies", "src", "go16_dependencies"))
					app.SetEnv("GO15VENDOREXPERIMENT", "0")
				})

				It("", func() {
					PushAppAndConfirm(app)
					Expect(app.Stdout.String()).To(MatchRegexp("Hello from foo!"))
					Expect(app.GetBody("/")).To(ContainSubstring("hello, world"))
				})
			})

			Context("app uses go1.6 with godep and no vendor dir or Godeps/_workspace dir", func() {
				BeforeEach(func() {
					app = cutlass.New(filepath.Join(bpDir, "fixtures", "go16_dependencies", "src", "go16_no_vendor"))
				})

				It("", func() {
					PushAppAndConfirm(app)
					Expect(app.Stdout.String()).To(MatchRegexp("vendor/ directory does not exist."))
				})
			})
		})

		Context("that are vendored", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "go17_vendor_experiment_flag", "src", "go_app"))
			})

			It("", func() {
				Expect(app.Push()).ToNot(BeNil())
				Expect(app.Stdout.String()).To(MatchRegexp("GO15VENDOREXPERIMENT is set, but is not supported by go1.7"))
			})
		})

		Context("app has vendored dependencies and no Godeps folder", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "native_vendoring", "src", "go_app"))
			})

			It("successfully stages", func() {
				bpFile, err := packager.Package(bpDir, packager.CacheDir, fmt.Sprintf("%s.%s", buildpackVersion, "notraffic2"), cutlass.Cached)

				PushAppAndConfirm(app)
				Expect(app.Stdout.String()).To(MatchRegexp("Init: a.A == 1"))
				Expect(app.GetBody("/")).To(ContainSubstring("Read: a.A == 1"))

				traffic, err := cutlass.InternetTraffic(
					bpDir,
					"fixtures/with_yarn_vendored",
					bpFile,
					[]string{},
				)
				Expect(err).To(BeNil())
				Expect(traffic).To(HaveLen(0))
			})
		})

		Context("app has vendored dependencies and custom package spec", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "vendored_custom_install_spec", "src", "go_app"))
				app.SetEnv("BP_DEBUG", "1")
			})

			It("successfully stages", func() {
				bpFile, err := packager.Package(bpDir, packager.CacheDir, fmt.Sprintf("%s.%s", buildpackVersion, "notraffic2"), cutlass.Cached)

				PushAppAndConfirm(app)
				Expect(app.Stdout.String()).To(MatchRegexp("Init: a.A == 1"))
				Expect(app.GetBody("/")).To(ContainSubstring("Read: a.A == 1"))

				traffic, err := cutlass.InternetTraffic(
					bpDir,
					"fixtures/with_yarn_vendored",
					bpFile,
					[]string{},
				)
				Expect(err).To(BeNil())
				Expect(traffic).To(HaveLen(0))
			})
		})

		Context("app has vendored dependencies and a vendor.json file", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "with_vendor_json", "src", "go_app"))
			})

			It("successfully stages", func() {
				PushAppAndConfirm(app)
				Expect(app.Stdout.String()).To(MatchRegexp("Init: a.A == 1"))
				Expect(app.GetBody("/")).To(ContainSubstring("Read: a.A == 1"))
			})
		})

		Context("app with only a single go file and GOPACKAGENAME specified", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "single_file", "src", "go_app"))
			})

			It("successfully stages", func() {
				PushAppAndConfirm(app)
				Expect(app.GetBody("/")).To(ContainSubstring("simple apps are good"))
			})
		})

		Context("app with only a single go file, a vendor directory, and no GOPACKAGENAME specified", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "vendored_no_gopackagename", "src", "go_app"))
			})

			It("fails with helpful error", func() {
				Expect(app.Push()).ToNot(BeNil())
				Expect(app.Stdout.String()).To(MatchRegexp("failed"))
				Expect(app.Stdout.String()).To(MatchRegexp("To use go native vendoring set the $GOPACKAGENAME"))
			})
		})

		Context("app has vendored dependencies with go1.6, but GO15VENDOREXPERIMENT=0", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "go16_vendor_bad_env", "src", "go_app"))
			})

			It("fails with helpful error", func() {
				Expect(app.Push()).ToNot(BeNil())
				Expect(app.Stdout.String()).To(MatchRegexp("failed"))
				Expect(app.Stdout.String()).To(MatchRegexp("with go 1.6 this environment variable must unset or set to 1."))
			})
		})

		Context("app has no dependencies", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "go_app", "src", "go_app"))
			})

			It("", func() {
				PushAppAndConfirm(app)
				Expect(app.GetBody("/")).To(ContainSubstring("go, world"))
				Expect(app.Stdout.String()).To(MatchRegexp(`"Installing go [\d\.]+"`))
				Expect(app.Stdout.String()).To(MatchRegexp(`"Copy \[\/tmp\/"`))
			})
		})

		Context("app has before/after compile hooks", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "go_app", "src", "go_app"))
				app.SetEnv("BP_DEBUG", "1")
			})

			It("", func() {
				PushAppAndConfirm(app)
				Expect(app.GetBody("/")).To(ContainSubstring("go, world"))
				Expect(app.Stdout.String()).To(MatchRegexp("HOOKS 1: BeforeCompile"))
				Expect(app.Stdout.String()).To(MatchRegexp("HOOKS 2: AfterCompile"))
			})
		})

		Context("app has no Procfile", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "no_procfile", "src", "no_procfile"))
			})

			It("", func() {
				bpFile, err := packager.Package(bpDir, packager.CacheDir, fmt.Sprintf("%s.%s", buildpackVersion, "notraffic1"), cutlass.Cached)

				PushAppAndConfirm(app)
				Expect(app.GetBody("/")).To(ContainSubstring("go, world"))
				Expect(app.Stdout.String()).To(MatchRegexp(`"Installing go [\d\.]+"`))
				Expect(app.Stdout.String()).To(MatchRegexp(`"Copy \[\/tmp\/"`))

				traffic, err := cutlass.InternetTraffic(
					bpDir,
					"fixtures/with_yarn_vendored",
					bpFile,
					[]string{},
				)
				Expect(err).To(BeNil())
				Expect(traffic).To(HaveLen(0))
			})
		})

		Context("expects a non-packaged version of go", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join(bpDir, "fixtures", "go99", "src", "go99"))
				resource_url = "https://storage.googleapis.com/golang/go99.99.99.linux-amd64.tar.gz"
			})

			It("displays useful understandable errors", func() {
				Expect(app.Push()).ToNot(BeNil())

				Expect(app.Stdout.String()).To(MatchRegexp("failed"))
				Expect(app.Stdout.String()).To(MatchRegexp("Unable to determine Go version to install: no match found for 99.99.99"))

				Expect(app.Stdout.String()).ToNot(MatchRegexp("Installing go99.99.99"))
				Expect(app.Stdout.String()).ToNot(MatchRegexp("Uploading droplet"))
			})
		})
	})
})
