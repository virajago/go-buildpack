package main

import "fmt"

func goVersionOverride(goVersion string) string {
	warning := `Using $GOVERSION override.
    $GOVERSION = %s

If this isn't what you want please run:
    cf unset-env <app> GOVERSION
`

	return fmt.Sprintf(warning, goVersion)
}
