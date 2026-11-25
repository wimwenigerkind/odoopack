package manifest

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/version"
)

// IndexTypes are the index types this odoopack can search. An index of
// another type is refused instead of skipped, since skipping it would
// resolve a name from the next index that a newer odoopack resolves from it.
var IndexTypes = []string{"registry"}

var (
	// seriesPattern spells a series like a version does: no leading zeros.
	seriesPattern    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	indexNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	// addonsDirPattern is a relative path with / as separator, of characters
	// every file system takes, so the same manifest means the same directory
	// on Linux, macOS and Windows: no backslash, no drive colon.
	addonsDirPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
)

// Validate reports every problem of m at once, so that a manifest can be
// fixed in one go.
func (m Manifest) Validate() error {
	var errs []error
	if err := ValidSeries(m.OdooSeries); err != nil {
		errs = append(errs, err)
	}
	for pkg, constraint := range m.Require {
		if err := name.ValidPackage(pkg); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := ValidConstraint(constraint); err != nil {
			errs = append(errs, fmt.Errorf("require %s: %w", pkg, err))
		}
	}
	if err := ValidAddonsDir(m.AddonsDir); err != nil {
		errs = append(errs, err)
	}
	if len(m.Extra) > 0 && m.Extra.Kind() != '{' {
		errs = append(errs, errors.New("extra must be an object"))
	}
	seen := make(map[string]bool, len(m.Indexes))
	for _, idx := range m.Indexes {
		if err := idx.Validate(); err != nil {
			errs = append(errs, err)
		}
		if seen[idx.Name] {
			errs = append(errs, fmt.Errorf("index name %q is used twice", idx.Name))
		}
		seen[idx.Name] = true
	}
	return errors.Join(errs...)
}

// ValidSeries reports whether series names an Odoo series such as 19.0.
func ValidSeries(series string) error {
	if series == "" {
		return errors.New(`odoo_series is not set; add "odoo_series": "19.0"`)
	}
	if !seriesPattern.MatchString(series) {
		return fmt.Errorf("odoo_series %q must be a series such as 19.0", series)
	}
	return nil
}

// ValidAddonsDir reports whether dir is a portable relative path inside the
// project. Whether it stays inside the project through symlinks depends on
// the file system, which addonsdir.Check looks at before anything is written.
func ValidAddonsDir(dir string) error {
	if dir == "" {
		return errors.New(`addons_dir is not set; add "addons_dir": "addons/odoopack"`)
	}
	if !addonsDirPattern.MatchString(dir) {
		return fmt.Errorf("addons_dir %q must be a relative path of letters, digits, '.', '_' and '-', with '/' as separator", dir)
	}
	for _, part := range strings.Split(dir, "/") {
		if part == "." || part == ".." {
			return fmt.Errorf("addons_dir %q must not contain %q", dir, part)
		}
	}
	return nil
}

// ValidConstraint reports whether constraint is one of the forms a
// requirement takes: an exact release, a series or a dev branch.
func ValidConstraint(constraint string) error {
	if constraint != strings.TrimSpace(constraint) {
		return fmt.Errorf("constraint %q has surrounding spaces", constraint)
	}
	_, err := version.ParseConstraint(constraint)
	return err
}

func (idx Index) Validate() error {
	if !indexNamePattern.MatchString(idx.Name) {
		return fmt.Errorf("index name %q must be 1 to 64 lowercase letters, digits, _ and -, starting with a letter or digit", idx.Name)
	}
	if err := ValidIndexURL(idx.Url); err != nil {
		return fmt.Errorf("index %s: %w", idx.Name, err)
	}
	for _, known := range IndexTypes {
		if idx.Type == known {
			return nil
		}
	}
	return fmt.Errorf("index %s has type %q, which this odoopack does not know (known: %s); upgrade odoopack", idx.Name, idx.Type, strings.Join(IndexTypes, ", "))
}

// ValidIndexURL accepts an http or https URL of scheme, host and an optional
// path. Credentials belong in ODOOPACK_AUTH, never in the manifest that is
// committed, and the lockfile copies the URL.
func ValidIndexURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", raw, err)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return fmt.Errorf("url %q must start with https:// or http://", raw)
	case u.Host == "":
		return fmt.Errorf("url %q has no host", raw)
	case u.User != nil:
		return fmt.Errorf("url %q must not contain credentials; put the token in ODOOPACK_AUTH", redact(u))
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#"):
		return fmt.Errorf("url %q must not have a query or fragment", raw)
	}
	return nil
}

// redact keeps credentials out of the error, which may end up in a CI log.
func redact(u *url.URL) string {
	c := *u
	c.User = url.User("REDACTED")
	return c.String()
}
