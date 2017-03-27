package main

type godep struct {
	ImportPath string   `json:"ImportPath"`
	GoVersion  string   `json:"GoVersion"`
	Packages   []string `json:"Packages"`
}

func NewGodep() VendorTool {
	return &godep{}
}

func (g *godep) Name() string {
	return "godep"
}

func (g *godep) Install() error {
	return nil
}

func (g *godep) SelectGoVersion() (string, error) {
	return "", nil
}

func (g *godep) MainPackageName() (string, error) {
	return "", nil
}

func (g *godep) PackagesToInstall() ([]string, error) {
	return []string{"hello"}, nil
}

func (g *godep) CompileApp() error {
	return nil
}
