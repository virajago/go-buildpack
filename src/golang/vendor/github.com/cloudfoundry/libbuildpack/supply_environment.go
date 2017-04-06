package libbuildpack

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var stagingEnvVarDirs = map[string]string{
	"PATH":            "bin",
	"LD_LIBRARY_PATH": "lib",
	"INCLUDE_PATH":    "include",
	"CPATH":           "include",
	"CPPPATH":         "include",
	"PKG_CONFIG_PATH": "pkgconfig",
}

var launchEnvVarDirs = map[string]string{
	"PATH":            "bin",
	"LD_LIBRARY_PATH": "lib",
}

func SetEnvironmentFromSupply(depsDir string) error {
	for envVar, dir := range stagingEnvVarDirs {
		oldVal := os.Getenv(envVar)

		depsPaths, err := existingDepsDirs(depsDir, dir, depsDir)
		if err != nil {
			return err
		}

		if len(depsPaths) != 0 {
			os.Setenv(envVar, fmt.Sprintf("%s:%s", strings.Join(depsPaths, ":"), oldVal))
		}
	}

	depsPaths, err := existingDepsDirs(depsDir, "env", depsDir)
	if err != nil {
		return err
	}

	for _, dir := range depsPaths {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.Mode().IsRegular() {
				val, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
				if err != nil {
					return err
				}

				if err := os.Setenv(file.Name(), string(val)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func existingDepsDirs(depsDir, subDir, prefix string) ([]string, error) {
	dirs, err := ioutil.ReadDir(depsDir)
	if err != nil {
		return nil, err
	}

	var existingDirs []string

	for _, dir := range dirs {
		filesystemDir := filepath.Join(depsDir, dir.Name(), subDir)
		dirToJoin := filepath.Join(prefix, dir.Name(), subDir)

		addToDirs, err := FileExists(filesystemDir)
		if err != nil {
			return nil, err
		}

		if addToDirs {
			existingDirs = append([]string{dirToJoin}, existingDirs...)
		}
	}

	return existingDirs, nil
}

func WriteProfileDFromSupply(depsDir, buildDir string) error {
	scriptContents := ""

	for envVar, dir := range launchEnvVarDirs {
		depsPaths, err := existingDepsDirs(depsDir, dir, "$DEPS_DIR")
		if err != nil {
			return err
		}

		if len(depsPaths) != 0 {
			scriptContents += fmt.Sprintf("export %s=%s:$%s\n", envVar, strings.Join(depsPaths, ":"), envVar)
		}
	}

	return WriteProfileD(buildDir, "00-multi-supply.sh", scriptContents)
}
