package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

func SelectGoVersion(s *libbuildpack.Stager, vendorTool string, godep Godep) (string, error) {
	goVersion := os.Getenv("GOVERSION")

	if vendorTool == "godep" {
		if goVersion != "" {
			s.Log.Warning(GoVersionOverride(goVersion))
		} else {
			goVersion = godep.GoVersion
		}
	} else {
		if goVersion == "" {
			defaultGo, err := s.Manifest.DefaultVersion("go")
			if err != nil {
				return "", err
			}
			goVersion = fmt.Sprintf("go%s", defaultGo.Version)
		}
	}

	return ParseGoVersion(s, goVersion)
}

func ParseGoVersion(s *libbuildpack.Stager, partialGoVersion string) (string, error) {
	existingVersions := s.Manifest.AllDependencyVersions("go")

	if len(strings.Split(partialGoVersion, ".")) < 3 {
		partialGoVersion += ".x"
	}

	strippedGoVersion := strings.TrimLeft(partialGoVersion, "go")

	expandedVer, err := libbuildpack.FindMatchingVersion(strippedGoVersion, existingVersions)
	if err != nil {
		return "", err
	}

	return expandedVer, nil
}
