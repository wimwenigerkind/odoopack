package manifest

type Requirements map[string]string

type Manifest struct {
	Name    string       `json:"name"`
	Require Requirements `json:"require"`
}

func NewManifest(name string) *Manifest {
	return &Manifest{
		Name:    name,
		Require: make(Requirements),
	}
}
