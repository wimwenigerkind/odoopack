package index

type Provider interface {
	Lookup(name string, version string) (AddonVersion, error)
}

type Type string

const (
	TypeGit Type = "git"
)

type AddonVersion struct {
	Name       string
	Version    string
	Type       Type
	Repository string
}
