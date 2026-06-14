package manifest

type Requirements map[string]string

type Index struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type Indexes map[string]Index

type Manifest struct {
	Name       string       `json:"name"`
	Require    Requirements `json:"require"`
	Indexes    Indexes      `json:"indexes"`
	AddonsPath string       `json:"addons_path"`
}

func NewManifest(name string, addonsPath string) *Manifest {
	return &Manifest{
		Name:       name,
		Require:    make(Requirements),
		Indexes:    Indexes{},
		AddonsPath: addonsPath,
	}
}
