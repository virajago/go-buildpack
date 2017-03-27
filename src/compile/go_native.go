package main

type goNative struct{}

func NewGoNative() VendorTool {
	return &goNative{}
}

func (g *goNative) Name() string {
	return "go_nativevendoring"
}

func (g *goNative) Install() error {
	return nil
}

func (g *goNative) SelectGoVersion() (string, error) {
	return "", nil
}

func (g *goNative) MainPackageName() (string, error) {
	return "", nil
}

func (g *goNative) PackagesToInstall() ([]string, error) {
	return []string{"hello"}, nil
}

func (g *goNative) CompileApp() error {
	return nil
}
