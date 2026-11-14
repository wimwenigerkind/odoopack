package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

type authConfig struct {
	Bearer map[string]string `json:"bearer"`
}

// TokenForURL returns the bearer token ODOOPACK_AUTH holds for the host of
// rawURL, or "" when it holds none. A value that does not parse is an error
// rather than no token, so that a typo does not surface as a confusing 401 or
// 404 from the registry.
func TokenForURL(rawURL string) (string, error) {
	raw := viper.GetString("auth")
	if raw == "" {
		return "", nil
	}
	var cfg authConfig
	// Unknown fields are refused so that a misspelled "bearer" is noticed.
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return "", invalid(err)
	}
	if dec.More() {
		return "", invalid(errors.New("unexpected data after the JSON object"))
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", nil
	}
	candidates := []string{
		u.Host,
		u.Scheme + "://" + u.Host,
	}
	for _, k := range candidates {
		if v, ok := cfg.Bearer[k]; ok && v != "" {
			return v, nil
		}
	}
	return "", nil
}

func invalid(err error) error {
	return fmt.Errorf(`ODOOPACK_AUTH is not valid, expected {"bearer":{"<host>":"<token>"}}: %w`, err)
}
