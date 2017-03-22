package main

import "fmt"

func goVersionOverride(goVersion string) string {
	warning := `Using $GOVERSION override.
    $GOVERSION = %s

If this isn't what you want please run:
    cf unset-env <app> GOVERSION`

	return fmt.Sprintf(warning, goVersion)
}

func godirError() string {
	errorMessage := `Deprecated, .godir file found! Please update to supported Godep or Glide dependency managers.
See https://github.com/tools/godep or https://github.com/Masterminds/glide for usage information.`

	return errorMessage
}

func gbError() string {
	errorMessage := `Cloud Foundry does not support the GB package manager.
We currently only support the Godep and Glide package managers for go apps.
For support please file an issue: https://github.com/cloudfoundry/go-buildpack/issues`

	return errorMessage
}
