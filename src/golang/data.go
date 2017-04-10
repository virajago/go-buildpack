package golang

import (
	"fmt"
	"path"
)

func ReleaseYAML(mainPackageName string) string {
	release := `---
default_process_types:
    web: %s
`
	return fmt.Sprintf(release, path.Base(mainPackageName))
}

func GoScript() string {
	return "PATH=$PATH:$HOME/bin"
}

func GoRootScript() string {
	contents := `export GOROOT=$HOME/.cloudfoundry/go
PATH=$PATH:$GOROOT/bin`

	return contents
}

func ZZGoPathScript(mainPackageName string) string {
	contents := `export GOPATH=$HOME
cd $GOPATH/src/%s
`
	return fmt.Sprintf(contents, path.Base(mainPackageName))
}
