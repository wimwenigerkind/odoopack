// Package buildinfo holds the version main receives through ldflags, so that
// packages below cmd can name the release they belong to.
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// UserAgent identifies odoopack to registries on every request.
func UserAgent() string {
	return "odoopack/" + Version
}
