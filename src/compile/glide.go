package main

type glide struct{}

func NewGlide() VendorTool {
	return &glide{}
}

func (g *glide) Name() string {
	return "glide"
}

func (g *glide) Install() error {
	return nil
}

func (g *glide) SelectGoVersion() (string, error) {
	return "", nil
}

func (g *glide) MainPackageName() (string, error) {
	return "", nil
}

func (g *glide) PackagesToInstall() ([]string, error) {
	return []string{"hello"}, nil
}

func (g *glide) CompileApp() error {
	return nil
}
