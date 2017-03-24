package main

import (
	"fmt"
	"path"
)

func releaseYAML(mainPackageName string) string {
	release := `---
default_process_types:
    web: %s
`
	return fmt.Sprintf(release, path.Base(mainPackageName))
}
