package manifest

type Requirements map[string]string

type Index struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type Indexes map[string]Index

type Manifest struct {
	Name       string       `json:"name"`
	Odoo       string       `json:"odoo"`
	Require    Requirements `json:"require"`
	Indexes    Indexes      `json:"indexes"`
	AddonsPath string       `json:"addons_path"`
}

func NewManifest(name, addonsPath, odoo string) *Manifest {
	return &Manifest{
		Name:       name,
		Odoo:       odoo,
		Require:    make(Requirements),
		Indexes:    Indexes{},
		AddonsPath: addonsPath,
	}
}
